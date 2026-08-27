-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Storage (Step D, part 2). Storage arrays / NAS are ordinary assets (types
-- added in step A). This adds the logical layer: pools carve raw into usable,
-- volumes/LUNs are provisioned from pools and optionally attached to a consumer
-- asset. Capacity rollups (usable vs allocated vs used) are computed in the API.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS stor;

CREATE TABLE stor.pools (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key            citext NOT NULL UNIQUE,
    name           text   NOT NULL,
    array_asset_id uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    raid           text,
    raw_gb         numeric(14,2),
    usable_gb      numeric(14,2),
    attributes     jsonb  NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stor_pools_array_idx ON stor.pools (array_asset_id);

CREATE TABLE stor.volumes (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key               citext NOT NULL UNIQUE,
    name              text   NOT NULL,
    array_asset_id    uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    pool_id           uuid REFERENCES stor.pools(id) ON DELETE SET NULL,
    capacity_gb       numeric(14,2),
    used_gb           numeric(14,2),
    protocol          text,                            -- iscsi|fc|nfs|smb|local
    attached_asset_id uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    attributes        jsonb NOT NULL DEFAULT '{}',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stor_volumes_array_idx    ON stor.volumes (array_asset_id);
CREATE INDEX stor_volumes_pool_idx     ON stor.volumes (pool_id);
CREATE INDEX stor_volumes_attached_idx ON stor.volumes (attached_asset_id);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA stor TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA stor TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA stor TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA stor GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA stor GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('storage.read',   'View storage pools and volumes'),
    ('storage.manage', 'Manage storage pools and volumes');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'storage.read'),
    ('admin',         'storage.manage'),
    ('asset_manager', 'storage.read'),
    ('asset_manager', 'storage.manage'),
    ('technician',    'storage.read'),
    ('technician',    'storage.manage'),
    ('viewer',        'storage.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('storage.read', 'storage.manage');
DROP SCHEMA IF EXISTS stor CASCADE;
-- +goose StatementEnd
