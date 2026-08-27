package http

import (
	"net/http"
	"time"

	"itam/internal/domain"
	"itam/internal/events"
	"itam/internal/scheduler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- notification channels ----------------------------------------------

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	items := []domain.NotificationChannel{}
	if err := s.db.NewSelect().Model(&items).Order("nch.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var in domain.NotificationChannel
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := ltreeKey(in.Key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "key: "+err.Error())
		return
	}
	in.Key = key
	if in.Name == "" || in.Type == "" {
		writeErr(w, http.StatusBadRequest, "name and type are required")
		return
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NotificationChannel
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "type", "config", "enabled").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.NotificationChannel)(nil))
}

// handleTestChannel sends a sample notification to an (enabled) channel.
func (s *Server) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ch := new(domain.NotificationChannel)
	if err := s.db.NewSelect().Model(ch).Where("nch.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "channel not found")
		return
	}
	env := events.Envelope{
		Subject: "itam.test", Actor: "test", EntityType: "channel", EntityID: ch.Key,
		OccurredAt: time.Now(),
		Payload:    map[string]any{"count": 1, "message": "Test notification from ITAM"},
	}
	if derr := events.DeliverToChannel(r.Context(), s.db, s.log, ch.Key, "ITAM test notification for {{entity_id}}", env); derr != nil {
		writeErr(w, http.StatusBadGateway, derr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// ---- scheduled checks ----------------------------------------------------

func (s *Server) handleListChecks(w http.ResponseWriter, r *http.Request) {
	items := []domain.ScheduledCheck{}
	if err := s.db.NewSelect().Model(&items).Order("sc.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCheck(w http.ResponseWriter, r *http.Request) {
	var in domain.ScheduledCheck
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := ltreeKey(in.Key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "key: "+err.Error())
		return
	}
	in.Key = key
	if in.Name == "" || in.Kind == "" || in.EventSubject == "" {
		writeErr(w, http.StatusBadRequest, "name, kind and event_subject are required")
		return
	}
	if in.IntervalSeconds <= 0 {
		in.IntervalSeconds = 86400
	}
	if in.Params == nil {
		in.Params = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.ScheduledCheck
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Params == nil {
		in.Params = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "kind", "interval_seconds", "params", "event_subject", "enabled").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteCheck(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.ScheduledCheck)(nil))
}

// handleRunCheck executes a check immediately (manual trigger).
func (s *Server) handleRunCheck(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	c := new(domain.ScheduledCheck)
	if err := s.db.NewSelect().Model(c).Where("sc.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "check not found")
		return
	}
	n, rerr := scheduler.New(s.db, s.bus, s.log).RunCheck(r.Context(), c)
	if rerr != nil {
		writeErr(w, http.StatusBadRequest, rerr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": n})
}

// ---- delivery log --------------------------------------------------------

func (s *Server) handleListNotificationLog(w http.ResponseWriter, r *http.Request) {
	items := []domain.NotificationLog{}
	if err := s.db.NewSelect().Model(&items).Order("nlg.created_at DESC").Limit(100).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}
