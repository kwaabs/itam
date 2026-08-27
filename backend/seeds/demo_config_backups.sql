-- Two versioned config snapshots per firewall so the backup-diff view has data.
-- v1.0 is a baseline; v1.1 changes/adds a few lines. Idempotent on (asset, version).

DROP TABLE IF EXISTS tmp_fw2;
CREATE TEMP TABLE tmp_fw2 AS
SELECT a.id, a.asset_tag
FROM core.assets a
JOIN meta.asset_types t ON t.id = a.asset_type_id
WHERE t.key = 'firewall';

-- Baseline v1.0
INSERT INTO netcfg.config_backups (asset_id, taken_at, source, version, content, note, size_bytes, hash)
SELECT f.id, now() - interval '14 days', 'manual', 'v1.0', c.content, 'baseline',
       length(c.content), md5(c.content)
FROM tmp_fw2 f
CROSS JOIN LATERAL (SELECT
'hostname ' || f.asset_tag || E'\n' ||
E'!\n' ||
E'interface eth0\n' ||
E' description WAN\n' ||
E' ip address 203.0.113.10/29\n' ||
E'!\n' ||
E'interface eth1\n' ||
E' description LAN\n' ||
E' ip address 10.10.0.1/24\n' ||
E'!\n' ||
E'zone trust\n' ||
E'zone untrust\n' ||
E'!\n' ||
E'policy 10 from trust to untrust action allow\n' ||
E'policy 20 from untrust to trust action deny\n' ||
E'!\n' ||
E'ntp server 10.10.0.5\n' ||
E'snmp-server community public ro\n' ||
E'end\n' AS content) c
WHERE NOT EXISTS (
    SELECT 1 FROM netcfg.config_backups b WHERE b.asset_id = f.id AND b.version = 'v1.0'
);

-- Updated v1.1: hardened SNMP, added DMZ interface + policy, new NTP server.
INSERT INTO netcfg.config_backups (asset_id, taken_at, source, version, content, note, size_bytes, hash)
SELECT f.id, now() - interval '2 days', 'manual', 'v1.1', c.content, 'hardening + DMZ',
       length(c.content), md5(c.content)
FROM tmp_fw2 f
CROSS JOIN LATERAL (SELECT
'hostname ' || f.asset_tag || E'\n' ||
E'!\n' ||
E'interface eth0\n' ||
E' description WAN\n' ||
E' ip address 203.0.113.10/29\n' ||
E'!\n' ||
E'interface eth1\n' ||
E' description LAN\n' ||
E' ip address 10.10.0.1/24\n' ||
E'!\n' ||
E'interface eth2\n' ||
E' description DMZ\n' ||
E' ip address 172.16.0.1/24\n' ||
E'!\n' ||
E'zone trust\n' ||
E'zone untrust\n' ||
E'zone dmz\n' ||
E'!\n' ||
E'policy 10 from trust to untrust action allow\n' ||
E'policy 20 from untrust to trust action deny\n' ||
E'policy 30 from dmz to untrust action allow\n' ||
E'!\n' ||
E'ntp server 10.10.0.6\n' ||
E'snmp-server community ChangeMe123 ro\n' ||
E'end\n' AS content) c
WHERE NOT EXISTS (
    SELECT 1 FROM netcfg.config_backups b WHERE b.asset_id = f.id AND b.version = 'v1.1'
);

SELECT version, count(*) FROM netcfg.config_backups GROUP BY version ORDER BY version;
