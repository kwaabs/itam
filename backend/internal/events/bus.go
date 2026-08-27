package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// StreamName is the durable JetStream stream that captures every domain event.
const StreamName = "ITAM_EVENTS"

// SubjectPrefix scopes all of our subjects.
const SubjectPrefix = "itam"

// Well-known subjects.
const (
	SubjectAssetCreated      = "itam.asset.created"
	SubjectAssetUpdated      = "itam.asset.updated"
	SubjectAssetDeleted      = "itam.asset.deleted"
	SubjectAssetStateChanged = "itam.asset.state_changed"
	SubjectAssetAssigned     = "itam.asset.assigned"
	SubjectAssetReturned     = "itam.asset.returned"
	SubjectAssetTransferred  = "itam.asset.transferred"
)

// Envelope is the standard event payload published to JetStream.
type Envelope struct {
	ID         string         `json:"id"`
	Subject    string         `json:"subject"`
	Actor      string         `json:"actor"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload"`
}

// Bus wraps a NATS connection + JetStream context.
type Bus struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
}

// Connect dials NATS, ensures the stream exists, and returns a Bus.
func Connect(url string) (*Bus, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamName,
		Subjects:  []string{SubjectPrefix + ".>"},
		Storage:   jetstream.FileStorage,
		Retention: jetstream.LimitsPolicy,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Bus{nc: nc, js: js, stream: stream}, nil
}

// JS exposes the JetStream context for consumer setup.
func (b *Bus) JS() jetstream.JetStream { return b.js }

// Stream exposes the configured stream.
func (b *Bus) Stream() jetstream.Stream { return b.stream }

// Publish completes the envelope and publishes it on its subject.
func (b *Bus) Publish(ctx context.Context, env Envelope) error {
	if env.ID == "" {
		env.ID = uuid.NewString()
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(ctx, env.Subject, data)
	return err
}

// Close drains and closes the NATS connection.
func (b *Bus) Close() {
	if b.nc != nil {
		_ = b.nc.Drain()
	}
}
