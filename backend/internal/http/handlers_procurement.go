package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"itam/internal/domain"
	"itam/internal/events"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// validPOStatuses are the statuses a purchase order may hold.
var validPOStatuses = map[string]bool{
	"draft": true, "approved": true, "ordered": true,
	"partially_received": true, "received": true, "closed": true, "cancelled": true,
}

// ---- vendors -------------------------------------------------------------

// vendorStatusBlocks reports whether a vendor status disables new purchasing.
func (s *Server) vendorStatusBlocks(ctx context.Context, status string) bool {
	var blocks bool
	_ = s.db.NewSelect().Model((*domain.VendorStatus)(nil)).
		Column("blocks_orders").Where("key = ?", status).Scan(ctx, &blocks)
	return blocks
}

func (s *Server) handleListVendors(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("summary") == "true" {
		type vendorSummary struct {
			bun.BaseModel `bun:"table:proc.vendors,alias:v"`
			domain.Vendor
			POCount    int     `bun:"po_count" json:"po_count"`
			TotalSpent float64 `bun:"total_spent" json:"total_spent"`
			AssetCount int     `bun:"asset_count" json:"asset_count"`
		}
		var rows []vendorSummary
		q := s.db.NewSelect().Model(&rows).
			ColumnExpr("v.*").
			ColumnExpr("(SELECT count(*) FROM proc.purchase_orders po WHERE po.vendor_id = v.id) AS po_count").
			ColumnExpr(`(SELECT COALESCE(SUM(pl.quantity * pl.unit_cost), 0)
				FROM proc.purchase_orders po
				JOIN proc.po_lines pl ON pl.po_id = po.id
				WHERE po.vendor_id = v.id) AS total_spent`).
			ColumnExpr("(SELECT count(*) FROM core.assets a WHERE a.vendor = v.name AND a.deleted_at IS NULL) AS asset_count").
			Order("v.name ASC")
		if st := r.URL.Query().Get("status"); st != "" {
			q = q.Where("v.status = ?", st)
		}
		if err := q.Scan(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rows)
		return
	}
	var vendors []domain.Vendor
	q := s.db.NewSelect().Model(&vendors).Order("name ASC")
	if st := r.URL.Query().Get("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vendors)
}

func (s *Server) handleCreateVendor(w http.ResponseWriter, r *http.Request) {
	var v domain.Vendor
	if err := decode(r, &v); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if v.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if v.Key == "" {
		v.Key = slugify(v.Name)
	}
	if v.Status == "" {
		v.Status = "active"
	}
	v.IsActive = !s.vendorStatusBlocks(r.Context(), v.Status)
	if _, err := s.db.NewInsert().Model(&v).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handleUpdateVendor(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Vendor
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Status == "" {
		in.Status = "active"
	}
	in.IsActive = !s.vendorStatusBlocks(r.Context(), in.Status)
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "contact_email", "contact_phone", "website", "notes", "is_active", "status").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteVendor(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var poCount int
	_ = s.db.NewSelect().Model((*domain.PurchaseOrder)(nil)).
		ColumnExpr("count(*)").Where("vendor_id = ?", id).Scan(r.Context(), &poCount)
	if poCount > 0 {
		writeErr(w, http.StatusConflict,
			"vendor has purchase orders and cannot be deleted; blacklist or deactivate it instead")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.Vendor)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---- purchase orders -----------------------------------------------------

func (s *Server) handleListPOs(w http.ResponseWriter, r *http.Request) {
	var pos []domain.PurchaseOrder
	q := s.db.NewSelect().Model(&pos).Relation("Vendor").Order("po.created_at DESC")
	if st := r.URL.Query().Get("status"); st != "" {
		q = q.Where("po.status = ?", st)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pos)
}

func (s *Server) handleGetPO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	po := new(domain.PurchaseOrder)
	err = s.db.NewSelect().Model(po).
		Relation("Vendor").Relation("Location").
		Relation("Lines", func(q *bunSelect) *bunSelect { return q.Order("sort ASC") }).
		Relation("Lines.AssetType").
		Where("po.id = ?", id).Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusNotFound, "purchase order not found")
		return
	}
	writeJSON(w, http.StatusOK, po)
}

type poLineInput struct {
	AssetTypeID    *int64         `json:"asset_type_id"`
	ModelID        *int64         `json:"model_id"`
	Description    string         `json:"description"`
	Quantity       int            `json:"quantity"`
	UnitCost       float64        `json:"unit_cost"`
	WarrantyMonths int            `json:"warranty_months"`
	Attributes     map[string]any `json:"attributes"`
}

