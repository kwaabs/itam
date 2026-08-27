package events

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"itam/internal/domain"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/uptrace/bun"
)

func parseUUID(s string) (uuid.UUID, error) { return uuid.Parse(s) }

// Consumers holds running JetStream consume handles so they can be stopped.
type Consumers struct {
	handles []jetstream.ConsumeContext
}

// Stop drains all running consumers.
func (c *Consumers) Stop() {
	for _, h := range c.handles {
		h.Stop()
	}
}

// Start wires the three Phase 1 consumers: audit writer, rules engine, and the
// (stub) integration reconciler.
func Start(ctx context.Context, bus *Bus, db *bun.DB, log *slog.Logger) (*Consumers, error) {
	c := &Consumers{}

	auditH, err := consume(ctx, bus, "audit-writer", "itam.>", func(env Envelope, raw []byte) {
		row := &domain.AuditLog{
			Subject:    env.Subject,
			Actor:      env.Actor,
			EntityType: env.EntityType,
			EntityID:   env.EntityID,
			OccurredAt: env.OccurredAt,
			Payload:    env.Payload,
		}
		if env.ID != "" {
			if id, perr := parseUUID(env.ID); perr == nil {
				row.EventID = id
			}
		}
		// ON CONFLICT keeps the log append-only + idempotent on redelivery.
		if _, derr := db.NewInsert().Model(row).
			On("CONFLICT (event_id) DO NOTHING").Exec(ctx); derr != nil {
			log.Error("audit insert failed", "err", derr, "subject", env.Subject)
		}
	})
	if err != nil {
		return nil, err
	}
	c.handles = append(c.handles, auditH)

	rulesH, err := consume(ctx, bus, "rules-engine", "itam.>", func(env Envelope, raw []byte) {
		runRules(ctx, db, log, env)
	})
	if err != nil {
		return nil, err
	}
	c.handles = append(c.handles, rulesH)

	// Stub for the later Intune/Defender reconciler. It just observes for now.
	intH, err := consume(ctx, bus, "integration-reconciler", "itam.asset.>", func(env Envelope, raw []byte) {
		log.Debug("integration reconciler observed event", "subject", env.Subject, "entity", env.EntityID)
	})
	if err != nil {
		return nil, err
	}
	c.handles = append(c.handles, intH)

	log.Info("event consumers started")
	return c, nil
}

func consume(ctx context.Context, bus *Bus, durable, filter string, fn func(Envelope, []byte)) (jetstream.ConsumeContext, error) {
	cons, err := bus.Stream().CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       durable,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: filter,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return cons.Consume(func(msg jetstream.Msg) {
		var env Envelope
		if err := json.Unmarshal(msg.Data(), &env); err != nil {
			_ = msg.Term() // poison message, don't redeliver
			return
		}
		fn(env, msg.Data())
		_ = msg.Ack()
	})
}

// runRules evaluates configured automation rules against an event.
func runRules(ctx context.Context, db *bun.DB, log *slog.Logger, env Envelope) {
	var rules []domain.AutomationRule
	err := db.NewSelect().Model(&rules).
		Where("enabled = ?", true).
		Where("? LIKE replace(on_subject, '>', '%')", env.Subject).
		Order("sort ASC").Scan(ctx)
	if err != nil {
		log.Error("rules load failed", "err", err)
		return
	}
	for _, r := range rules {
		if !conditionMatches(r.Condition, env) {
			continue
		}
		doAction(ctx, db, log, r, env)
	}
}

// conditionMatches does a shallow equality check of condition keys against the
// event payload. An empty condition always matches.
func conditionMatches(cond map[string]any, env Envelope) bool {
	for k, want := range cond {
		got, ok := env.Payload[k]
		if !ok {
			return false
		}
		wb, _ := json.Marshal(want)
		gb, _ := json.Marshal(got)
		if !bytes.Equal(wb, gb) {
			return false
		}
	}
	return true
}

func doAction(ctx context.Context, db *bun.DB, log *slog.Logger, r domain.AutomationRule, env Envelope) {
	actionType, _ := r.Action["type"].(string)
	switch actionType {
	case "log", "":
		log.Info("automation rule fired",
			"rule", r.Key, "subject", env.Subject, "entity", env.EntityID,
			"message", r.Action["message"])
	case "notify":
		channel, _ := r.Action["channel"].(string)
		template, _ := r.Action["template"].(string)
		if channel == "" {
			log.Warn("notify rule missing channel", "rule", r.Key)
			return
		}
		_ = DeliverToChannel(ctx, db, log, channel, template, env)
	case "webhook":
		url, _ := r.Action["url"].(string)
		if url == "" {
			return
		}
		body, _ := json.Marshal(env)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Error("webhook failed", "rule", r.Key, "err", err)
			return
		}
		_ = resp.Body.Close()
	default:
		log.Warn("unknown automation action", "rule", r.Key, "type", actionType)
	}
}
