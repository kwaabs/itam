-- +goose Up
-- +goose StatementBegin

-- Per-entity dedupe ledger so recurring checks (warranty/license/stale) notify
-- once per (check, entity, dedupe-key) instead of every daily run. The dedupe
-- key encodes the relevant date (e.g. warranty_expiry), so renewing/extending a
-- warranty produces a fresh key and a new reminder.
CREATE TABLE IF NOT EXISTS meta.check_notified (
    check_key   text        NOT NULL,
    entity_id   text        NOT NULL,
    dedupe_key  text        NOT NULL DEFAULT '',
    notified_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (check_key, entity_id, dedupe_key)
);
CREATE INDEX IF NOT EXISTS check_notified_key_idx ON meta.check_notified (check_key);

DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, DELETE ON meta.check_notified TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS meta.check_notified;
-- +goose StatementEnd
