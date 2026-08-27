-- ===========================================================================
-- Demo data: 3 data centers (incl. a DR site), each with 2 floors -> 2 rooms ->
-- 2 zones -> 2 racks, redundant power feeds + PDUs, electrical & cooling plant,
-- servers/switches/firewalls mounted in racks, environmental sensors + readings,
-- plus virtualization, storage and firewall config on the London site.
--
-- Idempotent: re-running is safe (keys/tags conflict -> do nothing). Everything
-- here is ordinary application data, not schema.
-- ===========================================================================

-- ---- 1. Sites -> floors -> rooms -> zones -> racks + power + assets ---------
DO $$
DECLARE
  hw_lc    bigint;
  st_inuse bigint;
  t_server bigint;
  t_switch bigint;
  t_fw     bigint;
  t_crac   bigint;
  sites    text[][] := ARRAY[
    ARRAY['dc_lon','DC1 London Docklands','primary','tier-4','Europe/London','East India Dock, London E14, UK','-0.0205','51.5085'],
    ARRAY['dc_man','DC2 Manchester Trafford','primary','tier-3','Europe/London','Trafford Park, Manchester M17, UK','-2.2901','53.4673'],
    ARRAY['dc_fra','DC3 Frankfurt (DR)','dr','tier-3','Europe/Berlin','Kleyerstrasse, 60326 Frankfurt, DE','8.6300','50.1000']
  ];
  plant    text[][] := ARRAY[
    ARRAY['generator','Generator','280000'],
    ARRAY['transformer','Transformer','0'],
    ARRAY['ats','Transfer Switch','0'],
    ARRAY['ups','UPS String','0'],
    ARRAY['avr','Voltage Regulator','0'],
    ARRAY['chiller','Chiller','45000']
  ];
  s text[]; p text[];
  site_id uuid; floor_id uuid; room_id uuid; zone_id uuid; rack_id uuid;
  feedA uuid; feedB uuid; srv_id uuid; v_sensor uuid;
  site_key text; floor_key text; room_key text; zone_key text; rack_key text;
  floor_path text; room_path text; zone_path text;
  fi int; ri int; zi int; rk int; jj int;
  temp_val numeric; rack_pw int;
