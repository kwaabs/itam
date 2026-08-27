-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Generic, metadata-driven ingestion mapping (Phase 1.5).
--   * raw_records : land WHATEVER arrives (push) or is pulled (Graph/Arc),
--                   verbatim, so nothing is ever lost and we can replay.
--   * mappings    : per (connector, source_object) rules that turn a raw record
--                   into one of our entities (asset/person/installation) using
--                   dot-paths + named transforms. Fully editable config.
-- Reconciliation, sync_runs and asset_identities from 00020 are reused as-is.
-- ---------------------------------------------------------------------------

-- Pull connectors (Graph/Arc) need an outbound OAuth client secret, kept out of
-- the read API just like the inbound token.
ALTER TABLE integration.connectors ADD COLUMN IF NOT EXISTS pull_secret text;

CREATE TABLE integration.mappings (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id  uuid NOT NULL REFERENCES integration.connectors(id) ON DELETE CASCADE,
    source_object text NOT NULL,                      -- managedDevice|user|detectedApp|...
    target_entity text NOT NULL DEFAULT 'asset',      -- asset|person|installation
    enabled       boolean NOT NULL DEFAULT true,
    sort          int NOT NULL DEFAULT 0,
    identity      text NOT NULL,                       -- dot-path to the external id
    match_fallbacks jsonb NOT NULL DEFAULT '[]',       -- ["serialNumber", ...]
    type_resolution jsonb NOT NULL DEFAULT '{}',       -- {default, by, map:{}}
    fields        jsonb NOT NULL DEFAULT '[]',         -- [{source,target,transform,arg}]
    filters       jsonb NOT NULL DEFAULT '[]',         -- [{source,op,value}]
    version       int NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connector_id, source_object)
);

CREATE TABLE integration.raw_records (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id  uuid NOT NULL REFERENCES integration.connectors(id) ON DELETE CASCADE,
    source_object text NOT NULL,
    external_id   text NOT NULL,
    payload       jsonb NOT NULL DEFAULT '{}',
    sync_run_id   uuid REFERENCES integration.sync_runs(id) ON DELETE SET NULL,
    processed     boolean NOT NULL DEFAULT false,
    status        text,                                -- created|updated|skipped|filtered|unmapped|error
    error         text,
    asset_id      uuid,
    received_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connector_id, source_object, external_id)
);
CREATE INDEX raw_records_conn_idx ON integration.raw_records (connector_id, source_object, received_at DESC);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA integration TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA integration TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS integration.raw_records;
DROP TABLE IF EXISTS integration.mappings;
ALTER TABLE integration.connectors DROP COLUMN IF EXISTS pull_secret;
-- +goose StatementEnd
