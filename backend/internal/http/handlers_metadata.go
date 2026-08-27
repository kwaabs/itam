package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

// bunSelect aliases the Bun select query for terse relation callbacks.
type bunSelect = bun.SelectQuery

func intParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func (s *Server) handleListAssetTypes(w http.ResponseWriter, r *http.Request) {
	types, err := s.meta.ListAssetTypes(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, types)
}

func (s *Server) handleAssetTypeFields(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	fields, err := s.meta.FieldsForType(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fields)
}

func (s *Server) handleListDataTypes(w http.ResponseWriter, r *http.Request) {
	var dts []domain.DataType
	if err := s.db.NewSelect().Model(&dts).Order("label ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dts)
}

func (s *Server) handleListUnits(w http.ResponseWriter, r *http.Request) {
	var units []domain.Unit
	if err := s.db.NewSelect().Model(&units).Order("label ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, units)
}

// defaultCurrency returns the metadata-flagged default (reporting) currency
// code, falling back to USD when none is configured.
func (s *Server) defaultCurrency(ctx context.Context) string {
	var code string
	err := s.db.NewSelect().Model((*domain.Currency)(nil)).
		Column("code").Where("is_default = true").Limit(1).Scan(ctx, &code)
	if err != nil || code == "" {
		return "USD"
	}
	return code
}

func (s *Server) handleListCurrencies(w http.ResponseWriter, r *http.Request) {
	var cur []domain.Currency
	q := s.db.NewSelect().Model(&cur).Order("sort ASC").Order("code ASC")
	if r.URL.Query().Get("all") != "true" {
		q = q.Where("enabled = true")
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cur)
}

func (s *Server) handleUpsertCurrency(w http.ResponseWriter, r *http.Request) {
	var in domain.Currency
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if in.Code == "" || in.Name == "" {
		writeErr(w, http.StatusBadRequest, "code and name are required")
		return
	}
	if in.Sort == 0 {
		in.Sort = 100
	}
	if in.IsDefault {
		// a single default: clear any existing default first
		if _, err := s.db.NewUpdate().Model((*domain.Currency)(nil)).
			Set("is_default = false").Where("is_default = true").Exec(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if _, err := s.db.NewInsert().Model(&in).
		On("CONFLICT (code) DO UPDATE").
		Set("name = EXCLUDED.name, symbol = EXCLUDED.symbol, is_default = EXCLUDED.is_default, enabled = EXCLUDED.enabled, sort = EXCLUDED.sort").
		Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleListVendorStatuses(w http.ResponseWriter, r *http.Request) {
	var st []domain.VendorStatus
	q := s.db.NewSelect().Model(&st).Order("sort ASC").Order("key ASC")
	if r.URL.Query().Get("all") != "true" {
		q = q.Where("enabled = true")
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpsertVendorStatus(w http.ResponseWriter, r *http.Request) {
	var in domain.VendorStatus
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = strings.ToLower(strings.TrimSpace(in.Key))
	if in.Key == "" || in.Label == "" {
		writeErr(w, http.StatusBadRequest, "key and label are required")
		return
	}
	if in.Sort == 0 {
		in.Sort = 100
	}
	if _, err := s.db.NewInsert().Model(&in).
		On("CONFLICT (key) DO UPDATE").
		Set("label = EXCLUDED.label, color = EXCLUDED.color, blocks_orders = EXCLUDED.blocks_orders, enabled = EXCLUDED.enabled, sort = EXCLUDED.sort").
		Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleListLifecycles(w http.ResponseWriter, r *http.Request) {
	var lcs []domain.Lifecycle
	if err := s.db.NewSelect().Model(&lcs).Order("name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lcs)
}

func (s *Server) handleGetLifecycle(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	lc := new(domain.Lifecycle)
	err = s.db.NewSelect().Model(lc).
		Relation("States", func(q *bunSelect) *bunSelect { return q.Order("sort ASC") }).
		Relation("Transitions", func(q *bunSelect) *bunSelect { return q.Order("sort ASC") }).
		Where("lc.id = ?", id).Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusNotFound, "lifecycle not found")
		return
	}
	writeJSON(w, http.StatusOK, lc)
}

func (s *Server) handleListRelationshipTypes(w http.ResponseWriter, r *http.Request) {
	var rts []domain.RelationshipType
	if err := s.db.NewSelect().Model(&rts).Order("sort ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rts)
}

func (s *Server) handleListAutomationRules(w http.ResponseWriter, r *http.Request) {
	var rules []domain.AutomationRule
	if err := s.db.NewSelect().Model(&rules).Order("sort ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

// ---- management ----------------------------------------------------------

func (s *Server) handleCreateAssetType(w http.ResponseWriter, r *http.Request) {
	var at domain.AssetType
	if err := decode(r, &at); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// derive ltree path from parent
	if at.ParentID != nil {
		parent := new(domain.AssetType)
		if err := s.db.NewSelect().Model(parent).Where("id = ?", *at.ParentID).Scan(r.Context()); err != nil {
			writeErr(w, http.StatusBadRequest, "parent not found")
			return
		}
		at.Path = parent.Path + "." + at.Key
	} else {
		at.Path = at.Key
	}
	if _, err := s.db.NewInsert().Model(&at).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, at)
}

func (s *Server) handleUpdateAssetType(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.AssetType
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "lifecycle_id", "icon", "is_abstract", "sort").
		Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteAssetType(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.AssetType)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleCreateField(w http.ResponseWriter, r *http.Request) {
	var fd domain.FieldDefinition
	if err := decode(r, &fd); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.db.NewInsert().Model(&fd).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, fd)
}

func (s *Server) handleUpdateField(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.FieldDefinition
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if _, err := s.db.NewUpdate().Model(&in).
		Column("label", "data_type_id", "required", "is_unique", "default_value",
			"validation", "unit_id", "enum_options", "reference_target", "indexed", "help_text", "sort").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteField(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.FieldDefinition)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- transition fields ---------------------------------------------------

func (s *Server) handleListTransitionFields(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	fields, err := s.meta.FieldsForTransition(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fields)
}

func (s *Server) handleCreateTransitionField(w http.ResponseWriter, r *http.Request) {
	var tf domain.TransitionField
	if err := decode(r, &tf); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.db.NewInsert().Model(&tf).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tf)
}

func (s *Server) handleUpdateTransitionField(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.TransitionField
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if _, err := s.db.NewUpdate().Model(&in).
		Column("label", "data_type_id", "required", "default_value", "validation",
			"enum_options", "help_text", "sort").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteTransitionField(w http.ResponseWriter, r *http.Request) {
	id, err := intParam(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.TransitionField)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleCreateRelationshipType(w http.ResponseWriter, r *http.Request) {
	var rt domain.RelationshipType
	if err := decode(r, &rt); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.db.NewInsert().Model(&rt).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rt)
}

func (s *Server) handleCreateAutomationRule(w http.ResponseWriter, r *http.Request) {
	var ar domain.AutomationRule
	if err := decode(r, &ar); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.db.NewInsert().Model(&ar).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ar)
}
