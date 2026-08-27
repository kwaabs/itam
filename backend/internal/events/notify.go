package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"itam/internal/domain"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// DeliverToChannel resolves a notification channel by key, renders the message
// template against the event, delivers it, and records the result. Everything
// (channel target, type, message template) comes from metadata.
func DeliverToChannel(ctx context.Context, db *bun.DB, log *slog.Logger, channelKey, template string, env Envelope) error {
	ch := new(domain.NotificationChannel)
	err := db.NewSelect().Model(ch).Where("key = ? AND enabled = true", channelKey).Scan(ctx)
	if err != nil {
		logDelivery(ctx, db, nil, channelKey, env.Subject, "error", "channel not found or disabled: "+channelKey, nil)
		return fmt.Errorf("channel %q not found or disabled", channelKey)
	}

	msg := renderTemplate(template, env)
	derr := sendToChannel(ctx, ch, msg, env)

	status, errStr := "sent", ""
	if derr != nil {
		status, errStr = "error", derr.Error()
		if log != nil {
			log.Error("notification delivery failed", "channel", ch.Key, "type", ch.Type, "err", derr)
		}
	}
	logDelivery(ctx, db, &ch.ID, ch.Key, env.Subject, status, errStr, map[string]any{"message": msg})
	return derr
}

func sendToChannel(ctx context.Context, ch *domain.NotificationChannel, msg string, env Envelope) error {
	switch ch.Type {
	case "slack":
		return postJSON(ctx, cfgStr(ch.Config, "url"), map[string]any{"text": msg})
	case "teams":
		// Teams accepts a simple {"text": ...} via incoming webhook connectors.
		return postJSON(ctx, cfgStr(ch.Config, "url"), map[string]any{"text": msg})
	case "webhook":
		return postJSON(ctx, cfgStr(ch.Config, "url"), map[string]any{"text": msg, "event": env})
	case "email":
		return sendEmail(ch.Config, env.Subject, msg)
	default:
		return fmt.Errorf("unsupported channel type %q", ch.Type)
	}
}

func postJSON(ctx context.Context, url string, body any) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("channel has no url configured")
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delivery returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func sendEmail(config map[string]any, subject, body string) error {
	host := cfgStr(config, "host")
	if host == "" {
		return fmt.Errorf("email channel missing host")
	}
	port := cfgStr(config, "port")
	if port == "" {
		port = "587"
	}
	from := cfgStr(config, "from")
	to := cfgStr(config, "to")
	if from == "" || to == "" {
		return fmt.Errorf("email channel missing from/to")
	}
	recipients := splitList(to)
	headers := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"
	msg := []byte(headers + body)

	var auth smtp.Auth
	if user := cfgStr(config, "username"); user != "" {
		auth = smtp.PlainAuth("", user, cfgStr(config, "password"), host)
	}
	return smtp.SendMail(host+":"+port, auth, from, recipients, msg)
}

// renderTemplate substitutes {{token}} placeholders using the envelope's
// well-known fields and payload values. A blank template falls back to a
// sensible default so a channel always sends something.
func renderTemplate(tmpl string, env Envelope) string {
	if strings.TrimSpace(tmpl) == "" {
		if _, ok := env.Payload["count"]; ok {
			tmpl = "[{{subject}}] {{count}} item(s) need attention"
		} else {
			tmpl = "[{{subject}}] {{entity_type}} {{entity_id}}"
		}
	}
	vals := map[string]string{
		"subject":     env.Subject,
		"actor":       env.Actor,
		"entity_id":   env.EntityID,
		"entity_type": env.EntityType,
	}
	for k, v := range env.Payload {
		vals[k] = fmt.Sprint(v)
	}
	out := tmpl
	for k, v := range vals {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func logDelivery(ctx context.Context, db *bun.DB, channelID *uuid.UUID, channelKey, subject, status, errStr string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	row := &domain.NotificationLog{
		ChannelID: channelID, ChannelKey: channelKey, Subject: subject, Status: status, Error: errStr, Payload: payload,
	}
	_, _ = db.NewInsert().Model(row).Exec(ctx)
}

func cfgStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
