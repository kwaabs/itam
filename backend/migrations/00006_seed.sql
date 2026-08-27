-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- Default configuration. Everything here is ordinary data and can be edited or
-- replaced at runtime via the metadata admin API; it is only a sensible start.
-- ===========================================================================

-- --- Field data types -------------------------------------------------------
INSERT INTO meta.data_types (key, label, value_kind, description) VALUES
    ('text',      'Text',           'text',     'Single line text'),
    ('textarea',  'Long text',      'text',     'Multi line text'),
    ('number',    'Number',         'number',   'Numeric value'),
    ('money',     'Money',          'number',   'Monetary amount'),
    ('boolean',   'Yes / No',       'bool',     'Boolean'),
    ('date',      'Date',           'date',     'Calendar date'),
    ('datetime',  'Date & time',    'datetime', 'Timestamp'),
    ('enum',      'Choice',         'enum',     'One of a fixed set of options'),
    ('reference', 'Reference',      'reference','Reference to another record');

-- --- Units ------------------------------------------------------------------
INSERT INTO meta.units (key, label, symbol) VALUES
    ('gb',    'Gigabytes',  'GB'),
    ('tb',    'Terabytes',  'TB'),
    ('mhz',   'Megahertz',  'MHz'),
    ('ghz',   'Gigahertz',  'GHz'),
    ('watt',  'Watts',      'W'),
    ('ru',    'Rack units', 'U'),
    ('count', 'Count',      'x');

-- --- Hardware lifecycle -----------------------------------------------------
INSERT INTO meta.lifecycles (key, name, description) VALUES
    ('hardware', 'Hardware Lifecycle', 'Procurement to disposal flow for physical assets');

INSERT INTO meta.lifecycle_states (lifecycle_id, key, label, is_initial, is_terminal, color, sort)
SELECT lc.id, v.key, v.label, v.is_initial, v.is_terminal, v.color, v.sort
FROM (VALUES
    ('procured',   'Procured',    true,  false, '#6b7280', 1),
    ('in_stock',   'In Stock',    false, false, '#3b82f6', 2),
    ('deployed',   'Deployed',    false, false, '#8b5cf6', 3),
    ('in_use',     'In Use',      false, false, '#22c55e', 4),
    ('in_repair',  'In Repair',   false, false, '#f59e0b', 5),
    ('in_transit', 'In Transit',  false, false, '#06b6d4', 6),
    ('retired',    'Retired',     false, true,  '#ef4444', 7),
    ('disposed',   'Disposed',    false, true,  '#111827', 8)
) AS v(key, label, is_initial, is_terminal, color, sort)
JOIN meta.lifecycles lc ON lc.key = 'hardware';

INSERT INTO meta.lifecycle_transitions
    (lifecycle_id, key, label, from_state_id, to_state_id, required_permission, emit_subject, sort)
SELECT lc.id, v.key, v.label, fs.id, ts.id, 'asset.transition', 'itam.asset.state_changed', v.sort
FROM (VALUES
    ('receive',       'Receive into stock', 'procured',   'in_stock',  1),
    ('deploy',        'Deploy',             'in_stock',   'deployed',  2),
    ('activate',      'Put into use',       'deployed',   'in_use',    3),
    ('send_repair',   'Send to repair',     'in_use',     'in_repair', 4),
    ('return_repair', 'Return from repair', 'in_repair',  'in_use',    5),
    ('transfer',      'Transfer',           NULL,         'in_transit',6),
    ('arrive',        'Arrive at location', 'in_transit', 'in_stock',  7),
    ('retire',        'Retire',             NULL,         'retired',   8),
    ('dispose',       'Dispose',            'retired',    'disposed',  9)
) AS v(key, label, from_key, to_key, sort)
JOIN meta.lifecycles lc ON lc.key = 'hardware'
LEFT JOIN meta.lifecycle_states fs ON fs.lifecycle_id = lc.id AND fs.key = v.from_key
JOIN meta.lifecycle_states ts ON ts.lifecycle_id = lc.id AND ts.key = v.to_key;

