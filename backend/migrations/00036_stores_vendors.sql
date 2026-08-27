-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Stores are just locations. Add "store" (a stock-holding location) and
-- "department" (a subdivision within a store) to the location-kind metadata.
-- ---------------------------------------------------------------------------
INSERT INTO meta.location_kinds (key, label, sort, geo, container) VALUES
    ('store',      'Store / Stockroom', 55, false, true),
    ('department', 'Department',        58, false, true)
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Vendor status is metadata-driven (active / preferred / inactive / blacklisted
-- ...). blocks_orders flags statuses that should prevent new purchasing.
-- ---------------------------------------------------------------------------
CREATE TABLE meta.vendor_statuses (
    key           text PRIMARY KEY,
    label         text NOT NULL,
    color         text NOT NULL DEFAULT '',
    blocks_orders boolean NOT NULL DEFAULT false,
    enabled       boolean NOT NULL DEFAULT true,
    sort          int NOT NULL DEFAULT 100
);

INSERT INTO meta.vendor_statuses (key, label, color, blocks_orders, sort) VALUES
    ('active',      'Active',      '#22c55e', false, 1),
    ('preferred',   'Preferred',   '#3b82f6', false, 2),
    ('on_hold',     'On hold',     '#f59e0b', true,  3),
    ('inactive',    'Inactive',    '#6b7280', true,  4),
    ('blacklisted', 'Blacklisted', '#ef4444', true,  5)
ON CONFLICT (key) DO NOTHING;

-- Vendor status column, backfilled from the legacy is_active flag.
ALTER TABLE proc.vendors ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
UPDATE proc.vendors SET status = CASE WHEN is_active THEN 'active' ELSE 'inactive' END;

DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.vendor_statuses TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE proc.vendors DROP COLUMN IF EXISTS status;
DROP TABLE IF EXISTS meta.vendor_statuses;
DELETE FROM meta.location_kinds WHERE key IN ('store', 'department');
-- +goose StatementEnd
