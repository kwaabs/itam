-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- integration: the ingestion foundation. Connectors describe an external
-- source (Intune/Defender/Entra/generic push), sync_runs log each ingest, and
-- core.asset_identities maps an external system's id to one of our assets so
-- repeated syncs UPDATE rather than duplicate.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS integration;

CREATE TABLE integration.connectors (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    kind        text   NOT NULL DEFAULT 'generic', -- generic|intune|defender|entra|csv
    enabled     boolean NOT NULL DEFAULT true,
    direction   text   NOT NULL DEFAULT 'inbound',  -- inbound (push) | pull (future)
    schedule    text,                               -- cron, for future pull jobs
    -- shared HMAC/token secret for inbound auth; never returned by the read API.
    secret      text,
    -- field map + defaults (default_asset_type, default_location, mapping rules)
    config      jsonb  NOT NULL DEFAULT '{}',
    last_run_at timestamptz,
    last_status text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE integration.sync_runs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id uuid NOT NULL REFERENCES integration.connectors(id) ON DELETE CASCADE,
    mode         text NOT NULL DEFAULT 'push',     -- push|pull|manual
    status       text NOT NULL DEFAULT 'running',  -- running|success|partial|error
    seen         int  NOT NULL DEFAULT 0,
    created      int  NOT NULL DEFAULT 0,
    updated      int  NOT NULL DEFAULT 0,
    skipped      int  NOT NULL DEFAULT 0,
    errors       int  NOT NULL DEFAULT 0,
    message      text,
    detail       jsonb NOT NULL DEFAULT '{}',
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz
);
CREATE INDEX sync_runs_connector_idx ON integration.sync_runs (connector_id, started_at DESC);

-- Maps (source, external_id) -> asset. Source is usually the connector key.
CREATE TABLE core.asset_identities (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id     uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    source       text NOT NULL,
    external_id  text NOT NULL,
    attributes   jsonb NOT NULL DEFAULT '{}',
    last_seen_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, external_id)
);
CREATE INDEX asset_identities_asset_idx ON core.asset_identities (asset_id);

-- --- Least-privilege grants for the app role --------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA integration TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA integration TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA integration TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA integration GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA integration GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON core.asset_identities TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('integration.read',   'View connectors and sync history'),
    ('integration.manage', 'Create and manage connectors / ingestion');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'integration.read'),
    ('admin',         'integration.manage'),
    ('asset_manager', 'integration.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('integration.read', 'integration.manage');
DROP TABLE IF EXISTS core.asset_identities;
DROP SCHEMA IF EXISTS integration CASCADE;
-- +goose StatementEnd
