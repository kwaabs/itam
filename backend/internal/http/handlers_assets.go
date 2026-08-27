package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"itam/internal/domain"
	"itam/internal/events"
	"itam/internal/metadata"
	"itam/internal/rbac"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// fmtAny renders a value for cheap equality comparison in diffs.
func fmtAny(v any) string { return fmt.Sprintf("%v", v) }

// emit publishes a domain event, logging (not failing) on error.
func (s *Server) emit(ctx context.Context, subject, entityType, entityID, actor string, payload map[string]any) {
	if s.bus == nil {
		return
	}
	if err := s.bus.Publish(ctx, events.Envelope{
		Subject:    subject,
		Actor:      actor,
		EntityType: entityType,
		EntityID:   entityID,
		Payload:    payload,
	}); err != nil {
		s.log.Error("event publish failed", "subject", subject, "err", err)
	}
}

func (s *Server) locationPath(ctx context.Context, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	loc := new(domain.Location)
	if err := s.db.NewSelect().Model(loc).Column("path").Where("id = ?", *id).Scan(ctx); err != nil {
		return ""
	}
	return loc.Path
}

func (s *Server) orgPath(ctx context.Context, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	ou := new(domain.OrgUnit)
	if err := s.db.NewSelect().Model(ou).Column("path").Where("id = ?", *id).Scan(ctx); err != nil {
		return ""
	}
	return ou.Path
}

func (s *Server) assetTarget(ctx context.Context, a *domain.Asset) rbac.Target {
	return rbac.Target{
		LocationPath: s.locationPath(ctx, a.LocationID),
		OrgPath:      s.orgPath(ctx, a.OwnerOrgUnitID),
	}
}

