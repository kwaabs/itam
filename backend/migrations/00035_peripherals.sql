-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Peripherals: monitors, keyboards, mice, printers, scanners, flash drives and
-- other small end-user hardware. These are concrete asset types under the
-- existing hardware.peripheral category, each with a few useful custom fields
-- and straight-line depreciation defaults.
-- ---------------------------------------------------------------------------

-- Units needed for peripheral fields.
INSERT INTO meta.units (key, label, symbol) VALUES
    ('inch', 'Inches',           'in'),
    ('dpi',  'Dots per inch',    'DPI'),
    ('ppm',  'Pages per minute', 'ppm'),
    ('hz',   'Hertz',            'Hz'),
    ('lumen','Lumens',           'lm')
ON CONFLICT (key) DO NOTHING;

-- Concrete peripheral types.
INSERT INTO meta.asset_types (key, name, parent_id, path, is_abstract, sort)
SELECT v.key, v.name, p.id, v.path::ltree, false, v.sort
FROM (VALUES
    ('monitor',         'Monitor / Screen',  'hardware.peripheral.monitor',         1),
    ('keyboard',        'Keyboard',          'hardware.peripheral.keyboard',        2),
    ('mouse',           'Mouse',             'hardware.peripheral.mouse',           3),
    ('printer',         'Printer',           'hardware.peripheral.printer',         4),
    ('scanner',         'Scanner',           'hardware.peripheral.scanner',         5),
    ('flash_drive',     'USB Flash Drive',   'hardware.peripheral.flash_drive',     6),
    ('external_drive',  'External Drive',    'hardware.peripheral.external_drive',  7),
    ('webcam',          'Webcam',            'hardware.peripheral.webcam',          8),
    ('headset',         'Headset',           'hardware.peripheral.headset',         9),
    ('docking_station', 'Docking Station',   'hardware.peripheral.docking_station', 10),
    ('projector',       'Projector',         'hardware.peripheral.projector',       11),
    ('speaker',         'Speaker',           'hardware.peripheral.speaker',         12)
) AS v(key, name, path, sort)
JOIN meta.asset_types p ON p.key = 'peripheral'
ON CONFLICT (key) DO NOTHING;

-- Use the hardware lifecycle for the new peripheral types.
UPDATE meta.asset_types
SET lifecycle_id = (SELECT id FROM meta.lifecycles WHERE key = 'hardware')
WHERE path <@ 'hardware.peripheral' AND lifecycle_id IS NULL;

-- Depreciation defaults (straight-line). Small peripherals depreciate faster.
UPDATE meta.asset_types SET useful_life_months = 36
WHERE key IN ('monitor','printer','scanner','projector','docking_station','external_drive') AND useful_life_months = 0;
UPDATE meta.asset_types SET useful_life_months = 24
WHERE key IN ('keyboard','mouse','flash_drive','webcam','headset','speaker') AND useful_life_months = 0;

-- Make the parent category abstract (not directly selectable) when nothing uses
-- it directly, now that concrete subtypes exist.
UPDATE meta.asset_types SET is_abstract = true
WHERE key = 'peripheral'
  AND NOT EXISTS (
    SELECT 1 FROM core.assets a
    JOIN meta.asset_types t ON t.id = a.asset_type_id
    WHERE t.key = 'peripheral'
  );

-- --- Custom field definitions -----------------------------------------------
INSERT INTO meta.field_definitions
    (asset_type_id, key, label, data_type_id, required, unit_id, enum_options, sort)
