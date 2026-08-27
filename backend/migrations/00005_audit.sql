-- +goose Up
-- +goose StatementBegin

-- Append-only audit trail. Populated by the audit-writer JetStream consumer,
-- so every domain event becomes a durable, queryable record.
CREATE TABLE audit.audit_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id    uuid NOT NULL,
    subject     text NOT NULL,
    actor       text,
    entity_type text,
    entity_id   text,
    occurred_at timestamptz NOT NULL,
    payload     jsonb NOT NULL DEFAULT '{}',
    recorded_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);
CREATE INDEX audit_log_subject_idx ON audit.audit_log (subject);
CREATE INDEX audit_log_entity_idx  ON audit.audit_log (entity_type, entity_id);
CREATE INDEX audit_log_occurred_idx ON audit.audit_log (occurred_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit.audit_log;
-- +goose StatementEnd