-- --- Asset type tree --------------------------------------------------------
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort) VALUES
    ('hardware', 'Hardware', NULL, 'hardware', true,  1),
    ('software', 'Software', NULL, 'software', true,  2),
    ('license',  'License',  NULL, 'license',  false, 3);

INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, v.abstract, v.sort
FROM (VALUES
    ('computer',   'Computer',        'hardware', 'hardware.computer',   true,  1),
    ('network',    'Network Device',  'hardware', 'hardware.network',    true,  2),
    ('component',  'Component',        'hardware', 'hardware.component',  true,  3),
    ('peripheral', 'Peripheral',       'hardware', 'hardware.peripheral', false, 4)
) AS v(key, name, parent_key, path, abstract, sort)
JOIN meta.asset_types p ON p.key = v.parent_key;

INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, v.abstract, v.sort
FROM (VALUES
    ('laptop',  'Laptop',  'computer', 'hardware.computer.laptop',  false, 1),
    ('desktop', 'Desktop', 'computer', 'hardware.computer.desktop', false, 2),
    ('server',  'Server',  'computer', 'hardware.computer.server',  false, 3),
    ('switch',  'Switch',  'network',  'hardware.network.switch',   false, 1),
    ('router',  'Router',  'network',  'hardware.network.router',   false, 2),
    ('ram',     'RAM Module', 'component', 'hardware.component.ram', false, 1),
    ('ssd',     'SSD',     'component', 'hardware.component.ssd',    false, 2),
    ('sfp',     'SFP Module','component','hardware.component.sfp',   false, 3)
) AS v(key, name, parent_key, path, abstract, sort)
JOIN meta.asset_types p ON p.key = v.parent_key;

-- All hardware subtypes use the hardware lifecycle.
UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE path <@ 'hardware';

-- --- Field definitions ------------------------------------------------------
INSERT INTO meta.field_definitions
    (asset_type_id, key, label, data_type_id, required, unit_id, sort)
