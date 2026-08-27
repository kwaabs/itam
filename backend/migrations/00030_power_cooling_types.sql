-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Power & cooling support equipment (follow-up to step A). Pure metadata: makes
-- AVRs, transfer switches, generators, transformers and cooling units (CRAC/
-- CRAH, chillers, in-row) first-class asset types so they mount in racks/rooms,
-- join the power chain and roll up in capacity. Editable at runtime.
-- ---------------------------------------------------------------------------

-- --- Extra units ------------------------------------------------------------
INSERT INTO meta.units (key, label, symbol) VALUES
    ('liter', 'Litres',          'L'),
    ('kva',   'Kilovolt-amperes','kVA'),
    ('kw',    'Kilowatts',       'kW'),
    ('cfm',   'Cubic ft / min',  'CFM'),
    ('ms',    'Milliseconds',    'ms')
ON CONFLICT (key) DO NOTHING;

-- --- New abstract branch: cooling -------------------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT 'cooling', 'Cooling Equipment', p.id, 'hardware.cooling', true, 7
FROM meta.asset_types p WHERE p.key = 'hardware'
ON CONFLICT (key) DO NOTHING;

-- --- Concrete electrical types (under power) --------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, false, v.sort
FROM (VALUES
    ('avr',         'Voltage Regulator (AVR)',  'power', 'hardware.power.avr',         3),
    ('ats',         'Transfer Switch (ATS)',    'power', 'hardware.power.ats',         4),
    ('sts',         'Static Transfer Switch',   'power', 'hardware.power.sts',         5),
    ('generator',   'Generator',                'power', 'hardware.power.generator',   6),
    ('transformer', 'Transformer / RPP',        'power', 'hardware.power.transformer', 7)
) AS v(key, name, parent_key, path, sort)
JOIN meta.asset_types p ON p.key = v.parent_key
ON CONFLICT (key) DO NOTHING;

-- --- Concrete cooling types (under cooling) ---------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, false, v.sort
FROM (VALUES
    ('crac',    'CRAC / CRAH Unit', 'cooling', 'hardware.cooling.crac',    1),
    ('chiller', 'Chiller',          'cooling', 'hardware.cooling.chiller', 2),
    ('inrow',   'In-Row Cooler',    'cooling', 'hardware.cooling.inrow',   3)
) AS v(key, name, parent_key, path, sort)
JOIN meta.asset_types p ON p.key = v.parent_key
ON CONFLICT (key) DO NOTHING;

-- All follow the hardware lifecycle.
UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE key IN ('avr', 'ats', 'sts', 'generator', 'transformer', 'crac', 'chiller', 'inrow');

-- --- Field sets -------------------------------------------------------------
INSERT INTO meta.field_definitions
    (asset_type_id, key, label, data_type_id, required, unit_id, sort)
SELECT at.id, v.key, v.label, dt.id, v.required, u.id, v.sort
FROM (VALUES
    ('avr',         'input_voltage',  'Input Voltage',    'number',  false, 'volt', 1),
    ('avr',         'output_voltage', 'Output Voltage',   'number',  false, 'volt', 2),
    ('avr',         'capacity_va',    'Capacity',         'number',  false, 'kva',  3),
    ('avr',         'rack_units',     'Rack Units',       'number',  false, 'ru',   4),
    ('avr',         'power_watts',    'Power Draw',       'number',  false, 'watt', 5),

    ('ats',         'max_amps',       'Max Current',      'number',  false, 'amp',  1),
    ('ats',         'input_voltage',  'Input Voltage',    'number',  false, 'volt', 2),
    ('ats',         'sources',        'Input Sources',    'number',  false, 'count',3),
    ('ats',         'monitored',      'Monitored',        'boolean', false, NULL,   4),
    ('ats',         'rack_units',     'Rack Units',       'number',  false, 'ru',   5),

    ('sts',         'max_amps',       'Max Current',      'number',  false, 'amp',  1),
    ('sts',         'input_voltage',  'Input Voltage',    'number',  false, 'volt', 2),
    ('sts',         'transfer_ms',    'Transfer Time',    'number',  false, 'ms',   3),
    ('sts',         'rack_units',     'Rack Units',       'number',  false, 'ru',   4),

    ('generator',   'output_kw',      'Output',           'number',  false, 'kw',   1),
    ('generator',   'fuel_type',      'Fuel Type',        'text',    false, NULL,   2),
    ('generator',   'fuel_capacity',  'Fuel Capacity',    'number',  false, 'liter',3),
    ('generator',   'runtime_min',    'Runtime at Load',  'number',  false, 'min',  4),
    ('generator',   'autostart',      'Auto-start',       'boolean', false, NULL,   5),

    ('transformer', 'rating_kva',     'Rating',           'number',  false, 'kva',  1),
    ('transformer', 'primary_v',      'Primary Voltage',  'number',  false, 'volt', 2),
    ('transformer', 'secondary_v',    'Secondary Voltage','number',  false, 'volt', 3),

    ('crac',        'cooling_kw',     'Cooling Capacity', 'number',  false, 'kw',   1),
    ('crac',        'airflow_cfm',    'Airflow',          'number',  false, 'cfm',  2),
    ('crac',        'type',           'Type (DX/CW)',     'text',    false, NULL,   3),
    ('crac',        'rack_units',     'Rack Units',       'number',  false, 'ru',   4),
    ('crac',        'power_watts',    'Power Draw',       'number',  false, 'watt', 5),

    ('chiller',     'cooling_kw',     'Cooling Capacity', 'number',  false, 'kw',   1),
    ('chiller',     'refrigerant',    'Refrigerant',      'text',    false, NULL,   2),
    ('chiller',     'power_watts',    'Power Draw',       'number',  false, 'watt', 3),

    ('inrow',       'cooling_kw',     'Cooling Capacity', 'number',  false, 'kw',   1),
    ('inrow',       'airflow_cfm',    'Airflow',          'number',  false, 'cfm',  2),
    ('inrow',       'rack_units',     'Rack Units',       'number',  false, 'ru',   3),
    ('inrow',       'power_watts',    'Power Draw',       'number',  false, 'watt', 4)
) AS v(type_key, key, label, dt_key, required, unit_key, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM meta.field_definitions WHERE asset_type_id IN (
  SELECT id FROM meta.asset_types
  WHERE key IN ('avr','ats','sts','generator','transformer','crac','chiller','inrow'));
DELETE FROM meta.asset_types
  WHERE key IN ('avr','ats','sts','generator','transformer','crac','chiller','inrow','cooling');
DELETE FROM meta.units WHERE key IN ('liter','kva','kw','cfm','ms');
-- +goose StatementEnd
