package http

import (
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- pools ---------------------------------------------------------------

func (s *Server) handleListPools(w http.ResponseWriter, r *http.Request) {
	items := []domain.StoragePool{}
	q := s.db.NewSelect().Model(&items).Relation("Array").Order("sp.name ASC")
	if a := r.URL.Query().Get("array_asset_id"); a != "" {
		if id, err := uuid.Parse(a); err == nil {
			q = q.Where("sp.array_asset_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreatePool(w http.ResponseWriter, r *http.Request) {
	var in domain.StoragePool
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name))
	if in.Name == "" || in.ArrayAssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "name and array_asset_id are required")
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

func (s *Server) handleUpdatePool(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.StoragePool
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "raid", "raw_gb", "usable_gb", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeletePool(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.StoragePool)(nil))
}

// ---- volumes -------------------------------------------------------------

func (s *Server) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	items := []domain.StorageVolume{}
	q := s.db.NewSelect().Model(&items).Relation("Pool").Relation("Attached").Order("sv.name ASC")
	if a := r.URL.Query().Get("array_asset_id"); a != "" {
		if id, err := uuid.Parse(a); err == nil {
			q = q.Where("sv.array_asset_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	var in domain.StorageVolume
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name))
	if in.Name == "" || in.ArrayAssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "name and array_asset_id are required")
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

func (s *Server) handleUpdateVolume(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.StorageVolume
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "pool_id", "capacity_gb", "used_gb", "protocol", "attached_asset_id", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.StorageVolume)(nil))
}
