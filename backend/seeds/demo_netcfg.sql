-- Demo data for the new network-depth features: DHCP scopes per subnet, plus
-- address objects, routes and NAT rules on every firewall. Idempotent.

-- 1) DHCP scope per subnet (.100-.200 pool, .1 gateway).
INSERT INTO ipam.dhcp_scopes (subnet_id, name, range_start, range_end, gateway, dns, domain, lease_hours)
SELECT s.id,
       s.name || ' pool',
       (network(s.cidr) + 100)::inet,
       (network(s.cidr) + 200)::inet,
       COALESCE(s.gateway, (network(s.cidr) + 1)::inet),
       host((network(s.cidr) + 10)::inet) || ', 1.1.1.1',
       'corp.local',
       24
FROM ipam.subnets s
WHERE NOT EXISTS (SELECT 1 FROM ipam.dhcp_scopes d WHERE d.subnet_id = s.id);

-- Firewalls we attach netcfg objects to.
DROP TABLE IF EXISTS tmp_fw;
CREATE TEMP TABLE tmp_fw AS
SELECT a.id
FROM core.assets a
JOIN meta.asset_types t ON t.id = a.asset_type_id
WHERE t.key = 'firewall';

-- 2) Address objects.
INSERT INTO netcfg.address_objects (asset_id, name, kind, value, description)
SELECT f.id, v.name, v.kind, v.value, v.description
FROM tmp_fw f
CROSS JOIN (VALUES
    ('AnyIPv4',    'network', '0.0.0.0/0',     'Catch-all'),
    ('LAN-net',    'network', '10.0.0.0/8',    'Internal RFC1918'),
    ('DMZ-net',    'network', '172.16.0.0/24', 'DMZ segment'),
    ('WebServer',  'host',    '10.10.0.20',    'Public web server'),
    ('DNS-Google', 'host',    '8.8.8.8',       'Upstream DNS')
) AS v(name, kind, value, description)
ON CONFLICT (asset_id, name) DO NOTHING;

-- 3) Routes.
INSERT INTO netcfg.routes (asset_id, destination, next_hop, interface, metric, description)
SELECT f.id, v.destination, v.next_hop, v.interface, v.metric, v.description
FROM tmp_fw f
CROSS JOIN (VALUES
    ('default',      '203.0.113.1', 'eth1', 1,  'Default gateway (WAN)'),
    ('10.10.0.0/24', '10.10.0.1',   'eth2', 10, 'Management LAN'),
    ('10.55.0.0/24', '10.55.0.1',   'eth3', 10, 'Server LAN'),
    ('172.16.0.0/24','172.16.0.1',  'eth4', 20, 'DMZ')
) AS v(destination, next_hop, interface, metric, description)
WHERE NOT EXISTS (
    SELECT 1 FROM netcfg.routes r WHERE r.asset_id = f.id AND r.destination = v.destination
);

-- 4) NAT rules.
INSERT INTO netcfg.nat_rules
    (asset_id, seq, name, nat_type, orig_src, orig_dst, orig_service, trans_src, trans_dst, trans_service, description)
SELECT f.id, v.seq, v.name, v.nat_type, v.orig_src, v.orig_dst, v.orig_service, v.trans_src, v.trans_dst, v.trans_service, v.description
FROM tmp_fw f
CROSS JOIN (VALUES
    (1, 'Outbound NAT', 'source',      '10.0.0.0/8', NULL::text,    NULL::text,  '203.0.113.10', NULL::text,   NULL::text,  'Masquerade internal to WAN'),
    (2, 'Web DNAT',     'destination', NULL::text,   '203.0.113.20','tcp/443',    NULL::text,    '10.10.0.20', 'tcp/443',   'Publish web server'),
    (3, 'Mail DNAT',    'destination', NULL::text,   '203.0.113.25','tcp/25',     NULL::text,    '10.55.0.25', 'tcp/25',    'Publish mail relay')
) AS v(seq, name, nat_type, orig_src, orig_dst, orig_service, trans_src, trans_dst, trans_service, description)
WHERE NOT EXISTS (
    SELECT 1 FROM netcfg.nat_rules n WHERE n.asset_id = f.id AND n.name = v.name
);

SELECT 'dhcp_scopes' AS t, count(*) FROM ipam.dhcp_scopes
UNION ALL SELECT 'address_objects', count(*) FROM netcfg.address_objects
UNION ALL SELECT 'routes', count(*) FROM netcfg.routes
UNION ALL SELECT 'nat_rules', count(*) FROM netcfg.nat_rules;
