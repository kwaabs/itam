package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// writeCSV streams a CSV download with the given filename, header row and data.
func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	cw := csv.NewWriter(w)
	_ = cw.Write(headers)
	for _, row := range rows {
		_ = cw.Write(row)
	}
	cw.Flush()
}

func csvDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func csvMoney(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// handleExportAssets streams the full asset inventory as CSV.
func (s *Server) handleExportAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type row struct {
		AssetTag     string     `bun:"asset_tag"`
		Name         string     `bun:"name"`
		Serial       string     `bun:"serial"`
		TypeName     string     `bun:"type_name"`
		StateName    string     `bun:"state_name"`
		LocationName string     `bun:"location_name"`
		AssignedTo   string     `bun:"assigned_to"`
		Vendor       string     `bun:"vendor"`
		PurchaseCost *float64   `bun:"purchase_cost"`
		PurchaseDate *time.Time `bun:"purchase_date"`
		Warranty     *time.Time `bun:"warranty_expiry"`
		CreatedAt    time.Time  `bun:"created_at"`
	}
	var rows []row
	if err := s.db.NewRaw(
		`SELECT a.asset_tag, a.name, coalesce(a.serial,'') AS serial,
		        coalesce(at.name,'') AS type_name,
		        coalesce(ls.label,'') AS state_name,
		        coalesce(l.name,'') AS location_name,
		        coalesce(trim(p.first_name || ' ' || p.last_name),'') AS assigned_to,
		        coalesce(a.vendor,'') AS vendor,
		        a.purchase_cost, a.purchase_date, a.warranty_expiry, a.created_at
		   FROM core.assets a
		   LEFT JOIN meta.asset_types at ON at.id = a.asset_type_id
		   LEFT JOIN meta.lifecycle_states ls ON ls.id = a.current_state_id
		   LEFT JOIN core.locations l ON l.id = a.location_id
		   LEFT JOIN core.people p ON p.id = a.assigned_person_id
		  WHERE a.deleted_at IS NULL
		  ORDER BY a.asset_tag`,
	).Scan(ctx, &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([][]string, 0, len(rows))
	for _, x := range rows {
		out = append(out, []string{
			x.AssetTag, x.Name, x.Serial, x.TypeName, x.StateName, x.LocationName,
			x.AssignedTo, x.Vendor, csvMoney(x.PurchaseCost), csvDate(x.PurchaseDate),
			csvDate(x.Warranty), x.CreatedAt.Format("2006-01-02"),
		})
	}
	writeCSV(w, "itam_assets.csv",
		[]string{"asset_tag", "name", "serial", "type", "state", "location", "assigned_to", "vendor", "purchase_cost", "purchase_date", "warranty_expiry", "created_at"},
		out)
}

// handleExportWarranty streams the warranty-expiry report as CSV (?days window).
func (s *Server) handleExportWarranty(w http.ResponseWriter, r *http.Request) {
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
	out := make([][]string, 0, len(rows))
	for _, x := range rows {
		out = append(out, []string{
			x.AssetTag, x.Name, x.TypeName, x.LocationName,
			x.WarrantyExpiry.Format("2006-01-02"), strconv.Itoa(x.DaysLeft),
		})
	}
	writeCSV(w, fmt.Sprintf("itam_warranty_%dd.csv", days),
		[]string{"asset_tag", "name", "type", "location", "warranty_expiry", "days_left"},
		out)
}

// handleExportCosts streams the per-asset cost/TCO breakdown as CSV.
func (s *Server) handleExportCosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type row struct {
		AssetTag     string   `bun:"asset_tag"`
		Name         string   `bun:"name"`
		TypeName     string   `bun:"type_name"`
		LocationName string   `bun:"location_name"`
		PurchaseCost *float64 `bun:"purchase_cost"`
		RepairCost   float64  `bun:"repair_cost"`
		NetCost      float64  `bun:"net_cost"`
	}
	var rows []row
	if err := s.db.NewRaw(
		`SELECT a.asset_tag, a.name,
		        coalesce(at.name,'') AS type_name,
		        coalesce(l.name,'') AS location_name,
		        a.purchase_cost,
		        coalesce((SELECT sum(amount) FROM core.asset_costs c WHERE c.asset_id=a.id AND c.kind='repair'),0) AS repair_cost,
		        coalesce(a.purchase_cost,0)
		          + coalesce((SELECT sum(CASE WHEN kind='disposal_proceeds' THEN -amount ELSE amount END) FROM core.asset_costs c WHERE c.asset_id=a.id),0) AS net_cost
		   FROM core.assets a
		   LEFT JOIN meta.asset_types at ON at.id = a.asset_type_id
		   LEFT JOIN core.locations l ON l.id = a.location_id
		  WHERE a.deleted_at IS NULL
		  ORDER BY net_cost DESC`,
	).Scan(ctx, &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([][]string, 0, len(rows))
	for _, x := range rows {
		out = append(out, []string{
			x.AssetTag, x.Name, x.TypeName, x.LocationName,
			csvMoney(x.PurchaseCost),
			strconv.FormatFloat(x.RepairCost, 'f', 2, 64),
			strconv.FormatFloat(x.NetCost, 'f', 2, 64),
		})
	}
	writeCSV(w, "itam_costs.csv",
		[]string{"asset_tag", "name", "type", "location", "purchase_cost", "repair_cost", "net_cost"},
		out)
}

func (s *Server) handleExportCustody(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type row struct {
		AssetTag   string     `bun:"asset_tag"`
		AssetName  string     `bun:"asset_name"`
		Kind       string     `bun:"kind"`
		Holder     string     `bun:"holder"`
		FromLoc    string     `bun:"from_loc"`
		ToLoc      string     `bun:"to_loc"`
		AssignedAt time.Time  `bun:"assigned_at"`
		ReturnedAt *time.Time `bun:"returned_at"`
		AckAt      *time.Time `bun:"acknowledged_at"`
		Reason     string     `bun:"reason"`
	}
	var rows []row
	if err := s.db.NewRaw(
		`SELECT a.asset_tag, a.name AS asset_name, asg.kind,
		        coalesce(trim(p.first_name || ' ' || p.last_name), ou.name, '') AS holder,
		        coalesce(fl.name, '') AS from_loc, coalesce(tl.name, '') AS to_loc,
		        asg.assigned_at, asg.returned_at, asg.acknowledged_at, coalesce(asg.reason, '') AS reason
		   FROM core.assignments asg
		   JOIN core.assets a ON a.id = asg.asset_id AND a.deleted_at IS NULL
		   LEFT JOIN core.people p ON p.id = asg.holder_person_id
		   LEFT JOIN core.org_units ou ON ou.id = asg.holder_org_unit_id
		   LEFT JOIN core.locations fl ON fl.id = asg.from_location_id
		   LEFT JOIN core.locations tl ON tl.id = asg.to_location_id
		  ORDER BY asg.assigned_at DESC
		  LIMIT 10000`,
	).Scan(ctx, &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([][]string, 0, len(rows))
	for _, x := range rows {
		out = append(out, []string{
			x.AssetTag, x.AssetName, x.Kind, x.Holder, x.FromLoc, x.ToLoc,
			x.AssignedAt.Format(time.RFC3339), csvDate(x.ReturnedAt), csvDate(x.AckAt), x.Reason,
		})
	}
	writeCSV(w, "itam_custody.csv",
		[]string{"asset_tag", "asset_name", "kind", "holder", "from_location", "to_location", "assigned_at", "returned_at", "acknowledged_at", "reason"},
		out)
}