SELECT at.id, v.key, v.label, dt.id, v.required, u.id, v.enum_options::jsonb, v.sort
FROM (VALUES
    -- monitor
    ('monitor', 'screen_size',  'Screen size',   'number',  false, 'inch', '[]', 1),
    ('monitor', 'resolution',   'Resolution',    'text',    false, NULL,   '[]', 2),
    ('monitor', 'panel_type',   'Panel type',    'enum',    false, NULL,   '[{"value":"ips","label":"IPS"},{"value":"va","label":"VA"},{"value":"tn","label":"TN"},{"value":"oled","label":"OLED"}]', 3),
    ('monitor', 'refresh_hz',   'Refresh rate',  'number',  false, 'hz',   '[]', 4),
    ('monitor', 'connectivity', 'Connectivity',  'text',    false, NULL,   '[]', 5),
    -- keyboard
    ('keyboard', 'layout',       'Layout',       'text',    false, NULL,   '[]', 1),
    ('keyboard', 'connectivity', 'Connectivity', 'enum',    false, NULL,   '[{"value":"wired","label":"Wired"},{"value":"wireless","label":"Wireless"},{"value":"bluetooth","label":"Bluetooth"}]', 2),
    -- mouse
    ('mouse', 'connectivity', 'Connectivity', 'enum',    false, NULL,   '[{"value":"wired","label":"Wired"},{"value":"wireless","label":"Wireless"},{"value":"bluetooth","label":"Bluetooth"}]', 1),
    ('mouse', 'dpi',          'Sensor DPI',   'number',  false, 'dpi',  '[]', 2),
    -- printer
    ('printer', 'printer_type', 'Printer type', 'enum',   false, NULL,   '[{"value":"laser","label":"Laser"},{"value":"inkjet","label":"Inkjet"},{"value":"thermal","label":"Thermal"},{"value":"dotmatrix","label":"Dot matrix"},{"value":"3d","label":"3D"}]', 1),
    ('printer', 'color',        'Color',        'boolean', false, NULL,   '[]', 2),
    ('printer', 'speed_ppm',    'Speed',        'number',  false, 'ppm',  '[]', 3),
    ('printer', 'duplex',       'Duplex',       'boolean', false, NULL,   '[]', 4),
    ('printer', 'connectivity', 'Connectivity', 'enum',    false, NULL,   '[{"value":"usb","label":"USB"},{"value":"network","label":"Network"},{"value":"wifi","label":"Wi-Fi"}]', 5),
    -- scanner
    ('scanner', 'scanner_type', 'Scanner type', 'enum',   false, NULL,   '[{"value":"flatbed","label":"Flatbed"},{"value":"sheetfed","label":"Sheet-fed"},{"value":"handheld","label":"Handheld"},{"value":"document","label":"Document"}]', 1),
    ('scanner', 'optical_dpi',  'Optical DPI',  'number',  false, 'dpi',  '[]', 2),
    ('scanner', 'color',        'Color',        'boolean', false, NULL,   '[]', 3),
    -- flash drive
    ('flash_drive', 'capacity_gb', 'Capacity',  'number',  true,  'gb',   '[]', 1),
    ('flash_drive', 'interface',   'Interface', 'enum',    false, NULL,   '[{"value":"usb-a","label":"USB-A"},{"value":"usb-c","label":"USB-C"},{"value":"usb2","label":"USB 2.0"},{"value":"usb3","label":"USB 3.x"}]', 2),
    ('flash_drive', 'encrypted',   'Encrypted', 'boolean', false, NULL,   '[]', 3),
    -- external drive
    ('external_drive', 'capacity_gb', 'Capacity',  'number',  true,  'gb',   '[]', 1),
    ('external_drive', 'interface',   'Interface', 'enum',    false, NULL,   '[{"value":"usb-c","label":"USB-C"},{"value":"usb3","label":"USB 3.x"},{"value":"thunderbolt","label":"Thunderbolt"},{"value":"esata","label":"eSATA"}]', 2),
    -- webcam
    ('webcam', 'resolution', 'Resolution', 'text',    false, NULL, '[]', 1),
    ('webcam', 'mic',        'Built-in mic','boolean', false, NULL, '[]', 2),
    -- headset
    ('headset', 'connectivity', 'Connectivity', 'enum',  false, NULL, '[{"value":"wired","label":"Wired"},{"value":"wireless","label":"Wireless"},{"value":"bluetooth","label":"Bluetooth"}]', 1),
    ('headset', 'mic',          'Microphone',   'boolean', false, NULL, '[]', 2),
    -- docking station
    ('docking_station', 'ports',            'Ports',          'text',   false, NULL,   '[]', 1),
    ('docking_station', 'power_delivery_w', 'Power delivery', 'number', false, 'watt', '[]', 2),
    -- projector
    ('projector', 'lumens',     'Brightness',  'number', false, 'lumen', '[]', 1),
    ('projector', 'resolution', 'Resolution',  'text',   false, NULL,    '[]', 2)
) AS v(type_key, key, label, dt_key, required, unit_key, enum_options, sort)
JOIN meta.asset_types at ON at.key = v.type_key
JOIN meta.data_types  dt ON dt.key = v.dt_key
LEFT JOIN meta.units  u  ON u.key  = v.unit_key
ON CONFLICT (asset_type_id, key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM meta.field_definitions
WHERE asset_type_id IN (SELECT id FROM meta.asset_types WHERE path <@ 'hardware.peripheral' AND key <> 'peripheral');
DELETE FROM meta.asset_types WHERE path <@ 'hardware.peripheral' AND key <> 'peripheral';
UPDATE meta.asset_types SET is_abstract = false WHERE key = 'peripheral';
DELETE FROM meta.units WHERE key IN ('inch','dpi','ppm','hz','lumen');
-- +goose StatementEnd