BEGIN
  SELECT id INTO hw_lc    FROM meta.lifecycles WHERE key='hardware';
  SELECT id INTO st_inuse FROM meta.lifecycle_states WHERE lifecycle_id=hw_lc AND key='in_use';
  SELECT id INTO t_server FROM meta.asset_types WHERE key='server';
  SELECT id INTO t_switch FROM meta.asset_types WHERE key='switch';
  SELECT id INTO t_fw     FROM meta.asset_types WHERE key='firewall';
  SELECT id INTO t_crac   FROM meta.asset_types WHERE key='crac';

  FOREACH s SLICE 1 IN ARRAY sites LOOP
    site_key := s[1];
    INSERT INTO core.locations (key,name,kind,parent_id,path,dr_role,tier,timezone,address,geog)
    VALUES (site_key, s[2], 'site', NULL, site_key::ltree, s[3], s[4], s[5], s[6],
            ST_SetSRID(ST_MakePoint(s[7]::float8, s[8]::float8),4326)::geography)
    ON CONFLICT (key) DO NOTHING;
    SELECT id INTO site_id FROM core.locations WHERE key=site_key;

    -- electrical + cooling plant for the site
    FOREACH p SLICE 1 IN ARRAY plant LOOP
      INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor)
      SELECT upper(site_key)||'-'||upper(p[1]), upper(site_key)||'-'||upper(p[1])||'-SN',
             p[2]||' · '||s[2], at.id, hw_lc, st_inuse, site_id,
             jsonb_build_object('power_watts', p[3]::int), 'Vertiv'
      FROM meta.asset_types at WHERE at.key = p[1]
      ON CONFLICT (asset_tag) DO NOTHING;
    END LOOP;

    FOR fi IN 1..2 LOOP
      floor_key  := site_key||'_f'||fi;
      floor_path := site_key||'.f'||fi;
      INSERT INTO core.locations (key,name,kind,parent_id,path)
      VALUES (floor_key, 'Floor '||fi, 'floor', site_id, floor_path::ltree)
      ON CONFLICT (key) DO NOTHING;
      SELECT id INTO floor_id FROM core.locations WHERE key=floor_key;

      FOR ri IN 1..2 LOOP
        room_key  := floor_key||'_r'||ri;
        room_path := floor_path||'.r'||ri;
        INSERT INTO core.locations (key,name,kind,parent_id,path)
        VALUES (room_key, 'Data Hall '||fi||'-'||ri, 'room', floor_id, room_path::ltree)
        ON CONFLICT (key) DO NOTHING;
        SELECT id INTO room_id FROM core.locations WHERE key=room_key;

        -- redundant power feeds for the hall (A=utility, B=UPS)
        INSERT INTO dcim.power_feeds (key,name,location_id,source,capacity_w,voltage,phase,redundancy)
        VALUES ('feed_'||room_key||'_a', 'Feed A · Hall '||fi||'-'||ri, room_id, 'utility', 32000, 400, '3P', 'A')
        ON CONFLICT (key) DO NOTHING;
        INSERT INTO dcim.power_feeds (key,name,location_id,source,capacity_w,voltage,phase,redundancy)
        VALUES ('feed_'||room_key||'_b', 'Feed B · Hall '||fi||'-'||ri, room_id, 'ups', 32000, 400, '3P', 'B')
        ON CONFLICT (key) DO NOTHING;
        SELECT id INTO feedA FROM dcim.power_feeds WHERE key='feed_'||room_key||'_a';
        SELECT id INTO feedB FROM dcim.power_feeds WHERE key='feed_'||room_key||'_b';

        -- CRAC unit for the hall
        INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor)
        VALUES (upper(room_key)||'-CRAC', upper(room_key)||'-CRAC-SN', 'CRAC · Hall '||fi||'-'||ri,
                t_crac, hw_lc, st_inuse, room_id,
                jsonb_build_object('cooling_kw',40,'airflow_cfm',6000,'type','chilled-water','power_watts',8000), 'Stulz')
        ON CONFLICT (asset_tag) DO NOTHING;

        -- firewall pair head for the hall
        INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor)
        VALUES (upper(room_key)||'-FW', upper(room_key)||'-FW-SN', 'Firewall · Hall '||fi||'-'||ri,
                t_fw, hw_lc, st_inuse, room_id,
                jsonb_build_object('mgmt_ip','10.'||fi||'.'||ri||'.1','throughput',20,'ha_role','primary','power_watts',200,'rack_units',1), 'Palo Alto')
        ON CONFLICT (asset_tag) DO NOTHING;

        -- environmental sensors for the hall (temp + humidity)
        INSERT INTO dcim.sensors (key,name,metric,unit,location_id,min_threshold,max_threshold,enabled)
        VALUES ('sen_'||room_key||'_temp','Temp · Hall '||fi||'-'||ri,'temperature','°C',room_id,18,27,true)
        ON CONFLICT (key) DO NOTHING;
        INSERT INTO dcim.sensors (key,name,metric,unit,location_id,min_threshold,max_threshold,enabled)
        VALUES ('sen_'||room_key||'_hum','Humidity · Hall '||fi||'-'||ri,'humidity','%',room_id,40,60,true)
        ON CONFLICT (key) DO NOTHING;
        -- a reading each; make hall 1-1 run hot so a breach shows up
        temp_val := CASE WHEN fi=1 AND ri=1 THEN 29 ELSE 21 + ri END;
        SELECT id INTO v_sensor FROM dcim.sensors WHERE key='sen_'||room_key||'_temp';
        INSERT INTO dcim.sensor_readings (sensor_id, value, status, source, ts)
        SELECT v_sensor, temp_val, CASE WHEN temp_val>27 THEN 'breach' ELSE 'ok' END, 'seed', now()
        WHERE NOT EXISTS (SELECT 1 FROM dcim.sensor_readings srd WHERE srd.sensor_id = v_sensor);
        SELECT id INTO v_sensor FROM dcim.sensors WHERE key='sen_'||room_key||'_hum';
        INSERT INTO dcim.sensor_readings (sensor_id, value, status, source, ts)
        SELECT v_sensor, 48, 'ok', 'seed', now()
        WHERE NOT EXISTS (SELECT 1 FROM dcim.sensor_readings srd WHERE srd.sensor_id = v_sensor);

        FOR zi IN 1..2 LOOP
          zone_key  := room_key||'_z'||zi;
          zone_path := room_path||'.z'||zi;
          INSERT INTO core.locations (key,name,kind,parent_id,path)
          VALUES (zone_key, 'Zone '||zi, 'zone', room_id, zone_path::ltree)
          ON CONFLICT (key) DO NOTHING;
          SELECT id INTO zone_id FROM core.locations WHERE key=zone_key;

          FOR rk IN 1..2 LOOP
            rack_key := 'rk_'||zone_key||'_'||rk;
            INSERT INTO dcim.racks (key,name,location_id,u_height,starting_unit,power_capacity_w,max_weight_kg,pos_x,pos_y)
            VALUES (rack_key, 'Rack '||zi||'-'||rk, zone_id, 42, 1, 14400, 900,
                    10 + (rk-1)*70, 10 + (zi-1)*0)
            ON CONFLICT (key) DO NOTHING;
            SELECT id INTO rack_id FROM dcim.racks WHERE key=rack_key;

            -- A/B PDUs fed from the two hall feeds
            INSERT INTO dcim.pdus (key,name,rack_id,feed_id,capacity_w)
            VALUES ('pdu_'||rack_key||'_a','PDU A',rack_id,feedA,7200) ON CONFLICT (key) DO NOTHING;
            INSERT INTO dcim.pdus (key,name,rack_id,feed_id,capacity_w)
            VALUES ('pdu_'||rack_key||'_b','PDU B',rack_id,feedB,7200) ON CONFLICT (key) DO NOTHING;

            -- two servers per rack, mounted
            FOR jj IN 1..2 LOOP
              rack_pw := 320 + rk*15 + jj*10;
              INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor, purchase_date, warranty_expiry)
              VALUES (upper(rack_key)||'-SRV'||jj, upper(rack_key)||'-SRV'||jj||'-SN',
                      'srv-'||replace(zone_key,'_','-')||'-'||rk||jj,
                      t_server, hw_lc, st_inuse, zone_id,
                      jsonb_build_object('hostname','srv-'||replace(zone_key,'_','-')||'-'||rk||jj,
                                         'cpu_sockets',2,'ram_gb',512,'rack_units',2,
                                         'power_watts',rack_pw,'weight_kg',22),
                      'Dell', date '2024-02-01', date '2027-02-01')
              ON CONFLICT (asset_tag) DO NOTHING;
              SELECT id INTO srv_id FROM core.assets WHERE asset_tag=upper(rack_key)||'-SRV'||jj;
              INSERT INTO dcim.rack_mounts (rack_id, asset_id, position, u_height, face)
              VALUES (rack_id, srv_id, 1 + (jj-1)*2, 2, 'front')
              ON CONFLICT (asset_id) DO NOTHING;
            END LOOP;

            -- top-of-rack switch, mounted at U42
            INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor)
            VALUES (upper(rack_key)||'-SW', upper(rack_key)||'-SW-SN', 'tor-'||replace(zone_key,'_','-')||'-'||rk,
                    t_switch, hw_lc, st_inuse, zone_id,
                    jsonb_build_object('ports',48,'mgmt_ip','10.'||fi||'.'||ri||'.'||(zi*10+rk),'power_watts',110,'weight_kg',8,'rack_units',1), 'Cisco')
            ON CONFLICT (asset_tag) DO NOTHING;
            SELECT id INTO srv_id FROM core.assets WHERE asset_tag=upper(rack_key)||'-SW';
            INSERT INTO dcim.rack_mounts (rack_id, asset_id, position, u_height, face)
            VALUES (rack_id, srv_id, 42, 1, 'front')
            ON CONFLICT (asset_id) DO NOTHING;
          END LOOP;
        END LOOP;
      END LOOP;
    END LOOP;
  END LOOP;
