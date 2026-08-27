-- Demo peripherals, all seeded directly into the "In Stock" lifecycle state.
-- Idempotent: tags are stable (STK-*) and guarded with ON CONFLICT DO NOTHING.
DO $$
DECLARE
    v_lc    bigint;
    v_stock bigint;
    v_loc   uuid;
BEGIN
    SELECT id INTO v_lc FROM meta.lifecycles WHERE key = 'hardware';
    SELECT s.id INTO v_stock
      FROM meta.lifecycle_states s
      JOIN meta.lifecycles l ON l.id = s.lifecycle_id
     WHERE l.key = 'hardware' AND s.key = 'in_stock';
    -- central stock location: prefer HQ, else any site
    SELECT id INTO v_loc FROM core.locations WHERE key = 'hq';
    IF v_loc IS NULL THEN
        SELECT id INTO v_loc FROM core.locations WHERE kind = 'site' ORDER BY path LIMIT 1;
    END IF;

    -- Monitors
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-MON-' || lpad(g::text, 3, '0'),
           (CASE WHEN g % 2 = 0 THEN 'Dell UltraSharp U2723' ELSE 'HP E24 G5' END) || ' #' || g,
           at.id, v_lc, v_stock, v_loc,
           (CASE WHEN g % 2 = 0 THEN 'Dell' ELSE 'HP' END), 320, now()::date - (g * 7),
           jsonb_build_object('screen_size', (CASE WHEN g % 2 = 0 THEN 27 ELSE 24 END),
                              'panel_type', 'ips', 'resolution', '2560x1440', 'refresh_hz', 60)
    FROM generate_series(1, 6) g JOIN meta.asset_types at ON at.key = 'monitor'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Keyboards
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-KBD-' || lpad(g::text, 3, '0'), 'Logitech MX Keys #' || g,
           at.id, v_lc, v_stock, v_loc, 'Logitech', 45, now()::date - (g * 5),
           jsonb_build_object('layout', 'US', 'connectivity', (CASE WHEN g % 3 = 0 THEN 'wired' ELSE 'wireless' END))
    FROM generate_series(1, 8) g JOIN meta.asset_types at ON at.key = 'keyboard'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Mice
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-MSE-' || lpad(g::text, 3, '0'), 'Logitech MX Master 3S #' || g,
           at.id, v_lc, v_stock, v_loc, 'Logitech', 30, now()::date - (g * 4),
           jsonb_build_object('connectivity', (CASE WHEN g % 2 = 0 THEN 'bluetooth' ELSE 'wireless' END), 'dpi', 8000)
    FROM generate_series(1, 8) g JOIN meta.asset_types at ON at.key = 'mouse'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Printers
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-PRN-' || lpad(g::text, 3, '0'), 'HP LaserJet Pro #' || g,
           at.id, v_lc, v_stock, v_loc, 'HP', 450, now()::date - (g * 30),
           jsonb_build_object('printer_type', 'laser', 'color', (g % 2 = 0), 'speed_ppm', 38, 'duplex', true, 'connectivity', 'network')
    FROM generate_series(1, 3) g JOIN meta.asset_types at ON at.key = 'printer'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Scanners
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-SCN-' || lpad(g::text, 3, '0'), 'Epson WorkForce DS #' || g,
           at.id, v_lc, v_stock, v_loc, 'Epson', 280, now()::date - (g * 25),
           jsonb_build_object('scanner_type', 'document', 'optical_dpi', 600, 'color', true)
    FROM generate_series(1, 2) g JOIN meta.asset_types at ON at.key = 'scanner'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- USB flash drives
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-USB-' || lpad(g::text, 3, '0'),
           'SanDisk Ultra ' || (CASE WHEN g % 3 = 0 THEN 256 WHEN g % 3 = 1 THEN 128 ELSE 64 END) || 'GB #' || g,
           at.id, v_lc, v_stock, v_loc, 'SanDisk', 18, now()::date - (g * 2),
           jsonb_build_object('capacity_gb', (CASE WHEN g % 3 = 0 THEN 256 WHEN g % 3 = 1 THEN 128 ELSE 64 END),
                              'interface', (CASE WHEN g % 2 = 0 THEN 'usb-c' ELSE 'usb3' END), 'encrypted', (g % 4 = 0))
    FROM generate_series(1, 10) g JOIN meta.asset_types at ON at.key = 'flash_drive'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- External drives
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-EXT-' || lpad(g::text, 3, '0'), 'Samsung T7 2TB #' || g,
           at.id, v_lc, v_stock, v_loc, 'Samsung', 90, now()::date - (g * 10),
           jsonb_build_object('capacity_gb', 2000, 'interface', 'usb-c')
    FROM generate_series(1, 3) g JOIN meta.asset_types at ON at.key = 'external_drive'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Webcams
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-CAM-' || lpad(g::text, 3, '0'), 'Logitech Brio 4K #' || g,
           at.id, v_lc, v_stock, v_loc, 'Logitech', 70, now()::date - (g * 6),
           jsonb_build_object('resolution', '1920x1080', 'mic', true)
    FROM generate_series(1, 4) g JOIN meta.asset_types at ON at.key = 'webcam'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Headsets
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-HST-' || lpad(g::text, 3, '0'), 'Jabra Evolve2 65 #' || g,
           at.id, v_lc, v_stock, v_loc, 'Jabra', 85, now()::date - (g * 5),
           jsonb_build_object('connectivity', (CASE WHEN g % 2 = 0 THEN 'bluetooth' ELSE 'wireless' END), 'mic', true)
    FROM generate_series(1, 6) g JOIN meta.asset_types at ON at.key = 'headset'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Docking stations
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-DOK-' || lpad(g::text, 3, '0'), 'Dell WD19S Dock #' || g,
           at.id, v_lc, v_stock, v_loc, 'Dell', 220, now()::date - (g * 12),
           jsonb_build_object('ports', '2x HDMI, 1x DP, 4x USB-A, 1x USB-C', 'power_delivery_w', 130)
    FROM generate_series(1, 4) g JOIN meta.asset_types at ON at.key = 'docking_station'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Projectors
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-PRJ-' || lpad(g::text, 3, '0'), 'Epson EB-1795F #' || g,
           at.id, v_lc, v_stock, v_loc, 'Epson', 700, now()::date - (g * 40),
           jsonb_build_object('lumens', 3200, 'resolution', '1920x1080')
    FROM generate_series(1, 2) g JOIN meta.asset_types at ON at.key = 'projector'
    ON CONFLICT (asset_tag) DO NOTHING;

    -- Speakers
    INSERT INTO core.assets (asset_tag, name, asset_type_id, lifecycle_id, current_state_id, location_id, vendor, purchase_cost, purchase_date, attributes)
    SELECT 'STK-SPK-' || lpad(g::text, 3, '0'), 'Jabra Speak 750 #' || g,
           at.id, v_lc, v_stock, v_loc, 'Jabra', 60, now()::date - (g * 8), '{}'::jsonb
    FROM generate_series(1, 3) g JOIN meta.asset_types at ON at.key = 'speaker'
    ON CONFLICT (asset_tag) DO NOTHING;
END $$;

SELECT t.name AS type, count(*) AS in_stock
FROM core.assets a
JOIN meta.asset_types t ON t.id = a.asset_type_id
JOIN meta.lifecycle_states s ON s.id = a.current_state_id
WHERE a.asset_tag LIKE 'STK-%' AND s.key = 'in_stock'
GROUP BY t.name ORDER BY t.name;
