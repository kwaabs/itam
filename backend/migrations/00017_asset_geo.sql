-- +goose Up
-- +goose StatementBegin

-- Optional per-asset point. Most assets inherit coordinates from their location,
-- but mobile / fixed-GPS / sensor assets can carry their own. The asset map view
-- uses COALESCE(asset.geog, location.geog).
ALTER TABLE core.assets ADD COLUMN geog geography(Point, 4326);
CREATE INDEX assets_geog_gix ON core.assets USING gist (geog);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS core.assets_geog_gix;
ALTER TABLE core.assets DROP COLUMN IF EXISTS geog;
-- +goose StatementEnd
