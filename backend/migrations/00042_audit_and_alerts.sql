-- +goose Up
-- +goose StatementBegin

-- Custody acknowledge (audit-lite sign-off on assign).
ALTER TABLE core.assignments
    ADD COLUMN acknowledged_at timestamptz,
    ADD COLUMN acknowledged_by uuid;

CREATE INDEX assignments_unacked_idx ON core.assignments (asset_id)
    WHERE kind = 'assign' AND returned_at IS NULL AND acknowledged_at IS NULL;

-- Weekly data quality alert (enabled by default; configure ops-alerts channel under Notifications).
INSERT INTO meta.scheduled_checks (key, name, kind, interval_seconds, params, event_subject, enabled)
VALUES (
    'data-quality-gaps',
    'Data quality gaps exceed threshold',
    'data_quality',
    604800,
    '{"gap_threshold": 5, "dedupe": false}',
    'itam.data_quality.gaps',
    true
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM meta.scheduled_checks WHERE key = 'data-quality-gaps';
ALTER TABLE core.assignments DROP COLUMN IF EXISTS acknowledged_by;
ALTER TABLE core.assignments DROP COLUMN IF EXISTS acknowledged_at;
-- +goose StatementEnd
