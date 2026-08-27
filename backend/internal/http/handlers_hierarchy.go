package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var keyRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// ltreeKey validates / normalises a key so it is safe as an ltree label.
func ltreeKey(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	if !keyRe.MatchString(k) {
		return "", errors.New("key must contain only lowercase letters, digits and underscores")
	}
	return k, nil
}

// ---- locations -----------------------------------------------------------

func (s *Server) handleListLocations(w http.ResponseWriter, r *http.Request) {
	var locs []domain.Location
	if err := s.db.NewSelect().Model(&locs).
		ColumnExpr("loc.*").
		ColumnExpr("ST_Y(loc.geog::geometry) AS latitude").
		ColumnExpr("ST_X(loc.geog::geometry) AS longitude").
		Order("path ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, locs)
}

func (s *Server) handleListLocationKinds(w http.ResponseWriter, r *http.Request) {
	var ks []domain.LocationKind
	if err := s.db.NewSelect().Model(&ks).Order("sort ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ks)
}

// setGeog writes (or clears) a location's PostGIS point from lat/long.
func (s *Server) setGeog(ctx context.Context, id uuid.UUID, lat, lng *float64) error {
	if lat == nil || lng == nil {
		_, err := s.db.NewRaw("UPDATE core.locations SET geog = NULL WHERE id = ?", id).Exec(ctx)
		return err
	}
	_, err := s.db.NewRaw(
		"UPDATE core.locations SET geog = ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography WHERE id = ?",
		*lng, *lat, id).Exec(ctx)
	return err
}

func (s *Server) handleCreateLocation(w http.ResponseWriter, r *http.Request) {
	var in domain.Location
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := ltreeKey(in.Key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = key
	path, err := s.childPath(r.Context(), "core.locations", in.ParentID, key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Path = path
	if in.Kind == "" {
		in.Kind = "site"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.setGeog(r.Context(), in.ID, in.Latitude, in.Longitude); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Location
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "kind", "dr_role", "tier", "timezone", "address", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.setGeog(r.Context(), id, in.Latitude, in.Longitude); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	children, _ := s.db.NewSelect().Model((*domain.Location)(nil)).Where("parent_id = ?", id).Count(r.Context())
	racks, _ := s.db.NewSelect().Model((*domain.Rack)(nil)).Where("location_id = ?", id).Count(r.Context())
	if children > 0 || racks > 0 {
		var blockers []string
		if children > 0 {
			blockers = append(blockers, plural(children, "sub-location", "sub-locations"))
		}
		if racks > 0 {
			blockers = append(blockers, plural(racks, "rack", "racks"))
		}
		writeErr(w, http.StatusConflict,
			"Can't delete: it still contains "+strings.Join(blockers, " and ")+". Move or delete them first.")
		return
	}
	s.deleteByID(w, r, (*domain.Location)(nil))
}

// reparentInput moves a hierarchy node under a new parent (or to root when nil).
type reparentInput struct {
	ParentID *uuid.UUID `json:"parent_id"`
}

// reparent re-roots a node and rewrites the ltree paths of its whole subtree.
// table must be a trusted constant (never user input).
func (s *Server) reparent(ctx context.Context, table string, id uuid.UUID, newParent *uuid.UUID) error {
	var cur struct {
		Key  string `bun:"key"`
		Path string `bun:"path"`
	}
	if err := s.db.NewRaw(
		"SELECT key, path::text AS path FROM "+table+" WHERE id = ?", id,
	).Scan(ctx, &cur); err != nil || cur.Path == "" {
		return errors.New("node not found")
	}
	if newParent != nil && *newParent == id {
		return errors.New("a node cannot be its own parent")
	}
	newPath, err := s.childPath(ctx, table, newParent, cur.Key)
	if err != nil {
		return err
	}
	if newPath == cur.Path {
		return nil // no-op
	}
	if strings.HasPrefix(newPath, cur.Path+".") {
		return errors.New("cannot move a node into itself or its own descendant")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	// Descendants: replace the old path prefix with the new one. subpath() is
	// only valid here because these rows have more labels than the moved node.
	if _, err := tx.NewRaw(
		"UPDATE "+table+" SET path = (?::ltree) || subpath(path, nlevel(?::ltree)) "+
			"WHERE path <@ ?::ltree AND path <> ?::ltree",
		newPath, cur.Path, cur.Path, cur.Path,
	).Exec(ctx); err != nil {
		return err
	}
	// The moved node itself.
	if _, err := tx.NewRaw(
		"UPDATE "+table+" SET path = ?::ltree, parent_id = ?, updated_at = now() WHERE id = ?",
		newPath, newParent, id,
	).Exec(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) handleMoveLocation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in reparentInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.reparent(r.Context(), "core.locations", id, in.ParentID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ---- org units -----------------------------------------------------------

func (s *Server) handleListOrgUnits(w http.ResponseWriter, r *http.Request) {
	var ous []domain.OrgUnit
	if err := s.db.NewSelect().Model(&ous).Order("path ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ous)
}

func (s *Server) handleListOrgUnitKinds(w http.ResponseWriter, r *http.Request) {
	var ks []domain.OrgUnitKind
	if err := s.db.NewSelect().Model(&ks).Order("sort ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ks)
}

func (s *Server) handleCreateOrgUnit(w http.ResponseWriter, r *http.Request) {
	var in domain.OrgUnit
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := ltreeKey(in.Key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = key
	path, err := s.childPath(r.Context(), "core.org_units", in.ParentID, key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Path = path
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateOrgUnit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in struct {
		Name              string     `json:"name"`
		Kind              *string    `json:"kind"`
		DefaultLocationID *uuid.UUID `json:"default_location_id"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	q := s.db.NewUpdate().Model((*domain.OrgUnit)(nil)).
		Set("name = ?", in.Name).
		Set("updated_at = now()").
		Where("ou.id = ?", id)
	if in.Kind == nil || *in.Kind == "" {
		q = q.Set("kind = NULL")
	} else {
		q = q.Set("kind = ?", *in.Kind)
	}
	if in.DefaultLocationID != nil {
		q = q.Set("default_location_id = ?", *in.DefaultLocationID)
	} else {
		q = q.Set("default_location_id = NULL")
	}
	if _, err := q.Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ou := new(domain.OrgUnit)
	if err := s.db.NewSelect().Model(ou).Where("ou.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "org unit not found")
		return
	}
	writeJSON(w, http.StatusOK, ou)
}

func (s *Server) handleDeleteOrgUnit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	children, _ := s.db.NewSelect().Model((*domain.OrgUnit)(nil)).Where("parent_id = ?", id).Count(r.Context())
	if children > 0 {
		writeErr(w, http.StatusConflict,
			"Can't delete: it still contains "+plural(children, "sub-unit", "sub-units")+". Move or delete them first.")
		return
	}
	s.deleteByID(w, r, (*domain.OrgUnit)(nil))
}

func (s *Server) handleMoveOrgUnit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in reparentInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.reparent(r.Context(), "core.org_units", id, in.ParentID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}

// ---- people --------------------------------------------------------------

func (s *Server) handleListPeople(w http.ResponseWriter, r *http.Request) {
	var people []domain.Person
	q := s.db.NewSelect().Model(&people).Order("last_name ASC", "first_name ASC")
	if ou := r.URL.Query().Get("org_unit_id"); ou != "" {
		q = q.Where("org_unit_id = ?", ou)
	}
	if mgr := r.URL.Query().Get("manager_id"); mgr != "" {
		q = q.Where("manager_id = ?", mgr)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, people)
}

func (s *Server) handleCreatePerson(w http.ResponseWriter, r *http.Request) {
	var in domain.Person
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.FirstName == "" || in.LastName == "" {
		writeErr(w, http.StatusBadRequest, "first_name and last_name are required")
		return
	}
	in.IsActive = true
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Person
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if _, err := s.db.NewUpdate().Model(&in).
		Column("first_name", "last_name", "email", "title", "org_unit_id", "manager_id", "is_active").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeletePerson(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Person)(nil))
}

// ---- shared --------------------------------------------------------------

// childPath builds an ltree path for a new node from its parent (or the key
// itself for a root).
func (s *Server) childPath(ctx context.Context, table string, parentID *uuid.UUID, key string) (string, error) {
	if parentID == nil {
		return key, nil
	}
	var parentPath string
	err := s.db.NewRaw("SELECT path::text FROM "+table+" WHERE id = ?", *parentID).Scan(ctx, &parentPath)
	if err != nil || parentPath == "" {
		return "", errors.New("parent not found")
	}
	return parentPath + "." + key, nil
}

func (s *Server) deleteByID(w http.ResponseWriter, r *http.Request, model any) {
	s.deleteByParam(w, r, "id", model)
}

// deleteByParam deletes a row keyed by the given chi URL param (e.g. "id" or
// "subID"), so nested resources can reuse one helper.
func (s *Server) deleteByParam(w http.ResponseWriter, r *http.Request, param string, model any) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model(model).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, "cannot delete (it may have children or references)")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- location aliases ----------------------------------------------------

func (s *Server) handleListLocationAliases(w http.ResponseWriter, r *http.Request) {
	var rows []domain.LocationAlias
	if err := s.db.NewSelect().Model(&rows).Relation("Location").Order("alias ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleUpsertLocationAlias(w http.ResponseWriter, r *http.Request) {
	var in domain.LocationAlias
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Alias = strings.TrimSpace(strings.ToLower(in.Alias))
	if in.Alias == "" || in.LocationID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "alias and location_id are required")
		return
	}
	if _, err := s.db.NewInsert().Model(&in).
		On("CONFLICT (alias) DO UPDATE SET location_id = EXCLUDED.location_id, note = EXCLUDED.note").
		Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteLocationAlias(w http.ResponseWriter, r *http.Request) {
	alias := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "alias")))
	if alias == "" {
		writeErr(w, http.StatusBadRequest, "alias required")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.LocationAlias)(nil)).Where("alias = ?", alias).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
