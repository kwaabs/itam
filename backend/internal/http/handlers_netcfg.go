package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// handleListNetDevices returns assets whose type sits under hardware.network
// (firewalls, routers, switches, load balancers) — the things that have a
// configuration worth managing.
func (s *Server) handleListNetDevices(w http.ResponseWriter, r *http.Request) {
	items := []domain.Asset{}
	err := s.db.NewSelect().Model(&items).
		Relation("AssetType").Relation("CurrentState").
		Join("JOIN meta.asset_types at ON at.id = a.asset_type_id").
		Where("at.path <@ 'hardware.network'").
		Where("a.deleted_at IS NULL").
		Order("a.name ASC").Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func assetID(r *http.Request) (uuid.UUID, error) { return uuid.Parse(chi.URLParam(r, "id")) }

// ---- zones ---------------------------------------------------------------

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.NetZone{}
	if err := s.db.NewSelect().Model(&items).Where("nz.asset_id = ?", id).Order("nz.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NetZone
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = id
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

func (s *Server) handleDeleteZone(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.NetZone)(nil))
}

// ---- interfaces ----------------------------------------------------------

func (s *Server) handleListInterfaces(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.NetInterface{}
	if err := s.db.NewSelect().Model(&items).Relation("Zone").Where("ni.asset_id = ?", id).Order("ni.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateInterface(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NetInterface
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = id
	in.Enabled = true
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

func (s *Server) handleUpdateInterface(w http.ResponseWriter, r *http.Request) {
	sid, err := uuid.Parse(chi.URLParam(r, "subID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NetInterface
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = sid
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "ip_cidr", "zone_id", "vlan", "enabled", "attributes").
		Where("id = ?", sid).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteInterface(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.NetInterface)(nil))
}

// ---- rules ---------------------------------------------------------------

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.NetRule{}
	if err := s.db.NewSelect().Model(&items).Where("nr.asset_id = ?", id).Order("nr.seq ASC", "nr.created_at ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NetRule
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = id
	in.Enabled = true
	if in.Action == "" {
		in.Action = "allow"
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

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	sid, err := uuid.Parse(chi.URLParam(r, "subID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NetRule
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = sid
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("seq", "name", "action", "src_zone", "dst_zone", "source", "destination",
			"service", "protocol", "ports", "enabled", "attributes").
		Set("updated_at = now()").Where("id = ?", sid).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.NetRule)(nil))
}

// ---- HA groups -----------------------------------------------------------

func (s *Server) handleListHAGroups(w http.ResponseWriter, r *http.Request) {
	items := []domain.HAGroup{}
	if err := s.db.NewSelect().Model(&items).
		Relation("Members").Relation("Members.Asset").
		Order("hg.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateHAGroup(w http.ResponseWriter, r *http.Request) {
	var in domain.HAGroup
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
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

func (s *Server) handleDeleteHAGroup(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.HAGroup)(nil))
}

func (s *Server) handleAddHAMember(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.HAMember
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.GroupID = gid
	if in.AssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteHAMember(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.HAMember)(nil))
}

// ---- config backups ------------------------------------------------------

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.ConfigBackup{}
	// list view omits the (potentially large) content column
	if err := s.db.NewSelect().Model(&items).
		Column("id", "asset_id", "taken_at", "source", "version", "hash", "size_bytes", "note").
		Where("cb.asset_id = ?", id).Order("cb.taken_at DESC").Limit(200).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetBackup(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "subID")
	item := new(domain.ConfigBackup)
	if err := s.db.NewSelect().Model(item).Where("cb.id = ?", sid).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "backup not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	id, err := assetID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.ConfigBackup
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = id
	if in.Source == "" {
		in.Source = "manual"
	}
	sum := sha256.Sum256([]byte(in.Content))
	in.Hash = hex.EncodeToString(sum[:])
	n := len(in.Content)
	in.SizeBytes = &n
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "subID")
	if _, err := s.db.NewDelete().Model((*domain.ConfigBackup)(nil)).Where("id = ?", sid).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
