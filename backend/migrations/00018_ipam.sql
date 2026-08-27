-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- ipam: IP address management. VLANs, subnets (CIDR) and individual IP
-- addresses, which can be bound to an asset and/or a specific device port.
-- Uses native postgres network types (cidr/inet/macaddr) for correctness.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS ipam;

CREATE TABLE ipam.vlans (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    vlan_id     int  NOT NULL CHECK (vlan_id BETWEEN 1 AND 4094),
    name        text NOT NULL,
    description text,
    location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vlans_location_idx ON ipam.vlans (location_id);

CREATE TABLE ipam.subnets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr        cidr NOT NULL UNIQUE,
    name        text NOT NULL,
    vlan_id     uuid REFERENCES ipam.vlans(id) ON DELETE SET NULL,
    location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    gateway     inet,
    description text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subnets_cidr_idx ON ipam.subnets USING gist (cidr inet_ops);

CREATE TABLE ipam.ip_addresses (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    address     inet NOT NULL UNIQUE,
    subnet_id   uuid REFERENCES ipam.subnets(id) ON DELETE SET NULL,
    asset_id    uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    port_id     uuid REFERENCES dcim.ports(id) ON DELETE SET NULL,
    status      text NOT NULL DEFAULT 'allocated',  -- allocated|reserved|deprecated
    dns_name    text,
    mac         macaddr,
    description text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ip_addresses_subnet_idx ON ipam.ip_addresses (subnet_id);
CREATE INDEX ip_addresses_asset_idx  ON ipam.ip_addresses (asset_id);
CREATE INDEX ip_addresses_port_idx   ON ipam.ip_addresses (port_id);

-- --- Least-privilege grants for the app role on the new schema --------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA ipam TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA ipam TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA ipam TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA ipam GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA ipam GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('ipam.read',   'View VLANs, subnets and IP addresses'),
    ('ipam.manage', 'Manage VLANs, subnets and IP addresses');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'ipam.read'),
    ('admin',         'ipam.manage'),
    ('asset_manager', 'ipam.read'),
    ('asset_manager', 'ipam.manage'),
    ('technician',    'ipam.read'),
    ('technician',    'ipam.manage'),
    ('viewer',        'ipam.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- --- Seed: a management subnet under HQ so the view isn't empty -------------
INSERT INTO ipam.vlans (vlan_id, name, description, location_id)
SELECT 10, 'Management', 'Out-of-band / management network', hq.id
FROM core.locations hq WHERE hq.key = 'hq'
ON CONFLICT DO NOTHING;

INSERT INTO ipam.subnets (cidr, name, gateway, location_id, vlan_id)
SELECT '10.10.0.0/24'::cidr, 'HQ Management', '10.10.0.1'::inet, hq.id, v.id
FROM core.locations hq
LEFT JOIN ipam.vlans v ON v.vlan_id = 10
WHERE hq.key = 'hq'
ON CONFLICT (cidr) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('ipam.read', 'ipam.manage');
DROP SCHEMA IF EXISTS ipam CASCADE;
-- +goose StatementEnd
