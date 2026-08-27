// Package scheduler runs metadata-defined periodic checks (meta.scheduled_checks)
// and emits domain events when a check finds matching rows. The events flow
// through the normal rules engine, so "what to watch" and "where to notify" stay
// fully data-driven.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"itam/internal/domain"
	"itam/internal/events"

	"github.com/uptrace/bun"
)

type Scheduler struct {
	db  *bun.DB
	bus *events.Bus
	log *slog.Logger
}

func New(db *bun.DB, bus *events.Bus, log *slog.Logger) *Scheduler {
	return &Scheduler{db: db, bus: bus, log: log}
}

// Run loops until the context is cancelled, evaluating due checks every minute.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	s.runDue(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDue(ctx)
		}
	}
}

func (s *Scheduler) runDue(ctx context.Context) {
	var checks []domain.ScheduledCheck
	err := s.db.NewSelect().Model(&checks).
		Where("enabled = ?", true).
		Where("next_run_at IS NULL OR next_run_at <= now()").
		Scan(ctx)
	if err != nil {
		s.log.Error("scheduler: load checks failed", "err", err)
		return
	}
	for i := range checks {
		if _, err := s.RunCheck(ctx, &checks[i]); err != nil {
			s.log.Error("scheduler: check failed", "check", checks[i].Key, "err", err)
		}
	}
}

// RunCheck executes a single check now, emits an event if there are hits, and
// records run metadata. It is reused by the "run now" API endpoint.
func (s *Scheduler) RunCheck(ctx context.Context, c *domain.ScheduledCheck) (int, error) {
	items, err := s.execCheck(ctx, c)
	status := "ok"
	if err != nil {
		status = "error"
	}

	// Suppress repeat alerts: only emit items not already notified for this
	// (check, entity, dedupe-key). The dedupe key encodes the relevant date so a
	// renewed warranty / extended licence produces a fresh reminder.
	field := dedupeField(c.Kind)
	emit := items
	if err == nil && dedupeEnabled(c) {
		emit = s.filterNotified(ctx, c.Key, items, field)
	}

	if err == nil && len(emit) > 0 && s.bus != nil {
		perr := s.bus.Publish(ctx, events.Envelope{
			Subject:    c.EventSubject,
			Actor:      "scheduler",
			EntityType: "check",
			EntityID:   c.Key,
			Payload: map[string]any{
				"check":  c.Key,
				"name":   c.Name,
				"count":  len(emit),
				"items":  emit,
				"params": c.Params,
			},
		})
		if perr != nil {
			s.log.Error("scheduler: publish failed", "check", c.Key, "err", perr)
		} else if dedupeEnabled(c) {
			s.recordNotified(ctx, c.Key, emit, field)
		}
	}

	next := time.Now().Add(time.Duration(c.IntervalSeconds) * time.Second)
	if _, uerr := s.db.NewUpdate().Model((*domain.ScheduledCheck)(nil)).
		Set("last_run_at = now()").
		Set("next_run_at = ?", next).
		Set("last_status = ?", status).
		Set("last_count = ?", len(items)).
		Set("updated_at = now()").
		Where("id = ?", c.ID).Exec(ctx); uerr != nil {
		s.log.Error("scheduler: update check failed", "check", c.Key, "err", uerr)
	}
	return len(items), err
}

// dedupeField returns the item field whose value (typically a date) forms the
// dedupe key for a built-in check kind. Empty means dedupe by entity id only.
func dedupeField(kind string) string {
	switch kind {
	case "warranty_expiry":
		return "warranty_expiry"
	case "license_expiry":
		return "expiry_date"
	case "asset_stale":
		return "last_seen_at"
	case "data_quality":
		return "issue"
	default:
		return ""
	}
}

// dedupeEnabled defaults on for built-in kinds; params.dedupe=false opts out.
func dedupeEnabled(c *domain.ScheduledCheck) bool {
	if c.Params != nil {
		if v, ok := c.Params["dedupe"].(bool); ok {
			return v
		}
	}
	return dedupeField(c.Kind) != ""
}