END $$;

-- ---- 2. Virtualization on the London site ---------------------------------
DO $$
DECLARE
  site_id uuid; cl_id uuid; arec record; nhosts int;
BEGIN
  SELECT id INTO site_id FROM core.locations WHERE key='dc_lon';
  INSERT INTO virt.clusters (key,name,location_id,hypervisor,ha,drs)
  VALUES ('cl_lon_prod','LON Prod Cluster',site_id,'esxi',true,true)
  ON CONFLICT (key) DO NOTHING;
  SELECT id INTO cl_id FROM virt.clusters WHERE key='cl_lon_prod';

  FOR arec IN
    SELECT a.id FROM core.assets a
    JOIN meta.asset_types at ON at.id=a.asset_type_id
    JOIN core.locations l ON l.id=a.location_id
    WHERE at.key='server' AND l.path <@ 'dc_lon'
    ORDER BY a.asset_tag LIMIT 4
  LOOP
    INSERT INTO virt.hosts (asset_id,cluster_id,hypervisor,cpu_cores,cpu_threads,ram_gb)
    VALUES (arec.id, cl_id, 'esxi', 32, 64, 512)
    ON CONFLICT (asset_id) DO NOTHING;
  END LOOP;

  SELECT count(*) INTO nhosts FROM virt.hosts WHERE cluster_id=cl_id;
  IF nhosts > 0 AND (SELECT count(*) FROM virt.vms WHERE cluster_id=cl_id) = 0 THEN
    INSERT INTO virt.vms (host_id, cluster_id, name, vcpus, ram_gb, disk_gb, power_state, guest_os, ip)
    SELECT h.id, cl_id, 'vm-lon-'||g,
           CASE WHEN g % 3 = 0 THEN 8 ELSE 4 END, 16, 100,
           CASE WHEN g = 8 THEN 'stopped' ELSE 'running' END,
           CASE WHEN g % 2 = 0 THEN 'Windows Server 2022' ELSE 'Ubuntu 22.04' END,
           '10.20.'||g||'.10'
    FROM generate_series(1,8) g
    JOIN LATERAL (
      SELECT id FROM virt.hosts WHERE cluster_id=cl_id ORDER BY asset_id
      OFFSET (g % nhosts) LIMIT 1
    ) h ON true;
  END IF;
