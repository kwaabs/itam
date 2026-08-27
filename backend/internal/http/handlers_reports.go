package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"itam/internal/domain"

	"github.com/google/uuid"
)

// bucket is a generic label/count pair for grouped aggregates.
type bucket struct {
	Label string `bun:"label" json:"label"`
	Count int    `bun:"count" json:"count"`
}

type warrantyBlock struct {
	Tracked int `bun:"tracked" json:"tracked"`
	Expired int `bun:"expired" json:"expired"`
	Next30  int `bun:"next30" json:"next_30"`
	Next90  int `bun:"next90" json:"next_90"`
}

type financialBlock struct {
	Currency      string  `json:"currency"`
	PurchaseValue float64 `json:"purchase_value"`
	LedgerNet     float64 `json:"ledger_net"`
}

type procurementBlock struct {
	OpenPOs   int      `json:"open_pos"`
	OpenValue float64  `json:"open_value"`
	ByStatus  []bucket `json:"by_status"`
}

type reportOverview struct {
	TotalAssets int               `json:"total_assets"`
	ByState     []bucket          `json:"by_state"`
	ByType      []bucket          `json:"by_type"`
	BySite      []bucket          `json:"by_site"`
	Warranty    warrantyBlock     `json:"warranty"`
	Financial   *financialBlock   `json:"financial,omitempty"`
	Procurement procurementBlock  `json:"procurement"`
}

// handleReportOverview returns headline analytics for the dashboard. Financial
// figures are only included for principals holding cost.read.
func (s *Server) handleReportOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := reportOverview{ByState: []bucket{}, ByType: []bucket{}, BySite: []bucket{}}

	if err := s.db.NewRaw(
		`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL`,
	).Scan(ctx, &out.TotalAssets); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT ls.label AS label, count(*) AS count
		   FROM core.assets a
		   JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
		  WHERE a.deleted_at IS NULL
		  GROUP BY ls.label ORDER BY count DESC`,
	).Scan(ctx, &out.ByState); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT at.name AS label, count(*) AS count
		   FROM core.assets a
		   JOIN meta.asset_types at ON at.id = a.asset_type_id
		  WHERE a.deleted_at IS NULL
		  GROUP BY at.name ORDER BY count DESC LIMIT 12`,
	).Scan(ctx, &out.ByType); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Assets grouped by their root location (the site at the top of the ltree).
	if err := s.db.NewRaw(
		`SELECT s.name AS label, count(*) AS count
		   FROM core.assets a
		   JOIN core.locations l ON l.id = a.location_id
		   JOIN core.locations s ON s.path = subltree(l.path, 0, 1)
		  WHERE a.deleted_at IS NULL
		  GROUP BY s.name ORDER BY count DESC LIMIT 12`,
	).Scan(ctx, &out.BySite); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT
		    count(*) FILTER (WHERE warranty_expiry IS NOT NULL)                                                    AS tracked,
		    count(*) FILTER (WHERE warranty_expiry < now())                                                        AS expired,
		    count(*) FILTER (WHERE warranty_expiry >= now() AND warranty_expiry < now() + interval '30 days')      AS next30,
		    count(*) FILTER (WHERE warranty_expiry >= now() AND warranty_expiry < now() + interval '90 days')      AS next90
		   FROM core.assets WHERE deleted_at IS NULL`,
	).Scan(ctx, &out.Warranty); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT status AS label, count(*) AS count
		   FROM proc.purchase_orders GROUP BY status ORDER BY count DESC`,
	).Scan(ctx, &out.Procurement.ByStatus); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var openPO struct {
		OpenCount int     `bun:"open_count"`
		OpenValue float64 `bun:"open_value"`
	}
	if err := s.db.NewRaw(
		`SELECT count(DISTINCT po.id) AS open_count,
		        coalesce(sum(pol.quantity * pol.unit_cost), 0) AS open_value
		   FROM proc.purchase_orders po
		   LEFT JOIN proc.po_lines pol ON pol.po_id = po.id
		  WHERE po.status IN ('draft','approved','ordered','partially_received')`,
	).Scan(ctx, &openPO); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Procurement.OpenPOs = openPO.OpenCount
	out.Procurement.OpenValue = openPO.OpenValue
	if out.Procurement.ByStatus == nil {
		out.Procurement.ByStatus = []bucket{}
	}

	if s.rbac.Can(ctx, s.principal(r), "cost.read") {
		fin := &financialBlock{Currency: s.defaultCurrency(r.Context())}
		_ = s.db.NewRaw(`SELECT coalesce(sum(purchase_cost),0) FROM core.assets WHERE deleted_at IS NULL`).Scan(ctx, &fin.PurchaseValue)
		_ = s.db.NewRaw(`SELECT coalesce(sum(CASE WHEN kind='disposal_proceeds' THEN -amount ELSE amount END),0) FROM core.asset_costs`).Scan(ctx, &fin.LedgerNet)
		out.Financial = fin
	}

	writeJSON(w, http.StatusOK, out)
}

