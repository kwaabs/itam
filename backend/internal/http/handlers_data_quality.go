package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"itam/internal/auth"
	"itam/internal/domain"
	"itam/internal/events"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ---- data quality extensions -----------------------------------------------

type duplicateSerialGroup struct {
	Serial string                `json:"serial"`
	Assets []dataQualityAssetRow `json:"assets"`
}

type dataQualityFixInput struct {
	Action         string      `json:"action"`
	AssetIDs       []uuid.UUID `json:"asset_ids"`
	LocationID     *uuid.UUID  `json:"location_id"`
	OwnerOrgUnitID *uuid.UUID  `json:"owner_org_unit_id"`
}

type dataQualityFixResult struct {
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func (s *Server) handleReportDataQualityFix(w http.ResponseWriter, r *http.Request) {
	var in dataQualityFixInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	action := strings.TrimSpace(in.Action)
	if action == "" {
		writeErr(w, http.StatusBadRequest, "action is required")
		return
	}

	ctx := r.Context()
	p := s.principal(r)
	out := dataQualityFixResult{Errors: []string{}}

	switch action {
	case "apply_assignee_org", "apply_assignee_org_all":
		ids := in.AssetIDs
		if action == "apply_assignee_org_all" {
			ids = s.assetsMissingOrg(ctx)
		}
		for _, id := range ids {
			if ok, err := s.fixAssigneeOrg(ctx, p, id); err != nil {
				out.Errors = append(out.Errors, err.Error())
			} else if ok {
				out.Updated++
			} else {
				out.Skipped++
			}
		}
	case "apply_org_default_location", "apply_org_default_location_all":
		ids := in.AssetIDs
		if action == "apply_org_default_location_all" {
			ids = s.assetsMissingLocation(ctx)
		}
		for _, id := range ids {
			if ok, err := s.fixOrgDefaultLocation(ctx, p, id); err != nil {
				out.Errors = append(out.Errors, err.Error())
			} else if ok {
				out.Updated++
			} else {
				out.Skipped++
			}
		}
	case "set_location":
		if in.LocationID == nil {
			writeErr(w, http.StatusBadRequest, "location_id is required")
			return
		}
		for _, id := range in.AssetIDs {
			if ok, err := s.fixSetLocation(ctx, p, id, *in.LocationID); err != nil {
				out.Errors = append(out.Errors, err.Error())
			} else if ok {
				out.Updated++
			} else {
				out.Skipped++
			}
		}
	case "set_org_unit":
		if in.OwnerOrgUnitID == nil {
			writeErr(w, http.StatusBadRequest, "owner_org_unit_id is required")
			return
		}
		for _, id := range in.AssetIDs {
			if ok, err := s.fixSetOrgUnit(ctx, p, id, *in.OwnerOrgUnitID); err != nil {
				out.Errors = append(out.Errors, err.Error())
			} else if ok {
				out.Updated++
			} else {
				out.Skipped++
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "unknown action")
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) assetsMissingLocation(ctx context.Context) []uuid.UUID {
	var ids []uuid.UUID
	_ = s.db.NewRaw(`SELECT id FROM core.assets WHERE deleted_at IS NULL AND location_id IS NULL`).Scan(ctx, &ids)
	return ids
}

func (s *Server) assetsMissingOrg(ctx context.Context) []uuid.UUID {
	var ids []uuid.UUID
	_ = s.db.NewRaw(`SELECT id FROM core.assets WHERE deleted_at IS NULL AND owner_org_unit_id IS NULL`).Scan(ctx, &ids)
	return ids
}

func (s *Server) gapAssetIDs(ctx context.Context, issue string) []uuid.UUID {
	var ids []uuid.UUID
	_ = s.db.NewRaw(
		`SELECT id FROM core.assets
		  WHERE deleted_at IS NULL
		    AND CASE
		          WHEN location_id IS NULL AND owner_org_unit_id IS NULL THEN 'missing location and org unit'
		          WHEN location_id IS NULL THEN 'missing location'
		          WHEN owner_org_unit_id IS NULL THEN 'missing org unit'
		          ELSE 'missing serial'
		        END = ?`,
		issue,
	).Scan(ctx, &ids)
	return ids
}

func (s *Server) fixAssigneeOrg(ctx context.Context, p *auth.Principal, id uuid.UUID) (bool, error) {
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(ctx); err != nil {
		return false, fmt.Errorf("%s: not found", id)
	}
	if a.OwnerOrgUnitID != nil || a.AssignedPersonID == nil {
		return false, nil
	}
	per := new(domain.Person)
	if err := s.db.NewSelect().Model(per).Column("org_unit_id").Where("id = ?", *a.AssignedPersonID).Scan(ctx); err != nil || per.OrgUnitID == nil {
		return false, nil
	}
	if !s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, a)) {
		return false, fmt.Errorf("%s: forbidden", id)
	}
	if _, err := s.db.NewUpdate().Model(a).Set("owner_org_unit_id = ?", *per.OrgUnitID).Set("updated_at = now()").Where("id = ?", id).Exec(ctx); err != nil {
		return false, err
	}
	s.recordAssetEvent(ctx, id, &p.UserID, assetEvent{
		Kind: "field_changed", Subject: events.SubjectAssetUpdated, Summary: "Owner org set from assignee",
		Data: map[string]any{"owner_org_unit_id": per.OrgUnitID},
	})
	return true, nil
}

func (s *Server) fixOrgDefaultLocation(ctx context.Context, p *auth.Principal, id uuid.UUID) (bool, error) {
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(ctx); err != nil {
		return false, fmt.Errorf("%s: not found", id)
	}
	if a.LocationID != nil {
		return false, nil
	}
	locID := s.defaultLocationForAsset(ctx, a)
	if locID == nil {
		return false, nil
	}
	if !s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, a)) {
		return false, fmt.Errorf("%s: forbidden", id)
	}
	if _, err := s.db.NewUpdate().Model(a).Set("location_id = ?", *locID).Set("updated_at = now()").Where("id = ?", id).Exec(ctx); err != nil {
		return false, err
	}
	s.recordAssetEvent(ctx, id, &p.UserID, assetEvent{
		Kind: "field_changed", Subject: events.SubjectAssetUpdated, Summary: "Location set from org default",
		Data: map[string]any{"location_id": locID},
	})
	return true, nil
}

