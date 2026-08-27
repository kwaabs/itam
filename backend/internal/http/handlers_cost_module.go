package http

import (
	"net/http"
	"strconv"
	"time"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// bookValueExpr is the straight-line book value of an asset `a` joined to its
// type `at`. No purchase cost -> 0; no useful life or unknown purchase date ->
// full cost (treated as not yet depreciating); otherwise linear to zero.
const bookValueExpr = `CASE
	WHEN a.purchase_cost IS NULL THEN 0
	WHEN COALESCE(at.useful_life_months,0) = 0 OR a.purchase_date IS NULL THEN a.purchase_cost
	ELSE GREATEST(0, a.purchase_cost * (1 - LEAST(1.0,
		(date_part('year', age(current_date, a.purchase_date)) * 12
		 + date_part('month', age(current_date, a.purchase_date)))::numeric
		/ at.useful_life_months)))
END`

const netLedgerSub = `COALESCE((SELECT sum(CASE WHEN ac.kind='disposal_proceeds' THEN -ac.amount ELSE ac.amount END)
	FROM core.asset_costs ac WHERE ac.asset_id = a.id), 0)`

type labeledAmount struct {
	Label  string  `bun:"label" json:"label"`
	Amount float64 `bun:"amount" json:"amount"`
}

type monthAmount struct {
	Month  string  `bun:"month" json:"month"`
	Amount float64 `bun:"amount" json:"amount"`
}

type costAnalytics struct {
	Currency string `json:"currency"`
	Totals   struct {
		TotalSpend float64 `json:"total_spend"`
		Proceeds   float64 `json:"proceeds"`
		Net        float64 `json:"net"`
		Capex      float64 `json:"capex"`
		Repair     float64 `json:"repair"`
		Upgrade    float64 `json:"upgrade"`
		Other      float64 `json:"other"`
	} `json:"totals"`
	Depreciation struct {
		PurchaseValue float64 `json:"purchase_value"`
		BookValue     float64 `json:"book_value"`
		Depreciation  float64 `json:"depreciation"`
	} `json:"depreciation"`
	ByType   []labeledAmount `json:"by_type"`
	BySite   []labeledAmount `json:"by_site"`
	ByVendor []labeledAmount `json:"by_vendor"`
	Monthly  []monthAmount   `json:"monthly"`
}

// handleCostSummary returns fleet-wide TCO + depreciation analytics.
func (s *Server) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := costAnalytics{Currency: s.defaultCurrency(ctx), ByType: []labeledAmount{}, BySite: []labeledAmount{}, ByVendor: []labeledAmount{}, Monthly: []monthAmount{}}

	var t struct {
		TotalSpend float64 `bun:"total_spend"`
		Proceeds   float64 `bun:"proceeds"`
		Capex      float64 `bun:"capex"`
		Repair     float64 `bun:"repair"`
		Upgrade    float64 `bun:"upgrade"`
		Other      float64 `bun:"other"`
	}
	if err := s.db.NewRaw(
		`SELECT
		    COALESCE(sum(amount) FILTER (WHERE kind <> 'disposal_proceeds'),0) AS total_spend,
		    COALESCE(sum(amount) FILTER (WHERE kind =  'disposal_proceeds'),0) AS proceeds,
		    COALESCE(sum(amount) FILTER (WHERE kind =  'purchase'),0)          AS capex,
		    COALESCE(sum(amount) FILTER (WHERE kind =  'repair'),0)            AS repair,
		    COALESCE(sum(amount) FILTER (WHERE kind =  'upgrade'),0)           AS upgrade,
		    COALESCE(sum(amount) FILTER (WHERE kind =  'other'),0)             AS other
		   FROM core.asset_costs`,
	).Scan(ctx, &t); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Totals.TotalSpend = t.TotalSpend
	out.Totals.Proceeds = t.Proceeds
	out.Totals.Net = t.TotalSpend - t.Proceeds
	out.Totals.Capex = t.Capex
	out.Totals.Repair = t.Repair
	out.Totals.Upgrade = t.Upgrade
	out.Totals.Other = t.Other

	var d struct {
		PurchaseValue float64 `bun:"purchase_value"`
		BookValue     float64 `bun:"book_value"`
	}
	if err := s.db.NewRaw(
		`SELECT COALESCE(sum(a.purchase_cost),0) AS purchase_value,
		        COALESCE(sum(`+bookValueExpr+`),0) AS book_value
		   FROM core.assets a
		   JOIN meta.asset_types at ON at.id = a.asset_type_id
		  WHERE a.deleted_at IS NULL`,
	).Scan(ctx, &d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Depreciation.PurchaseValue = d.PurchaseValue
	out.Depreciation.BookValue = d.BookValue
	out.Depreciation.Depreciation = d.PurchaseValue - d.BookValue

	if err := s.db.NewRaw(
		`SELECT at.name AS label,
		        COALESCE(sum(CASE WHEN ac.kind='disposal_proceeds' THEN -ac.amount ELSE ac.amount END),0) AS amount
		   FROM core.asset_costs ac
		   JOIN core.assets a ON a.id = ac.asset_id
		   JOIN meta.asset_types at ON at.id = a.asset_type_id
		  GROUP BY at.name ORDER BY amount DESC LIMIT 12`,
	).Scan(ctx, &out.ByType); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT s.name AS label,
		        COALESCE(sum(CASE WHEN ac.kind='disposal_proceeds' THEN -ac.amount ELSE ac.amount END),0) AS amount
		   FROM core.asset_costs ac
		   JOIN core.assets a ON a.id = ac.asset_id
		   JOIN core.locations l ON l.id = a.location_id
		   JOIN core.locations s ON s.path = subltree(l.path, 0, 1)
		  GROUP BY s.name ORDER BY amount DESC LIMIT 12`,
	).Scan(ctx, &out.BySite); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT COALESCE(NULLIF(ac.vendor,''),'—') AS label,
		        COALESCE(sum(ac.amount) FILTER (WHERE ac.kind <> 'disposal_proceeds'),0) AS amount
		   FROM core.asset_costs ac
		  GROUP BY 1 ORDER BY amount DESC LIMIT 12`,
	).Scan(ctx, &out.ByVendor); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.NewRaw(
		`SELECT to_char(date_trunc('month', incurred_at), 'YYYY-MM') AS month,
		        COALESCE(sum(CASE WHEN kind='disposal_proceeds' THEN -amount ELSE amount END),0) AS amount
		   FROM core.asset_costs
		  WHERE incurred_at >= (current_date - interval '24 months')
		  GROUP BY 1 ORDER BY 1`,
	).Scan(ctx, &out.Monthly); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, out)
}

