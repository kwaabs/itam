package http

import (
	"context"
	"net/http"
	"time"

	"itam/internal/domain"
	"itam/internal/events"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type assignInput struct {
	HolderPersonID  *uuid.UUID `json:"holder_person_id"`
	HolderOrgUnitID *uuid.UUID `json:"holder_org_unit_id"`
	ToLocationID    *uuid.UUID `json:"to_location_id"`
	Reason          string     `json:"reason"`
}

func (s *Server) handleAssign(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in assignInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.HolderPersonID == nil && in.HolderOrgUnitID == nil {
		writeErr(w, http.StatusBadRequest, "holder_person_id or holder_org_unit_id is required")
		return
	}
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.assign", s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	// close any open custody first
	s.closeOpenAssignments(r, id)

	toLoc := a.LocationID
	if in.ToLocationID != nil && *in.ToLocationID != uuid.Nil {
		toLoc = in.ToLocationID
	} else if in.HolderPersonID != nil {
		if def := s.defaultLocationForPerson(r.Context(), *in.HolderPersonID); def != nil {
			toLoc = def
		}
	} else if in.HolderOrgUnitID != nil {
		if def := s.defaultLocationForOrgUnit(r.Context(), *in.HolderOrgUnitID); def != nil {
			toLoc = def
		}
	}

	asg := &domain.Assignment{
		AssetID: id, Kind: "assign",
		HolderPersonID: in.HolderPersonID, HolderOrgUnitID: in.HolderOrgUnitID,
		AssignedBy: &p.UserID, Reason: in.Reason,
		FromLocationID: a.LocationID, ToLocationID: toLoc,
	}
	if _, err := s.db.NewInsert().Model(asg).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	updates := s.db.NewUpdate().Model((*domain.Asset)(nil)).Set("updated_at = now()").Where("id = ?", id)
	if in.HolderPersonID != nil {
		updates = updates.Set("assigned_person_id = ?", *in.HolderPersonID)
	}
	if in.ToLocationID != nil && *in.ToLocationID != uuid.Nil {
		updates = updates.Set("location_id = ?", *in.ToLocationID)
	} else if in.HolderPersonID != nil {
		if def := s.defaultLocationForPerson(r.Context(), *in.HolderPersonID); def != nil {
			updates = updates.Set("location_id = ?", *def)
		}
	} else if in.HolderOrgUnitID != nil {
		if def := s.defaultLocationForOrgUnit(r.Context(), *in.HolderOrgUnitID); def != nil {
			updates = updates.Set("location_id = ?", *def)
		}
	}
	if _, err := updates.Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	evData := map[string]any{
		"holder_person_id": in.HolderPersonID, "holder_org_unit_id": in.HolderOrgUnitID,
		"reason": in.Reason,
	}
	if in.ToLocationID != nil && *in.ToLocationID != uuid.Nil {
		evData["to_location_id"] = *in.ToLocationID
		evData["to_location"] = s.locationName(r.Context(), in.ToLocationID)
		if a.LocationID != nil {
			evData["from_location_id"] = a.LocationID
			evData["from_location"] = s.locationName(r.Context(), a.LocationID)
		}
	}
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "assigned", Subject: events.SubjectAssetAssigned, Summary: s.holderSummary(r, in),
		Data: evData,
		RefTable: "core.assignments", RefID: &asg.ID,
	})
	s.emit(r.Context(), events.SubjectAssetAssigned, "asset", id.String(), p.UserID.String(),
		map[string]any{"holder_person_id": in.HolderPersonID, "holder_org_unit_id": in.HolderOrgUnitID})

	s.assignToInUse(r.Context(), a, &p.UserID, in.Reason)

	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}

func (s *Server) handleReturn(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in assignInput
	_ = decode(r, &in)
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.assign", s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	s.closeOpenAssignments(r, id)
	ret := &domain.Assignment{
		AssetID: id, Kind: "return", AssignedBy: &p.UserID, Reason: in.Reason,
		ReturnedAt: ptrTime(time.Now().UTC()),
	}
	_, _ = s.db.NewInsert().Model(ret).Exec(r.Context())
	_, _ = s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("assigned_person_id = NULL").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context())

	// Returning custody also moves the asset back into stock so its lifecycle
	// state reflects reality. Only do this when the asset has an "in_stock"
	// state and isn't in a terminal state (retired/disposed).
	movedToStock := s.returnToStock(r, a, &p.UserID, in.Reason)

	summary := "Returned"
	if movedToStock {
		summary = "Returned to stock"
	}
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "returned", Subject: events.SubjectAssetReturned, Summary: summary,
		Data: map[string]any{"reason": in.Reason}, RefTable: "core.assignments", RefID: &ret.ID,
	})
	s.emit(r.Context(), events.SubjectAssetReturned, "asset", id.String(), p.UserID.String(), nil)
	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}

