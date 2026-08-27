-- +goose Up
-- +goose StatementBegin

-- Spatial support (PostGIS ships with the Supabase image). Installed into the
-- default schema so ST_* functions resolve on the app role's search_path.
CREATE EXTENSION IF NOT EXISTS postgis;

-- ---------------------------------------------------------------------------
-- Location taxonomy as configuration (nothing hardcoded): region -> site ->
-- building -> floor -> room -> zone -> row -> rack -> slot. `geo` flags the
-- kinds for which map coordinates are meaningful; `container` whether the kind
-- can hold children.
-- ---------------------------------------------------------------------------
CREATE TABLE meta.location_kinds (
    key       citext PRIMARY KEY,
    label     text    NOT NULL,
    sort      int     NOT NULL DEFAULT 0,
    geo       boolean NOT NULL DEFAULT false,
    container boolean NOT NULL DEFAULT true
);

INSERT INTO meta.location_kinds (key, label, sort, geo, container) VALUES
    ('region',   'Region',    10, true,  true),
    ('site',     'Site / DC', 20, true,  true),
    ('building', 'Building',  30, true,  true),
    ('floor',    'Floor',     40, false, true),
    ('room',     'Room',      50, false, true),
    ('zone',     'Zone',      60, false, true),
    ('row',      'Row',       70, false, true),
    ('rack',     'Rack',      80, false, true),
    ('slot',     'Slot',      90, false, false);

GRANT SELECT, INSERT, UPDATE, DELETE ON meta.location_kinds TO itam_app;

-- ---------------------------------------------------------------------------
-- Site/building metadata + geography on the existing location tree. We keep a
-- single tree (it powers ltree RBAC scoping) rather than a separate sites table.
-- ---------------------------------------------------------------------------
ALTER TABLE core.locations ADD COLUMN dr_role  text;                       -- primary | dr | edge | colo ...
ALTER TABLE core.locations ADD COLUMN tier     text;                       -- tier-1..tier-4 (Uptime), free text
ALTER TABLE core.locations ADD COLUMN timezone text;
ALTER TABLE core.locations ADD COLUMN address  text;
ALTER TABLE core.locations ADD COLUMN geog     geography(Point, 4326);

CREATE INDEX locations_geog_gix ON core.locations USING gist (geog);
CREATE INDEX locations_kind_idx ON core.locations (kind);

-- ---------------------------------------------------------------------------
-- "Last seen" truth on the asset (denormalised; fed by events / future
-- Intune/Defender connectors). Detailed time-series lands in a Timescale
-- hypertable in a later phase.
-- ---------------------------------------------------------------------------
ALTER TABLE core.assets ADD COLUMN last_seen_at       timestamptz;
ALTER TABLE core.assets ADD COLUMN last_seen_location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL;
ALTER TABLE core.assets ADD COLUMN last_seen_source   text;                -- manual | intune | defender | agent | scan
CREATE INDEX assets_last_seen_idx ON core.assets (last_seen_at);

-- ---------------------------------------------------------------------------
-- Enrich the seed so the map has something to show out of the box.
-- ---------------------------------------------------------------------------
UPDATE core.locations
   SET tier = 'tier-3', dr_role = 'primary', timezone = 'Europe/London',
       address = '1 HQ Way, London, UK',
       geog = ST_SetSRID(ST_MakePoint(-0.1276, 51.5072), 4326)::geography
 WHERE key = 'hq';

INSERT INTO core.locations (key, name, kind, parent_id, path, dr_role, tier, timezone, address, geog)
VALUES (
    'dc_dr', 'DR Site - Frankfurt', 'site', NULL, 'dc_dr',
    'dr', 'tier-3', 'Europe/Berlin', 'Kleyerstrasse 90, Frankfurt, DE',
    ST_SetSRID(ST_MakePoint(8.6821, 50.1109), 4326)::geography
) ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.locations WHERE key = 'dc_dr';

DROP INDEX IF EXISTS core.assets_last_seen_idx;
ALTER TABLE core.assets DROP COLUMN IF EXISTS last_seen_source;
ALTER TABLE core.assets DROP COLUMN IF EXISTS last_seen_location_id;
ALTER TABLE core.assets DROP COLUMN IF EXISTS last_seen_at;

DROP INDEX IF EXISTS core.locations_kind_idx;
DROP INDEX IF EXISTS core.locations_geog_gix;
ALTER TABLE core.locations DROP COLUMN IF EXISTS geog;
ALTER TABLE core.locations DROP COLUMN IF EXISTS address;
ALTER TABLE core.locations DROP COLUMN IF EXISTS timezone;
ALTER TABLE core.locations DROP COLUMN IF EXISTS tier;
ALTER TABLE core.locations DROP COLUMN IF EXISTS dr_role;

DROP TABLE IF EXISTS meta.location_kinds;
-- +goose StatementEnd