func (s *Server) defaultLocationForAsset(ctx context.Context, a *domain.Asset) *uuid.UUID {
	if a.AssignedPersonID != nil {
		if def := s.defaultLocationForPerson(ctx, *a.AssignedPersonID); def != nil {
			return def
		}
	}
	if a.OwnerOrgUnitID != nil {
		return s.defaultLocationForOrgUnit(ctx, *a.OwnerOrgUnitID)
	}
	return nil
}

func (s *Server) fixSetLocation(ctx context.Context, p *auth.Principal, id, locID uuid.UUID) (bool, error) {
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(ctx); err != nil {
		return false, fmt.Errorf("%s: not found", id)
	}
	if !s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, a)) {
		return false, fmt.Errorf("%s: forbidden", id)
	}
	if _, err := s.db.NewUpdate().Model(a).Set("location_id = ?", locID).Set("updated_at = now()").Where("id = ?", id).Exec(ctx); err != nil {
		return false, err
	}
	s.recordAssetEvent(ctx, id, &p.UserID, assetEvent{
		Kind: "field_changed", Subject: events.SubjectAssetUpdated, Summary: "Location set via bulk fix",
		Data: map[string]any{"location_id": locID},
	})
	return true, nil
}

func (s *Server) fixSetOrgUnit(ctx context.Context, p *auth.Principal, id, orgID uuid.UUID) (bool, error) {
	a := new(domain.Asset)
	if err := s.db.NewSelect().Model(a).Where("a.id = ?", id).Scan(ctx); err != nil {
		return false, fmt.Errorf("%s: not found", id)
	}
	if !s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, a)) {
		return false, fmt.Errorf("%s: forbidden", id)
	}
	if _, err := s.db.NewUpdate().Model(a).Set("owner_org_unit_id = ?", orgID).Set("updated_at = now()").Where("id = ?", id).Exec(ctx); err != nil {
		return false, err
	}
	s.recordAssetEvent(ctx, id, &p.UserID, assetEvent{
		Kind: "field_changed", Subject: events.SubjectAssetUpdated, Summary: "Owner org set via bulk fix",
		Data: map[string]any{"owner_org_unit_id": orgID},
	})
	return true, nil
}

