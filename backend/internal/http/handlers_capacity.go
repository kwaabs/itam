package http

import (
	"net/http"

	"itam/internal/domain"
)

type capacityRow struct {
	RackID       string   `json:"rack_id"`
	RackName     string   `json:"rack_name"`
	LocationID   *string  `json:"location_id"`
	LocationName string   `json:"location_name"`
	UTotal       int      `json:"u_total"`
	UUsed        int      `json:"u_used"`
	UPct         float64  `json:"u_pct"`
	PowerCapW    int      `json:"power_capacity_w"`
	PowerDrawW   float64  `json:"power_draw_w"`
	PowerPct     float64  `json:"power_pct"`
	WeightCapKg  *float64 `json:"weight_capacity_kg"`
	WeightUsedKg float64  `json:"weight_used_kg"`
	WeightPct    float64  `json:"weight_pct"`
}

// handleCapacity aggregates space (rack U), power and weight utilization across
// all racks, plus per-location and global rollups. No new storage: it reads
// mounts (space), each mounted asset's attributes.power_watts / weight_kg, PDU
// capacity (step B) and the rack budgets.
func (s *Server) handleCapacity(w http.ResponseWriter, r *http.Request) {
	racks := []domain.Rack{}
	if err := s.db.NewSelect().Model(&racks).Relation("Location").Order("rk.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	uUsed := s.sumByKey(r, `SELECT rack_id::text, COALESCE(SUM(u_height),0) FROM dcim.rack_mounts GROUP BY rack_id`)
	pduCap := s.sumByKey(r, `SELECT rack_id::text, COALESCE(SUM(capacity_w),0) FROM dcim.pdus GROUP BY rack_id`)
	drawW := s.sumByKeyF(r, `SELECT rm.rack_id::text, COALESCE(SUM((a.attributes->>'power_watts')::numeric),0)
		FROM dcim.rack_mounts rm JOIN core.assets a ON a.id = rm.asset_id
		WHERE a.deleted_at IS NULL AND a.attributes->>'power_watts' ~ '^[0-9]+(\.[0-9]+)?$'
		GROUP BY rm.rack_id`)
	weightKg := s.sumByKeyF(r, `SELECT rm.rack_id::text, COALESCE(SUM((a.attributes->>'weight_kg')::numeric),0)
		FROM dcim.rack_mounts rm JOIN core.assets a ON a.id = rm.asset_id
		WHERE a.deleted_at IS NULL AND a.attributes->>'weight_kg' ~ '^[0-9]+(\.[0-9]+)?$'
		GROUP BY rm.rack_id`)

	rows := make([]capacityRow, 0, len(racks))
	for _, rk := range racks {
		key := rk.ID.String()
		row := capacityRow{
			RackID: key, RackName: rk.Name,
			UTotal: rk.UHeight, UUsed: uUsed[key],
			PowerDrawW: drawW[key], WeightUsedKg: weightKg[key],
		}
		// capacity: prefer PDU sum, else rack design capacity
		cap := pduCap[key]
		if cap == 0 && rk.PowerCapacityW != nil {
			cap = *rk.PowerCapacityW
		}
		row.PowerCapW = cap
		if rk.Location != nil {
			row.LocationName = rk.Location.Name
		}
		if rk.LocationID != nil {
			lid := rk.LocationID.String()
			row.LocationID = &lid
		}
		row.WeightCapKg = rk.MaxWeightKg
		if row.UTotal > 0 {
			row.UPct = float64(row.UUsed) / float64(row.UTotal) * 100
		}
		if row.PowerCapW > 0 {
			row.PowerPct = row.PowerDrawW / float64(row.PowerCapW) * 100
		}
		if rk.MaxWeightKg != nil && *rk.MaxWeightKg > 0 {
			row.WeightPct = row.WeightUsedKg / *rk.MaxWeightKg * 100
		}
		rows = append(rows, row)
	}

	// totals
	var tU, tUUsed, tPCap int
	var tDraw, tWeight float64
	for _, x := range rows {
		tU += x.UTotal
		tUUsed += x.UUsed
		tPCap += x.PowerCapW
		tDraw += x.PowerDrawW
		tWeight += x.WeightUsedKg
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"racks": rows,
		"totals": map[string]any{
			"u_total": tU, "u_used": tUUsed,
			"power_capacity_w": tPCap, "power_draw_w": tDraw,
			"weight_used_kg": tWeight, "rack_count": len(rows),
		},
	})
}

// sumByKeyF runs a "SELECT k, v" aggregate and returns a key->float map.
func (s *Server) sumByKeyF(r *http.Request, query string, args ...any) map[string]float64 {
	out := map[string]float64{}
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out
}
