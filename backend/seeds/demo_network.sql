-- Demo network wiring: generate ports for all network devices from their port
-- profiles, then cable each non-switch network device to its site switch so the
-- topology graph and front panels have data. Idempotent.

-- 1) Generate ports from profiles for every network device.
INSERT INTO dcim.ports (asset_id, name, port_type, speed, sort)
SELECT a.id,
       pf.name_prefix || (pf.start_index + g)::text,
       pf.port_type,
       pf.speed,
       0
FROM core.assets a
JOIN meta.asset_types t  ON t.id = a.asset_type_id
JOIN meta.port_profiles pf ON pf.asset_type_id = a.asset_type_id
CROSS JOIN LATERAL generate_series(0, pf.count - 1) AS g
WHERE t.path <@ 'hardware.network'
ON CONFLICT (asset_id, name) DO NOTHING;

-- Devices grouped by their site (the top-2 labels of the location ltree path),
-- so a firewall in a room connects to a switch elsewhere in the same site.
DROP TABLE IF EXISTS tmp_devloc;
CREATE TEMP TABLE tmp_devloc AS
SELECT a.id AS asset_id, t.key AS kind, a.created_at,
       subpath(l.path, 0, LEAST(2, nlevel(l.path))) AS site
FROM core.assets a
JOIN meta.asset_types t ON t.id = a.asset_type_id
JOIN core.locations l   ON l.id = a.location_id
WHERE t.path <@ 'hardware.network';

-- 2a) Cable each non-switch network device to a switch in the same site.
WITH one_switch AS (
    SELECT DISTINCT ON (site) asset_id AS switch_id, site
    FROM tmp_devloc WHERE kind = 'switch' ORDER BY site, created_at
),
others AS (
    SELECT asset_id AS dev_id, site,
           row_number() OVER (PARTITION BY site ORDER BY created_at) AS rn
    FROM tmp_devloc WHERE kind <> 'switch'
),
pairs AS (
    SELECT
        (SELECT p.id FROM dcim.ports p WHERE p.asset_id = o.dev_id ORDER BY p.sort, p.name LIMIT 1) AS dev_port,
        (SELECT p.id FROM dcim.ports p WHERE p.asset_id = s.switch_id AND p.name = 'Gi1/0/' || o.rn) AS sw_port
    FROM others o JOIN one_switch s ON s.site = o.site
)
INSERT INTO dcim.connections (a_port_id, b_port_id, cable_type)
SELECT dev_port, sw_port, 'cat6'
FROM pairs
WHERE dev_port IS NOT NULL AND sw_port IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM dcim.connections c
      WHERE c.a_port_id IN (dev_port, sw_port) OR c.b_port_id IN (dev_port, sw_port)
  );

-- 2b) Uplink leaf switches to the site's core switch via 10G SFP+ ports.
WITH sw_ranked AS (
    SELECT asset_id AS switch_id, site,
           row_number() OVER (PARTITION BY site ORDER BY created_at) AS rn
    FROM tmp_devloc WHERE kind = 'switch'
),
core_sw AS (SELECT switch_id, site FROM sw_ranked WHERE rn = 1),
uplinks AS (
    SELECT
        (SELECT p.id FROM dcim.ports p WHERE p.asset_id = r.switch_id AND p.name = 'Te1/1/1') AS leaf_port,
        (SELECT p.id FROM dcim.ports p WHERE p.asset_id = c.switch_id AND p.name = 'Te1/1/' || (r.rn - 1)) AS core_port
    FROM sw_ranked r JOIN core_sw c ON c.site = r.site
    WHERE r.rn > 1
)
INSERT INTO dcim.connections (a_port_id, b_port_id, cable_type)
SELECT leaf_port, core_port, 'fiber'
FROM uplinks
WHERE leaf_port IS NOT NULL AND core_port IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM dcim.connections c
      WHERE c.a_port_id IN (leaf_port, core_port) OR c.b_port_id IN (leaf_port, core_port)
  );

SELECT 'ports' AS t, count(*) FROM dcim.ports
UNION ALL SELECT 'connections', count(*) FROM dcim.connections;