END $$;

-- ---- 3. Storage on the London site ----------------------------------------
DO $$
DECLARE
  hw_lc bigint; st_inuse bigint; t_arr bigint; site_id uuid; arr_id uuid; pool_id uuid;
BEGIN
  SELECT id INTO hw_lc FROM meta.lifecycles WHERE key='hardware';
  SELECT id INTO st_inuse FROM meta.lifecycle_states WHERE lifecycle_id=hw_lc AND key='in_use';
  SELECT id INTO t_arr FROM meta.asset_types WHERE key='storage_array';
  SELECT id INTO site_id FROM core.locations WHERE key='dc_lon';

  INSERT INTO core.assets (asset_tag, serial, name, asset_type_id, lifecycle_id, current_state_id, location_id, attributes, vendor)
  VALUES ('DC_LON-SAN1','DC_LON-SAN1-SN','Primary SAN · London', t_arr, hw_lc, st_inuse, site_id,
          jsonb_build_object('raw_tb',200,'usable_tb',150,'protocol','iSCSI/FC','controllers',2,'power_watts',1800,'rack_units',4), 'NetApp')
  ON CONFLICT (asset_tag) DO NOTHING;
  SELECT id INTO arr_id FROM core.assets WHERE asset_tag='DC_LON-SAN1';

  INSERT INTO stor.pools (key,name,array_asset_id,raid,raw_gb,usable_gb)
  VALUES ('pool_lon_ssd','SSD Tier',arr_id,'RAID6',102400,81920) ON CONFLICT (key) DO NOTHING;
  SELECT id INTO pool_id FROM stor.pools WHERE key='pool_lon_ssd';
  INSERT INTO stor.pools (key,name,array_asset_id,raid,raw_gb,usable_gb)
  VALUES ('pool_lon_sas','SAS Tier',arr_id,'RAID10',102400,51200) ON CONFLICT (key) DO NOTHING;

  INSERT INTO stor.volumes (key,name,array_asset_id,pool_id,capacity_gb,used_gb,protocol)
  VALUES ('vol_lon_vmfs1','vmfs-prod-01',arr_id,pool_id,20480,12800,'iscsi') ON CONFLICT (key) DO NOTHING;
  INSERT INTO stor.volumes (key,name,array_asset_id,pool_id,capacity_gb,used_gb,protocol)
  VALUES ('vol_lon_vmfs2','vmfs-prod-02',arr_id,pool_id,20480,9100,'iscsi') ON CONFLICT (key) DO NOTHING;
  INSERT INTO stor.volumes (key,name,array_asset_id,pool_id,capacity_gb,used_gb,protocol)
  VALUES ('vol_lon_backup','nfs-backup',arr_id,NULL,40960,30200,'nfs') ON CONFLICT (key) DO NOTHING;
