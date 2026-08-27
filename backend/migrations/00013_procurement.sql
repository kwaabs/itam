-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- proc: lightweight procurement domain. Purchase orders track what is "on
-- order"; receiving a delivery spawns the actual asset rows in core.assets.
-- ---------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS proc;

CREATE TABLE proc.vendors (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key           citext NOT NULL UNIQUE,
    name          text   NOT NULL,
    contact_email citext,
    contact_phone text,
    website       text,
    notes         text,
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE proc.purchase_orders (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    po_number    citext NOT NULL UNIQUE,
    vendor_id    uuid REFERENCES proc.vendors(id) ON DELETE RESTRICT,
    -- draft|approved|ordered|partially_received|received|closed|cancelled
    status       text NOT NULL DEFAULT 'draft',
    requester_id uuid REFERENCES core.people(id) ON DELETE SET NULL,
    approver_id  uuid REFERENCES core.people(id) ON DELETE SET NULL,
    location_id  uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    currency     text NOT NULL DEFAULT 'USD',
    ordered_at   date,
    expected_at  date,
    notes        text,
    created_by   uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX purchase_orders_vendor_idx ON proc.purchase_orders (vendor_id);
CREATE INDEX purchase_orders_status_idx ON proc.purchase_orders (status);

CREATE TABLE proc.po_lines (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id         uuid NOT NULL REFERENCES proc.purchase_orders(id) ON DELETE CASCADE,
    asset_type_id bigint REFERENCES meta.asset_types(id) ON DELETE SET NULL,
    model_id      bigint REFERENCES meta.models(id) ON DELETE SET NULL,
    description   text   NOT NULL,
    quantity      int    NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_cost     numeric(14,2) NOT NULL DEFAULT 0,
    received_qty  int    NOT NULL DEFAULT 0,
    attributes    jsonb  NOT NULL DEFAULT '{}',
    sort          int    NOT NULL DEFAULT 0
);
CREATE INDEX po_lines_po_idx ON proc.po_lines (po_id);

CREATE TABLE proc.receipts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id       uuid REFERENCES proc.purchase_orders(id) ON DELETE SET NULL,
    received_by uuid,
    received_at timestamptz NOT NULL DEFAULT now(),
    location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    notes       text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX receipts_po_idx ON proc.receipts (po_id);

CREATE TABLE proc.receipt_lines (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_id uuid NOT NULL REFERENCES proc.receipts(id) ON DELETE CASCADE,
    po_line_id uuid REFERENCES proc.po_lines(id) ON DELETE SET NULL,
    quantity   int  NOT NULL CHECK (quantity > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX receipt_lines_receipt_idx ON proc.receipt_lines (receipt_id);

-- --- Least-privilege grants for the app role on the new schema --------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA proc TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA proc TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA proc TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA proc GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA proc GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('procurement.read',   'View vendors and purchase orders'),
    ('procurement.manage', 'Manage vendors, purchase orders and receiving');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'procurement.read'),
    ('admin',         'procurement.manage'),
    ('asset_manager', 'procurement.read'),
    ('asset_manager', 'procurement.manage'),
    ('technician',    'procurement.read'),
    ('viewer',        'procurement.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('procurement.read', 'procurement.manage');
DROP SCHEMA IF EXISTS proc CASCADE;
-- +goose StatementEnd
