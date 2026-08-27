package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// handleListCosts returns an asset's cost ledger plus its net lifetime cost
// (TCO). disposal_proceeds count as an inflow and reduce the net.
func (s *Server) handleListCosts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var costs []domain.AssetCost
	if err := s.db.NewSelect().Model(&costs).Where("asset_id = ?", id).
		Order("incurred_at DESC", "created_at DESC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var net float64
	currency := s.defaultCurrency(r.Context())
	for _, c := range costs {
		if c.Kind == "disposal_proceeds" {
			net -= c.Amount
		} else {
			net += c.Amount
		}
		if c.Currency != "" {
			currency = c.Currency
		}
	}
	if costs == nil {
		costs = []domain.AssetCost{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": costs, "net_cost": net, "currency": currency,
	})
}

type costInput struct {
	Kind       string   `json:"kind"`
	Amount     float64  `json:"amount"`
	Currency   string   `json:"currency"`
	IncurredAt string   `json:"incurred_at"`
	Vendor     string   `json:"vendor"`
	Reference  string   `json:"reference"`
	Note       string   `json:"note"`
}

func (s *Server) handleCreateCost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in costInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Kind == "" {
		in.Kind = "other"
	}
	if in.Currency == "" {
		in.Currency = s.defaultCurrency(r.Context())
	}
	p := s.principal(r)
	c := &domain.AssetCost{
		AssetID: id, Kind: in.Kind, Amount: in.Amount, Currency: in.Currency,
		Vendor: in.Vendor, Reference: in.Reference, Note: in.Note, CreatedBy: &p.UserID,
	}
	if in.IncurredAt != "" {
		if t, e := time.Parse("2006-01-02", in.IncurredAt); e == nil {
			c.IncurredAt = t
		}
	}
	if _, err := s.db.NewInsert().Model(c).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "cost_recorded", Summary: costSummary(in.Kind, in.Amount, in.Currency),
		Data:     map[string]any{"kind": in.Kind, "amount": in.Amount, "currency": in.Currency, "vendor": in.Vendor},
		RefTable: "core.asset_costs", RefID: &c.ID,
	})
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleDeleteCost(w http.ResponseWriter, r *http.Request) {
	costID, err := uuid.Parse(chi.URLParam(r, "costID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.AssetCost)(nil)).Where("id = ?", costID).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// recordCost inserts a cost row (best-effort) from an automated source such as
// asset creation, a repair transition, or a disposal.
func (s *Server) recordCost(ctx context.Context, c *domain.AssetCost) {
	if c.Amount == 0 {
		return
	}
	if c.Currency == "" {
		c.Currency = s.defaultCurrency(ctx)
	}
	if _, err := s.db.NewInsert().Model(c).Exec(ctx); err != nil {
		s.log.Error("cost write failed", "asset", c.AssetID, "kind", c.Kind, "err", err)
	}
}

// numFromMap reads a numeric value from a decoded JSON map (numbers arrive as
// float64; numeric strings are also accepted).
func numFromMap(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, n != 0
	case int:
		return float64(n), n != 0
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%f", &f); err == nil {
			return f, f != 0
		}
	}
	return 0, false
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func costSummary(kind string, amount float64, currency string) string {
	label := map[string]string{
		"purchase": "Purchase cost", "repair": "Repair cost", "upgrade": "Upgrade cost",
		"disposal_proceeds": "Disposal proceeds", "other": "Cost",
	}[kind]
	if label == "" {
		label = "Cost"
	}
	return fmt.Sprintf("%s: %.2f %s", label, amount, currency)
}
