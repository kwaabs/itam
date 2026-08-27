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

ALTER TABLE netcfg.interfaces
    ADD COLUMN IF NOT EXISTS ip_id uuid REFERENCES ipam.ip_addresses(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS netcfg.address_objects (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    name        text NOT NULL,
    kind        text NOT NULL DEFAULT 'host',
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
    destination text NOT NULL,
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
    nat_type      text NOT NULL DEFAULT 'source',
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
    ('patch_panel', 'ports',      'Port Count',           'number', 'count', 1),
    ('patch_panel', 'rack_units', 'Rack Units',           'number', 'ru',    2),
    ('patch_panel', 'category',   'Category (Cat6/Fiber)','text',   NULL,    3)
) AS v(type_key, key, label, dt_key, unit_key, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key
ON CONFLICT DO NOTHING;

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

GRANT SELECT, INSERT, UPDATE, DELETE ON meta.port_profiles TO itam_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ipam.dhcp_scopes TO itam_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.address_objects TO itam_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.routes TO itam_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON netcfg.nat_rules TO itam_app;

INSERT INTO public.goose_db_version (version_id, is_applied) VALUES (37, true) ON CONFLICT DO NOTHING;

SELECT 'port_profiles' AS t, count(*) FROM meta.port_profiles
UNION ALL SELECT 'dhcp_scopes', count(*) FROM ipam.dhcp_scopes
UNION ALL SELECT 'address_objects', count(*) FROM netcfg.address_objects
UNION ALL SELECT 'patch_panel_type', count(*) FROM meta.asset_types WHERE key='patch_panel';
