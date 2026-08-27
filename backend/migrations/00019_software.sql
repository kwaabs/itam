-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- swm: software & licensing. A catalog of software titles (+ versions), the
-- installations of those titles on assets, license pools (with seat counts)
-- and the assignment of seats to assets/people for compliance tracking.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS swm;

CREATE TABLE swm.software (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    publisher   text,
    category    text NOT NULL DEFAULT 'application', -- application|os|driver|utility|firmware
    description text,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name, publisher)
);

CREATE TABLE swm.software_versions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    software_id  uuid NOT NULL REFERENCES swm.software(id) ON DELETE CASCADE,
    version      text NOT NULL,
    release_date date,
    eol_date     date,
    attributes   jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (software_id, version)
);

CREATE TABLE swm.installations (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    software_id  uuid NOT NULL REFERENCES swm.software(id) ON DELETE CASCADE,
    version_id   uuid REFERENCES swm.software_versions(id) ON DELETE SET NULL,
    asset_id     uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    person_id    uuid REFERENCES core.people(id) ON DELETE SET NULL,
    source       text NOT NULL DEFAULT 'manual', -- manual|intune|defender|import
    installed_at timestamptz,
    attributes   jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (software_id, asset_id)
);
CREATE INDEX installations_asset_idx    ON swm.installations (asset_id);
CREATE INDEX installations_software_idx ON swm.installations (software_id);

CREATE TABLE swm.licenses (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    software_id   uuid REFERENCES swm.software(id) ON DELETE SET NULL,
    name          text NOT NULL,
    license_key   text,
    license_type  text NOT NULL DEFAULT 'subscription', -- perpetual|subscription|oem|volume|open_source
    seats         int,                                  -- NULL / 0 = unlimited
    vendor_id     uuid REFERENCES proc.vendors(id) ON DELETE SET NULL,
    purchase_cost numeric(14,2),
    currency      text NOT NULL DEFAULT 'USD',
    start_date    date,
    expiry_date   date,
    notes         text,
    attributes    jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX licenses_software_idx ON swm.licenses (software_id);

CREATE TABLE swm.license_assignments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    license_id      uuid NOT NULL REFERENCES swm.licenses(id) ON DELETE CASCADE,
    asset_id        uuid REFERENCES core.assets(id) ON DELETE CASCADE,
    person_id       uuid REFERENCES core.people(id) ON DELETE CASCADE,
    installation_id uuid REFERENCES swm.installations(id) ON DELETE SET NULL,
    notes           text,
    assigned_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (asset_id IS NOT NULL OR person_id IS NOT NULL)
);
CREATE INDEX license_assignments_license_idx ON swm.license_assignments (license_id);
CREATE INDEX license_assignments_asset_idx   ON swm.license_assignments (asset_id);

-- --- Least-privilege grants for the app role on the new schema --------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA swm TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA swm TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA swm TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA swm GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA swm GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('software.read',   'View software catalog, installs and licenses'),
    ('software.manage', 'Manage software, installs and licenses');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'software.read'),
    ('admin',         'software.manage'),
    ('asset_manager', 'software.read'),
    ('asset_manager', 'software.manage'),
    ('technician',    'software.read'),
    ('viewer',        'software.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- --- Seed: a couple of common titles ---------------------------------------
INSERT INTO swm.software (name, publisher, category) VALUES
    ('Windows 11 Pro', 'Microsoft', 'os'),
    ('Microsoft 365 Apps', 'Microsoft', 'application')
ON CONFLICT (name, publisher) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('software.read', 'software.manage');
DROP SCHEMA IF EXISTS swm CASCADE;
-- +goose StatementEnd