type assetCostRow struct {
	ID           string     `bun:"id" json:"id"`
	AssetTag     string     `bun:"asset_tag" json:"asset_tag"`
	Name         string     `bun:"name" json:"name"`
	TypeName     string     `bun:"type_name" json:"type_name"`
	LocationName string     `bun:"location_name" json:"location_name"`
	PurchaseCost float64    `bun:"purchase_cost" json:"purchase_cost"`
	PurchaseDate *time.Time `bun:"purchase_date" json:"purchase_date"`
	NetCost      float64    `bun:"net_cost" json:"net_cost"`
	RepairCost   float64    `bun:"repair_cost" json:"repair_cost"`
	BookValue    float64    `bun:"book_value" json:"book_value"`
	AgeMonths    *int       `bun:"age_months" json:"age_months"`
}

var costSortColumns = map[string]string{
	"net_cost":      "net_cost",
	"purchase_cost": "a.purchase_cost",
	"repair_cost":   "repair_cost",
	"book_value":    "book_value",
}

// handleCostAssets returns a per-asset TCO table (ranked, filterable).
func (s *Server) handleCostAssets(w http.ResponseWriter, r *http.Request) {
	sortCol := costSortColumns["net_cost"]
	if v := r.URL.Query().Get("sort"); v != "" {
		if c, ok := costSortColumns[v]; ok {
			sortCol = c
		}
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	q := s.db.NewRaw(
		`SELECT a.id::text AS id, a.asset_tag, a.name,
		        COALESCE(at.name,'—') AS type_name,
		        COALESCE(l.name,'—') AS location_name,
		        COALESCE(a.purchase_cost,0) AS purchase_cost,
		        a.purchase_date,
		        `+netLedgerSub+` AS net_cost,
		        COALESCE((SELECT sum(ac.amount) FROM core.asset_costs ac WHERE ac.asset_id=a.id AND ac.kind='repair'),0) AS repair_cost,
		        `+bookValueExpr+` AS book_value,
		        CASE WHEN a.purchase_date IS NULL THEN NULL
		             ELSE (date_part('year', age(current_date, a.purchase_date))*12
		                 + date_part('month', age(current_date, a.purchase_date)))::int END AS age_months
		   FROM core.assets a
		   JOIN meta.asset_types at ON at.id = a.asset_type_id
		   LEFT JOIN core.locations l ON l.id = a.location_id
		  WHERE a.deleted_at IS NULL`+
			func() string {
				if v := r.URL.Query().Get("type_id"); v != "" {
					if _, err := strconv.ParseInt(v, 10, 64); err == nil {
						return ` AND a.asset_type_id = ` + v
					}
				}
				return ""
			}()+
			` ORDER BY `+sortCol+` DESC NULLS LAST LIMIT `+strconv.Itoa(limit),
	)
	rows := []assetCostRow{}
	if err := q.Scan(r.Context(), &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// ---- budgets (cost centers = org units) ----------------------------------

type budgetRow struct {
	OrgUnitID   string  `bun:"org_unit_id" json:"org_unit_id"`
	OrgUnitName string  `bun:"org_unit_name" json:"org_unit_name"`
	Path        string  `bun:"path" json:"path"`
	BudgetID    string  `bun:"budget_id" json:"budget_id"`
	Budget      float64 `bun:"budget" json:"budget"`
	Actual      float64 `bun:"actual" json:"actual"`
}

// handleListBudgets returns every org unit that has a budget or any spend in the
// requested fiscal year, with actuals rolled up the org-unit subtree.
func (s *Server) handleListBudgets(w http.ResponseWriter, r *http.Request) {
	year := time.Now().UTC().Year()
	if v := r.URL.Query().Get("year"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 2000 && n <= 2100 {
			year = n
		}
	}
	rows := []budgetRow{}
	if err := s.db.NewRaw(
		`SELECT ou.id::text AS org_unit_id, ou.name AS org_unit_name, ou.path::text AS path,
		        COALESCE(b.id::text,'') AS budget_id,
		        COALESCE(b.amount,0) AS budget,
		        COALESCE(spend.actual,0) AS actual
		   FROM core.org_units ou
		   LEFT JOIN fin.budgets b ON b.org_unit_id = ou.id AND b.period_year = ?
		   LEFT JOIN LATERAL (
		       SELECT sum(CASE WHEN ac.kind='disposal_proceeds' THEN -ac.amount ELSE ac.amount END) AS actual
		         FROM core.assets a
		         JOIN core.org_units sub ON sub.id = a.owner_org_unit_id AND sub.path <@ ou.path
		         JOIN core.asset_costs ac ON ac.asset_id = a.id AND date_part('year', ac.incurred_at) = ?
		   ) spend ON true
		  WHERE b.id IS NOT NULL OR spend.actual IS NOT NULL
		  ORDER BY ou.path`, year, year,
	).Scan(r.Context(), &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"year": year, "rows": rows})
}

type budgetInput struct {
	OrgUnitID  uuid.UUID `json:"org_unit_id"`
	PeriodYear int       `json:"period_year"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Notes      string    `json:"notes"`
}

// handleUpsertBudget creates or updates the budget for an org unit + year.
func (s *Server) handleUpsertBudget(w http.ResponseWriter, r *http.Request) {
	var in budgetInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.OrgUnitID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "org_unit_id is required")
		return
	}
	if in.PeriodYear < 2000 || in.PeriodYear > 2100 {
		writeErr(w, http.StatusBadRequest, "period_year out of range")
		return
	}
	if in.Currency == "" {
		in.Currency = s.defaultCurrency(r.Context())
	}
	p := s.principal(r)
	b := &domain.Budget{
		OrgUnitID: in.OrgUnitID, PeriodYear: in.PeriodYear, Amount: in.Amount,
		Currency: in.Currency, Notes: in.Notes, CreatedBy: &p.UserID,
	}
	if _, err := s.db.NewInsert().Model(b).
		On("CONFLICT (org_unit_id, period_year) DO UPDATE").
		Set("amount = EXCLUDED.amount, currency = EXCLUDED.currency, notes = EXCLUDED.notes, updated_at = now()").
		Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleUpdateBudget(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in budgetInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Currency == "" {
		in.Currency = s.defaultCurrency(r.Context())
	}
	if _, err := s.db.NewUpdate().Model((*domain.Budget)(nil)).
		Set("amount = ?", in.Amount).Set("currency = ?", in.Currency).Set("notes = ?", in.Notes).
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteBudget(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.Budget)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- depreciation config (useful life per asset type) --------------------

type depreciationRow struct {
	ID               int64  `bun:"id" json:"id"`
	Key              string `bun:"key" json:"key"`
	Name             string `bun:"name" json:"name"`
	UsefulLifeMonths int    `bun:"useful_life_months" json:"useful_life_months"`
	AssetCount       int    `bun:"asset_count" json:"asset_count"`
}

func (s *Server) handleListDepreciation(w http.ResponseWriter, r *http.Request) {
	rows := []depreciationRow{}
	if err := s.db.NewRaw(
		`SELECT at.id, at.key, at.name, at.useful_life_months,
		        COALESCE(cnt.n,0) AS asset_count
		   FROM meta.asset_types at
		   LEFT JOIN (SELECT asset_type_id, count(*) AS n FROM core.assets WHERE deleted_at IS NULL GROUP BY asset_type_id) cnt
		     ON cnt.asset_type_id = at.id
		  WHERE at.is_abstract = false
		  ORDER BY at.name`,
	).Scan(r.Context(), &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

type depreciationInput struct {
	UsefulLifeMonths int `json:"useful_life_months"`
}

func (s *Server) handleUpdateDepreciation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in depreciationInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.UsefulLifeMonths < 0 || in.UsefulLifeMonths > 1200 {
		writeErr(w, http.StatusBadRequest, "useful_life_months out of range")
		return
	}
	if _, err := s.db.NewUpdate().Model((*domain.AssetType)(nil)).
		Set("useful_life_months = ?", in.UsefulLifeMonths).Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