type poInput struct {
	PONumber    string        `json:"po_number"`
	VendorID    *uuid.UUID    `json:"vendor_id"`
	Status      string        `json:"status"`
	RequesterID *uuid.UUID    `json:"requester_id"`
	ApproverID  *uuid.UUID    `json:"approver_id"`
	LocationID  *uuid.UUID    `json:"location_id"`
	Currency    string        `json:"currency"`
	OrderedAt   string        `json:"ordered_at"`
	ExpectedAt  string        `json:"expected_at"`
	Notes       string        `json:"notes"`
	Lines       []poLineInput `json:"lines"`
}

func (s *Server) handleCreatePO(w http.ResponseWriter, r *http.Request) {
	var in poInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.PONumber == "" {
		writeErr(w, http.StatusBadRequest, "po_number is required")
		return
	}
	if in.Currency == "" {
		in.Currency = s.defaultCurrency(r.Context())
	}
	p := s.principal(r)
	po := &domain.PurchaseOrder{
		PONumber: in.PONumber, VendorID: in.VendorID, Status: "draft",
		RequesterID: in.RequesterID, ApproverID: in.ApproverID, LocationID: in.LocationID,
		Currency: in.Currency, OrderedAt: parseDate(in.OrderedAt), ExpectedAt: parseDate(in.ExpectedAt),
		Notes: in.Notes, CreatedBy: &p.UserID,
	}
	if in.Status != "" && validPOStatuses[in.Status] {
		po.Status = in.Status
	}
	if _, err := s.db.NewInsert().Model(po).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.insertPOLines(r.Context(), po.ID, in.Lines); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.handleGetPOByID(w, r, po.ID)
}

func (s *Server) handleUpdatePO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing := new(domain.PurchaseOrder)
	if err := s.db.NewSelect().Model(existing).Where("po.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "purchase order not found")
		return
	}
	var in poInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	status := existing.Status
	if in.Status != "" {
		if !validPOStatuses[in.Status] {
			writeErr(w, http.StatusBadRequest, "invalid status")
			return
		}
		status = in.Status
	}
	upd := &domain.PurchaseOrder{
		ID: id, VendorID: in.VendorID, Status: status, RequesterID: in.RequesterID,
		ApproverID: in.ApproverID, LocationID: in.LocationID, Currency: in.Currency,
		OrderedAt: parseDate(in.OrderedAt), ExpectedAt: parseDate(in.ExpectedAt), Notes: in.Notes,
	}
	if upd.Currency == "" {
		upd.Currency = existing.Currency
	}
	if _, err := s.db.NewUpdate().Model(upd).
		Column("vendor_id", "status", "requester_id", "approver_id", "location_id",
			"currency", "ordered_at", "expected_at", "notes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Line edits are only allowed while the PO is still a draft.
	if in.Lines != nil && existing.Status == "draft" {
		if _, err := s.db.NewDelete().Model((*domain.POLine)(nil)).Where("po_id = ?", id).Exec(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.insertPOLines(r.Context(), id, in.Lines); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	s.handleGetPOByID(w, r, id)
}

func (s *Server) handleDeletePO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.PurchaseOrder)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleSetPOStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		if _, err := s.db.NewUpdate().Model((*domain.PurchaseOrder)(nil)).
			Set("status = ?", status).Set("updated_at = now()").
			Where("id = ?", id).Exec(r.Context()); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		s.handleGetPOByID(w, r, id)
	}
}

