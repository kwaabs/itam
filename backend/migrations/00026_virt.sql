-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Virtualization (Step D, part 1). Logical compute layer on top of physical
-- servers. Hosts wrap a physical server asset and declare capacity; VMs declare
-- allocation and (optionally) link to their own asset row. Allocation-vs-capacity
-- rollups are computed in the API.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS virt;

-- VM is a concrete compute type; hosts stay ordinary servers.
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT 'vm', 'Virtual Machine', p.id, 'hardware.computer.vm', false, 4
FROM meta.asset_types p WHERE p.key = 'computer'
ON CONFLICT (key) DO NOTHING;
UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE key = 'vm';

CREATE TABLE virt.clusters (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    hypervisor  text,                                  -- esxi|hyperv|proxmox|kvm|nutanix
    ha          boolean NOT NULL DEFAULT false,
    drs         boolean NOT NULL DEFAULT false,
    attributes  jsonb  NOT NULL DEFAULT '{}',
    notes       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE virt.hosts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid NOT NULL UNIQUE REFERENCES core.assets(id) ON DELETE CASCADE,
    cluster_id  uuid REFERENCES virt.clusters(id) ON DELETE SET NULL,
    hypervisor  text,
    cpu_cores   int,
    cpu_threads int,
    ram_gb      numeric(10,2),
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX virt_hosts_cluster_idx ON virt.hosts (cluster_id);

CREATE TABLE virt.vms (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    host_id     uuid REFERENCES virt.hosts(id) ON DELETE SET NULL,
    cluster_id  uuid REFERENCES virt.clusters(id) ON DELETE SET NULL,
    name        text NOT NULL,
    vcpus       int,
    ram_gb      numeric(10,2),
    disk_gb     numeric(12,2),
    power_state text NOT NULL DEFAULT 'unknown',       -- running|stopped|suspended|unknown
    guest_os    text,
    ip          text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX virt_vms_host_idx    ON virt.vms (host_id);
CREATE INDEX virt_vms_cluster_idx ON virt.vms (cluster_id);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA virt TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA virt TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA virt TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA virt GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA virt GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('virt.read',   'View clusters, hosts and virtual machines'),
    ('virt.manage', 'Manage clusters, hosts and virtual machines');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'virt.read'),
    ('admin',         'virt.manage'),
    ('asset_manager', 'virt.read'),
    ('asset_manager', 'virt.manage'),
    ('technician',    'virt.read'),
    ('technician',    'virt.manage'),
    ('viewer',        'virt.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('virt.read', 'virt.manage');
DROP SCHEMA IF EXISTS virt CASCADE;
DELETE FROM meta.field_definitions WHERE asset_type_id IN (SELECT id FROM meta.asset_types WHERE key='vm');
DELETE FROM meta.asset_types WHERE key = 'vm';
-- +goose StatementEnd
