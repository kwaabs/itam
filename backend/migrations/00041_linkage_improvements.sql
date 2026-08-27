-- +goose Up
-- +goose StatementBegin

-- Default site/room for an org unit — used when assigning assets to people in that unit.
ALTER TABLE core.org_units
    ADD COLUMN default_location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL;

CREATE INDEX org_units_default_location_idx ON core.org_units (default_location_id);

-- Map external labels (OpManager map/probe names, legacy site codes) to ITAM locations.
CREATE TABLE meta.location_aliases (
    alias       citext PRIMARY KEY,
    location_id uuid   NOT NULL REFERENCES core.locations(id) ON DELETE CASCADE,
    note        text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX location_aliases_location_idx ON meta.location_aliases (location_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON meta.location_aliases TO itam_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS meta.location_aliases;
ALTER TABLE core.org_units DROP COLUMN IF EXISTS default_location_id;
-- +goose StatementEnd
