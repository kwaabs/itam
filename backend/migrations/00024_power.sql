-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- DC power chain (Step B). Models the path: power feed (utility/UPS/generator)
-- -> PDU in a rack -> mounted devices' draw. Per-device draw is read from the
-- asset's attributes.power_watts (the field added in step A), so nothing is
-- duplicated. Rack/feed capacity vs draw rollups are computed in the API.
-- ---------------------------------------------------------------------------

ALTER TABLE dcim.racks ADD COLUMN IF NOT EXISTS power_capacity_w int;

-- A power feed into a room/site (e.g. Feed A from utility, Feed B from UPS).
CREATE TABLE dcim.power_feeds (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    source      text   NOT NULL DEFAULT 'utility',  -- utility|ups|generator|other
    capacity_w  int,
    voltage     int,
    phase       text,                                -- 1P|3P
    redundancy  text,                                -- A|B|N+1|2N|...
    attributes  jsonb  NOT NULL DEFAULT '{}',
    notes       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX power_feeds_location_idx ON dcim.power_feeds (location_id);

-- A PDU lives in a rack, optionally is itself a tracked asset, and draws from a
-- feed. Its capacity_w caps what the rack can pull on that path.
CREATE TABLE dcim.pdus (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    rack_id     uuid NOT NULL REFERENCES dcim.racks(id) ON DELETE CASCADE,
    asset_id    uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    feed_id     uuid REFERENCES dcim.power_feeds(id) ON DELETE SET NULL,
    capacity_w  int,
    attributes  jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pdus_rack_idx ON dcim.pdus (rack_id);
CREATE INDEX pdus_feed_idx ON dcim.pdus (feed_id);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON dcim.power_feeds TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON dcim.pdus        TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dcim.pdus;
DROP TABLE IF EXISTS dcim.power_feeds;
ALTER TABLE dcim.racks DROP COLUMN IF EXISTS power_capacity_w;
-- +goose StatementEnd
