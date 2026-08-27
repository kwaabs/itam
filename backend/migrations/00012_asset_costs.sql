-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Lifetime cost ledger (TCO). Amounts are stored positive; net cost treats
-- disposal_proceeds as an inflow (see the API's TCO calculation).
-- ---------------------------------------------------------------------------
CREATE TABLE core.asset_costs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id     uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    kind         text NOT NULL DEFAULT 'other',  -- purchase|repair|upgrade|disposal_proceeds|other
    amount       numeric(14,2) NOT NULL,
    currency     text NOT NULL DEFAULT 'USD',
    incurred_at  date NOT NULL DEFAULT current_date,
    vendor       text,
    reference    text,
    note         text,
    source_table text,
    source_id    uuid,
    created_by   uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX asset_costs_asset_idx ON core.asset_costs (asset_id, incurred_at DESC);
CREATE INDEX asset_costs_kind_idx  ON core.asset_costs (kind);

-- --- Permissions ------------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('cost.read',   'View asset costs'),
    ('cost.manage', 'Record and edit asset costs');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'cost.read'),
    ('admin',         'cost.manage'),
    ('asset_manager', 'cost.read'),
    ('asset_manager', 'cost.manage'),
    ('technician',    'cost.read'),
    ('viewer',        'cost.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('cost.read', 'cost.manage');
DROP TABLE IF EXISTS core.asset_costs;
-- +goose StatementEnd