// filterNotified drops items already recorded in meta.check_notified for this
// check + (entity_id, dedupe_key) pair.
func (s *Scheduler) filterNotified(ctx context.Context, checkKey string, items []map[string]any, field string) []map[string]any {
	if len(items) == 0 {
		return items
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, fmt.Sprint(it["id"]))
	}
	var rows []struct {
		EntityID  string `bun:"entity_id"`
		DedupeKey string `bun:"dedupe_key"`
	}
	if err := s.db.NewRaw(
		`SELECT entity_id, dedupe_key FROM meta.check_notified
		  WHERE check_key = ? AND entity_id IN (?)`, checkKey, bun.In(ids),
	).Scan(ctx, &rows); err != nil {
		s.log.Error("scheduler: dedupe lookup failed", "check", checkKey, "err", err)
		return items
	}
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.EntityID+"|"+r.DedupeKey] = true
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		dk := ""
		if field != "" {
			dk = fmt.Sprint(it[field])
		}
		if !seen[fmt.Sprint(it["id"])+"|"+dk] {
			out = append(out, it)
		}
	}
	return out
}

// recordNotified marks items as notified so future runs skip them.
func (s *Scheduler) recordNotified(ctx context.Context, checkKey string, items []map[string]any, field string) {
	for _, it := range items {
		dk := ""
		if field != "" {
			dk = fmt.Sprint(it[field])
		}
		if _, err := s.db.NewRaw(
			`INSERT INTO meta.check_notified (check_key, entity_id, dedupe_key)
			 VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			checkKey, fmt.Sprint(it["id"]), dk,
		).Exec(ctx); err != nil {
			s.log.Error("scheduler: record notified failed", "check", checkKey, "err", err)
		}
	}
}

func intParam(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	switch v := params[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

// execCheck runs the read-only query for a check kind and returns hit rows.
func (s *Scheduler) execCheck(ctx context.Context, c *domain.ScheduledCheck) ([]map[string]any, error) {
	days := intParam(c.Params, "days", 30)
	switch c.Kind {
	case "license_expiry":
		return s.query(ctx, `
			SELECT l.id, l.name, l.expiry_date, s.name AS software
			FROM swm.licenses l
			LEFT JOIN swm.software s ON s.id = l.software_id
			WHERE l.expiry_date IS NOT NULL
			  AND l.expiry_date >= now()::date
			  AND l.expiry_date <= (now() + ($1 || ' days')::interval)::date
			ORDER BY l.expiry_date ASC`, days)
	case "warranty_expiry":
		return s.query(ctx, `
			SELECT id, asset_tag, name, warranty_expiry
			FROM core.assets
			WHERE deleted_at IS NULL AND warranty_expiry IS NOT NULL
			  AND warranty_expiry >= now()::date
			  AND warranty_expiry <= (now() + ($1 || ' days')::interval)::date
			ORDER BY warranty_expiry ASC`, days)
	case "asset_stale":
		return s.query(ctx, `
			SELECT id, asset_tag, name, last_seen_at, last_seen_source
			FROM core.assets
			WHERE deleted_at IS NULL AND last_seen_at IS NOT NULL
			  AND last_seen_at < now() - ($1 || ' days')::interval
			ORDER BY last_seen_at ASC`, days)
	case "data_quality":
		threshold := intParam(c.Params, "gap_threshold", 5)
		var count int
		if err := s.db.NewRaw(
			`SELECT count(*) FROM core.assets
			  WHERE deleted_at IS NULL
			    AND (location_id IS NULL OR owner_org_unit_id IS NULL OR coalesce(serial, '') = '')`,
		).Scan(ctx, &count); err != nil {
			return nil, err
		}
		if count < threshold {
			return nil, nil
		}
		return []map[string]any{{
			"id":    "data-quality",
			"count": count,
			"issue": fmt.Sprintf("%d assets with data gaps (threshold %d)", count, threshold),
		}}, nil
	case "sql":
		q := strings.TrimSpace(fmt.Sprint(c.Params["query"]))
		if q == "" {
			return nil, fmt.Errorf("sql check has no params.query")
		}
		if !strings.HasPrefix(strings.ToLower(q), "select") {
			return nil, fmt.Errorf("sql check must be a SELECT statement")
		}
		return s.query(ctx, q)
	default:
		return nil, fmt.Errorf("unknown check kind %q", c.Kind)
	}
}

// query runs an arbitrary read query and returns rows as maps, normalising
// []byte values to strings so they serialise cleanly to JSON.
func (s *Scheduler) query(ctx context.Context, q string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = normalize(cells[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func normalize(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return v
	}
}
