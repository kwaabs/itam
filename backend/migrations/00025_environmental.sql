-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Environmental monitoring (Step C). Sensors are scoped to a location/rack and
-- optionally backed by a sensor asset. Readings are a time-series; thresholds
-- drive alerting via the normal events -> rules -> notify pipeline. A plain
-- table + (sensor, ts) index is plenty; if the TimescaleDB extension is present
-- it can later be promoted to a hypertable without schema changes.
-- ---------------------------------------------------------------------------

CREATE TABLE dcim.sensors (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key           citext NOT NULL UNIQUE,
    name          text   NOT NULL,
    metric        text   NOT NULL DEFAULT 'temperature', -- temperature|humidity|airflow|power|water
    unit          text,                                   -- C|%|cfm|W
    location_id   uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    rack_id       uuid REFERENCES dcim.racks(id) ON DELETE SET NULL,
    asset_id      uuid REFERENCES core.assets(id) ON DELETE SET NULL,
    min_threshold numeric(10,2),
    max_threshold numeric(10,2),
    enabled       boolean NOT NULL DEFAULT true,
    attributes    jsonb  NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sensors_location_idx ON dcim.sensors (location_id);
CREATE INDEX sensors_rack_idx     ON dcim.sensors (rack_id);

CREATE TABLE dcim.sensor_readings (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sensor_id  uuid NOT NULL REFERENCES dcim.sensors(id) ON DELETE CASCADE,
    value      numeric(12,3) NOT NULL,
    status     text,                                   -- ok|warn|breach (snapshot at write time)
    source     text NOT NULL DEFAULT 'manual',
    ts         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sensor_readings_sensor_ts_idx ON dcim.sensor_readings (sensor_id, ts DESC);

-- --- Least-privilege grants -------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON dcim.sensors         TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON dcim.sensor_readings TO itam_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA dcim TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dcim.sensor_readings;
DROP TABLE IF EXISTS dcim.sensors;
-- +goose StatementEnd
