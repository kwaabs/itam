-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Phase 2: metadata-driven notifications + scheduling.
--   * notification_channels  : WHERE to deliver (slack/teams/webhook/email)
--   * scheduled_checks        : WHAT to watch on a timer (expiry/stale/sql)
--   * notification_log        : delivery audit trail
-- Routing (which event -> which channel) reuses meta.automation_rules with an
-- action of {"type":"notify","channel":"<key>","template":"..."}. Nothing about
-- the behaviour is hardcoded: channels, checks, thresholds, subjects, templates
-- and routing are all rows.
-- ---------------------------------------------------------------------------

CREATE TABLE meta.notification_channels (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    type        text   NOT NULL DEFAULT 'webhook',  -- webhook|slack|teams|email
    config      jsonb  NOT NULL DEFAULT '{}',        -- {url} or {host,port,from,to,username,password}
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta.scheduled_checks (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key              citext NOT NULL UNIQUE,
    name             text   NOT NULL,
    kind             text   NOT NULL,                -- license_expiry|warranty_expiry|asset_stale|sql
    interval_seconds int    NOT NULL DEFAULT 86400,
    params           jsonb  NOT NULL DEFAULT '{}',   -- {days:30} or {query:"select ..."}
    event_subject    text   NOT NULL,                -- emitted on hits, e.g. itam.license.expiring
    enabled          boolean NOT NULL DEFAULT false,
    last_run_at      timestamptz,
    next_run_at      timestamptz,
    last_status      text,
    last_count       int    NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta.notification_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    channel_id  uuid REFERENCES meta.notification_channels(id) ON DELETE SET NULL,
    channel_key text,
    subject     text,
    status      text NOT NULL,                       -- sent|error
    error       text,
    payload     jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_log_created_idx ON meta.notification_log (created_at DESC);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.notification_channels TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.scheduled_checks      TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.notification_log      TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA meta TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('notification.read',   'View notification channels, checks and delivery log'),
    ('notification.manage', 'Manage notification channels and scheduled checks');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'notification.read'),
    ('admin',         'notification.manage'),
    ('asset_manager', 'notification.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- --- Seed checks (disabled; admins enable + point a rule at a channel) ------
INSERT INTO meta.scheduled_checks (key, name, kind, interval_seconds, params, event_subject, enabled) VALUES
    ('license-expiry',  'License expiring soon',    'license_expiry',  86400, '{"days":30}', 'itam.license.expiring',  false),
    ('warranty-expiry', 'Warranty expiring soon',   'warranty_expiry', 86400, '{"days":30}', 'itam.warranty.expiring', false),
    ('asset-stale',     'Assets not seen recently', 'asset_stale',     86400, '{"days":30}', 'itam.asset.stale',       false);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('notification.read', 'notification.manage');
DROP TABLE IF EXISTS meta.notification_log;
DROP TABLE IF EXISTS meta.scheduled_checks;
DROP TABLE IF EXISTS meta.notification_channels;
-- +goose StatementEnd
