-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Configurable per-transition data capture. Each lifecycle transition can
-- declare a small form (vendor/fault/cost for "send to repair", method/wipe
-- certificate/proceeds for "dispose", etc). Captured values are validated at
-- the app layer and stored on core.lifecycle_history.data.
-- ---------------------------------------------------------------------------
CREATE TABLE meta.transition_fields (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transition_id bigint NOT NULL REFERENCES meta.lifecycle_transitions(id) ON DELETE CASCADE,
    key           citext NOT NULL,
    label         text   NOT NULL,
    data_type_id  bigint NOT NULL REFERENCES meta.data_types(id),
    required      boolean NOT NULL DEFAULT false,
    default_value jsonb,
    validation    jsonb  NOT NULL DEFAULT '{}',
    enum_options  jsonb  NOT NULL DEFAULT '[]',
    help_text     text,
    sort          int    NOT NULL DEFAULT 0,
    UNIQUE (transition_id, key)
);
CREATE INDEX transition_fields_transition_idx ON meta.transition_fields (transition_id);

-- Structured values captured when the transition ran.
ALTER TABLE core.lifecycle_history ADD COLUMN data jsonb NOT NULL DEFAULT '{}';

-- --- Seed example transition forms on the hardware lifecycle -----------------
INSERT INTO meta.transition_fields
    (transition_id, key, label, data_type_id, required, enum_options, help_text, sort)
SELECT lt.id, v.key, v.label, dt.id, v.required, v.enum_options::jsonb, v.help_text, v.sort
FROM (VALUES
    ('receive',       'po_reference', 'PO reference', 'text',     false, '[]',
        'Purchase order this delivery fulfills', 1),
    ('receive',       'condition',    'Condition',    'enum',     false,
        '[{"value":"new","label":"New"},{"value":"good","label":"Good"},{"value":"damaged","label":"Damaged"}]',
        NULL, 2),
    ('send_repair',   'vendor',       'Repair vendor','text',     true,  '[]', NULL, 1),
    ('send_repair',   'fault',        'Fault description','textarea', true, '[]', NULL, 2),
    ('send_repair',   'rma',          'RMA number',   'text',     false, '[]', NULL, 3),
    ('send_repair',   'cost',         'Estimated cost','money',    false, '[]', NULL, 4),
    ('return_repair', 'resolution',   'Resolution',   'textarea', false, '[]', NULL, 1),
    ('return_repair', 'cost',         'Final repair cost','money', false, '[]', NULL, 2),
    ('transfer',      'carrier',      'Carrier',      'text',     false, '[]', NULL, 1),
    ('transfer',      'tracking',     'Tracking number','text',   false, '[]', NULL, 2),
    ('retire',        'reason',       'Retirement reason','textarea', true, '[]', NULL, 1),
    ('dispose',       'method',       'Disposal method','enum',   true,
        '[{"value":"sold","label":"Sold"},{"value":"recycled","label":"Recycled"},{"value":"destroyed","label":"Destroyed"},{"value":"returned","label":"Returned to vendor"}]',
        NULL, 1),
    ('dispose',       'data_wiped',   'Data wiped / sanitised','boolean', true, '[]',
        'Confirm storage media was wiped or destroyed', 2),
    ('dispose',       'certificate',  'Certificate ref','text',   false, '[]',
        'Certificate of destruction / data wipe reference', 3),
    ('dispose',       'proceeds',     'Proceeds',     'money',    false, '[]',
        'Amount recovered (sale / scrap value)', 4)
) AS v(trans_key, key, label, dt_key, required, enum_options, help_text, sort)
JOIN meta.lifecycles lc ON lc.key = 'hardware'
JOIN meta.lifecycle_transitions lt ON lt.lifecycle_id = lc.id AND lt.key = v.trans_key
JOIN meta.data_types dt ON dt.key = v.dt_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE core.lifecycle_history DROP COLUMN IF EXISTS data;
DROP TABLE IF EXISTS meta.transition_fields;
-- +goose StatementEnd
