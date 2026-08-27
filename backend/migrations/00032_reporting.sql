-- +goose Up
-- +goose StatementBegin

-- Reporting / analytics dashboards are read-only aggregates over existing data.
INSERT INTO iam.permissions (key, description) VALUES
    ('report.read', 'View reporting and analytics dashboards')
ON CONFLICT (key) DO NOTHING;

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('admin',         'report.read'),
    ('asset_manager', 'report.read'),
    ('technician',    'report.read'),
    ('viewer',        'report.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key = 'report.read';
-- +goose StatementEnd