func (s *Server) loadAsset(ctx context.Context, id uuid.UUID) (*domain.Asset, error) {
	a := new(domain.Asset)
	err := s.db.NewSelect().Model(a).
		ColumnExpr("a.*").
		ColumnExpr("ST_Y(a.geog::geometry) AS latitude").
		ColumnExpr("ST_X(a.geog::geometry) AS longitude").
		Relation("AssetType").Relation("CurrentState").
		Relation("Location").Relation("OwnerOrgUnit").Relation("AssignedTo").
		Relation("LastSeenLocation").
		Where("a.id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("page_size"))
	if size < 1 || size > 200 {
		size = 50
	}

	var assets []domain.Asset
	sel := s.db.NewSelect().Model(&assets).
		Relation("AssetType").Relation("CurrentState").
		Relation("Location").Relation("OwnerOrgUnit").Relation("AssignedTo")

	if typeKey := q.Get("type"); typeKey != "" {
		at := new(domain.AssetType)
		if err := s.db.NewSelect().Model(at).Where("key = ?", typeKey).Scan(r.Context()); err == nil {
			// include descendant types
			sel = sel.Where("a.asset_type_id IN (SELECT id FROM meta.asset_types WHERE path <@ ?)", at.Path)
		}
	}
	if st := q.Get("state"); st != "" {
		sel = sel.Where("a.current_state_id IN (SELECT id FROM meta.lifecycle_states WHERE key = ?)", st)
	}
	if loc := q.Get("location_id"); loc != "" {
		// With ?subtree=true the filter rolls up the whole location subtree
		// (the node itself plus every descendant: floors, rooms, departments…)
		// using the ltree path. Otherwise it matches the exact location only.
		if sub := q.Get("subtree"); sub == "true" || sub == "1" {
			sel = sel.Where(`a.location_id IN (
				SELECT d.id FROM core.locations d
				JOIN core.locations p ON p.id = ?
				WHERE d.path <@ p.path)`, loc)
		} else {
			sel = sel.Where("a.location_id = ?", loc)
		}
	}
	if owner := q.Get("owner_org_unit_id"); owner != "" {
		if sub := q.Get("subtree"); sub == "true" || sub == "1" {
			sel = sel.Where(`a.owner_org_unit_id IN (
				SELECT d.id FROM core.org_units d
				JOIN core.org_units p ON p.id = ?
				WHERE d.path <@ p.path)`, owner)
		} else {
			sel = sel.Where("a.owner_org_unit_id = ?", owner)
		}
	}
	if assignee := q.Get("assigned_person_id"); assignee != "" {
		sel = sel.Where("a.assigned_person_id = ?", assignee)
	}
	if search := q.Get("q"); search != "" {
		sel = sel.Where("(a.name ILIKE ? OR a.asset_tag::text ILIKE ?)", "%"+search+"%", "%"+search+"%")
	}

	total, err := sel.Count(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = sel.Order("a.created_at DESC").Limit(size).Offset((page - 1) * size).Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if assets == nil {
		assets = []domain.Asset{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": assets, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	a, err := s.loadAsset(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

type assetInput struct {
	AssetTag         string         `json:"asset_tag"`
	Serial           string         `json:"serial"`
	Name             string         `json:"name"`
	AssetTypeID      int64          `json:"asset_type_id"`
	LocationID       *uuid.UUID     `json:"location_id"`
	OwnerOrgUnitID   *uuid.UUID     `json:"owner_org_unit_id"`
	AssignedPersonID *uuid.UUID     `json:"assigned_person_id"`
	Attributes       map[string]any `json:"attributes"`
	PurchaseCost     *float64       `json:"purchase_cost"`
	Vendor           string         `json:"vendor"`
	Notes            string         `json:"notes"`
	Latitude         *float64       `json:"latitude"`
	Longitude        *float64       `json:"longitude"`
}

// setAssetGeog writes (or clears) an asset's own PostGIS point from lat/long.
func (s *Server) setAssetGeog(ctx context.Context, id uuid.UUID, lat, lng *float64) error {
	if lat == nil || lng == nil {
		_, err := s.db.NewRaw("UPDATE core.assets SET geog = NULL WHERE id = ?", id).Exec(ctx)
		return err
	}
	_, err := s.db.NewRaw(
		"UPDATE core.assets SET geog = ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography WHERE id = ?",
		*lng, *lat, id).Exec(ctx)
	return err
}

type assetMapPoint struct {
	ID           uuid.UUID `bun:"id" json:"id"`
	AssetTag     string    `bun:"asset_tag" json:"asset_tag"`
	Name         string    `bun:"name" json:"name"`
	TypeName     string    `bun:"type_name" json:"type_name"`
	Lat          float64   `bun:"lat" json:"lat"`
	Lng          float64   `bun:"lng" json:"lng"`
	LocationName string    `bun:"location_name" json:"location_name"`
	Source       string    `bun:"source" json:"source"` // asset (own point) | location (inherited)
}

// handleAssetsMap returns plottable points for every asset that resolves to a
// coordinate, preferring the asset's own point and falling back to its location.
func (s *Server) handleAssetsMap(w http.ResponseWriter, r *http.Request) {
	pts := []assetMapPoint{}
	err := s.db.NewRaw(`
		SELECT a.id, a.asset_tag, a.name,
		       COALESCE(t.name, '') AS type_name,
		       ST_Y(COALESCE(a.geog, loc.geog)::geometry) AS lat,
		       ST_X(COALESCE(a.geog, loc.geog)::geometry) AS lng,
		       COALESCE(loc.name, '') AS location_name,
		       CASE WHEN a.geog IS NOT NULL THEN 'asset' ELSE 'location' END AS source
		FROM core.assets a
		LEFT JOIN core.locations loc ON loc.id = a.location_id
		LEFT JOIN meta.asset_types t ON t.id = a.asset_type_id
		WHERE a.deleted_at IS NULL AND COALESCE(a.geog, loc.geog) IS NOT NULL
		ORDER BY a.asset_tag
	`).Scan(r.Context(), &pts)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pts)
}

func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var in assetInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" || in.AssetTag == "" || in.AssetTypeID == 0 {
		writeErr(w, http.StatusBadRequest, "asset_tag, name and asset_type_id are required")
		return
	}
	at, err := s.meta.GetAssetType(r.Context(), in.AssetTypeID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unknown asset type")
		return
	}
	if at.IsAbstract {
		writeErr(w, http.StatusBadRequest, "cannot create an asset of an abstract type")
		return
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if err := s.meta.ValidateAttributes(r.Context(), in.AssetTypeID, in.Attributes); err != nil {
		s.writeValidation(w, err)
		return
	}

	lifecycleID, initial, err := s.meta.ResolveLifecycle(r.Context(), in.AssetTypeID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	a := &domain.Asset{
		AssetTag:         in.AssetTag,
		Serial:           in.Serial,
		Name:             in.Name,
		AssetTypeID:      in.AssetTypeID,
		LifecycleID:      lifecycleID,
		LocationID:       in.LocationID,
		OwnerOrgUnitID:   in.OwnerOrgUnitID,
		AssignedPersonID: in.AssignedPersonID,
		Attributes:       in.Attributes,
		PurchaseCost:     in.PurchaseCost,
		Vendor:           in.Vendor,
		Notes:            in.Notes,
	}
	if initial != nil {
		a.CurrentStateID = &initial.ID
	}

	// scoped authorization against the target location/org
	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.write", s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	if _, err := s.db.NewInsert().Model(a).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Latitude != nil && in.Longitude != nil {
		_ = s.setAssetGeog(r.Context(), a.ID, in.Latitude, in.Longitude)
	}

	if initial != nil {
		_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
			AssetID: a.ID, ToStateID: initial.ID, Actor: &p.UserID, Note: "created",
		}).Exec(r.Context())
	}

	s.recordAssetEvent(r.Context(), a.ID, &p.UserID, assetEvent{
		Kind:    "created",
		Subject: events.SubjectAssetCreated,
		Summary: "Asset created",
		Data:    map[string]any{"asset_tag": a.AssetTag, "name": a.Name},
	})
	if a.PurchaseCost != nil && *a.PurchaseCost != 0 {
		s.recordCost(r.Context(), &domain.AssetCost{
			AssetID: a.ID, Kind: "purchase", Amount: *a.PurchaseCost,
			Vendor: a.Vendor, CreatedBy: &p.UserID,
		})
	}
	s.emit(r.Context(), events.SubjectAssetCreated, "asset", a.ID.String(), p.UserID.String(),
		map[string]any{"asset_tag": a.AssetTag, "asset_type_id": a.AssetTypeID})

	full, _ := s.loadAsset(r.Context(), a.ID)
	writeJSON(w, http.StatusCreated, full)
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing := new(domain.Asset)
	if err := s.db.NewSelect().Model(existing).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	var in assetInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Attributes == nil {
		in.Attributes = existing.Attributes
	}
	if err := s.meta.ValidateAttributes(r.Context(), existing.AssetTypeID, in.Attributes); err != nil {
		s.writeValidation(w, err)
		return
	}

	changes := diffAsset(existing, in)

	existing.Name = in.Name
	existing.Serial = in.Serial
	existing.LocationID = in.LocationID
	existing.OwnerOrgUnitID = in.OwnerOrgUnitID
	existing.AssignedPersonID = in.AssignedPersonID
	existing.Attributes = in.Attributes
	existing.PurchaseCost = in.PurchaseCost
	existing.Vendor = in.Vendor
	existing.Notes = in.Notes

	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.write", s.assetTarget(r.Context(), existing)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	if _, err := s.db.NewUpdate().Model(existing).
		Column("name", "serial", "location_id", "owner_org_unit_id", "assigned_person_id",
			"attributes", "purchase_cost", "vendor", "notes").
		Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// The edit form always sends coordinates, so honour clears (null) too.
	_ = s.setAssetGeog(r.Context(), id, in.Latitude, in.Longitude)
	if len(changes) > 0 {
		s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
			Kind:    "field_changed",
			Subject: events.SubjectAssetUpdated,
			Summary: "Asset edited",
			Data:    map[string]any{"changes": changes},
		})
	}
	s.emit(r.Context(), events.SubjectAssetUpdated, "asset", id.String(), p.UserID.String(), nil)
	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}

