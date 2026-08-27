-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- dcim: physical data-center layer. Sites/rooms remain core.locations; this
-- schema adds racks, the mounting of assets into racks (U positions), device
-- ports, and the cables that connect ports. Devices themselves stay as ordinary
-- core.assets rows so they keep their lifecycle, timeline and costs.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS dcim;

CREATE TABLE dcim.racks (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key           citext NOT NULL UNIQUE,
    name          text   NOT NULL,
    -- the room / site this rack physically sits in
    location_id   uuid REFERENCES core.locations(id) ON DELETE RESTRICT,
    -- optional link to the rack as a procurable/tracked asset
    asset_id      uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    u_height      int  NOT NULL DEFAULT 42 CHECK (u_height BETWEEN 1 AND 100),
    starting_unit int  NOT NULL DEFAULT 1,
    desc_units    boolean NOT NULL DEFAULT false,   -- true = numbered top-to-bottom
    width_mm      int,
    depth_mm      int,
    attributes    jsonb NOT NULL DEFAULT '{}',
    notes         text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX racks_location_idx ON dcim.racks (location_id);

-- One asset occupies a contiguous span of rack units on a face. The EXCLUDE
-- constraint makes overlapping mounts impossible per (rack, face).
CREATE TABLE dcim.rack_mounts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rack_id    uuid NOT NULL REFERENCES dcim.racks(id) ON DELETE CASCADE,
    asset_id   uuid NOT NULL UNIQUE REFERENCES core.assets(id) ON DELETE CASCADE,
    position   int  NOT NULL CHECK (position >= 1),         -- lowest U occupied
    u_height   int  NOT NULL DEFAULT 1 CHECK (u_height >= 1),
    face       text NOT NULL DEFAULT 'front',              -- front|rear|full
    mounted_by uuid,
    mounted_at timestamptz NOT NULL DEFAULT now(),
    EXCLUDE USING gist (
        rack_id WITH =,
        face    WITH =,
        int4range(position, position + u_height) WITH &&
    )
);
CREATE INDEX rack_mounts_rack_idx ON dcim.rack_mounts (rack_id);

CREATE TABLE dcim.ports (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id   uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    name       text NOT NULL,
    port_type  text NOT NULL DEFAULT 'ethernet',  -- ethernet|sfp|sfp_plus|qsfp|power|console|usb|other
    speed      text,                              -- e.g. 1G, 10G, 40G
    attributes jsonb NOT NULL DEFAULT '{}',
    sort       int  NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_id, name)
);
CREATE INDEX ports_asset_idx ON dcim.ports (asset_id);

-- A cable joins two ports. A given port participates in at most one cable; that
-- is enforced in the app layer (a port can appear as either end).
CREATE TABLE dcim.connections (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    a_port_id  uuid NOT NULL REFERENCES dcim.ports(id) ON DELETE CASCADE,
    b_port_id  uuid NOT NULL REFERENCES dcim.ports(id) ON DELETE CASCADE,
    cable_type text NOT NULL DEFAULT 'cat6',  -- cat6|fiber|dac|power|other
    label      text,
    length_m   numeric(6,2),
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (a_port_id <> b_port_id),
    UNIQUE (a_port_id),
    UNIQUE (b_port_id)
);

-- --- Least-privilege grants for the app role on the new schema --------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA dcim TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA dcim TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA dcim TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA dcim GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA dcim GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('dcim.read',   'View data-center racks, ports and connections'),
    ('dcim.manage', 'Manage racks, mounting, ports and cabling');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'dcim.read'),
    ('admin',         'dcim.manage'),
    ('asset_manager', 'dcim.read'),
    ('asset_manager', 'dcim.manage'),
    ('technician',    'dcim.read'),
    ('technician',    'dcim.manage'),
    ('viewer',        'dcim.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- --- Seed: a couple of location kinds + a sample room/rack ------------------
-- A server room under HQ and one rack, so the Data Center view isn't empty.
INSERT INTO core.locations (key, name, kind, parent_id, path)
SELECT 'server_room', 'Server Room', 'room', hq.id, hq.path::text::ltree || 'server_room'
FROM core.locations hq WHERE hq.key = 'hq'
ON CONFLICT (key) DO NOTHING;

INSERT INTO dcim.racks (key, name, location_id, u_height)
SELECT 'rack_a1', 'Rack A1', room.id, 42
FROM core.locations room WHERE room.key = 'server_room'
ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('dcim.read', 'dcim.manage');
DROP SCHEMA IF EXISTS dcim CASCADE;
DELETE FROM core.locations WHERE key = 'server_room';
-- +goose StatementEnd