type transferInput struct {
	ToLocationID uuid.UUID `json:"to_location_id"`
	Reason       string    `json:"reason"`
}

// handleTransfer moves an asset to another location (e.g. deployed from HQ),
// recording the movement and updating the asset's location.
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in transferInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.ToLocationID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "to_location_id is required")
		return
	}
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.assign", s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	from := a.LocationID
	mv := &domain.Assignment{
		AssetID: id, Kind: "transfer", AssignedBy: &p.UserID, Reason: in.Reason,
		FromLocationID: from, ToLocationID: &in.ToLocationID,
	}
	_, _ = s.db.NewInsert().Model(mv).Exec(r.Context())
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("location_id = ?", in.ToLocationID).
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	fromName := s.locationName(r.Context(), from)
	toName := s.locationName(r.Context(), &in.ToLocationID)
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "transferred", Subject: events.SubjectAssetTransferred,
		Summary: "Moved " + orDash(fromName) + " → " + orDash(toName),
		Data: map[string]any{
			"from_location_id": from, "to_location_id": in.ToLocationID,
			"from_location": fromName, "to_location": toName, "reason": in.Reason,
		},
		RefTable: "core.assignments", RefID: &mv.ID,
	})
	s.emit(r.Context(), events.SubjectAssetTransferred, "asset", id.String(), p.UserID.String(),
		map[string]any{"from_location_id": from, "to_location_id": in.ToLocationID})
	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}

// returnToStock moves an asset's lifecycle state back to "in_stock" when it is
// returned from custody, recording lifecycle history so the change is visible
// on the asset's state badge and timeline. It is a no-op (returns false) when
// the asset has no lifecycle, no in_stock state, is already in stock, or sits in
// a terminal state (retired/disposed).
func (s *Server) returnToStock(r *http.Request, a *domain.Asset, actor *uuid.UUID, reason string) bool {
	if a.LifecycleID == nil {
		return false
	}
	type stateRow struct {
		ID         int64  `bun:"id"`
		Key        string `bun:"key"`
		IsTerminal bool   `bun:"is_terminal"`
	}
	var states []stateRow
	if err := s.db.NewSelect().
		Table("meta.lifecycle_states").
		Column("id", "key", "is_terminal").
		Where("lifecycle_id = ?", *a.LifecycleID).
		Scan(r.Context(), &states); err != nil {
		return false
	}
	var stockID *int64
	for i := range states {
		if states[i].Key == "in_stock" {
			id := states[i].ID
			stockID = &id
		}
		// don't drag an asset out of a terminal state on return
		if a.CurrentStateID != nil && states[i].ID == *a.CurrentStateID && states[i].IsTerminal {
			return false
		}
	}
	if stockID == nil {
		return false
	}
	if a.CurrentStateID != nil && *a.CurrentStateID == *stockID {
		return false
	}

	from := a.CurrentStateID
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("current_state_id = ?", *stockID).
		Set("updated_at = now()").Where("id = ?", a.ID).Exec(r.Context()); err != nil {
		return false
	}
	note := "Returned to stock"
	if reason != "" {
		note = note + ": " + reason
	}
	hist := &domain.LifecycleHistory{
		AssetID: a.ID, FromStateID: from, ToStateID: *stockID,
		Actor: actor, Note: note, Data: map[string]any{"via": "return"},
	}
	_, _ = s.db.NewInsert().Model(hist).Exec(r.Context())
	return true
}

func (s *Server) locationName(ctx context.Context, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	loc := new(domain.Location)
	if err := s.db.NewSelect().Model(loc).Column("name").Where("id = ?", *id).Scan(ctx); err != nil {
		return ""
	}
	return loc.Name
}

