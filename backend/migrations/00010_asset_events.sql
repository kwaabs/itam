-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Append-only event ledger: the canonical, unified asset timeline. Every
-- meaningful change (created, state_changed, assigned, returned, transferred,
-- moved, field_changed, cost_recorded, received, disposed, ...) writes one row
-- here, in addition to the specialized tables that remain systems of record.
-- ---------------------------------------------------------------------------
CREATE TABLE core.asset_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    kind        text NOT NULL,                 -- created|state_changed|assigned|...
    subject     text,                          -- mirrors the NATS subject, if any
    actor       uuid,                          -- GoTrue user id
    summary     text,                          -- human-readable one-liner
    data        jsonb NOT NULL DEFAULT '{}',   -- structured details
    ref_table   text,                          -- linked system-of-record table
    ref_id      uuid,                          -- linked row id
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX asset_events_asset_idx ON core.asset_events (asset_id, occurred_at DESC);
CREATE INDEX asset_events_kind_idx  ON core.asset_events (kind);

-- Append-only for the app role: it may INSERT/SELECT but never mutate history.
-- (Cascade deletes still work; they run as the table owner, not itam_app.)
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    REVOKE UPDATE, DELETE ON core.asset_events FROM itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.asset_events;
-- +goose StatementEnd
