package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// defaultNearbyRadiusKm caps how far "Near me" will reach when no setting is
// configured. Keeps GPS from snapping to a location on another continent.
const defaultNearbyRadiusKm = 25.0

// handleFieldConfig exposes the small set of runtime knobs the mobile field app
// needs. It is authenticated but needs no special permission, so ordinary field
// users (who lack settings.read) can still read it.
func (s *Server) handleFieldConfig(w http.ResponseWriter, r *http.Request) {
	radius := defaultNearbyRadiusKm
	raw := strings.Trim(strings.TrimSpace(s.set.GetOr(r.Context(), "field.nearby_radius_km", "")), `"`)
	if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
		radius = f
	}
	writeJSON(w, http.StatusOK, map[string]any{"nearby_radius_km": radius})
}

func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.set.ListPublic(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

type settingUpdate struct {
	Value json.RawMessage `json:"value"`
}

func (s *Server) handleUpdateSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		writeErr(w, http.StatusBadRequest, "missing key")
		return
	}
	var in settingUpdate
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(in.Value) == 0 {
		writeErr(w, http.StatusBadRequest, "value is required")
		return
	}
	if err := s.set.SetPublic(r.Context(), key, in.Value); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type secretUpdate struct {
	Value string `json:"value"`
}

// handleUpdateSecret rotates a Vault-encrypted secret setting (e.g. the Azure
// client secret). The plaintext is write-only: it is encrypted at rest and is
// never returned by the settings API.
func (s *Server) handleUpdateSecret(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		writeErr(w, http.StatusBadRequest, "missing key")
		return
	}
	var in secretUpdate
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(in.Value) == "" {
		writeErr(w, http.StatusBadRequest, "value is required")
		return
	}
	if err := s.set.SetSecretByKey(r.Context(), key, in.Value); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