func (s *Server) insertPOLines(ctx context.Context, poID uuid.UUID, lines []poLineInput) error {
	for i, l := range lines {
		if l.Description == "" {
			continue
		}
		qty := l.Quantity
		if qty < 1 {
			qty = 1
		}
		attrs := l.Attributes
		if attrs == nil {
			attrs = map[string]any{}
		}
		months := l.WarrantyMonths
		if months < 0 {
			months = 0
		}
		line := &domain.POLine{
			POID: poID, AssetTypeID: l.AssetTypeID, ModelID: l.ModelID, Description: l.Description,
			Quantity: qty, UnitCost: l.UnitCost, WarrantyMonths: months, Attributes: attrs, Sort: i,
		}
		if _, err := s.db.NewInsert().Model(line).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleGetPOByID(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	po := new(domain.PurchaseOrder)
	err := s.db.NewSelect().Model(po).
		Relation("Vendor").Relation("Location").
		Relation("Lines", func(q *bunSelect) *bunSelect { return q.Order("sort ASC") }).
		Relation("Lines.AssetType").
		Where("po.id = ?", id).Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, po)
}

// ---- receiving (spawns assets) -------------------------------------------

type receiveLineInput struct {
	POLineID  uuid.UUID `json:"po_line_id"`
	Quantity  int       `json:"quantity"`
	AssetTags []string  `json:"asset_tags"`
	Serials   []string  `json:"serials"`
}

type receiveInput struct {
	LocationID *uuid.UUID         `json:"location_id"`
	Notes      string             `json:"notes"`
	Lines      []receiveLineInput `json:"lines"`
}

func (s *Server) handleReceivePO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	po := new(domain.PurchaseOrder)
	if err := s.db.NewSelect().Model(po).Relation("Lines").Where("po.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "purchase order not found")
		return
	}
	if po.Status == "cancelled" || po.Status == "closed" {
		writeErr(w, http.StatusConflict, "cannot receive against a "+po.Status+" purchase order")
		return
	}
	var in receiveInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(in.Lines) == 0 {
		writeErr(w, http.StatusBadRequest, "at least one line is required")
		return
	}

	lineByID := map[uuid.UUID]*domain.POLine{}
	for _, l := range po.Lines {
		lineByID[l.ID] = l
	}

	p := s.principal(r)
	loc := in.LocationID
	if loc == nil {
		loc = po.LocationID
	}
	vendorName := s.vendorName(r.Context(), po.VendorID)

	receipt := &domain.Receipt{
		POID: &po.ID, ReceivedBy: &p.UserID, ReceivedAt: time.Now().UTC(),
		LocationID: loc, Notes: in.Notes,
	}
	if _, err := s.db.NewInsert().Model(receipt).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	createdAssets := []uuid.UUID{}
	for _, rl := range in.Lines {
		line, ok := lineByID[rl.POLineID]
		if !ok {
			writeErr(w, http.StatusBadRequest, "line does not belong to this PO")
			return
		}
		if rl.Quantity < 1 {
			continue
		}
		remaining := line.Quantity - line.ReceivedQty
		if rl.Quantity > remaining {
			writeErr(w, http.StatusUnprocessableEntity, "received quantity exceeds outstanding for line: "+line.Description)
			return
		}
		if line.AssetTypeID == nil {
			writeErr(w, http.StatusUnprocessableEntity, "line has no asset type and cannot be received into assets: "+line.Description)
			return
		}

		rlRow := &domain.ReceiptLine{ReceiptID: receipt.ID, POLineID: &line.ID, Quantity: rl.Quantity}
		if _, err := s.db.NewInsert().Model(rlRow).Exec(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		for i := 0; i < rl.Quantity; i++ {
			tag := genAssetTag(po.PONumber, rl.AssetTags, i)
			serial := nthOr(rl.Serials, i)
			a, e := s.spawnReceivedAsset(r.Context(), p.UserID, spawnParams{
				AssetTypeID: *line.AssetTypeID, Name: line.Description, AssetTag: tag,
				Serial: serial, LocationID: loc, Attributes: line.Attributes,
				UnitCost: line.UnitCost, Vendor: vendorName, Currency: po.Currency,
				PONumber: po.PONumber, ReceiptID: receipt.ID,
				ReceivedAt: receipt.ReceivedAt, WarrantyMonths: line.WarrantyMonths,
			})
			if e != nil {
				writeErr(w, http.StatusBadRequest, "failed to create asset: "+e.Error())
				return
			}
			createdAssets = append(createdAssets, a.ID)
		}

		line.ReceivedQty += rl.Quantity
		_, _ = s.db.NewUpdate().Model((*domain.POLine)(nil)).
			Set("received_qty = ?", line.ReceivedQty).Where("id = ?", line.ID).Exec(r.Context())
	}

	s.recalcPOStatus(r.Context(), po)

	writeJSON(w, http.StatusCreated, map[string]any{
		"receipt":         receipt,
		"created_assets":  createdAssets,
		"created_count":   len(createdAssets),
	})
}

type spawnParams struct {
	AssetTypeID    int64
	Name           string
	AssetTag       string
	Serial         string
	LocationID     *uuid.UUID
	Attributes     map[string]any
	UnitCost       float64
	Vendor         string
	Currency       string
	PONumber       string
	ReceiptID      uuid.UUID
	ReceivedAt     time.Time
	WarrantyMonths int
}

// spawnReceivedAsset creates an asset in its lifecycle's initial state and
// records the create + received timeline events and a purchase cost row.
func (s *Server) spawnReceivedAsset(ctx context.Context, actor uuid.UUID, in spawnParams) (*domain.Asset, error) {
	lifecycleID, initial, err := s.meta.ResolveLifecycle(ctx, in.AssetTypeID)
	if err != nil {
		return nil, err
	}
	attrs := map[string]any{}
	for k, v := range in.Attributes {
		attrs[k] = v
	}
	a := &domain.Asset{
		AssetTag: in.AssetTag, Serial: in.Serial, Name: in.Name, AssetTypeID: in.AssetTypeID,
		LifecycleID: lifecycleID, LocationID: in.LocationID, Attributes: attrs,
		Vendor: in.Vendor,
	}
	if in.UnitCost != 0 {
		c := in.UnitCost
		a.PurchaseCost = &c
	}
	// Stamp the purchase date (receipt date) and, if a warranty term was set on
	// the PO line, derive the warranty expiry.
	received := in.ReceivedAt
	if received.IsZero() {
		received = time.Now().UTC()
	}
	a.PurchaseDate = &received
	if in.WarrantyMonths > 0 {
		exp := received.AddDate(0, in.WarrantyMonths, 0)
		a.WarrantyExpiry = &exp
	}
	if initial != nil {
		a.CurrentStateID = &initial.ID
	}
	if _, err := s.db.NewInsert().Model(a).Returning("*").Exec(ctx); err != nil {
		return nil, err
	}
	if initial != nil {
		_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
			AssetID: a.ID, ToStateID: initial.ID, Actor: &actor, Note: "received via " + in.PONumber,
		}).Exec(ctx)
	}
	s.recordAssetEvent(ctx, a.ID, &actor, assetEvent{
		Kind: "created", Subject: events.SubjectAssetCreated, Summary: "Asset created",
		Data: map[string]any{"asset_tag": a.AssetTag, "name": a.Name},
	})
	rid := in.ReceiptID
	s.recordAssetEvent(ctx, a.ID, &actor, assetEvent{
		Kind: "received", Summary: "Received via PO " + in.PONumber,
		Data:     map[string]any{"po_number": in.PONumber, "vendor": in.Vendor},
		RefTable: "proc.receipts", RefID: &rid,
	})
	if in.UnitCost != 0 {
		s.recordCost(ctx, &domain.AssetCost{
			AssetID: a.ID, Kind: "purchase", Amount: in.UnitCost, Currency: in.Currency,
			Vendor: in.Vendor, Reference: in.PONumber, SourceTable: "proc.receipts", SourceID: &rid,
			CreatedBy: &actor,
		})
	}
	s.emit(ctx, events.SubjectAssetCreated, "asset", a.ID.String(), actor.String(),
		map[string]any{"asset_tag": a.AssetTag, "asset_type_id": a.AssetTypeID, "source": "receipt"})
	return a, nil
}