// diffAsset returns a map of field -> {from,to} for the columns and attributes
// that changed between the stored asset and the incoming edit.
func diffAsset(before *domain.Asset, in assetInput) map[string]any {
	changes := map[string]any{}
	addStr := func(key, old, new string) {
		if old != new {
			changes[key] = map[string]any{"from": old, "to": new}
		}
	}
	addStr("name", before.Name, in.Name)
	addStr("serial", before.Serial, in.Serial)
	addStr("vendor", before.Vendor, in.Vendor)
	addStr("notes", before.Notes, in.Notes)

	for k, nv := range in.Attributes {
		ov, ok := before.Attributes[k]
		if !ok || fmtAny(ov) != fmtAny(nv) {
			changes["attr:"+k] = map[string]any{"from": ov, "to": nv}
		}
	}
	for k, ov := range before.Attributes {
		if _, ok := in.Attributes[k]; !ok {
			changes["attr:"+k] = map[string]any{"from": ov, "to": nil}
		}
	}
	return changes
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	p := s.principal(r)
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "deleted", Subject: events.SubjectAssetDeleted, Summary: "Asset deleted",
	})
	if _, err := s.db.NewDelete().Model((*domain.Asset)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.emit(r.Context(), events.SubjectAssetDeleted, "asset", id.String(), p.UserID.String(), nil)
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) writeValidation(w http.ResponseWriter, err error) {
	var ve metadata.ValidationErrors
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "validation failed",
			"fields": ve,
		})
		return
	}
	writeErr(w, http.StatusInternalServerError, err.Error())
}

// ---- relationships -------------------------------------------------------

func (s *Server) handleListRelationships(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var rels []domain.AssetRelationship
	if err := s.db.NewSelect().Model(&rels).
		Where("from_asset_id = ? OR to_asset_id = ?", id, id).
		Order("created_at DESC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

type relInput struct {
	ToAssetID          uuid.UUID `json:"to_asset_id"`
	RelationshipTypeID int64     `json:"relationship_type_id"`
}

func (s *Server) handleCreateRelationship(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in relInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	rel := &domain.AssetRelationship{
		FromAssetID: id, ToAssetID: in.ToAssetID, RelationshipTypeID: in.RelationshipTypeID,
	}
	if _, err := s.db.NewInsert().Model(rel).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rel)
}

func (s *Server) handleDeleteRelationship(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "relID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.AssetRelationship)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
