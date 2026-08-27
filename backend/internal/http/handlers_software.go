package http

import (
	"net/http"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- software catalog ----------------------------------------------------

type softwareOut struct {
	domain.Software
	InstallCount int `json:"install_count"`
	LicenseCount int `json:"license_count"`
}

func (s *Server) groupCounts(r *http.Request, model any, col string) map[string]int {
	out := map[string]int{}
	var rows []struct {
		K uuid.UUID `bun:"k"`
		N int       `bun:"n"`
	}
	_ = s.db.NewSelect().Model(model).
		ColumnExpr(col+" AS k").ColumnExpr("count(*) AS n").
		Where(col+" IS NOT NULL").GroupExpr(col).Scan(r.Context(), &rows)
	for _, row := range rows {
		out[row.K.String()] = row.N
	}
	return out
}

func (s *Server) handleListSoftware(w http.ResponseWriter, r *http.Request) {
	items := []domain.Software{}
	if err := s.db.NewSelect().Model(&items).Order("sw.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	installs := s.groupCounts(r, (*domain.Installation)(nil), "software_id")
	licenses := s.groupCounts(r, (*domain.License)(nil), "software_id")
	out := make([]softwareOut, len(items))
	for i, it := range items {
		out[i] = softwareOut{Software: it, InstallCount: installs[it.ID.String()], LicenseCount: licenses[it.ID.String()]}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSoftware(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	sw := new(domain.Software)
	if err := s.db.NewSelect().Model(sw).
		Relation("Versions", func(q *bunSelect) *bunSelect { return q.Order("version DESC") }).
		Where("sw.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "software not found")
		return
	}
	installs := []domain.Installation{}
	_ = s.db.NewSelect().Model(&installs).Relation("Asset").Relation("Version").
		Where("inst.software_id = ?", id).Order("inst.created_at DESC").Scan(r.Context())
	licenses := []domain.License{}
	_ = s.db.NewSelect().Model(&licenses).Where("lc.software_id = ?", id).Order("lc.name ASC").Scan(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"software":      sw,
		"installations": installs,
		"licenses":      licenses,
	})
}

func (s *Server) handleCreateSoftware(w http.ResponseWriter, r *http.Request) {
	var in domain.Software
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.Category == "" {
		in.Category = "application"
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

func (s *Server) handleUpdateSoftware(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Software
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "publisher", "category", "description", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteSoftware(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Software)(nil))
}

// ---- versions ------------------------------------------------------------

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.SoftwareVersion
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.SoftwareID = id
	if in.Version == "" {
		writeErr(w, http.StatusBadRequest, "version is required")
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

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "verID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.SoftwareVersion)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, "cannot delete")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- installations -------------------------------------------------------

func (s *Server) handleListInstallations(w http.ResponseWriter, r *http.Request) {
	items := []domain.Installation{}
	q := s.db.NewSelect().Model(&items).Relation("Software").Relation("Version").Relation("Asset").
		Order("inst.created_at DESC")
	if v := r.URL.Query().Get("asset_id"); v != "" {
		q = q.Where("inst.asset_id = ?", v)
	}
	if v := r.URL.Query().Get("software_id"); v != "" {
		q = q.Where("inst.software_id = ?", v)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateInstallation(w http.ResponseWriter, r *http.Request) {
	var in domain.Installation
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.SoftwareID == uuid.Nil || in.AssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "software_id and asset_id are required")
		return
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, "could not record install (already installed on this asset?)")
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteInstallation(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Installation)(nil))
}

// ---- licenses ------------------------------------------------------------

type licenseOut struct {
	domain.License
	SeatsUsed int `json:"seats_used"`
}

func (s *Server) handleListLicenses(w http.ResponseWriter, r *http.Request) {
	items := []domain.License{}
	if err := s.db.NewSelect().Model(&items).Relation("Software").Relation("Vendor").
		Order("lc.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	used := s.groupCounts(r, (*domain.LicenseAssignment)(nil), "license_id")
	out := make([]licenseOut, len(items))
	for i, it := range items {
		out[i] = licenseOut{License: it, SeatsUsed: used[it.ID.String()]}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetLicense(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	lc := new(domain.License)
	if err := s.db.NewSelect().Model(lc).Relation("Software").Relation("Vendor").
		Where("lc.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "license not found")
		return
	}
	asg := []domain.LicenseAssignment{}
	_ = s.db.NewSelect().Model(&asg).Relation("Asset").Relation("Person").
		Where("la.license_id = ?", id).Order("la.assigned_at DESC").Scan(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"license":     lc,
		"assignments": asg,
		"seats_used":  len(asg),
	})
}

func (s *Server) handleCreateLicense(w http.ResponseWriter, r *http.Request) {
	var in domain.License
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.LicenseType == "" {
		in.LicenseType = "subscription"
	}
	if in.Currency == "" {
		in.Currency = s.defaultCurrency(r.Context())
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

func (s *Server) handleUpdateLicense(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.License
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("software_id", "name", "license_key", "license_type", "seats", "vendor_id",
			"purchase_cost", "currency", "start_date", "expiry_date", "notes", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteLicense(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.License)(nil))
}

// ---- license assignments -------------------------------------------------

func (s *Server) handleCreateLicenseAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.LicenseAssignment
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.LicenseID = id
	if in.AssetID == nil && in.PersonID == nil {
		writeErr(w, http.StatusBadRequest, "assign to an asset or a person")
		return
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteLicenseAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "asgID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.LicenseAssignment)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, "cannot delete")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