func duplicateSerialGroups(ctx context.Context, db *bun.DB) []duplicateSerialGroup {
	type row struct {
		ID       string `bun:"id"`
		AssetTag string `bun:"asset_tag"`
		Name     string `bun:"name"`
		Serial   string `bun:"serial"`
	}
	var rows []row
	_ = db.NewRaw(
		`SELECT a.id::text, a.asset_tag, a.name, a.serial
		   FROM core.assets a
		   JOIN (
		     SELECT serial FROM core.assets
		      WHERE deleted_at IS NULL AND coalesce(serial, '') <> ''
		      GROUP BY serial HAVING count(*) > 1
		   ) d ON d.serial = a.serial
		  WHERE a.deleted_at IS NULL
		  ORDER BY a.serial, a.asset_tag`,
	).Scan(ctx, &rows)
	bySerial := map[string][]dataQualityAssetRow{}
	order := []string{}
	for _, r := range rows {
		if _, ok := bySerial[r.Serial]; !ok {
			order = append(order, r.Serial)
		}
		bySerial[r.Serial] = append(bySerial[r.Serial], dataQualityAssetRow{
			ID: r.ID, AssetTag: r.AssetTag, Name: r.Name, Issue: "duplicate serial",
		})
	}
	out := make([]duplicateSerialGroup, 0, len(order))
	for _, serial := range order {
		out = append(out, duplicateSerialGroup{Serial: serial, Assets: bySerial[serial]})
	}
	return out
}

// ---- merge assets ----------------------------------------------------------

type mergeAssetsInput struct {
	SurvivorID uuid.UUID `json:"survivor_id"`
	MergeID    uuid.UUID `json:"merge_id"`
}

func (s *Server) handleMergeAssets(w http.ResponseWriter, r *http.Request) {
	var in mergeAssetsInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.SurvivorID == uuid.Nil || in.MergeID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "survivor_id and merge_id are required")
		return
	}
	if in.SurvivorID == in.MergeID {
		writeErr(w, http.StatusBadRequest, "survivor and merge must differ")
		return
	}

	ctx := r.Context()
	p := s.principal(r)
	survivor, err := s.loadAsset(ctx, in.SurvivorID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "survivor not found")
		return
	}
	merge, err := s.loadAsset(ctx, in.MergeID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "merge asset not found")
		return
	}
	if !s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, survivor)) ||
		!s.rbac.CanOn(ctx, p, "asset.write", s.assetTarget(ctx, merge)) {
		writeErr(w, http.StatusForbidden, "not permitted in this scope")
		return
	}

	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Drop merge identities that would collide with survivor.
		if _, err := tx.NewRaw(
			`DELETE FROM core.asset_identities mi
			  USING core.asset_identities si
			  WHERE mi.asset_id = ? AND si.asset_id = ?
			    AND mi.source = si.source AND mi.external_id = si.external_id`,
			in.MergeID, in.SurvivorID,
		).Exec(ctx); err != nil {
			return err
		}
		tables := []string{
			"core.assignments", "core.lifecycle_history", "core.asset_events", "core.asset_costs",
			"core.asset_identities", "software.installations",
		}
		for _, tbl := range tables {
			if _, err := tx.NewRaw(fmt.Sprintf("UPDATE %s SET asset_id = ? WHERE asset_id = ?", tbl), in.SurvivorID, in.MergeID).Exec(ctx); err != nil {
				return err
			}
		}
		if _, err := tx.NewRaw(`UPDATE core.asset_relationships SET from_asset_id = ? WHERE from_asset_id = ? AND to_asset_id <> ?`, in.SurvivorID, in.MergeID, in.SurvivorID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`UPDATE core.asset_relationships SET to_asset_id = ? WHERE to_asset_id = ? AND from_asset_id <> ?`, in.SurvivorID, in.MergeID, in.SurvivorID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`DELETE FROM core.asset_relationships WHERE from_asset_id = to_asset_id`).Exec(ctx); err != nil {
			return err
		}

		// Fill empty survivor fields from merge.
		upd := survivor
		if strings.TrimSpace(upd.Serial) == "" && strings.TrimSpace(merge.Serial) != "" {
			upd.Serial = merge.Serial
		}
		if strings.TrimSpace(upd.Notes) == "" && strings.TrimSpace(merge.Notes) != "" {
			upd.Notes = merge.Notes
		}
		if upd.Attributes == nil {
			upd.Attributes = map[string]any{}
		}
		for k, v := range merge.Attributes {
			if _, ok := upd.Attributes[k]; !ok {
				upd.Attributes[k] = v
			}
		}
		if _, err := tx.NewUpdate().Model(upd).
			Column("serial", "notes", "attributes").
			Set("updated_at = now()").
			Where("id = ?", in.SurvivorID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*domain.Asset)(nil)).
			Set("deleted_at = now()").Set("updated_at = now()").
			Where("id = ?", in.MergeID).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	s.recordAssetEvent(ctx, in.SurvivorID, &p.UserID, assetEvent{
		Kind: "merged", Subject: events.SubjectAssetUpdated, Summary: "Merged duplicate asset",
		Data: map[string]any{"merged_asset_id": in.MergeID, "merged_asset_tag": merge.AssetTag},
	})
	s.emit(ctx, events.SubjectAssetUpdated, "asset", in.SurvivorID.String(), p.UserID.String(), map[string]any{"merged": in.MergeID.String()})

	full, _ := s.loadAsset(ctx, in.SurvivorID)
	writeJSON(w, http.StatusOK, full)
}

