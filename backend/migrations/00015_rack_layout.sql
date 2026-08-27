-- +goose Up
-- +goose StatementBegin

-- Floor-plan coordinates for racks within their room, used by the layout canvas.
-- Coordinates are free units (pixels on the floor plan); rotation is in degrees.
ALTER TABLE dcim.racks ADD COLUMN pos_x    numeric(10,2);
ALTER TABLE dcim.racks ADD COLUMN pos_y    numeric(10,2);
ALTER TABLE dcim.racks ADD COLUMN rotation int NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE dcim.racks DROP COLUMN IF EXISTS pos_x;
ALTER TABLE dcim.racks DROP COLUMN IF EXISTS pos_y;
ALTER TABLE dcim.racks DROP COLUMN IF EXISTS rotation;
-- +goose StatementEnd
