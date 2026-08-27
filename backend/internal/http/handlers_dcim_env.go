package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- sensors -------------------------------------------------------------

type sensorOut struct {
	domain.Sensor
	LastValue *float64   `json:"last_value"`
	LastTS    *time.Time `json:"last_ts"`
	Status    string     `json:"status"`
}

func (s *Server) handleListSensors(w http.ResponseWriter, r *http.Request) {
	items := []domain.Sensor{}
	q := s.db.NewSelect().Model(&items).Relation("Location").Order("sen.name ASC")
	if loc := r.URL.Query().Get("location_id"); loc != "" {
		if id, err := uuid.Parse(loc); err == nil {
			q = q.Where("sen.location_id = ?", id)
		}
	}
	if rack := r.URL.Query().Get("rack_id"); rack != "" {
		if id, err := uuid.Parse(rack); err == nil {
			q = q.Where("sen.rack_id = ?", id)
		}
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]sensorOut, len(items))
	for i, sen := range items {
		row := sensorOut{Sensor: sen, Status: "unknown"}
		var rd domain.SensorReading
		if err := s.db.NewSelect().Model(&rd).Where("srd.sensor_id = ?", sen.ID).
			Order("srd.ts DESC").Limit(1).Scan(r.Context()); err == nil {
			v := rd.Value
			row.LastValue = &v
			t := rd.TS
			row.LastTS = &t
			row.Status = sensorStatus(&sen, v)
		}
		out[i] = row
	}
	writeJSON(w, http.StatusOK, out)
}

// sensorStatus classifies a value against the sensor's thresholds.
func sensorStatus(sen *domain.Sensor, v float64) string {
	if (sen.MaxThreshold != nil && v > *sen.MaxThreshold) ||
		(sen.MinThreshold != nil && v < *sen.MinThreshold) {
		return "breach"
	}
	// within 5% of a configured limit -> warn
	if sen.MaxThreshold != nil && v >= *sen.MaxThreshold*0.95 {
		return "warn"
	}
	if sen.MinThreshold != nil && *sen.MinThreshold != 0 && v <= *sen.MinThreshold*1.05 {
		return "warn"
	}
	return "ok"
}

func (s *Server) handleCreateSensor(w http.ResponseWriter, r *http.Request) {
	var in domain.Sensor
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Key = firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name))
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.Metric == "" {
		in.Metric = "temperature"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	in.Enabled = true
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateSensor(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Sensor
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "metric", "unit", "location_id", "rack_id", "asset_id",
			"min_threshold", "max_threshold", "enabled", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteSensor(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Sensor)(nil))
}

// ---- readings (time-series) ----------------------------------------------

type readingIn struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
	TS     string  `json:"ts"`
}

// handleAddReading records a reading and, when it breaches a threshold, emits
// itam.env.threshold so the notification pipeline can alert. The breach detail
// is data, so routing/throttling lives in automation rules + channels.
func (s *Server) handleAddReading(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	sen := new(domain.Sensor)
	if err := s.db.NewSelect().Model(sen).Where("sen.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "sensor not found")
		return
	}
	var in readingIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	status := sensorStatus(sen, in.Value)
	rd := &domain.SensorReading{
		SensorID: id, Value: in.Value, Status: status,
		Source: firstNonEmpty(in.Source, "manual"),
	}
	if in.TS != "" {
		if t, perr := time.Parse(time.RFC3339, in.TS); perr == nil {
			rd.TS = t
		}
	}
	if _, err := s.db.NewInsert().Model(rd).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if status == "breach" {
		s.emit(r.Context(), "itam.env.threshold", "sensor", sen.Key, "scheduler", map[string]any{
			"sensor": sen.Key, "name": sen.Name, "metric": sen.Metric,
			"value": in.Value, "unit": sen.Unit,
			"min": sen.MinThreshold, "max": sen.MaxThreshold,
		})
	}
	writeJSON(w, http.StatusCreated, rd)
}

// handleListReadings returns a sensor's recent series for charting.
func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if n, e := strconv.Atoi(h); e == nil && n > 0 && n <= 24*90 {
			hours = n
		}
	}
	items := []domain.SensorReading{}
	if err := s.db.NewSelect().Model(&items).
		Where("srd.sensor_id = ?", id).
		Where("srd.ts >= now() - (? || ' hours')::interval", hours).
		Order("srd.ts ASC").Limit(5000).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}
