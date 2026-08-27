package http

import (
	"net/http"

	"itam/internal/domain"

	"github.com/uptrace/bun"
)

// handleListStores returns stock-holding locations (kind = "store") enriched
// with the amount of stock they currently hold. A store is just a location, so
// counts roll up the whole subtree (departments and any nested locations) and
// only assets in the "in_stock" lifecycle state are counted.
func (s *Server) handleListStores(w http.ResponseWriter, r *http.Request) {
	type storeRow struct {
		bun.BaseModel `bun:"table:core.locations,alias:loc"`
		domain.Location
		DepartmentCount int     `bun:"department_count" json:"department_count"`
		InStockCount    int     `bun:"in_stock_count" json:"in_stock_count"`
		InStockValue    float64 `bun:"in_stock_value" json:"in_stock_value"`
	}
	var rows []storeRow
	err := s.db.NewSelect().Model(&rows).
		ColumnExpr("loc.*").
		ColumnExpr("ST_Y(loc.geog::geometry) AS latitude").
		ColumnExpr("ST_X(loc.geog::geometry) AS longitude").
		ColumnExpr(`(SELECT count(*) FROM core.locations d
			WHERE d.kind = 'department' AND d.path <@ loc.path AND d.id <> loc.id) AS department_count`).
		ColumnExpr(`(SELECT count(*) FROM core.assets a
			JOIN core.locations al ON al.id = a.location_id
			JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
			WHERE al.path <@ loc.path AND ls.key = 'in_stock' AND a.deleted_at IS NULL) AS in_stock_count`).
		ColumnExpr(`(SELECT COALESCE(SUM(a.purchase_cost), 0) FROM core.assets a
			JOIN core.locations al ON al.id = a.location_id
			JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
			WHERE al.path <@ loc.path AND ls.key = 'in_stock' AND a.deleted_at IS NULL) AS in_stock_value`).
		Where("loc.kind = 'store'").
		Order("loc.path ASC")
	if err := err.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