SELECT at.id, v.key, v.label, dt.id, v.required, u.id, v.sort
FROM (VALUES
    ('laptop',  'hostname',    'Hostname',     'text',    false, NULL,  1),
    ('laptop',  'cpu',         'CPU',          'text',    false, NULL,  2),
    ('laptop',  'ram_gb',      'RAM',          'number',  false, 'gb',  3),
    ('laptop',  'storage_gb',  'Storage',      'number',  false, 'gb',  4),
    ('laptop',  'os',          'Operating System','text', false, NULL,  5),
    ('desktop', 'hostname',    'Hostname',     'text',    false, NULL,  1),
    ('desktop', 'cpu',         'CPU',          'text',    false, NULL,  2),
    ('desktop', 'ram_gb',      'RAM',          'number',  false, 'gb',  3),
    ('server',  'hostname',    'Hostname',     'text',    true,  NULL,  1),
    ('server',  'cpu_sockets', 'CPU Sockets',  'number',  false, NULL,  2),
    ('server',  'ram_gb',      'RAM',          'number',  false, 'gb',  3),
    ('server',  'rack_units',  'Rack Units',   'number',  false, 'ru',  4),
    ('server',  'os',          'Operating System','text', false, NULL,  5),
    ('ram',     'capacity_gb', 'Capacity',     'number',  true,  'gb',  1),
    ('ram',     'speed_mhz',   'Speed',        'number',  false, 'mhz', 2),
    ('ssd',     'capacity_gb', 'Capacity',     'number',  true,  'gb',  1),
    ('switch',  'ports',       'Port Count',   'number',  false, NULL,  1),
    ('switch',  'mgmt_ip',     'Management IP', 'text',   false, NULL,  2)
) AS v(type_key, key, label, dt_key, required, unit_key, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key;

-- --- Relationship types -----------------------------------------------------
INSERT INTO meta.relationship_types (key, label, inverse_label, from_type_key, to_type_key, cardinality, sort) VALUES
    ('rack_contains', 'Contains',      'Contained in', NULL, NULL, 'one_to_many',  1),
    ('runs_vm',       'Runs VM',       'Hosted on',    NULL, NULL, 'one_to_many',  2),
    ('has_component', 'Has component', 'Component of', NULL, NULL, 'one_to_many',  3),
    ('connects_to',   'Connects to',   'Connected to', NULL, NULL, 'many_to_many', 4);

-- --- Automation rule example ------------------------------------------------
INSERT INTO meta.automation_rules (key, name, on_subject, condition, action, enabled, sort) VALUES
    ('log_state_changes', 'Log all state changes', 'itam.asset.state_changed', '{}',
     '{"type":"log","message":"asset changed state"}', true, 1);

-- --- RBAC: permissions ------------------------------------------------------
INSERT INTO iam.permissions (key, description) VALUES
    ('asset.read',        'View assets'),
    ('asset.write',       'Create and edit assets'),
    ('asset.delete',      'Delete assets'),
    ('asset.transition',  'Run lifecycle transitions'),
    ('asset.assign',      'Assign / transfer custody'),
    ('metadata.read',     'View metadata configuration'),
    ('metadata.manage',   'Edit metadata configuration'),
    ('hierarchy.read',    'View locations and org structure'),
    ('hierarchy.manage',  'Edit locations and org structure'),
    ('iam.manage',        'Manage roles and grants'),
    ('audit.read',        'Read the audit log');

-- --- RBAC: roles ------------------------------------------------------------
INSERT INTO iam.roles (key, name, description) VALUES
    ('admin',         'Administrator',  'Full access'),
    ('asset_manager', 'Asset Manager',  'Manage assets, custody and lifecycle'),
    ('technician',    'Technician',     'Run lifecycle transitions'),
    ('viewer',        'Viewer',         'Read-only access');

-- admin: every permission
INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM iam.roles r CROSS JOIN iam.permissions p WHERE r.key = 'admin';

-- scoped roles
INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('asset_manager', 'asset.read'),
    ('asset_manager', 'asset.write'),
    ('asset_manager', 'asset.transition'),
    ('asset_manager', 'asset.assign'),
    ('asset_manager', 'metadata.read'),
    ('asset_manager', 'hierarchy.read'),
    ('technician',    'asset.read'),
    ('technician',    'asset.transition'),
    ('technician',    'metadata.read'),
    ('technician',    'hierarchy.read'),
    ('viewer',        'asset.read'),
    ('viewer',        'metadata.read'),
    ('viewer',        'hierarchy.read')
) AS v(role_key, perm_key)
JOIN iam.roles r       ON r.key = v.role_key
JOIN iam.permissions p ON p.key = v.perm_key;

-- --- Starter hierarchy ------------------------------------------------------
INSERT INTO core.locations (key, name, kind, parent_id, path) VALUES
    ('hq', 'Headquarters', 'site', NULL, 'hq');

INSERT INTO core.org_units (key, name, parent_id, path) VALUES
    ('org', 'Organization', NULL, 'org');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.org_units WHERE key = 'org';
DELETE FROM core.locations WHERE key = 'hq';
DELETE FROM iam.role_permissions;
DELETE FROM iam.roles;
DELETE FROM iam.permissions;
DELETE FROM meta.automation_rules;
DELETE FROM meta.relationship_types;
DELETE FROM meta.field_definitions;
DELETE FROM meta.asset_types;
DELETE FROM meta.lifecycle_transitions;
DELETE FROM meta.lifecycle_states;
DELETE FROM meta.lifecycles;
DELETE FROM meta.units;
DELETE FROM meta.data_types;
-- +goose StatementEnd
