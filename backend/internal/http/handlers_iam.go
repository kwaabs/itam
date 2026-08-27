package http

import (
	"net/http"
	"strconv"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := s.principal(r)
	perms, err := s.rbac.Permissions(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var profile domain.UserProfile
	_ = s.db.NewSelect().Model(&profile).Where("user_id = ?", p.UserID).Scan(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      p.UserID,
		"email":        p.Email,
		"is_superuser": p.IsSuperuser,
		"permissions":  perms,
		"profile":      profile,
	})
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	var roles []domain.Role
	if err := s.db.NewSelect().Model(&roles).Order("key ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	var perms []domain.Permission
	if err := s.db.NewSelect().Model(&perms).Order("key ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, perms)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	var users []domain.UserProfile
	if err := s.db.NewSelect().Model(&users).Order("email ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListGrants(w http.ResponseWriter, r *http.Request) {
	var grants []domain.RoleGrant
	q := s.db.NewSelect().Model(&grants).Relation("Role").Order("created_at DESC")
	if u := r.URL.Query().Get("user_id"); u != "" {
		q = q.Where("rg.user_id = ?", u)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, grants)
}

type grantInput struct {
	UserID    uuid.UUID `json:"user_id"`
	RoleID    int64     `json:"role_id"`
	ScopeType string    `json:"scope_type"`
	ScopePath string    `json:"scope_path"`
}

func (s *Server) handleCreateGrant(w http.ResponseWriter, r *http.Request) {
	var in grantInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.ScopeType == "" {
		in.ScopeType = "global"
	}
	g := &domain.RoleGrant{
		UserID:    in.UserID,
		RoleID:    in.RoleID,
		ScopeType: in.ScopeType,
		ScopePath: in.ScopePath,
	}
	if _, err := s.db.NewInsert().Model(g).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.rbac.Invalidate(r.Context(), in.UserID)
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleDeleteGrant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	g := new(domain.RoleGrant)
	if err := s.db.NewSelect().Model(g).Where("id = ?", id).Scan(r.Context()); err == nil {
		s.rbac.Invalidate(r.Context(), g.UserID)
	}
	if _, err := s.db.NewDelete().Model((*domain.RoleGrant)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	var rows []domain.AuditLog
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	q := s.db.NewSelect().Model(&rows).Order("occurred_at DESC").Limit(limit)
	if et := r.URL.Query().Get("entity_type"); et != "" {
		q = q.Where("entity_type = ?", et)
	}
	if eid := r.URL.Query().Get("entity_id"); eid != "" {
		q = q.Where("entity_id = ?", eid)
	}
	if subj := r.URL.Query().Get("subject"); subj != "" {
		q = q.Where("subject = ?", subj)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []domain.AuditLog{}
	}
	writeJSON(w, http.StatusOK, rows)
}
