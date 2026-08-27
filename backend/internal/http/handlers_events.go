package http

import (
	"context"
	"net/http"
	"strconv"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// assetEvent describes one entry to append to the asset timeline ledger.
type assetEvent struct {
	Kind     string
	Subject  string
	Summary  string
	Data     map[string]any
	RefTable string
	RefID    *uuid.UUID
}

// recordAssetEvent appends a row to core.asset_events. It is best-effort: a
// ledger failure is logged but never fails the originating request (the
// specialized tables remain the systems of record).
func (s *Server) recordAssetEvent(ctx context.Context, assetID uuid.UUID, actor *uuid.UUID, ev assetEvent) {
	if ev.Data == nil {
		ev.Data = map[string]any{}
	}
	row := &domain.AssetEvent{
		AssetID:  assetID,
		Kind:     ev.Kind,
		Subject:  ev.Subject,
		Actor:    actor,
		Summary:  ev.Summary,
		Data:     ev.Data,
		RefTable: ev.RefTable,
		RefID:    ev.RefID,
	}
	if _, err := s.db.NewInsert().Model(row).Exec(ctx); err != nil {
		s.log.Error("asset event write failed", "kind", ev.Kind, "asset", assetID, "err", err)
	}
}

// handleAssetTimeline returns the unified, append-only event ledger for an asset.
func (s *Server) handleAssetTimeline(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, e := strconv.Atoi(l); e == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	var events []domain.AssetEvent
	q := s.db.NewSelect().Model(&events).Where("asset_id = ?", id).Order("occurred_at DESC").Limit(limit)
	if k := r.URL.Query().Get("kind"); k != "" {
		q = q.Where("kind = ?", k)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// stateLabel resolves a lifecycle state's human label (best-effort, "" on miss).
func (s *Server) stateLabel(ctx context.Context, stateID *int64) string {
	if stateID == nil {
		return ""
	}
	st := new(domain.LifecycleState)
	if err := s.db.NewSelect().Model(st).Column("label").Where("id = ?", *stateID).Scan(ctx); err != nil {
		return ""
	}
	return st.Label
}