// recalcPOStatus advances the PO status based on received quantities, without
// overriding terminal statuses.
func (s *Server) recalcPOStatus(ctx context.Context, po *domain.PurchaseOrder) {
	if po.Status == "cancelled" || po.Status == "closed" {
		return
	}
	var lines []domain.POLine
	if err := s.db.NewSelect().Model(&lines).Where("po_id = ?", po.ID).Scan(ctx); err != nil {
		return
	}
	total, received, anyReceived := 0, 0, false
	for _, l := range lines {
		total += l.Quantity
		received += l.ReceivedQty
		if l.ReceivedQty > 0 {
			anyReceived = true
		}
	}
	status := po.Status
	switch {
	case total > 0 && received >= total:
		status = "received"
	case anyReceived:
		status = "partially_received"
	}
	if status != po.Status {
		_, _ = s.db.NewUpdate().Model((*domain.PurchaseOrder)(nil)).
			Set("status = ?", status).Set("updated_at = now()").Where("id = ?", po.ID).Exec(ctx)
	}
}

func (s *Server) vendorName(ctx context.Context, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	v := new(domain.Vendor)
	if err := s.db.NewSelect().Model(v).Column("name").Where("id = ?", *id).Scan(ctx); err != nil {
		return ""
	}
	return v.Name
}

// ---- helpers -------------------------------------------------------------

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "v_" + uuid.NewString()[:8]
	}
	return out
}

func parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}

func genAssetTag(poNumber string, provided []string, i int) string {
	if i < len(provided) && provided[i] != "" {
		return provided[i]
	}
	prefix := poNumber
	if prefix == "" {
		prefix = "ASSET"
	}
	return prefix + "-" + strings.ToUpper(uuid.NewString()[:8])
}

func nthOr(list []string, i int) string {
	if i < len(list) {
		return list[i]
	}
	return ""
}
