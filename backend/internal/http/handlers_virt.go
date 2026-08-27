package http

import (
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- clusters ------------------------------------------------------------

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	items := []domain.Cluster{}
	if err := s.db.NewSelect().Model(&items).Relation("Location").Order("cl.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	var in domain.Cluster
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

func (s *Server) handleUpdateCluster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Cluster
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "location_id", "hypervisor", "ha", "drs", "attributes", "notes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Cluster)(nil))
}

// ---- hosts ---------------------------------------------------------------

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	items := []domain.Host{}
	q := s.db.NewSelect().Model(&items).
		Relation("Asset").Relation("Cluster").
		Relation("VMs").Order("ht.created_at ASC")
	if c := r.URL.Query().Get("cluster_id"); c != "" {
		if id, err := uuid.Parse(c); err == nil {
			q = q.Where("ht.cluster_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var in domain.Host
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.AssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "asset_id is required (the physical server)")
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

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Host
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("cluster_id", "hypervisor", "cpu_cores", "cpu_threads", "ram_gb", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Host)(nil))
}

// ---- vms -----------------------------------------------------------------

func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	items := []domain.VM{}
	q := s.db.NewSelect().Model(&items).Relation("Asset").Order("vm.name ASC")
	if h := r.URL.Query().Get("host_id"); h != "" {
		if id, err := uuid.Parse(h); err == nil {
			q = q.Where("vm.host_id = ?", id)
		}
	}
	if c := r.URL.Query().Get("cluster_id"); c != "" {
		if id, err := uuid.Parse(c); err == nil {
			q = q.Where("vm.cluster_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var in domain.VM
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.PowerState == "" {
		in.PowerState = "unknown"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	// keep cluster consistent with the chosen host when not set explicitly
	if in.HostID != nil && in.ClusterID == nil {
		host := new(domain.Host)
		if err := s.db.NewSelect().Model(host).Where("ht.id = ?", *in.HostID).Scan(r.Context()); err == nil {
			in.ClusterID = host.ClusterID
		}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateVM(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.VM
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("asset_id", "host_id", "cluster_id", "name", "vcpus", "ram_gb", "disk_gb",
			"power_state", "guest_os", "ip", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.VM)(nil))
}
