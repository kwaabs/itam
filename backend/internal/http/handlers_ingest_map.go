package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ===========================================================================
// Transform engine: dot-path selection + named transforms. This is the only
// "code" in the mapping path; field names, types, targets and filters all live
// in integration.mappings rows, so nothing source-specific is hardcoded.
// ===========================================================================

// dotGet walks a nested JSON map by a dot-path. Numeric segments index arrays,
// e.g. "hardwareInformation.disks.0.serial".
func dotGet(m map[string]any, path string) any {
	if path == "" || m == nil {
		return nil
	}
	var cur any = m
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil
			}
			cur = node[idx]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

func anyToStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

var dateLayouts = []string{
	time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z0700",
	"2006-01-02 15:04:05", "2006-01-02", "01/02/2006", "02/01/2006",
	"2006/01/02", time.RFC1123Z, time.RFC1123,
	"2 Jan 2006 03:04:05 PM MST", // ManageEngine OpManager addedTime
}

func parseDateFlexible(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

// applyTransform runs a named transform. Unknown names pass the value through.
func applyTransform(name, arg string, v any) any {
	switch name {
	case "", "string":
		return v
	case "to_lower":
		return strings.ToLower(anyToStr(v))
	case "to_upper":
		return strings.ToUpper(anyToStr(v))
	case "trim":
		return strings.TrimSpace(anyToStr(v))
	case "to_int":
		n, _ := strconv.Atoi(strings.TrimSpace(anyToStr(v)))
		return n
	case "to_bool":
		b, _ := strconv.ParseBool(strings.TrimSpace(anyToStr(v)))
		return b
	case "default":
		if anyToStr(v) == "" {
			return arg
		}
		return v
	case "prefix":
		return arg + anyToStr(v)
	case "suffix":
		return anyToStr(v) + arg
	case "first":
		if arr, ok := v.([]any); ok && len(arr) > 0 {
			return arr[0]
		}
		return v
	case "split":
		parts := strings.Split(anyToStr(v), arg)
		if len(parts) > 0 {
			return parts[0]
		}
		return v
	case "parse_date":
		return parseDateFlexible(anyToStr(v))
	case "map":
		var m map[string]any
		if json.Unmarshal([]byte(arg), &m) == nil {
			if mv, ok := m[anyToStr(v)]; ok {
				return mv
			}
		}
		return v
	default:
		return v
	}
}

func evalFilters(conds []domain.FilterCond, payload map[string]any) bool {
	for _, c := range conds {
		v := dotGet(payload, c.Source)
		sv := anyToStr(v)
		want := anyToStr(c.Value)
		switch c.Op {
		case "exists":
			if sv == "" {
				return false
			}
		case "not_exists":
			if sv != "" {
				return false
			}
		case "eq":
			if sv != want {
				return false
			}
		case "ne":
			if sv == want {
				return false
			}
		case "contains":
			if !strings.Contains(strings.ToLower(sv), strings.ToLower(want)) {
				return false
			}
		case "in":
			ok := false
			if arr, isArr := c.Value.([]any); isArr {
				for _, a := range arr {
					if anyToStr(a) == sv {
						ok = true
						break
					}
				}
			}
			if !ok {
				return false
			}
		}
	}
	return true
}

func resolveType(tr map[string]any, payload map[string]any) string {
	def := str(tr["default"])
	if by := str(tr["by"]); by != "" {
		key := anyToStr(dotGet(payload, by))
		if m, ok := tr["map"].(map[string]any); ok {
			if mv, ok2 := m[key]; ok2 {
				return anyToStr(mv)
			}
		}
	}
	return def
}

// ===========================================================================
// Mapping application: raw record -> normalized entity
// ===========================================================================

func mapExternalID(mp domain.Mapping, payload map[string]any) string {
	if ext := anyToStr(dotGet(payload, mp.Identity)); ext != "" {
		return ext
	}
	for _, fb := range mp.MatchFallbacks {
		if v := anyToStr(dotGet(payload, fb)); v != "" {
			return v
		}
	}
	return ""
}

func mapToDevice(mp domain.Mapping, payload map[string]any) ingestDevice {
	dev := ingestDevice{Attributes: map[string]any{}}
	dev.ExternalID = mapExternalID(mp, payload)
	dev.AssetType = resolveType(mp.TypeResolution, payload)
	for _, f := range mp.Fields {
		val := applyTransform(f.Transform, f.Arg, dotGet(payload, f.Source))
		setDeviceTarget(&dev, f.Target, val)
	}
	return dev
}

func setDeviceTarget(dev *ingestDevice, target string, val any) {
	if strings.HasPrefix(target, "attr.") {
		if key := strings.TrimPrefix(target, "attr."); key != "" {
			dev.Attributes[key] = val
		}
		return
	}
	sv := anyToStr(val)
	switch target {
	case "name":
		dev.Name = sv
	case "serial":
		dev.Serial = sv
	case "asset_tag":
		dev.AssetTag = sv
	case "asset_type":
		if sv != "" {
			dev.AssetType = sv
		}
	case "location":
		dev.Location = sv
	case "last_seen":
		dev.LastSeen = sv
	case "assigned_email", "assigned_upn", "primary_user_email":
		if sv != "" {
			dev.AssignedEmail = sv
		}
	case "assigned_name", "primary_user_name":
		if sv != "" {
			dev.AssignedName = sv
		}
	case "state":
		dev.State = sv
	case "external_id":
		if sv != "" {
			dev.ExternalID = sv
		}
	default:
		dev.Attributes[target] = val
	}
}

func mapFields(mp domain.Mapping, payload map[string]any) map[string]string {
	out := map[string]string{}
	for _, f := range mp.Fields {
		out[f.Target] = anyToStr(applyTransform(f.Transform, f.Arg, dotGet(payload, f.Source)))
	}
	return out
}

// ===========================================================================
// Raw landing + routed reconciliation (reuses the 00020 reconciler/sync_runs)
// ===========================================================================

type rawItem struct {
	SourceObject string
	Payload      map[string]any
}

// guessID is a best-effort identifier when no mapping exists yet, so we can
// still land raw records for schema discovery.
func guessID(payload map[string]any) string {
	for _, k := range []string{"id", "Id", "ID", "objectId", "azureADDeviceId", "serialNumber", "deviceName", "name"} {
		if v := anyToStr(payload[k]); v != "" {
			return v
		}
	}
	return uuid.NewString()
}

func (s *Server) runRawItems(ctx context.Context, c *domain.Connector, items []rawItem, mode string) *domain.SyncRun {
	run := &domain.SyncRun{ConnectorID: c.ID, Mode: mode, Status: "running", Seen: len(items), Detail: map[string]any{}}
	_, _ = s.db.NewInsert().Model(run).Returning("*").Exec(ctx)

	// Preload mappings for this connector, keyed by source object.
	maps := map[string]domain.Mapping{}
	var mlist []domain.Mapping
	if err := s.db.NewSelect().Model(&mlist).Where("connector_id = ?", c.ID).Scan(ctx); err == nil {
		for _, m := range mlist {
			maps[m.SourceObject] = m
		}
	}

	var sampleErrors []string
	for _, it := range items {
		mp, hasMapping := maps[it.SourceObject]
		ext := guessID(it.Payload)
		if hasMapping {
			if e := mapExternalID(mp, it.Payload); e != "" {
				ext = e
			}
		}

		status := "unmapped"
		var assetID *uuid.UUID
		var rowErr string

		switch {
		case !hasMapping:
			status = "unmapped"
			run.Skipped++
		case !mp.Enabled:
			status = "skipped"
			run.Skipped++
		case !evalFilters(mp.Filters, it.Payload):
			status = "filtered"
			run.Skipped++
		default:
			action, id, err := s.reconcileMapped(ctx, c, mp, it.Payload)
			if err != nil {
				status = "error"
				rowErr = err.Error()
				run.Errors++
				if len(sampleErrors) < 20 {
					sampleErrors = append(sampleErrors, it.SourceObject+"/"+ext+": "+err.Error())
				}
			} else {
				status = action
				if id != uuid.Nil {
					assetID = &id
				}
				if action == "created" {
					run.Created++
				} else {
					run.Updated++
				}
			}
		}

		// Land/refresh the raw record for replay + discovery.
		_, _ = s.db.NewRaw(
			`INSERT INTO integration.raw_records
			   (connector_id, source_object, external_id, payload, sync_run_id, processed, status, error, asset_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (connector_id, source_object, external_id) DO UPDATE SET
			   payload = EXCLUDED.payload, sync_run_id = EXCLUDED.sync_run_id,
			   processed = EXCLUDED.processed, status = EXCLUDED.status,
			   error = EXCLUDED.error, asset_id = EXCLUDED.asset_id, received_at = now()`,
			c.ID, it.SourceObject, ext, mapToJSONB(it.Payload), run.ID,
			status != "unmapped" && status != "error", status, nullStr(rowErr), assetID,
		).Exec(ctx)
	}

	finalizeRun(run, sampleErrors)
	_, _ = s.db.NewUpdate().Model(run).
		Column("status", "created", "updated", "skipped", "errors", "detail", "finished_at").
		Where("id = ?", run.ID).Exec(ctx)
	now := time.Now()
	_, _ = s.db.NewUpdate().Model((*domain.Connector)(nil)).
		Set("last_run_at = ?", now).Set("last_status = ?", run.Status).
		Where("id = ?", c.ID).Exec(ctx)
	return run
}

// reconcileMapped routes a mapped record to the right reconciler by target entity.
func (s *Server) reconcileMapped(ctx context.Context, c *domain.Connector, mp domain.Mapping, payload map[string]any) (string, uuid.UUID, error) {
	switch mp.TargetEntity {
	case "", "asset":
		dev := mapToDevice(mp, payload)
		// Carry detected software if the mapping declared a software path.
		return s.reconcileDevice(ctx, c, dev)
	case "person":
		return s.reconcilePerson(ctx, mapFields(mp, payload))
	case "installation":
		return s.reconcileInstall(ctx, c, mapFields(mp, payload))
	default:
		return "", uuid.Nil, errInline("unknown target_entity: " + mp.TargetEntity)
	}
}

func (s *Server) reconcilePerson(ctx context.Context, f map[string]string) (string, uuid.UUID, error) {
	email := strings.TrimSpace(f["email"])
	empNo := strings.TrimSpace(f["employee_no"])
	if email == "" && empNo == "" {
		return "", uuid.Nil, errInline("person needs an email or employee_no")
	}
	var orgID *uuid.UUID
	if ou := strings.TrimSpace(f["org_unit"]); ou != "" {
		if id, err := s.resolveOrgKey(ctx, ou); err == nil {
			orgID = id
		}
	}
	// Find existing by email then employee_no.
	existing := new(domain.Person)
	q := s.db.NewSelect().Model(existing).Limit(1)
	if email != "" {
		q = q.Where("lower(email) = lower(?)", email)
	} else {
		q = q.Where("employee_no = ?", empNo)
	}
	if err := q.Scan(ctx); err == nil && existing.ID != uuid.Nil {
		upd := s.db.NewUpdate().Model((*domain.Person)(nil)).Where("id = ?", existing.ID).Set("updated_at = now()")
		if v := f["first_name"]; v != "" {
			upd = upd.Set("first_name = ?", v)
		}
		if v := f["last_name"]; v != "" {
			upd = upd.Set("last_name = ?", v)
		}
		if v := f["title"]; v != "" {
			upd = upd.Set("title = ?", v)
		}
		if orgID != nil {
			upd = upd.Set("org_unit_id = ?", *orgID)
		}
		if _, err := upd.Exec(ctx); err != nil {
			return "", uuid.Nil, err
		}
		return "updated", existing.ID, nil
	}
	p := &domain.Person{
		EmployeeNo: empNo, Email: email, Title: f["title"],
		FirstName: firstNonEmpty(f["first_name"], "?"), LastName: firstNonEmpty(f["last_name"], "?"),
		OrgUnitID: orgID, IsActive: true,
	}
	if _, err := s.db.NewInsert().Model(p).Returning("*").Exec(ctx); err != nil {
		return "", uuid.Nil, err
	}
	return "created", p.ID, nil
}

func (s *Server) reconcileInstall(ctx context.Context, c *domain.Connector, f map[string]string) (string, uuid.UUID, error) {
	ext := strings.TrimSpace(f["asset_external_id"])
	name := strings.TrimSpace(f["software"])
	if ext == "" || name == "" {
		return "", uuid.Nil, errInline("installation needs asset_external_id and software")
	}
	var assetID uuid.UUID
	if err := s.db.NewRaw(
		"SELECT asset_id FROM core.asset_identities WHERE source = ? AND external_id = ?", c.Key, ext,
	).Scan(ctx, &assetID); err != nil || assetID == uuid.Nil {
		return "", uuid.Nil, errInline("no asset for external_id " + ext + " (ingest the device first)")
	}
	s.upsertInstallation(ctx, assetID, ingestSoftware{Name: name, Publisher: f["publisher"], Version: f["version"]}, c.Kind)
	return "updated", assetID, nil
}

func finalizeRun(run *domain.SyncRun, sampleErrors []string) {
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
}

func mapToJSONB(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ===========================================================================
// Mapping CRUD
// ===========================================================================

func (s *Server) handleListMappings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	items := []domain.Mapping{}
	if err := s.db.NewSelect().Model(&items).Where("connector_id = ?", id).
		Order("sort ASC", "source_object ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateMapping(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Mapping
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ConnectorID = id
	if in.SourceObject == "" || in.Identity == "" {
		writeErr(w, http.StatusBadRequest, "source_object and identity are required")
		return
	}
	if in.TargetEntity == "" {
		in.TargetEntity = "asset"
	}
	normalizeMapping(&in)
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateMapping(w http.ResponseWriter, r *http.Request) {
	mid, err := uuid.Parse(chi.URLParam(r, "mid"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Mapping
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = mid
	if in.TargetEntity == "" {
		in.TargetEntity = "asset"
	}
	normalizeMapping(&in)
	if _, err := s.db.NewUpdate().Model(&in).
		Column("source_object", "target_entity", "enabled", "sort", "identity",
			"match_fallbacks", "type_resolution", "fields", "filters").
		Set("version = version + 1").Set("updated_at = now()").
		Where("id = ?", mid).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	mid, err := uuid.Parse(chi.URLParam(r, "mid"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.db.NewDelete().Model((*domain.Mapping)(nil)).Where("id = ?", mid).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// normalizeMapping guards against null jsonb slices/maps.
func normalizeMapping(m *domain.Mapping) {
	if m.MatchFallbacks == nil {
		m.MatchFallbacks = []string{}
	}
	if m.TypeResolution == nil {
		m.TypeResolution = map[string]any{}
	}
	if m.Fields == nil {
		m.Fields = []domain.FieldMap{}
	}
	if m.Filters == nil {
		m.Filters = []domain.FilterCond{}
	}
}

// ===========================================================================
// Reprocess (replay raw through current mappings) + field discovery
// ===========================================================================

func (s *Server) handleReprocess(w http.ResponseWriter, r *http.Request) {
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
	raws := []domain.RawRecord{}
	q := s.db.NewSelect().Model(&raws).Where("connector_id = ?", id)
	if so := r.URL.Query().Get("source_object"); so != "" {
		q = q.Where("source_object = ?", so)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]rawItem, 0, len(raws))
	for _, rr := range raws {
		items = append(items, rawItem{SourceObject: rr.SourceObject, Payload: rr.Payload})
	}
	run := s.runRawItems(r.Context(), c, items, "reprocess")
	writeJSON(w, http.StatusOK, run)
}

type discoveredField struct {
	Path   string `json:"path"`
	Sample string `json:"sample"`
	Mapped bool   `json:"mapped"`
}

func (s *Server) handleDiscoverFields(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	so := r.URL.Query().Get("source_object")
	raws := []domain.RawRecord{}
	q := s.db.NewSelect().Model(&raws).Where("connector_id = ?", id).
		Order("received_at DESC").Limit(50)
	if so != "" {
		q = q.Where("source_object = ?", so)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	samples := map[string]string{}
	for _, rr := range raws {
		flatten("", rr.Payload, samples)
	}

	// Which paths are already referenced by the mapping for this source object.
	used := map[string]bool{}
	if so != "" {
		mp := new(domain.Mapping)
		if err := s.db.NewSelect().Model(mp).
			Where("connector_id = ?", id).Where("source_object = ?", so).Scan(r.Context()); err == nil {
			used[mp.Identity] = true
			for _, fb := range mp.MatchFallbacks {
				used[fb] = true
			}
			for _, f := range mp.Fields {
				used[f.Source] = true
			}
		}
	}

	out := make([]discoveredField, 0, len(samples))
	for path, sample := range samples {
		if len(sample) > 120 {
			sample = sample[:120] + "…"
		}
		out = append(out, discoveredField{Path: path, Sample: sample, Mapped: used[path]})
	}
	writeJSON(w, http.StatusOK, out)
}

func flatten(prefix string, v any, out map[string]string) {
	switch node := v.(type) {
	case map[string]any:
		for k, val := range node {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(p, val, out)
		}
	case []any:
		if len(node) > 0 {
			flatten(prefix+".0", node[0], out)
		}
	default:
		if _, exists := out[prefix]; !exists && prefix != "" {
			out[prefix] = anyToStr(v)
		}
	}
}

// ===========================================================================
// Pull worker: OAuth2 client-credentials, API-key, and paged/bare-array fetch
// ===========================================================================

type pullAuth struct {
	bearer  string
	headers map[string]string
}

type cachedTok struct {
	token string
	exp   time.Time
}

var (
	tokMu    sync.Mutex
	tokCache = map[uuid.UUID]cachedTok{}
)

func pullAuthType(c *domain.Connector) string {
	auth, _ := c.Config["auth"].(map[string]any)
	if t := str(auth["type"]); t != "" {
		return t
	}
	if auth != nil && str(auth["token_endpoint"]) != "" {
		return "oauth2_client_credentials"
	}
	if auth != nil {
		return "api_key"
	}
	return ""
}

func (s *Server) resolvePullAuth(ctx context.Context, c *domain.Connector) (pullAuth, error) {
	auth, _ := c.Config["auth"].(map[string]any)
	if auth == nil {
		return pullAuth{}, errInline("connector has no config.auth")
	}
	switch pullAuthType(c) {
	case "api_key":
		if c.PullSecret == "" {
			return pullAuth{}, errInline("connector has no pull_secret (API key)")
		}
		in := strings.ToLower(str(auth["in"]))
		if in == "" {
			in = "query"
		}
		if in == "header" {
			param := str(auth["param"])
			if param == "" {
				param = "X-API-Key"
			}
			return pullAuth{headers: map[string]string{param: c.PullSecret}}, nil
		}
		return pullAuth{}, nil
	case "oauth2_client_credentials", "":
		token, err := s.getOAuthPullToken(ctx, c, auth)
		if err != nil {
			return pullAuth{}, err
		}
		return pullAuth{bearer: token}, nil
	default:
		return pullAuth{}, errInline("unknown auth type: " + pullAuthType(c))
	}
}

func buildPullURL(endpoint string, c *domain.Connector) string {
	if pullAuthType(c) != "api_key" {
		return endpoint
	}
	auth, _ := c.Config["auth"].(map[string]any)
	if strings.ToLower(str(auth["in"])) == "header" {
		return endpoint
	}
	param := str(auth["param"])
	if param == "" {
		param = "apiKey"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	q := u.Query()
	q.Set(param, c.PullSecret)
	u.RawQuery = q.Encode()
	return u.String()
}

func applyPullAuth(req *http.Request, pa pullAuth) {
	if pa.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+pa.bearer)
	}
	for k, v := range pa.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")
}

func decodeFetchRecords(body []byte) ([]map[string]any, error) {
	var page struct {
		Value    []map[string]any `json:"value"`
		NextLink string           `json:"@odata.nextLink"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	if page.Value == nil {
		var arr []map[string]any
		if json.Unmarshal(body, &arr) == nil && len(arr) > 0 {
			page.Value = arr
		} else {
			var one map[string]any
			if json.Unmarshal(body, &one) == nil && len(one) > 0 {
				page.Value = []map[string]any{one}
			}
		}
	}
	return page.Value, nil
}

// extractNextLink returns the next page URL from Graph (@odata.nextLink) or Azure ARM (nextLink).
func extractNextLink(body []byte) string {
	var page struct {
		ODataNext string `json:"@odata.nextLink"`
		ARMNext   string `json:"nextLink"`
	}
	if json.Unmarshal(body, &page) != nil {
		return ""
	}
	if page.ODataNext != "" {
		return page.ODataNext
	}
	return page.ARMNext
}

func (s *Server) getOAuthPullToken(ctx context.Context, c *domain.Connector, auth map[string]any) (string, error) {
	tokMu.Lock()
	if t, ok := tokCache[c.ID]; ok && time.Now().Before(t.exp) {
		tokMu.Unlock()
		return t.token, nil
	}
	tokMu.Unlock()

	endpoint := str(auth["token_endpoint"])
	if endpoint == "" {
		return "", errInline("config.auth.token_endpoint is required")
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", str(auth["client_id"]))
	form.Set("client_secret", c.PullSecret)
	form.Set("scope", str(auth["scope"]))

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", errInline("token endpoint returned " + resp.Status + ": " + string(body))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", errInline("could not parse token response")
	}
	ttl := tr.ExpiresIn
	if ttl <= 0 {
		ttl = 3600
	}
	tokMu.Lock()
	tokCache[c.ID] = cachedTok{token: tr.AccessToken, exp: time.Now().Add(time.Duration(ttl-60) * time.Second)}
	tokMu.Unlock()
	return tr.AccessToken, nil
}

// fetchPaged GETs a remote object list. Supports OData paging (@odata.nextLink),
// Azure ARM paging (nextLink), bare JSON arrays (OpManager, etc.), and OAuth bearer or API-key auth.
func fetchPaged(ctx context.Context, pa pullAuth, c *domain.Connector, endpoint, sourceObject string) ([]rawItem, error) {
	var out []rawItem
	next := buildPullURL(endpoint, c)
	client := &http.Client{Timeout: 60 * time.Second}
	for pages := 0; next != "" && pages < 100; pages++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		applyPullAuth(req, pa)
		resp, err := client.Do(req)
		if err != nil {
			return out, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return out, errInline("fetch " + endpoint + " returned " + resp.Status)
		}
		records, err := decodeFetchRecords(body)
		if err != nil {
			return out, err
		}
		for _, rec := range records {
			out = append(out, rawItem{SourceObject: sourceObject, Payload: rec})
		}
		next = extractNextLink(body)
	}
	return out, nil
}

// runPull authenticates and fetches every configured object, then feeds the raw
// items through the same mapping/reconcile pipeline as push.
func (s *Server) runPull(ctx context.Context, c *domain.Connector) (*domain.SyncRun, error) {
	pa, err := s.resolvePullAuth(ctx, c)
	if err != nil {
		s.recordFailedRun(ctx, c, "pull", err)
		return nil, err
	}
	objs, _ := c.Config["objects"].([]any)
	if len(objs) == 0 {
		return nil, errInline("connector has no config.objects to pull")
	}
	var items []rawItem
	for _, o := range objs {
		om, _ := o.(map[string]any)
		so := str(om["source_object"])
		u := str(om["url"])
		if so == "" || u == "" {
			continue
		}
		part, err := fetchPaged(ctx, pa, c, u, so)
		if err != nil {
			s.recordFailedRun(ctx, c, "pull", err)
			return nil, err
		}
		items = append(items, part...)
	}
	return s.runRawItems(ctx, c, items, "pull"), nil
}

func (s *Server) recordFailedRun(ctx context.Context, c *domain.Connector, mode string, e error) {
	now := time.Now()
	run := &domain.SyncRun{
		ConnectorID: c.ID, Mode: mode, Status: "error", Message: e.Error(),
		FinishedAt: &now, Detail: map[string]any{},
	}
	_, _ = s.db.NewInsert().Model(run).Exec(ctx)
	_, _ = s.db.NewUpdate().Model((*domain.Connector)(nil)).
		Set("last_run_at = ?", now).Set("last_status = ?", "error").
		Where("id = ?", c.ID).Exec(ctx)
}

// handleRunConnector triggers a pull immediately (for pull connectors).
func (s *Server) handleRunConnector(w http.ResponseWriter, r *http.Request) {
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
	if _, ok := c.Config["objects"]; !ok {
		writeErr(w, http.StatusBadRequest, "this connector has no pull objects configured")
		return
	}
	run, err := s.runPull(r.Context(), c)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// handleTestConnector verifies a pull connector can authenticate and, when objects
// are configured, fetches a sample from the first object endpoint.
func (s *Server) handleTestConnector(w http.ResponseWriter, r *http.Request) {
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
	if _, ok := c.Config["auth"]; !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "stage": "config", "message": "connector has no config.auth"})
		return
	}
	pa, err := s.resolvePullAuth(r.Context(), c)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "stage": "auth", "message": err.Error()})
		return
	}
	out := map[string]any{"ok": true, "stage": "auth", "message": "authenticated successfully"}
	if pullAuthType(c) == "api_key" {
		out["message"] = "API key configured"
	}

	// Optional: probe the first object endpoint for one record.
	if objs, _ := c.Config["objects"].([]any); len(objs) > 0 {
		om, _ := objs[0].(map[string]any)
		u := str(om["url"])
		if u != "" {
			probe := buildPullURL(u, c)
			if pullAuthType(c) == "oauth2_client_credentials" &&
				strings.Contains(probe, "graph.microsoft.com") &&
				!strings.Contains(probe, "$top") {
				if strings.Contains(probe, "?") {
					probe += "&$top=1"
				} else {
					probe += "?$top=1"
				}
			}
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, probe, nil)
			applyPullAuth(req, pa)
			resp, ferr := (&http.Client{Timeout: 30 * time.Second}).Do(req)
			if ferr != nil {
				out["stage"] = "fetch"
				out["fetch_ok"] = false
				out["message"] = "auth ok, but fetch failed: " + ferr.Error()
			} else {
				defer resp.Body.Close()
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if resp.StatusCode != http.StatusOK {
					out["ok"] = false
					out["stage"] = "fetch"
					out["fetch_ok"] = false
					out["message"] = "auth ok, but " + str(om["source_object"]) + " returned " + resp.Status
				} else {
					records, _ := decodeFetchRecords(body)
					out["stage"] = "fetch"
					out["fetch_ok"] = true
					out["sample_count"] = len(records)
					out["source_object"] = str(om["source_object"])
					if len(records) > 0 {
						keys := make([]string, 0, len(records[0]))
						for k := range records[0] {
							keys = append(keys, k)
						}
						out["sample_fields"] = keys
					}
					out["message"] = "auth ok; fetched sample from " + str(om["source_object"])
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// StartPullLoop periodically runs due pull connectors based on
// config.schedule_seconds. Started from main.go alongside the event scheduler.
func (s *Server) StartPullLoop(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	s.runDuePulls(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.runDuePulls(ctx)
		}
	}
}

func (s *Server) runDuePulls(ctx context.Context) {
	conns := []domain.Connector{}
	if err := s.db.NewSelect().Model(&conns).
		Where("enabled = ?", true).Where("direction = ?", "pull").Scan(ctx); err != nil {
		return
	}
	for i := range conns {
		c := &conns[i]
		interval := intFromConfig(c.Config, "schedule_seconds")
		if interval <= 0 {
			continue
		}
		if c.LastRunAt != nil && time.Since(*c.LastRunAt) < time.Duration(interval)*time.Second {
			continue
		}
		s.log.Info("pull connector due", "connector", c.Key)
		if _, err := s.runPull(ctx, c); err != nil {
			s.log.Error("pull failed", "connector", c.Key, "err", err)
		}
	}
}

func intFromConfig(cfg map[string]any, key string) int {
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}
