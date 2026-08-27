package http

import (
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- power feeds ---------------------------------------------------------

func (s *Server) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	items := []domain.PowerFeed{}
	q := s.db.NewSelect().Model(&items).Relation("Location").Order("pf.name ASC")
	if loc := r.URL.Query().Get("location_id"); loc != "" {
		if id, err := uuid.Parse(loc); err == nil {
			q = q.Where("pf.location_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Attach connected PDU capacity per feed so the UI can show headroom.
	conn := s.sumByKey(r, `SELECT feed_id::text AS k, COALESCE(SUM(capacity_w),0) AS v
		FROM dcim.pdus WHERE feed_id IS NOT NULL GROUP BY feed_id`)
	out := make([]map[string]any, len(items))
	for i, f := range items {
		out[i] = map[string]any{"feed": f, "connected_w": conn[f.ID.String()]}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateFeed(w http.ResponseWriter, r *http.Request) {
	var in domain.PowerFeed
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name))
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.Source == "" {
		in.Source = "utility"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateFeed(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.PowerFeed
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "location_id", "source", "capacity_w", "voltage", "phase", "redundancy", "attributes", "notes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.PowerFeed)(nil))
}

// ---- PDUs ----------------------------------------------------------------

func (s *Server) handleListPDUs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.PDU{}
	if err := s.db.NewSelect().Model(&items).Relation("Feed").
		Where("pd.rack_id = ?", id).Order("pd.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreatePDU(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.PDU
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.RackID = id
	in.Key = firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name))
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdatePDU(w http.ResponseWriter, r *http.Request) {
	pduID, err := uuid.Parse(chi.URLParam(r, "pduID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.PDU
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = pduID
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "asset_id", "feed_id", "capacity_w", "attributes").
		Set("updated_at = now()").Where("id = ?", pduID).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeletePDU(w http.ResponseWriter, r *http.Request) {
	pduID, err := uuid.Parse(chi.URLParam(r, "pduID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.PDU)(nil)).Where("id = ?", pduID).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---- rack power rollup ---------------------------------------------------

// handleRackPower returns capacity vs measured draw for one rack. Draw is the
// sum of mounted assets' attributes.power_watts; capacity is the sum of PDU
// capacities (preferred) or the rack's design capacity.
func (s *Server) handleRackPower(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	rack := new(domain.Rack)
	if err := s.db.NewSelect().Model(rack).Where("rk.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "rack not found")
		return
	}
	pdus := []domain.PDU{}
	_ = s.db.NewSelect().Model(&pdus).Relation("Feed").Where("pd.rack_id = ?", id).Order("pd.name ASC").Scan(r.Context())

	var pduCapacity int
	for _, p := range pdus {
		if p.CapacityW != nil {
			pduCapacity += *p.CapacityW
		}
	}
	var draw float64
	_ = s.db.NewRaw(`
		SELECT COALESCE(SUM((a.attributes->>'power_watts')::numeric), 0)
		FROM dcim.rack_mounts rm
		JOIN core.assets a ON a.id = rm.asset_id
		WHERE rm.rack_id = ? AND a.deleted_at IS NULL
		  AND a.attributes->>'power_watts' ~ '^[0-9]+(\.[0-9]+)?$'`, id).Scan(r.Context(), &draw)

	capacity := pduCapacity
	if capacity == 0 && rack.PowerCapacityW != nil {
		capacity = *rack.PowerCapacityW
	}
	util := 0.0
	if capacity > 0 {
		util = draw / float64(capacity) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rack_capacity_w": rack.PowerCapacityW,
		"pdu_capacity_w":  pduCapacity,
		"capacity_w":      capacity,
		"draw_w":          draw,
		"util_pct":        util,
		"pdus":            pdus,
	})
}

// sumByKey runs a "SELECT k, v" aggregate query and returns a key->int map.
func (s *Server) sumByKey(r *http.Request, query string, args ...any) map[string]int {
	out := map[string]int{}
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v int
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out
}
