-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- DC device types (Step A). Pure metadata: makes firewalls, load balancers,
-- power and storage gear first-class asset types with sensible field sets.
-- Everything here is editable at runtime via the metadata admin API.
-- ---------------------------------------------------------------------------

-- --- Extra units used by the new fields -------------------------------------
INSERT INTO meta.units (key, label, symbol) VALUES
    ('amp',  'Amperes',           'A'),
    ('volt', 'Volts',             'V'),
    ('va',   'Volt-amperes',      'VA'),
    ('gbps', 'Gigabits / second', 'Gbps'),
    ('min',  'Minutes',           'min')
ON CONFLICT (key) DO NOTHING;

-- --- New abstract branches: power + storage ---------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, v.abstract, v.sort
FROM (VALUES
    ('power',   'Power Equipment', 'hardware', 'hardware.power',   true, 5),
    ('storage', 'Storage',         'hardware', 'hardware.storage', true, 6)
) AS v(key, name, parent_key, path, abstract, sort)
JOIN meta.asset_types p ON p.key = v.parent_key
ON CONFLICT (key) DO NOTHING;

-- --- Concrete device types --------------------------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, v.abstract, v.sort
FROM (VALUES
    ('firewall',      'Firewall',       'network', 'hardware.network.firewall',       false, 3),
    ('load_balancer', 'Load Balancer',  'network', 'hardware.network.load_balancer',  false, 4),
    ('pdu',           'PDU',            'power',   'hardware.power.pdu',               false, 1),
    ('ups',           'UPS',            'power',   'hardware.power.ups',               false, 2),
    ('storage_array', 'Storage Array',  'storage', 'hardware.storage.storage_array',  false, 1),
    ('nas',           'NAS',            'storage', 'hardware.storage.nas',            false, 2)
) AS v(key, name, parent_key, path, abstract, sort)
JOIN meta.asset_types p ON p.key = v.parent_key
ON CONFLICT (key) DO NOTHING;

-- All new physical types follow the hardware lifecycle.
UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE key IN ('firewall', 'load_balancer', 'pdu', 'ups', 'storage_array', 'nas');

-- --- Field sets -------------------------------------------------------------
INSERT INTO meta.field_definitions
    (asset_type_id, key, label, data_type_id, required, unit_id, sort)
SELECT at.id, v.key, v.label, dt.id, v.required, u.id, v.sort
FROM (VALUES
    ('firewall',      'mgmt_ip',       'Management IP',     'text',    false, NULL,   1),
    ('firewall',      'throughput',    'Throughput',        'number',  false, 'gbps', 2),
    ('firewall',      'ha_role',       'HA Role',           'text',    false, NULL,   3),
    ('firewall',      'firmware',      'Firmware',          'text',    false, NULL,   4),
    ('firewall',      'os',            'OS / Version',      'text',    false, NULL,   5),
    ('firewall',      'rack_units',    'Rack Units',        'number',  false, 'ru',   6),
    ('firewall',      'power_watts',   'Power Draw',        'number',  false, 'watt', 7),

    ('load_balancer', 'mgmt_ip',       'Management IP',     'text',    false, NULL,   1),
    ('load_balancer', 'throughput',    'Throughput',        'number',  false, 'gbps', 2),
    ('load_balancer', 'ha_role',       'HA Role',           'text',    false, NULL,   3),
    ('load_balancer', 'firmware',      'Firmware',          'text',    false, NULL,   4),
    ('load_balancer', 'rack_units',    'Rack Units',        'number',  false, 'ru',   5),
    ('load_balancer', 'power_watts',   'Power Draw',        'number',  false, 'watt', 6),

    ('pdu',           'outlets',       'Outlets',           'number',  false, 'count',1),
    ('pdu',           'input_voltage', 'Input Voltage',     'number',  false, 'volt', 2),
    ('pdu',           'max_amps',      'Max Current',       'number',  false, 'amp',  3),
    ('pdu',           'phase',         'Phase',             'text',    false, NULL,   4),
    ('pdu',           'feed',          'Feed (A/B)',        'text',    false, NULL,   5),
    ('pdu',           'monitored',     'Metered / Monitored','boolean',false, NULL,   6),
    ('pdu',           'rack_units',    'Rack Units',        'number',  false, 'ru',   7),

    ('ups',           'capacity_va',   'Capacity',          'number',  false, 'va',   1),
    ('ups',           'output_watts',  'Output',            'number',  false, 'watt', 2),
    ('ups',           'runtime_min',   'Runtime',           'number',  false, 'min',  3),
    ('ups',           'battery_date',  'Battery Installed', 'date',    false, NULL,   4),
    ('ups',           'rack_units',    'Rack Units',        'number',  false, 'ru',   5),

    ('storage_array', 'mgmt_ip',       'Management IP',     'text',    false, NULL,   1),
    ('storage_array', 'raw_tb',        'Raw Capacity',      'number',  false, 'tb',   2),
    ('storage_array', 'usable_tb',     'Usable Capacity',   'number',  false, 'tb',   3),
    ('storage_array', 'protocol',      'Protocol',          'text',    false, NULL,   4),
    ('storage_array', 'controllers',   'Controllers',       'number',  false, 'count',5),
    ('storage_array', 'rack_units',    'Rack Units',        'number',  false, 'ru',   6),
    ('storage_array', 'power_watts',   'Power Draw',        'number',  false, 'watt', 7),

    ('nas',           'mgmt_ip',       'Management IP',     'text',    false, NULL,   1),
    ('nas',           'usable_tb',     'Usable Capacity',   'number',  false, 'tb',   2),
    ('nas',           'protocol',      'Protocol',          'text',    false, NULL,   3),
    ('nas',           'bays',          'Drive Bays',        'number',  false, 'count',4)
) AS v(type_key, key, label, dt_key, required, unit_key, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM meta.field_definitions
 WHERE asset_type_id IN (SELECT id FROM meta.asset_types
   WHERE key IN ('firewall','load_balancer','pdu','ups','storage_array','nas'));
DELETE FROM meta.asset_types
 WHERE key IN ('firewall','load_balancer','pdu','ups','storage_array','nas','power','storage');
DELETE FROM meta.units WHERE key IN ('amp','volt','va','gbps','min');
-- +goose StatementEnd
