-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Capacity planning (Step F). No new entities: capacity dashboards aggregate
-- existing data (rack U space from mounts, power from step B). This only adds an
-- optional rack weight budget; per-device weight is read from the asset's
-- attributes.weight_kg, mirroring how power_watts is used.
-- ---------------------------------------------------------------------------
ALTER TABLE dcim.racks ADD COLUMN IF NOT EXISTS max_weight_kg numeric(10,2);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE dcim.racks DROP COLUMN IF EXISTS max_weight_kg;
-- +goose StatementEnd