type warrantyRow struct {
	ID             string    `bun:"id" json:"id"`
	AssetTag       string    `bun:"asset_tag" json:"asset_tag"`
	Name           string    `bun:"name" json:"name"`
	WarrantyExpiry time.Time `bun:"warranty_expiry" json:"warranty_expiry"`
	LocationName   string    `bun:"location_name" json:"location_name"`
	TypeName       string    `bun:"type_name" json:"type_name"`
	DaysLeft       int       `bun:"days_left" json:"days_left"`
}

// handleReportWarranty lists assets whose warranty has expired or expires within
// the given window (?days, default 90), soonest first.
func (s *Server) handleReportWarranty(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 3650 {
			days = n
		}
	}
	rows := []warrantyRow{}
	if err := s.queryWarranty(r.Context(), days, &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) queryWarranty(ctx context.Context, days int, dest *[]warrantyRow) error {
	return s.db.NewRaw(
		`SELECT a.id::text AS id, a.asset_tag, a.name, a.warranty_expiry,
		        coalesce(l.name, '—') AS location_name,
		        coalesce(at.name, '—') AS type_name,
		        (a.warranty_expiry::date - current_date) AS days_left
		   FROM core.assets a
		   LEFT JOIN core.locations l ON l.id = a.location_id
		   LEFT JOIN meta.asset_types at ON at.id = a.asset_type_id
		  WHERE a.deleted_at IS NULL
		    AND a.warranty_expiry IS NOT NULL
		    AND a.warranty_expiry < now() + make_interval(days => ?)
		  ORDER BY a.warranty_expiry ASC
		  LIMIT 250`, days,
	).Scan(ctx, dest)
}

type orgInventoryRow struct {
	Key   string `bun:"key" json:"key"`
	Label string `bun:"label" json:"label"`
	Count int    `bun:"count" json:"count"`
}

type orgBreadcrumb struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
}

type orgInventoryReport struct {
	OrgUnit    domain.OrgUnit    `json:"org_unit"`
	Breadcrumb []orgBreadcrumb   `json:"breadcrumb"`
	Total      int               `json:"total"`
	ByType     []orgInventoryRow `json:"by_type"`
}

