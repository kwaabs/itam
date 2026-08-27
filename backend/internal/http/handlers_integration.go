package http

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"itam/internal/domain"
	"itam/internal/events"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- connector CRUD ------------------------------------------------------

func (s *Server) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	items := []domain.Connector{}
	if err := s.db.NewSelect().Model(&items).Order("cn.name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetConnector(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	c := new(domain.Connector)
	if err := s.db.NewSelect().Model(c).Where("cn.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "connector not found")
		return
	}
	runs := []domain.SyncRun{}
	_ = s.db.NewSelect().Model(&runs).Where("sr.connector_id = ?", id).
		Order("sr.started_at DESC").Limit(20).Scan(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"connector":   c,
		"runs":        runs,
		"has_secret":  c.Secret != "",
		"ingest_path": "/ingest/" + c.Key,
	})
}

func newSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleCreateConnector(w http.ResponseWriter, r *http.Request) {
	var body struct {
		domain.Connector
		PullSecret string `json:"pull_secret"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in := body.Connector
	in.PullSecret = body.PullSecret
	key, err := ltreeKey(in.Key)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "key: "+err.Error())
		return
	}
	in.Key = key
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if in.Kind == "" {
		in.Kind = "generic"
	}
	if in.Direction == "" {
		in.Direction = "inbound"
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	in.Secret = newSecret()
	in.Enabled = true
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Return the token exactly once, on creation.
	writeJSON(w, http.StatusCreated, map[string]any{
		"connector":    in,
		"ingest_token": in.Secret,
		"ingest_path":  "/ingest/" + in.Key,
	})
}

func (s *Server) handleUpdateConnector(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		domain.Connector
		PullSecret string `json:"pull_secret"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in := body.Connector
	in.ID = id
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	q := s.db.NewUpdate().Model(&in).
		Column("name", "kind", "enabled", "direction", "schedule", "config").
		Set("updated_at = now()").Where("id = ?", id)
	// Only overwrite the OAuth secret when a new one is supplied.
	if body.PullSecret != "" {
		q = q.Set("pull_secret = ?", body.PullSecret)
	}
	if _, err := q.Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleRotateConnectorSecret(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	secret := newSecret()
	if _, err := s.db.NewUpdate().Model((*domain.Connector)(nil)).
		Set("secret = ?", secret).Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ingest_token": secret})
}

func (s *Server) handleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Connector)(nil))
}

// ---- inbound ingest (token-authenticated, no JWT) ------------------------

// ingestDevice is the normalized record an external source pushes. Connectors
// can map their native shape to this in their own client, or send it directly.
type ingestDevice struct {
	ExternalID    string           `json:"external_id"`
	AssetTag      string           `json:"asset_tag"`
	Serial        string           `json:"serial"`
	Name          string           `json:"name"`
	AssetType     string           `json:"asset_type"`     // type key; falls back to connector default
	Location      string           `json:"location"`       // location key (optional)
	LastSeen      string           `json:"last_seen"`      // RFC3339 (optional)
	AssignedEmail string           `json:"assigned_email"` // primary user; links the asset to a person
	AssignedName  string           `json:"assigned_name"`  // primary user's display name (for auto-create)
	State         string           `json:"state"`          // lifecycle state key (optional, discovery only)
	Attributes    map[string]any   `json:"attributes"`
	Software      []ingestSoftware `json:"software"`
}

type ingestSoftware struct {
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
	Version   string `json:"version"`
}

