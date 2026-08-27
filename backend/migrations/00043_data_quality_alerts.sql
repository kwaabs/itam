-- +goose Up
-- +goose StatementBegin

-- Data quality gap alerts are a core hygiene feature: enable the weekly check,
-- seed a delivery channel placeholder, and route events to it.
UPDATE meta.scheduled_checks
   SET enabled = true,
       updated_at = now()
 WHERE key = 'data-quality-gaps';

INSERT INTO meta.notification_channels (key, name, type, config, enabled)
VALUES (
    'ops-alerts',
    'Ops alerts',
    'webhook',
    '{}',
    false
)
ON CONFLICT (key) DO NOTHING;

INSERT INTO meta.automation_rules (key, name, on_subject, condition, action, enabled, sort)
VALUES (
    'route_data_quality_gaps',
    'Route data quality gap alerts',
    'itam.data_quality.gaps',
    '{}',
    '{"type":"notify","channel":"ops-alerts","template":"[ITAM] {{name}}: {{count}} item(s) need attention. Open Reports → Data quality to fix."}',
    true,
    50
)
ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE meta.scheduled_checks SET enabled = false WHERE key = 'data-quality-gaps';
DELETE FROM meta.automation_rules WHERE key = 'route_data_quality_gaps';
DELETE FROM meta.notification_channels WHERE key = 'ops-alerts';
-- +goose StatementEnd
