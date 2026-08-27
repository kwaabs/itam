-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- Network & ports depth: metadata-driven port profiles, DHCP scopes,
-- netcfg address objects / routes / NAT, interface<->IPAM link, patch panels.
-- ===========================================================================

-- --- Port profiles: per asset-type templates to auto-generate dcim.ports ----
CREATE TABLE IF NOT EXISTS meta.port_profiles (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_type_id bigint NOT NULL REFERENCES meta.asset_types(id) ON DELETE CASCADE,
    label         text NOT NULL,
    port_type     text NOT NULL DEFAULT 'ethernet',
    speed         text,
    name_prefix   text NOT NULL DEFAULT '',
    start_index   int  NOT NULL DEFAULT 1,
    count         int  NOT NULL DEFAULT 1 CHECK (count BETWEEN 1 AND 1024),
    sort          int  NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS port_profiles_type_idx ON meta.port_profiles (asset_type_id);

-- --- DHCP scopes (one or more per subnet) -----------------------------------
CREATE TABLE IF NOT EXISTS ipam.dhcp_scopes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subnet_id   uuid NOT NULL REFERENCES ipam.subnets(id) ON DELETE CASCADE,
    name        text NOT NULL,
    range_start inet NOT NULL,
    range_end   inet NOT NULL,
    gateway     inet,
    dns         text,
    domain      text,
    lease_hours int  NOT NULL DEFAULT 24,
    enabled     boolean NOT NULL DEFAULT true,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS dhcp_scopes_subnet_idx ON ipam.dhcp_scopes (subnet_id);

-- --- Link netcfg interfaces to an IPAM address ------------------------------
ALTER TABLE netcfg.interfaces
    ADD COLUMN IF NOT EXISTS ip_id uuid REFERENCES ipam.ip_addresses(id) ON DELETE SET NULL;

-- --- netcfg: address objects, routes, NAT rules -----------------------------
CREATE TABLE IF NOT EXISTS netcfg.address_objects (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    name        text NOT NULL,
    kind        text NOT NULL DEFAULT 'host',   -- host|network|range|fqdn|group
    value       text NOT NULL,
    description text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_id, name)
);
CREATE INDEX IF NOT EXISTS address_objects_asset_idx ON netcfg.address_objects (asset_id);

CREATE TABLE IF NOT EXISTS netcfg.routes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    destination text NOT NULL,    -- CIDR or 'default'
    next_hop    text,
    interface   text,
    metric      int,
    enabled     boolean NOT NULL DEFAULT true,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS routes_asset_idx ON netcfg.routes (asset_id);

CREATE TABLE IF NOT EXISTS netcfg.nat_rules (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id      uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    seq           int  NOT NULL DEFAULT 0,
    name          text,
    nat_type      text NOT NULL DEFAULT 'source',  -- source|destination|static
    orig_src      text,
    orig_dst      text,
    orig_service  text,
    trans_src     text,
    trans_dst     text,
    trans_service text,
    enabled       boolean NOT NULL DEFAULT true,
    description   text,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS nat_rules_asset_idx ON netcfg.nat_rules (asset_id);

-- --- Patch panel asset type + fields ---------------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT 'patch_panel', 'Patch Panel', p.id, 'hardware.network.patch_panel'::ltree, false, 5
FROM meta.asset_types p WHERE p.key = 'network'
ON CONFLICT (key) DO NOTHING;

UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE key = 'patch_panel';

INSERT INTO meta.field_definitions (asset_type_id, key, label, data_type_id, required, unit_id, sort)
SELECT at.id, v.key, v.label, dt.id, false, u.id, v.sort
FROM (VALUES
    ('patch_panel', 'ports',      'Port Count',          'number', 'count', 1),
    ('patch_panel', 'rack_units', 'Rack Units',          'number', 'ru',    2),
    ('patch_panel', 'category',   'Category (Cat6/Fiber)','text',  NULL,    3)
) AS v(type_key, key, label, dt_key, unit_key, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key
ON CONFLICT DO NOTHING;

-- --- Seed default port profiles --------------------------------------------
INSERT INTO meta.port_profiles (asset_type_id, label, port_type, speed, name_prefix, start_index, count, sort)
SELECT at.id, v.label, v.port_type, v.speed, v.name_prefix, v.start_index, v.count, v.sort
FROM (VALUES
    ('switch',        'Access ports',    'ethernet',    '1G',  'Gi1/0/', 1, 48, 1),
    ('switch',        'Uplinks',         'sfp_plus',    '10G', 'Te1/1/', 1,  4, 2),
    ('router',        'Interfaces',      'ethernet',    '1G',  'Gi0/',   0,  4, 1),
    ('firewall',      'Data interfaces', 'ethernet',    '1G',  'eth',    1,  8, 1),
    ('load_balancer', 'Interfaces',      'ethernet',    '1G',  'eth',    1,  4, 1),
    ('patch_panel',   'Front',           'passthrough', NULL,  'F',      1, 24, 1),
    ('patch_panel',   'Rear',            'passthrough', NULL,  'R',      1, 24, 2)
) AS v(type_key, label, port_type, speed, name_prefix, start_index, count, sort)
JOIN meta.asset_types at ON at.key = v.type_key
WHERE NOT EXISTS (SELECT 1 FROM meta.port_profiles pp WHERE pp.asset_type_id = at.id);

-- --- Grants -----------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.port_profiles TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ipam.dhcp_scopes TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.address_objects TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.routes TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.nat_rules TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS netcfg.nat_rules;
DROP TABLE IF EXISTS netcfg.routes;
DROP TABLE IF EXISTS netcfg.address_objects;
ALTER TABLE netcfg.interfaces DROP COLUMN IF EXISTS ip_id;
DROP TABLE IF EXISTS ipam.dhcp_scopes;
DROP TABLE IF EXISTS meta.port_profiles;
DELETE FROM meta.field_definitions WHERE asset_type_id IN (SELECT id FROM meta.asset_types WHERE key='patch_panel');
DELETE FROM meta.asset_types WHERE key='patch_panel';
-- +goose StatementEnd