type ingestPayload struct {
	Devices      []ingestDevice   `json:"devices"`
	Records      []map[string]any `json:"records"`
	SourceObject string           `json:"source_object"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	c := new(domain.Connector)
	if err := s.db.NewSelect().Model(c).Where("key = ?", key).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "unknown connector")
		return
	}
	if !c.Enabled {
		writeErr(w, http.StatusForbidden, "connector is disabled")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 25<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read body")
		return
	}
	if !verifyIngestAuth(c, raw, r) {
		writeErr(w, http.StatusUnauthorized, "invalid or missing ingest token/signature")
		return
	}
	var p ingestPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		// Allow a bare array of devices too.
		if err2 := json.Unmarshal(raw, &p.Devices); err2 != nil {
			writeErr(w, http.StatusBadRequest, "body must be {\"devices\":[...]}, {\"records\":[...]} or a JSON array")
			return
		}
	}

	// Raw records flow through the metadata mapping layer; normalized devices
	// go straight to the reconciler (backward compatible).
	if len(p.Records) > 0 {
		so := firstNonEmpty(p.SourceObject, r.URL.Query().Get("source_object"))
		items := make([]rawItem, 0, len(p.Records))
		for _, rec := range p.Records {
			items = append(items, rawItem{SourceObject: so, Payload: rec})
		}
		run := s.runRawItems(r.Context(), c, items, "push")
		writeJSON(w, http.StatusOK, run)
		return
	}
	if len(p.Devices) == 0 {
		writeErr(w, http.StatusBadRequest, "no devices or records in payload")
		return
	}
	run := s.runIngest(r.Context(), c, p.Devices, "push")
	writeJSON(w, http.StatusOK, run)
}

func verifyIngestAuth(c *domain.Connector, body []byte, r *http.Request) bool {
	if c.Secret == "" {
		return false
	}
	if sig := r.Header.Get("X-ITAM-Signature"); sig != "" {
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(sig), []byte(want))
	}
	if tok := r.Header.Get("X-ITAM-Token"); tok != "" {
		return subtle.ConstantTimeCompare([]byte(tok), []byte(c.Secret)) == 1
	}
	return false
}

// ---- reconciliation ------------------------------------------------------

func (s *Server) runIngest(ctx context.Context, c *domain.Connector, devices []ingestDevice, mode string) *domain.SyncRun {
	run := &domain.SyncRun{ConnectorID: c.ID, Mode: mode, Status: "running", Seen: len(devices), Detail: map[string]any{}}
	_, _ = s.db.NewInsert().Model(run).Returning("*").Exec(ctx)

	var sampleErrors []string
	for _, dev := range devices {
		action, _, err := s.reconcileDevice(ctx, c, dev)
		if err != nil {
			run.Errors++
			if len(sampleErrors) < 20 {
				sampleErrors = append(sampleErrors, ident(dev)+": "+err.Error())
			}
			continue
		}
		switch action {
		case "created":
			run.Created++
		case "updated":
			run.Updated++
		default:
			run.Skipped++
		}
	}

	run.Status = "success"
	if run.Errors > 0 {
		run.Status = "partial"
		if run.Created == 0 && run.Updated == 0 {
			run.Status = "error"
		}
	}
	now := time.Now()
	run.FinishedAt = &now
	if len(sampleErrors) > 0 {
		run.Detail = map[string]any{"errors": sampleErrors}
	}
	_, _ = s.db.NewUpdate().Model(run).
		Column("status", "created", "updated", "skipped", "errors", "detail", "finished_at").
		Where("id = ?", run.ID).Exec(ctx)
	_, _ = s.db.NewUpdate().Model((*domain.Connector)(nil)).
		Set("last_run_at = ?", now).Set("last_status = ?", run.Status).
		Where("id = ?", c.ID).Exec(ctx)
	return run
}

func ident(d ingestDevice) string {
	return firstNonEmpty(firstNonEmpty(d.ExternalID, d.Serial), d.AssetTag)
}

func (s *Server) reconcileDevice(ctx context.Context, c *domain.Connector, dev ingestDevice) (string, uuid.UUID, error) {
	ext := ident(dev)
	if ext == "" {
		return "", uuid.Nil, errInline("device has no external_id, serial or asset_tag")
	}

	seen := time.Now()
	if dev.LastSeen != "" {
		if t, err := time.Parse(time.RFC3339, dev.LastSeen); err == nil {
			seen = t
		}
	}

	locID := s.resolveIngestLocation(ctx, c, dev)

	// Who the device is attributed to. Resolved from the mapped primary-user
	// email/UPN against core.people (optionally auto-created with the display name).
	personID := s.resolveIngestPerson(ctx, c, dev.AssignedEmail, dev.AssignedName)
	// Lifecycle state for *newly discovered* assets. Discovery skips the manual
	// procure→receive→deploy flow, so these are live and default to "in use"
	// rather than the lifecycle's procurement initial state.
	stateKey := firstNonEmpty(dev.State, str(c.Config["discovered_state"]))

	// 1) Find an existing asset: identity -> serial -> asset_tag -> ip -> name.
	match := s.findAssetForDevice(ctx, c.Key, ext, dev)
	assetID := match.AssetID
	found := match.AssetID != uuid.Nil
	dev.Attributes = setIngestMatchMeta(dev.Attributes, match, !found)

	if found {
		if err := s.updateIngestedAsset(ctx, assetID, c, seen, locID, dev.Attributes); err != nil {
			return "", uuid.Nil, err
		}
		// A device that is live in the source but still sitting in the lifecycle's
		// procurement initial state was mis-seeded by an earlier sync; advance it.
		s.promoteDiscoveredState(ctx, assetID, stateKey)
	} else {
		typeKey := firstNonEmpty(dev.AssetType, str(c.Config["default_asset_type"]))
		if typeKey == "" {
			return "", uuid.Nil, errInline("no asset_type (set one on the device or a default_asset_type on the connector)")
		}
		at, e := s.assetTypeByKey(ctx, typeKey)
		if e != nil {
			return "", uuid.Nil, errInline("unknown asset_type: " + typeKey)
		}
		if at.IsAbstract {
			return "", uuid.Nil, errInline("asset_type is abstract: " + typeKey)
		}
		tag := firstNonEmpty(firstNonEmpty(dev.AssetTag, dev.Serial), ext)
		id, e := s.createIngestedAsset(ctx, at, tag, dev.Name, dev.Serial, locID, dev.Attributes, seen, c.Kind, stateKey)
		if e != nil {
			return "", uuid.Nil, e
		}
		assetID = id
	}

	// 2) Upsert the external identity mapping (includes match metadata).
	idAttrs, _ := json.Marshal(map[string]any{
		"match_method": match.Method, "match_confidence": match.Confidence,
	})
	if !found {
		idAttrs, _ = json.Marshal(map[string]any{"match_method": "new", "match_confidence": "high"})
	}
	if _, err := s.db.NewRaw(
		`INSERT INTO core.asset_identities (asset_id, source, external_id, attributes, last_seen_at)
		 VALUES (?, ?, ?, ?::jsonb, ?)
		 ON CONFLICT (source, external_id)
		 DO UPDATE SET asset_id = EXCLUDED.asset_id, attributes = EXCLUDED.attributes, last_seen_at = EXCLUDED.last_seen_at`,
		assetID, c.Key, ext, string(idAttrs), seen,
	).Exec(ctx); err != nil {
		return "", uuid.Nil, err
	}

	// 2b) Attribute the asset to its primary user (best-effort, only when changed).
	s.applyDiscoveredAssignee(ctx, assetID, personID)
	s.applyDiscoveredOwnerOrg(ctx, assetID, personID)

	// 3) Reconcile detected software (best-effort).
	for _, sw := range dev.Software {
		if strings.TrimSpace(sw.Name) == "" {
			continue
		}
		s.upsertInstallation(ctx, assetID, sw, c.Kind)
	}

	if found {
		return "updated", assetID, nil
	}
	return "created", assetID, nil
}

type ingestMatch struct {
	AssetID    uuid.UUID
	Method     string
	Confidence string
}

func (s *Server) findAssetForDevice(ctx context.Context, source, ext string, dev ingestDevice) ingestMatch {
	none := ingestMatch{}
	var id uuid.UUID
	if err := s.db.NewRaw(
		"SELECT asset_id FROM core.asset_identities WHERE source = ? AND external_id = ?", source, ext,
	).Scan(ctx, &id); err == nil && id != uuid.Nil {
		return ingestMatch{AssetID: id, Method: "identity", Confidence: "high"}
	}
	if dev.Serial != "" {
		if err := s.db.NewRaw(
			"SELECT id FROM core.assets WHERE serial = ? AND deleted_at IS NULL LIMIT 1", dev.Serial,
		).Scan(ctx, &id); err == nil && id != uuid.Nil {
			return ingestMatch{AssetID: id, Method: "serial", Confidence: "high"}
		}
	}
	if dev.AssetTag != "" {
		if err := s.db.NewRaw(
			"SELECT id FROM core.assets WHERE asset_tag = ? AND deleted_at IS NULL LIMIT 1", dev.AssetTag,
		).Scan(ctx, &id); err == nil && id != uuid.Nil {
			return ingestMatch{AssetID: id, Method: "asset_tag", Confidence: "high"}
		}
	}
	if ip := ingestIP(dev); ip != "" {
		if err := s.db.NewRaw(
			`SELECT id FROM core.assets
			  WHERE deleted_at IS NULL
			    AND (
			      attributes->>'ip_address' = ? OR attributes->>'ipaddress' = ?
			      OR attributes->>'ip' = ? OR attributes->>'primary_ip' = ?
			    )
			  LIMIT 1`, ip, ip, ip, ip,
		).Scan(ctx, &id); err == nil && id != uuid.Nil {
			return ingestMatch{AssetID: id, Method: "ip", Confidence: "medium"}
		}
	}
	if name := strings.TrimSpace(dev.Name); name != "" {
		var ids []uuid.UUID
		if err := s.db.NewRaw(
			"SELECT id FROM core.assets WHERE deleted_at IS NULL AND lower(name) = lower(?) LIMIT 2", name,
		).Scan(ctx, &ids); err == nil && len(ids) == 1 {
			return ingestMatch{AssetID: ids[0], Method: "name", Confidence: "low"}
		}
	}
	return none
}

func setIngestMatchMeta(attrs map[string]any, match ingestMatch, created bool) map[string]any {
	if attrs == nil {
		attrs = map[string]any{}
	}
	if created || match.Method != "" {
		method := match.Method
		conf := match.Confidence
		if created {
			method = "new"
			conf = "high"
		}
		attrs["ingest_match_method"] = method
		attrs["ingest_match_confidence"] = conf
	}
	return attrs
}

func ingestIP(dev ingestDevice) string {
	for _, k := range []string{"ip_address", "ipaddress", "ip", "primary_ip"} {
		if v := strings.TrimSpace(str(dev.Attributes[k])); v != "" {
			return v
		}
	}
	return ""
}

func (s *Server) resolveIngestLocation(ctx context.Context, c *domain.Connector, dev ingestDevice) *uuid.UUID {
	keys := []string{dev.Location}
	if dev.Attributes != nil {
		keys = append(keys,
			str(dev.Attributes["network_map"]),
			str(dev.Attributes["probe"]),
			str(dev.Attributes["mapName"]),
		)
	}
	for _, k := range keys {
		if id := s.resolveLocationBestEffort(ctx, strings.TrimSpace(k)); id != nil {
			return id
		}
	}
	return nil
}

func connectorUpdateLocation(c *domain.Connector) bool {
	if c == nil || c.Config == nil {
		return true
	}
	if v, ok := c.Config["update_location_on_sync"].(bool); ok {
		return v
	}
	return true
}

func (s *Server) updateIngestedAsset(ctx context.Context, id uuid.UUID, c *domain.Connector, seen time.Time, locID *uuid.UUID, attrs map[string]any) error {
	q := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("last_seen_at = ?", seen).
		Set("last_seen_source = ?", c.Kind).
		Set("updated_at = now()")
	if locID != nil {
		q = q.Set("last_seen_location_id = ?", *locID)
		if connectorUpdateLocation(c) {
			q = q.Set("location_id = ?", *locID)
		}
	}
	if len(attrs) > 0 {
		b, _ := json.Marshal(attrs)
		q = q.Set("attributes = attributes || ?::jsonb", string(b))
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

// createIngestedAsset creates an asset from a discovery source without an actor.
// Discovered assets are already live in the field, so they enter the lifecycle in
// the "in use" state (or the connector's discovered_state / a per-record state),
// not the procurement initial state used by the manual receive→deploy flow.
func (s *Server) createIngestedAsset(ctx context.Context, at *domain.AssetType, tag, name, serial string,
	locID *uuid.UUID, attrs map[string]any, seen time.Time, source, stateKey string) (uuid.UUID, error) {

	lifecycleID, initial, err := s.meta.ResolveLifecycle(ctx, at.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if attrs == nil {
		attrs = map[string]any{}
	}
	if name == "" {
		name = tag
	}
	// Prefer a real "live" state for discoveries; fall back to the initial state.
	state := initial
	if ds := s.discoveredState(ctx, lifecycleID, stateKey); ds != nil {
		state = ds
	}
	a := &domain.Asset{
		AssetTag: tag, Serial: serial, Name: name, AssetTypeID: at.ID,
		LifecycleID: lifecycleID, LocationID: locID, Attributes: attrs,
		LastSeenAt: &seen, LastSeenLocationID: locID, LastSeenSource: source,
	}
	if state != nil {
		a.CurrentStateID = &state.ID
	}
	if _, err := s.db.NewInsert().Model(a).Returning("*").Exec(ctx); err != nil {
		return uuid.Nil, err
	}
	if state != nil {
		_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
			AssetID: a.ID, ToStateID: state.ID, Note: "discovered via " + source,
		}).Exec(ctx)
	}
	s.recordAssetEvent(ctx, a.ID, nil, assetEvent{
		Kind: "created", Subject: events.SubjectAssetCreated, Summary: "Asset discovered (" + source + ")",
		Data: map[string]any{"asset_tag": a.AssetTag, "name": a.Name, "source": source},
	})
	s.emit(ctx, events.SubjectAssetCreated, "asset", a.ID.String(), "system",
		map[string]any{"asset_tag": a.AssetTag, "asset_type_id": a.AssetTypeID, "source": source})
	return a.ID, nil
}

// discoveredState picks the lifecycle state a freshly discovered asset should
// enter. It tries the requested key first, then "in_use", then "deployed", so a
// device that is already active in the field never lands in "procured". Returns
// nil when the lifecycle has none of these (caller falls back to the initial).
func (s *Server) discoveredState(ctx context.Context, lifecycleID *int64, want string) *domain.LifecycleState {
	if lifecycleID == nil {
		return nil
	}
	candidates := []string{}
	if w := strings.TrimSpace(want); w != "" {
		candidates = append(candidates, w)
	}
	candidates = append(candidates, "in_use", "deployed")
	for _, k := range candidates {
		st := new(domain.LifecycleState)
		if err := s.db.NewSelect().Model(st).
			Where("lifecycle_id = ?", *lifecycleID).Where("key = ?", k).
			Limit(1).Scan(ctx); err == nil && st.ID != 0 {
			return st
		}
	}
	return nil
}

// promoteDiscoveredState advances an asset out of the lifecycle's *initial*
// (procurement) state into the live discovered state, but only while it is still
// in that initial state — so a human who has deliberately set another state is
// never overridden. This lets a re-sync correct assets created before discovery
// used a sensible default state.
func (s *Server) promoteDiscoveredState(ctx context.Context, assetID uuid.UUID, stateKey string) {
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).
		Column("id", "lifecycle_id", "current_state_id").
		Where("a.id = ?", assetID).Scan(ctx); err != nil || a.LifecycleID == nil {
		return
	}
	initial := new(domain.LifecycleState)
	if err := s.db.NewSelect().Model(initial).
		Where("lifecycle_id = ?", *a.LifecycleID).Where("is_initial = ?", true).
		Order("sort ASC").Limit(1).Scan(ctx); err != nil {
		return
	}
	if a.CurrentStateID == nil || *a.CurrentStateID != initial.ID {
		return // human (or a prior promotion) already moved it; leave alone
	}
	target := s.discoveredState(ctx, a.LifecycleID, stateKey)
	if target == nil || target.ID == initial.ID {
		return
	}
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("current_state_id = ?", target.ID).Set("updated_at = now()").
		Where("id = ?", assetID).Exec(ctx); err != nil {
		return
	}
	_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
		AssetID: assetID, FromStateID: &initial.ID, ToStateID: target.ID,
		Note: "auto-advanced from discovery",
	}).Exec(ctx)
	s.recordAssetEvent(ctx, assetID, nil, assetEvent{
		Kind: "state_changed", Subject: events.SubjectAssetStateChanged,
		Summary: "Advanced to " + target.Label + " (discovered live)",
		Data:    map[string]any{"from_state": initial.Label, "to_state": target.Label, "source": "discovery"},
	})
}

// resolveIngestPerson maps a primary-user email/UPN to a person. It matches an
// existing person by email (case-insensitive) and, when the connector sets
// auto_create_people, creates a person so devices can be attributed even before
// the directory has been synced — using the display name carried on the device
// record when available, falling back to the email local part. Returns nil when
// nothing resolves.
func (s *Server) resolveIngestPerson(ctx context.Context, c *domain.Connector, email, displayName string) *uuid.UUID {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	p := new(domain.Person)
	if err := s.db.NewSelect().Model(p).Where("lower(email) = lower(?)", email).Limit(1).Scan(ctx); err == nil && p.ID != uuid.Nil {
		return &p.ID
	}
	auto, _ := c.Config["auto_create_people"].(bool)
	if !auto {
		return nil
	}
	first, last := splitName(displayName)
	if first == "" {
		first, last = nameFromEmail(email)
	}
	np := &domain.Person{Email: email, FirstName: first, LastName: last, IsActive: true}
	if _, err := s.db.NewInsert().Model(np).Returning("*").Exec(ctx); err != nil {
		return nil
	}
	return &np.ID
}

// splitName turns a display name ("Lucy Pearl Arthur") into first/last
// ("Lucy","Arthur"): first token is the given name, last token the surname.
func splitName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], "?"
	default:
		return parts[0], parts[len(parts)-1]
	}
}

// nameFromEmail derives a rough first/last name from an email local part
// ("first.last@x" -> "First","Last") so auto-created people are not blank.
func nameFromEmail(email string) (string, string) {
	local := email
	if i := strings.IndexByte(email, '@'); i > 0 {
		local = email[:i]
	}
	local = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(local)
	parts := strings.Fields(local)
	title := func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
	switch len(parts) {
	case 0:
		return "?", "?"
	case 1:
		return title(parts[0]), "?"
	default:
		return title(parts[0]), title(parts[len(parts)-1])
	}
}

// applyDiscoveredAssignee attributes an asset to its primary user, but only when
// the assignee actually changes, so repeated syncs do not spam the event log or
// clobber a manual assignment with the same value.
func (s *Server) applyDiscoveredAssignee(ctx context.Context, assetID uuid.UUID, personID *uuid.UUID) {
	if personID == nil {
		return
	}
	var cur uuid.UUID
	_ = s.db.NewSelect().Model((*domain.Asset)(nil)).Column("assigned_person_id").
		Where("id = ?", assetID).Scan(ctx, &cur)
	if cur == *personID {
		return
	}
	if _, err := s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("assigned_person_id = ?", *personID).Set("updated_at = now()").
		Where("id = ?", assetID).Exec(ctx); err != nil {
		return
	}
	s.recordAssetEvent(ctx, assetID, nil, assetEvent{
		Kind: "assigned", Subject: events.SubjectAssetAssigned,
		Summary: "Primary user attributed from discovery",
		Data:    map[string]any{"holder_person_id": personID.String()},
	})
}

// applyDiscoveredOwnerOrg sets owner_org_unit from the assignee's org when empty.
func (s *Server) applyDiscoveredOwnerOrg(ctx context.Context, assetID uuid.UUID, personID *uuid.UUID) {
	if personID == nil {
		return
	}
	p := new(domain.Person)
	if err := s.db.NewSelect().Model(p).Column("org_unit_id").Where("id = ?", *personID).Scan(ctx); err != nil || p.OrgUnitID == nil {
		return
	}
	var owner *uuid.UUID
	_ = s.db.NewSelect().Model((*domain.Asset)(nil)).Column("owner_org_unit_id").
		Where("id = ?", assetID).Scan(ctx, &owner)
	if owner != nil {
		return
	}
	_, _ = s.db.NewUpdate().Model((*domain.Asset)(nil)).
		Set("owner_org_unit_id = ?", *p.OrgUnitID).Set("updated_at = now()").
		Where("id = ?", assetID).Exec(ctx)
}

func (s *Server) upsertInstallation(ctx context.Context, assetID uuid.UUID, sw ingestSoftware, source string) {
	var swID uuid.UUID
	if err := s.db.NewRaw(
		"SELECT id FROM swm.software WHERE lower(name) = lower(?) LIMIT 1", sw.Name,
	).Scan(ctx, &swID); err != nil || swID == uuid.Nil {
		nsw := &domain.Software{Name: sw.Name, Publisher: sw.Publisher, Category: "application", Attributes: map[string]any{}}
		if _, e := s.db.NewInsert().Model(nsw).Returning("*").Exec(ctx); e != nil {
			return
		}
		swID = nsw.ID
	}
	_, _ = s.db.NewRaw(
		`INSERT INTO swm.installations (software_id, asset_id, source)
		 VALUES (?, ?, ?) ON CONFLICT (software_id, asset_id) DO NOTHING`,
		swID, assetID, source,
	).Exec(ctx)
}

// ---- run history ---------------------------------------------------------

func (s *Server) handleListConnectorRuns(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	runs := []domain.SyncRun{}
	if err := s.db.NewSelect().Model(&runs).Where("sr.connector_id = ?", id).
		Order("sr.started_at DESC").Limit(50).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// ---- small helpers -------------------------------------------------------

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type inlineErr string

func (e inlineErr) Error() string { return string(e) }
func errInline(msg string) error  { return inlineErr(msg) }
