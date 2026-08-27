package http

import (
	"net/http"

	"itam/internal/domain"
	"itam/internal/events"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// handleListTransitions returns the lifecycle transitions available from the
// asset's current state, annotated with whether the caller may run each.
func (s *Server) handleListTransitions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	if a.LifecycleID == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	var trans []domain.LifecycleTrans
	q := s.db.NewSelect().Model(&trans).
		Where("lt.lifecycle_id = ?", *a.LifecycleID).
		Order("lt.sort ASC")
	if a.CurrentStateID != nil {
		q = q.Where("(lt.from_state_id = ? OR lt.from_state_id IS NULL)", *a.CurrentStateID)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	p := s.principal(r)
	target := s.assetTarget(r.Context(), a)
	type outT struct {
		domain.LifecycleTrans
		Allowed bool `json:"allowed"`
	}
	out := make([]outT, 0, len(trans))
	for _, t := range trans {
		allowed := true
		if t.RequiredPermission != "" {
			allowed = s.rbac.CanOn(r.Context(), p, t.RequiredPermission, target)
		}
		out = append(out, outT{LifecycleTrans: t, Allowed: allowed})
	}
	writeJSON(w, http.StatusOK, out)
}

type transitionInput struct {
	TransitionKey string         `json:"transition_key"`
	Note          string         `json:"note"`
	Data          map[string]any `json:"data"`
}

func (s *Server) handleTransition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in transitionInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	if a.LifecycleID == nil {
		writeErr(w, http.StatusBadRequest, "asset has no lifecycle")
		return
	}

	t := new(domain.LifecycleTrans)
	if err := s.db.NewSelect().Model(t).
		Where("lt.lifecycle_id = ?", *a.LifecycleID).
		Where("lt.key = ?", in.TransitionKey).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, "unknown transition")
		return
	}

	// from-state guard
	if t.FromStateID != nil {
		if a.CurrentStateID == nil || *a.CurrentStateID != *t.FromStateID {
			writeErr(w, http.StatusConflict, "transition not allowed from current state")
			return
		}
	}

	// permission (scoped)
	p := s.principal(r)
	perm := t.RequiredPermission
	if perm == "" {
		perm = "asset.transition"
	}
	if !s.rbac.CanOn(r.Context(), p, perm, s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	// required asset attributes that must already be set
	for _, f := range t.RequiredFields {
		if v, ok := a.Attributes[f]; !ok || v == nil {
			writeErr(w, http.StatusUnprocessableEntity, "required field missing for transition: "+f)
			return
		}
	}

	// configurable per-transition data captured at run time
	if in.Data == nil {
		in.Data = map[string]any{}
	}
	if err := s.meta.ValidateTransitionData(r.Context(), t.ID, in.Data); err != nil {
		s.writeValidation(w, err)
		return
	}

	from := a.CurrentStateID
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("current_state_id = ?", t.ToStateID).
		Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	hist := &domain.LifecycleHistory{
		AssetID: id, FromStateID: from, ToStateID: t.ToStateID,
		TransitionID: &t.ID, Actor: &p.UserID, Note: in.Note, Data: in.Data,
	}
	_, _ = s.db.NewInsert().Model(hist).Exec(r.Context())

	subject := t.EmitSubject
	if subject == "" {
		subject = events.SubjectAssetStateChanged
	}
	fromLabel := s.stateLabel(r.Context(), from)
	toLabel := s.stateLabel(r.Context(), &t.ToStateID)
	summary := t.Label
	if fromLabel != "" {
		summary = fromLabel + " → " + toLabel
	} else if toLabel != "" {
		summary = "→ " + toLabel
	}
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind:    "state_changed",
		Subject: subject,
		Summary: summary,
		Data: map[string]any{
			"transition": t.Key, "transition_label": t.Label,
			"from_state": fromLabel, "to_state": toLabel,
			"from_state_id": from, "to_state_id": t.ToStateID,
			"note": in.Note, "fields": in.Data,
		},
		RefTable: "core.lifecycle_history", RefID: &hist.ID,
	})
	// Auto-record costs captured by the transition form. By convention a numeric
	// "cost" field is a repair/service cost and "proceeds" is disposal income.
	if amt, ok := numFromMap(in.Data, "cost"); ok {
		s.recordCost(r.Context(), &domain.AssetCost{
			AssetID: id, Kind: "repair", Amount: amt,
			Vendor: strFromMap(in.Data, "vendor"), Reference: strFromMap(in.Data, "rma"),
			SourceTable: "core.lifecycle_history", SourceID: &hist.ID, CreatedBy: &p.UserID,
		})
	}
	if amt, ok := numFromMap(in.Data, "proceeds"); ok {
		s.recordCost(r.Context(), &domain.AssetCost{
			AssetID: id, Kind: "disposal_proceeds", Amount: amt,
			SourceTable: "core.lifecycle_history", SourceID: &hist.ID, CreatedBy: &p.UserID,
		})
	}

	s.emit(r.Context(), subject, "asset", id.String(), p.UserID.String(), map[string]any{
		"transition":    t.Key,
		"from_state_id": from,
		"to_state_id":   t.ToStateID,
	})

	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}

func (s *Server) handleAssetHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var lc []domain.LifecycleHistory
	if err := s.db.NewSelect().Model(&lc).Where("asset_id = ?", id).
		Order("created_at DESC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var asg []domain.Assignment
	if err := s.db.NewSelect().Model(&asg).Where("asset_id = ?", id).
		Relation("HolderPerson").Relation("HolderOrgUnit").
		Relation("FromLocation").Relation("ToLocation").
		Order("assigned_at DESC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lc == nil {
		lc = []domain.LifecycleHistory{}
	}
	if asg == nil {
		asg = []domain.Assignment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lifecycle":   lc,
		"assignments": asg,
	})
}