// ---- unmapped ingest location labels ---------------------------------------

type unmappedLocationRow struct {
	Label      string `bun:"label" json:"label"`
	Source     string `bun:"source" json:"source"`
	AssetCount int    `bun:"asset_count" json:"asset_count"`
}

func (s *Server) handleReportUnmappedLocations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var rows []unmappedLocationRow
	_ = s.db.NewRaw(
		`WITH labels AS (
		   SELECT lower(trim(attributes->>'network_map')) AS label, 'network_map' AS source FROM core.assets
		    WHERE deleted_at IS NULL AND coalesce(trim(attributes->>'network_map'), '') <> ''
		   UNION ALL
		   SELECT lower(trim(attributes->>'probe')), 'probe' FROM core.assets
		    WHERE deleted_at IS NULL AND coalesce(trim(attributes->>'probe'), '') <> ''
		   UNION ALL
		   SELECT lower(trim(attributes->>'mapName')), 'mapName' FROM core.assets
		    WHERE deleted_at IS NULL AND coalesce(trim(attributes->>'mapName'), '') <> ''
		 ),
		 known AS (
		   SELECT lower(key) AS label FROM core.locations
		   UNION SELECT lower(name) FROM core.locations
		   UNION SELECT alias::text FROM meta.location_aliases
		 )
		 SELECT l.label, min(l.source) AS source, count(*)::int AS asset_count
		   FROM labels l
		   LEFT JOIN known k ON k.label = l.label
		  WHERE k.label IS NULL
		  GROUP BY l.label
		  ORDER BY asset_count DESC, l.label ASC
		  LIMIT 50`,
	).Scan(ctx, &rows)
	if rows == nil {
		rows = []unmappedLocationRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// ---- setup health checklist ------------------------------------------------

type setupHealthItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	Count int    `json:"count,omitempty"`
	Href  string `json:"href"`
}

type setupHealthReport struct {
	Score int               `json:"score"`
	Items []setupHealthItem `json:"items"`
}