// handleReportOrgInventory counts assets under an org unit (subtree by default),
// broken down by asset type. ?type=keyboard narrows to one type.
func (s *Server) handleReportOrgInventory(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_unit_id")
	if orgID == "" {
		writeErr(w, http.StatusBadRequest, "org_unit_id is required")
		return
	}
	id, err := uuid.Parse(orgID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid org_unit_id")
		return
	}
	subtreeParam := r.URL.Query().Get("subtree")
	subtree := subtreeParam == "" || subtreeParam == "true" || subtreeParam == "1"
	typeKey := r.URL.Query().Get("type")

	scope := new(domain.OrgUnit)
	if err := s.db.NewSelect().Model(scope).Where("id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "org unit not found")
		return
	}

	out := orgInventoryReport{
		OrgUnit:    *scope,
		Breadcrumb: []orgBreadcrumb{},
		ByType:     []orgInventoryRow{},
	}

	// Ancestor chain for display (region → district → …).
	var crumbs []domain.OrgUnit
	if err := s.db.NewSelect().Model(&crumbs).
		Where("?::ltree <@ path", scope.Path).
		OrderExpr("nlevel(path) ASC").
		Scan(r.Context()); err == nil {
		for _, c := range crumbs {
			out.Breadcrumb = append(out.Breadcrumb, orgBreadcrumb{
				Key: c.Key, Name: c.Name, Kind: c.Kind, Path: c.Path,
			})
		}
	}

	orgFilter := "a.owner_org_unit_id = ?"
	orgArg := any(id)
	if subtree {
		orgFilter = `a.owner_org_unit_id IN (
			SELECT d.id FROM core.org_units d
			JOIN core.org_units p ON p.id = ?
			WHERE d.path <@ p.path)`
		orgArg = id
	}

	typeFilter := ""
	if typeKey != "" {
		typeFilter = ` AND a.asset_type_id IN (
			SELECT t.id FROM meta.asset_types t WHERE t.key = ? OR t.path <@ (
				SELECT path FROM meta.asset_types WHERE key = ? LIMIT 1))`
	}

	if err := s.db.NewRaw(
		`SELECT count(*) FROM core.assets a
		  WHERE a.deleted_at IS NULL AND `+orgFilter+typeFilter,
		append([]any{orgArg}, typeArgs(typeKey)...)...,
	).Scan(r.Context(), &out.Total); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT coalesce(at.key, '') AS key, coalesce(at.name, 'Unknown') AS label, count(*) AS count
		   FROM core.assets a
		   LEFT JOIN meta.asset_types at ON at.id = a.asset_type_id
		  WHERE a.deleted_at IS NULL AND `+orgFilter+typeFilter+`
		  GROUP BY at.key, at.name
		  ORDER BY count DESC, at.name ASC`,
		append([]any{orgArg}, typeArgs(typeKey)...)...,
	).Scan(r.Context(), &out.ByType); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func typeArgs(typeKey string) []any {
	if typeKey == "" {
		return nil
	}
	return []any{typeKey, typeKey}
}

type custodyHolderRow struct {
	PersonID string `bun:"person_id" json:"person_id"`
	Name     string `bun:"name" json:"name"`
	Count    int    `bun:"count" json:"count"`
}

type custodyAnomalyRow struct {
	ID        string `bun:"id" json:"id"`
	AssetTag  string `bun:"asset_tag" json:"asset_tag"`
	Name      string `bun:"name" json:"name"`
	StateName string `bun:"state_name" json:"state_name"`
}

type custodyReport struct {
	Assigned              int                 `json:"assigned"`
	Available             int                 `json:"available"`
	ByHolder              []custodyHolderRow  `json:"by_holder"`
	NotInStockAfterReturn []custodyAnomalyRow `json:"not_in_stock_after_return"`
	Unacknowledged        []custodyAnomalyRow `json:"unacknowledged"`
	AssignedInStock       []custodyAnomalyRow `json:"assigned_in_stock"`
}

// handleReportCustody returns custody analytics: who holds what, and assets
// returned but still in a non-stock lifecycle state.
func (s *Server) handleReportCustody(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := custodyReport{ByHolder: []custodyHolderRow{}, NotInStockAfterReturn: []custodyAnomalyRow{}, Unacknowledged: []custodyAnomalyRow{}, AssignedInStock: []custodyAnomalyRow{}}

	if err := s.db.NewRaw(
		`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL AND assigned_person_id IS NOT NULL`,
	).Scan(ctx, &out.Assigned); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.db.NewRaw(
		`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL AND assigned_person_id IS NULL`,
	).Scan(ctx, &out.Available); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.db.NewRaw(
		`SELECT p.id::text AS person_id,
		        trim(coalesce(p.first_name,'') || ' ' || coalesce(p.last_name,'')) AS name,
		        count(*) AS count
		   FROM core.assets a
		   JOIN core.people p ON p.id = a.assigned_person_id
		  WHERE a.deleted_at IS NULL
		  GROUP BY p.id, p.first_name, p.last_name
		  ORDER BY count DESC, name ASC
		  LIMIT 25`,
	).Scan(ctx, &out.ByHolder); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.db.NewRaw(
		`SELECT a.id::text AS id, a.asset_tag, a.name, coalesce(ls.label, '—') AS state_name
		   FROM core.assets a
		   LEFT JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
		  WHERE a.deleted_at IS NULL
		    AND a.assigned_person_id IS NULL
		    AND (ls.key IS NULL OR ls.key NOT IN ('in_stock', 'retired', 'disposed', 'lost'))
		    AND (ls.is_terminal IS NULL OR ls.is_terminal = false)
		  ORDER BY a.asset_tag ASC
		  LIMIT 50`,
	).Scan(ctx, &out.NotInStockAfterReturn); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.db.NewRaw(
		`SELECT a.id::text AS id, a.asset_tag, a.name, 'awaiting acknowledge' AS state_name
		   FROM core.assets a
		   JOIN core.assignments asg ON asg.asset_id = a.id
		  WHERE a.deleted_at IS NULL AND asg.kind = 'assign' AND asg.returned_at IS NULL
		    AND asg.acknowledged_at IS NULL
		  ORDER BY asg.assigned_at DESC LIMIT 50`,
	).Scan(ctx, &out.Unacknowledged)
	_ = s.db.NewRaw(
		`SELECT a.id::text AS id, a.asset_tag, a.name, coalesce(ls.label, '—') AS state_name
		   FROM core.assets a
		   JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
		  WHERE a.deleted_at IS NULL AND a.assigned_person_id IS NOT NULL AND ls.key = 'in_stock'
		  ORDER BY a.asset_tag ASC LIMIT 50`,
	).Scan(ctx, &out.AssignedInStock)
	writeJSON(w, http.StatusOK, out)
}

type dataQualityReport struct {
	MissingLocation   int                    `json:"missing_location"`
	MissingOrgUnit    int                    `json:"missing_org_unit"`
	MissingSerial     int                    `json:"missing_serial"`
	DuplicateSerials  []duplicateSerialRow   `json:"duplicate_serials"`
	DuplicateGroups   []duplicateSerialGroup `json:"duplicate_groups"`
	Gaps              []dataQualityAssetRow  `json:"gaps"`
}

type duplicateSerialRow struct {
	Serial string `bun:"serial" json:"serial"`
	Count  int    `bun:"count" json:"count"`
}

type dataQualityAssetRow struct {
	ID       string `bun:"id" json:"id"`
	AssetTag string `bun:"asset_tag" json:"asset_tag"`
	Name     string `bun:"name" json:"name"`
	Issue    string `bun:"issue" json:"issue"`
}

func (s *Server) handleReportDataQuality(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := dataQualityReport{DuplicateSerials: []duplicateSerialRow{}, DuplicateGroups: []duplicateSerialGroup{}, Gaps: []dataQualityAssetRow{}}

	_ = s.db.NewRaw(`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL AND location_id IS NULL`).Scan(ctx, &out.MissingLocation)
	_ = s.db.NewRaw(`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL AND owner_org_unit_id IS NULL`).Scan(ctx, &out.MissingOrgUnit)
	_ = s.db.NewRaw(`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL AND coalesce(serial, '') = ''`).Scan(ctx, &out.MissingSerial)

	_ = s.db.NewRaw(
		`SELECT serial, count(*) AS count FROM core.assets
		  WHERE deleted_at IS NULL AND coalesce(serial, '') <> ''
		  GROUP BY serial HAVING count(*) > 1
		  ORDER BY count DESC LIMIT 20`,
	).Scan(ctx, &out.DuplicateSerials)

	_ = s.db.NewRaw(
		`SELECT id::text, asset_tag, name,
		        CASE
		          WHEN location_id IS NULL AND owner_org_unit_id IS NULL THEN 'missing location and org unit'
		          WHEN location_id IS NULL THEN 'missing location'
		          WHEN owner_org_unit_id IS NULL THEN 'missing org unit'
		          ELSE 'missing serial'
		        END AS issue
		   FROM core.assets
		  WHERE deleted_at IS NULL
		    AND (location_id IS NULL OR owner_org_unit_id IS NULL OR coalesce(serial, '') = '')
		  ORDER BY asset_tag ASC LIMIT 50`,
	).Scan(ctx, &out.Gaps)

	out.DuplicateGroups = duplicateSerialGroups(ctx, s.db)

	writeJSON(w, http.StatusOK, out)
}