END $$;

-- ---- 4. Firewall config on a London firewall ------------------------------
DO $$
DECLARE
  fw_id uuid; z_trust uuid; z_untrust uuid;
BEGIN
  SELECT id INTO fw_id FROM core.assets WHERE asset_tag='DC_LON_F1_R1-FW';
  IF fw_id IS NULL THEN RETURN; END IF;

  INSERT INTO netcfg.zones (asset_id,name,description) VALUES (fw_id,'trust','Internal / server zone') ON CONFLICT (asset_id,name) DO NOTHING;
  INSERT INTO netcfg.zones (asset_id,name,description) VALUES (fw_id,'untrust','Internet edge') ON CONFLICT (asset_id,name) DO NOTHING;
  INSERT INTO netcfg.zones (asset_id,name,description) VALUES (fw_id,'dmz','DMZ services') ON CONFLICT (asset_id,name) DO NOTHING;
  SELECT id INTO z_trust   FROM netcfg.zones WHERE asset_id=fw_id AND name='trust';
  SELECT id INTO z_untrust FROM netcfg.zones WHERE asset_id=fw_id AND name='untrust';

  INSERT INTO netcfg.interfaces (asset_id,name,ip_cidr,zone_id,vlan,enabled)
  VALUES (fw_id,'ethernet1/1','203.0.113.2/29',z_untrust,NULL,true) ON CONFLICT (asset_id,name) DO NOTHING;
  INSERT INTO netcfg.interfaces (asset_id,name,ip_cidr,zone_id,vlan,enabled)
  VALUES (fw_id,'ethernet1/2','10.1.1.1/24',z_trust,10,true) ON CONFLICT (asset_id,name) DO NOTHING;

  IF (SELECT count(*) FROM netcfg.rules WHERE asset_id=fw_id) = 0 THEN
    INSERT INTO netcfg.rules (asset_id,seq,name,action,src_zone,dst_zone,source,destination,service,protocol,ports,enabled) VALUES
      (fw_id,10,'allow-web-out','allow','trust','untrust','10.1.1.0/24','any','web-browsing','tcp','80,443',true),
      (fw_id,20,'allow-dns','allow','trust','untrust','any','any','dns','udp','53',true),
      (fw_id,30,'permit-dmz-https','allow','untrust','dmz','any','10.1.2.0/24','https','tcp','443',true),
      (fw_id,99,'deny-all','deny','any','any','any','any','any',NULL,NULL,true);
  END IF;

  IF (SELECT count(*) FROM netcfg.config_backups WHERE asset_id=fw_id) = 0 THEN
    INSERT INTO netcfg.config_backups (asset_id,source,version,hash,size_bytes,content,note)
    VALUES (fw_id,'seed','v10.2.3',md5('demo-config'),64,
            E'set deviceconfig system hostname FW-LON-01\nset zone trust\nset zone untrust\nset rulebase security rules allow-web-out from trust to untrust action allow',
            'Initial seeded snapshot');
  END IF;
END $$;