func (s *Server) holderSummary(r *http.Request, in assignInput) string {
	if in.HolderPersonID != nil {
		pr := new(domain.Person)
		if err := s.db.NewSelect().Model(pr).Column("first_name", "last_name").
			Where("id = ?", *in.HolderPersonID).Scan(r.Context()); err == nil {
			return "Assigned to " + pr.FirstName + " " + pr.LastName
		}
	}
	if in.HolderOrgUnitID != nil {
		ou := new(domain.OrgUnit)
		if err := s.db.NewSelect().Model(ou).Column("name").
			Where("id = ?", *in.HolderOrgUnitID).Scan(r.Context()); err == nil {
			return "Assigned to " + ou.Name
		}
	}
	return "Assigned"
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (s *Server) closeOpenAssignments(r *http.Request, assetID uuid.UUID) {
	_, _ = s.db.NewUpdate().Model((*domain.Assignment)(nil)).
		Set("returned_at = now()").
		Where("asset_id = ?", assetID).
		Where("returned_at IS NULL").
		Where("kind = ?", "assign").
		Exec(r.Context())
}

func ptrTime(t time.Time) *time.Time { return &t }

func (s *Server) defaultLocationForPerson(ctx context.Context, personID uuid.UUID) *uuid.UUID {
	p := new(domain.Person)
	if err := s.db.NewSelect().Model(p).Column("org_unit_id").Where("id = ?", personID).Scan(ctx); err != nil || p.OrgUnitID == nil {
		return nil
	}
	return s.defaultLocationForOrgUnit(ctx, *p.OrgUnitID)
}

func (s *Server) defaultLocationForOrgUnit(ctx context.Context, orgUnitID uuid.UUID) *uuid.UUID {
	ou := new(domain.OrgUnit)
	if err := s.db.NewSelect().Model(ou).Column("default_location_id").Where("id = ?", orgUnitID).Scan(ctx); err != nil {
		return nil
	}
	return ou.DefaultLocationID
}

// assignToInUse moves a checked-out asset into in_use when it is still in_stock
// or the lifecycle initial state (mirrors discovery promotion on manual assign).
func (s *Server) assignToInUse(ctx context.Context, a *domain.Asset, actor *uuid.UUID, reason string) bool {
	if a.LifecycleID == nil {
		return false
	}
	target := s.discoveredState(ctx, a.LifecycleID, "in_use")
	if target == nil {
		return false
	}
	if a.CurrentStateID != nil && *a.CurrentStateID == target.ID {
		return false
	}
	type stateRow struct {
		ID         int64  `bun:"id"`
		Key        string `bun:"key"`
		IsInitial  bool   `bun:"is_initial"`
		IsTerminal bool   `bun:"is_terminal"`
	}
	var states []stateRow
	if err := s.db.NewSelect().Table("meta.lifecycle_states").
		Column("id", "key", "is_initial", "is_terminal").
		Where("lifecycle_id = ?", *a.LifecycleID).Scan(ctx, &states); err != nil {
		return false
	}
	if a.CurrentStateID != nil {
		for _, st := range states {
			if st.ID != *a.CurrentStateID {
				continue
			}
			if st.IsTerminal {
				return false
			}
			if st.Key != "in_stock" && !st.IsInitial {
				return false
			}
		}
	}
	from := a.CurrentStateID
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("current_state_id = ?", target.ID).
		Set("updated_at = now()").Where("id = ?", a.ID).Exec(ctx); err != nil {
		return false
	}
	note := "Assigned — moved to in use"
	if reason != "" {
		note = note + ": " + reason
	}
	_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
		AssetID: a.ID, FromStateID: from, ToStateID: target.ID,
		Actor: actor, Note: note, Data: map[string]any{"via": "assign"},
	}).Exec(ctx)
	return true
}

func (s *Server) handleAcknowledgeCustody(w http.ResponseWriter, r *http.Request) {
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
	p := s.principal(r)
	if !s.rbac.CanOn(r.Context(), p, "asset.assign", s.assetTarget(r.Context(), a)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}
	res, err := s.db.NewUpdate().Model((*domain.Assignment)(nil)).
		Set("acknowledged_at = now()").
		Set("acknowledged_by = ?", p.UserID).
		Where("asset_id = ?", id).
		Where("kind = ?", "assign").
		Where("returned_at IS NULL").
		Where("acknowledged_at IS NULL").
		Exec(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusBadRequest, "no open unacknowledged assignment")
		return
	}
	s.recordAssetEvent(r.Context(), id, &p.UserID, assetEvent{
		Kind: "custody_ack", Subject: events.SubjectAssetAssigned, Summary: "Custody acknowledged",
	})
	full, _ := s.loadAsset(r.Context(), id)
	writeJSON(w, http.StatusOK, full)
}
