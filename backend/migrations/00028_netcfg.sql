-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Network / firewall configuration (Step E). Hangs off any network/security
-- asset (firewall, router, switch, load balancer). Models security zones, L3
-- interfaces, an ordered policy/rule base, HA pairs and config-backup snapshots.
-- Everything is data; rendering/diffing of backups happens in the UI.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS netcfg;

CREATE TABLE netcfg.zones (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    name        text NOT NULL,
    description text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_id, name)
);
CREATE INDEX netcfg_zones_asset_idx ON netcfg.zones (asset_id);

CREATE TABLE netcfg.interfaces (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    name        text NOT NULL,
    ip_cidr     text,
    zone_id     uuid REFERENCES netcfg.zones(id) ON DELETE SET NULL,
    vlan        int,
    enabled     boolean NOT NULL DEFAULT true,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_id, name)
);
CREATE INDEX netcfg_interfaces_asset_idx ON netcfg.interfaces (asset_id);

CREATE TABLE netcfg.rules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    seq         int  NOT NULL DEFAULT 0,
    name        text,
    action      text NOT NULL DEFAULT 'allow',          -- allow|deny|reject
    src_zone    text,
    dst_zone    text,
    source      text,
    destination text,
    service     text,
    protocol    text,
    ports       text,
    enabled     boolean NOT NULL DEFAULT true,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX netcfg_rules_asset_seq_idx ON netcfg.rules (asset_id, seq);

CREATE TABLE netcfg.ha_groups (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text NOT NULL,
    mode        text,                                   -- active-passive|active-active
    vip         text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE netcfg.ha_members (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id  uuid NOT NULL REFERENCES netcfg.ha_groups(id) ON DELETE CASCADE,
    asset_id  uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    role      text,                                     -- primary|secondary
    priority  int,
    UNIQUE (group_id, asset_id)
);
CREATE INDEX netcfg_ha_members_asset_idx ON netcfg.ha_members (asset_id);

CREATE TABLE netcfg.config_backups (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    asset_id   uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    taken_at   timestamptz NOT NULL DEFAULT now(),
    source     text NOT NULL DEFAULT 'manual',
    version    text,
    hash       text,
    size_bytes int,
    content    text,
    note       text
);
CREATE INDEX netcfg_backups_asset_idx ON netcfg.config_backups (asset_id, taken_at DESC);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA netcfg TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA netcfg TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA netcfg TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA netcfg GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA netcfg GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('netcfg.read',   'View firewall/network device configuration'),
    ('netcfg.manage', 'Manage firewall/network device configuration');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'netcfg.read'),
    ('admin',         'netcfg.manage'),
    ('asset_manager', 'netcfg.read'),
    ('technician',    'netcfg.read'),
    ('technician',    'netcfg.manage'),
    ('viewer',        'netcfg.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('netcfg.read', 'netcfg.manage');
DROP SCHEMA IF EXISTS netcfg CASCADE;
-- +goose StatementEnd