func (s *Server) handleReportSetupHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items := []setupHealthItem{}
	add := func(key, label, href string, ok bool, count int) {
		items = append(items, setupHealthItem{Key: key, Label: label, OK: ok, Count: count, Href: href})
	}

	var orgKindCount int
	_ = s.db.NewRaw(`SELECT count(*) FROM meta.org_unit_kinds`).Scan(ctx, &orgKindCount)
	add("org_kinds", "Org unit kinds configured", "/org-units", orgKindCount > 0, orgKindCount)

	var orgDefaultCount int
	_ = s.db.NewRaw(`SELECT count(*) FROM core.org_units WHERE default_location_id IS NOT NULL`).Scan(ctx, &orgDefaultCount)
	add("org_defaults", "Org units with default location", "/org-units", orgDefaultCount > 0, orgDefaultCount)

	var aliasCount int
	_ = s.db.NewRaw(`SELECT count(*) FROM meta.location_aliases`).Scan(ctx, &aliasCount)
	add("location_aliases", "Location aliases for ingest", "/locations", aliasCount > 0, aliasCount)

	var gapCount int
	_ = s.db.NewRaw(
		`SELECT count(*) FROM core.assets WHERE deleted_at IS NULL
		   AND (location_id IS NULL OR owner_org_unit_id IS NULL OR coalesce(serial, '') = '')`,
	).Scan(ctx, &gapCount)
	add("data_gaps", "Assets with data gaps", "/reports", gapCount == 0, gapCount)

	var unmappedCount int
	_ = s.db.NewRaw(
		`WITH labels AS (
		   SELECT lower(trim(attributes->>'network_map')) AS label FROM core.assets WHERE deleted_at IS NULL AND coalesce(trim(attributes->>'network_map'), '') <> ''
		   UNION ALL SELECT lower(trim(attributes->>'probe')) FROM core.assets WHERE deleted_at IS NULL AND coalesce(trim(attributes->>'probe'), '') <> ''
		 ),
		 known AS (
		   SELECT lower(key) AS label FROM core.locations UNION SELECT lower(name) FROM core.locations UNION SELECT alias::text FROM meta.location_aliases
		 )
		 SELECT count(DISTINCT l.label) FROM labels l LEFT JOIN known k ON k.label = l.label WHERE k.label IS NULL`,
	).Scan(ctx, &unmappedCount)
	add("unmapped_labels", "Unmapped ingest location labels", "/reports", unmappedCount == 0, unmappedCount)

	var dupCount int
	_ = s.db.NewRaw(
		`SELECT count(*) FROM (
		   SELECT serial FROM core.assets WHERE deleted_at IS NULL AND coalesce(serial,'') <> '' GROUP BY serial HAVING count(*) > 1
		 ) d`,
	).Scan(ctx, &dupCount)
	add("duplicate_serials", "Duplicate serial numbers", "/reports", dupCount == 0, dupCount)

	var connectorCount int
	_ = s.db.NewRaw(`SELECT count(*) FROM integration.connectors WHERE enabled = true`).Scan(ctx, &connectorCount)
	add("connectors", "Active integration connectors", "/integrations", connectorCount > 0, connectorCount)

	var dqCheckEnabled bool
	_ = s.db.NewRaw(
		`SELECT enabled FROM meta.scheduled_checks WHERE key = 'data-quality-gaps'`,
	).Scan(ctx, &dqCheckEnabled)
	var dqChannelReady bool
	_ = s.db.NewRaw(
		`SELECT count(*) > 0 FROM meta.notification_channels
		  WHERE key = 'ops-alerts' AND enabled = true
		    AND coalesce(config->>'url', '') <> ''`,
	).Scan(ctx, &dqChannelReady)
	add("data_quality_alerts", "Data quality alerts (channel configured)", "/notifications", dqCheckEnabled && dqChannelReady, 0)

	ssoReady := s.azureSSOEnabled(ctx)
	add("azure_sso", "Microsoft Entra ID SSO configured", "/admin", ssoReady, 0)

	okCount := 0
	for _, it := range items {
		if it.OK {
			okCount++
		}
	}
	score := 0
	if len(items) > 0 {
		score = (okCount * 100) / len(items)
	}
	writeJSON(w, http.StatusOK, setupHealthReport{Score: score, Items: items})
}

// ---- connector health ------------------------------------------------------

type connectorHealthRow struct {
	ID           string  `bun:"id" json:"id"`
	Key          string  `bun:"key" json:"key"`
	Name         string  `bun:"name" json:"name"`
	Kind         string  `bun:"kind" json:"kind"`
	Enabled      bool    `bun:"enabled" json:"enabled"`
	LastRunAt    *string `bun:"last_run_at" json:"last_run_at"`
	LastStatus   string  `bun:"last_status" json:"last_status"`
	Runs24h      int     `bun:"runs_24h" json:"runs_24h"`
	Errors24h    int     `bun:"errors_24h" json:"errors_24h"`
	LastSeen     int     `bun:"last_seen" json:"last_seen"`
	LastCreated  int     `bun:"last_created" json:"last_created"`
	LastUpdated  int     `bun:"last_updated" json:"last_updated"`
}

func (s *Server) handleReportConnectorHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var rows []connectorHealthRow
	_ = s.db.NewRaw(
		`SELECT c.id::text, c.key, c.name, c.kind, c.enabled,
		        to_char(c.last_run_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS last_run_at,
		        coalesce(c.last_status, '') AS last_status,
		        coalesce((SELECT count(*) FROM integration.sync_runs sr WHERE sr.connector_id = c.id AND sr.started_at > now() - interval '24 hours'), 0)::int AS runs_24h,
		        coalesce((SELECT count(*) FROM integration.sync_runs sr WHERE sr.connector_id = c.id AND sr.started_at > now() - interval '24 hours' AND sr.status IN ('error','partial')), 0)::int AS errors_24h,
		        coalesce((SELECT seen FROM integration.sync_runs sr WHERE sr.connector_id = c.id ORDER BY sr.started_at DESC LIMIT 1), 0) AS last_seen,
		        coalesce((SELECT created FROM integration.sync_runs sr WHERE sr.connector_id = c.id ORDER BY sr.started_at DESC LIMIT 1), 0) AS last_created,
		        coalesce((SELECT updated FROM integration.sync_runs sr WHERE sr.connector_id = c.id ORDER BY sr.started_at DESC LIMIT 1), 0) AS last_updated
		   FROM integration.connectors c
		  ORDER BY c.name ASC`,
	).Scan(ctx, &rows)
	if rows == nil {
		rows = []connectorHealthRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}
