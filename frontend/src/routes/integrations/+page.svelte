<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { API_URL } from '$lib/config';
	import { me, can } from '$lib/me';
	import type { Connector, SyncRun, AssetType, Mapping, FieldMap, ConnectorHealthRow } from '$lib/types';

	let connectors = $state<Connector[]>([]);
	let connectorHealth = $state<ConnectorHealthRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	const manage = $derived(can($me, 'integration.manage'));

	// detail drawer
	let sel = $state<Connector | null>(null);
	let runs = $state<SyncRun[]>([]);
	let mappings = $state<Mapping[]>([]);
	let assetTypes = $state<AssetType[]>([]);
	let busy = $state(false);
	let testResult = $state<Record<string, unknown> | null>(null);

	// create form
	let showForm = $state(false);
	let mode = $state<'push' | 'intune' | 'defender' | 'ops_manager' | 'azure_arc'>('push');
	let form = $state({ key: '', name: '', kind: 'generic', default_asset_type: '' });
	let pull = $state({ tenant: '', client_id: '', client_secret: '', scope: 'https://graph.microsoft.com/.default', schedule_minutes: '60' });
	let arc = $state({ subscription_id: '' });
	let ops = $state({ base_url: '', api_key: '', schedule_minutes: '60' });
	let saving = $state(false);

	// freshly-minted token, shown once
	let newToken = $state('');
	let newPath = $state('');

	const KINDS = [
		{ id: 'generic', label: 'Generic (push / webhook)' },
		{ id: 'intune', label: 'Microsoft Intune' },
		{ id: 'defender', label: 'Microsoft Defender' },
		{ id: 'azure_arc', label: 'Azure Arc (Hybrid Compute)' },
		{ id: 'entra', label: 'Microsoft Entra ID' },
		{ id: 'ops_manager', label: 'ManageEngine OpManager' },
		{ id: 'csv', label: 'CSV / SFTP' }
	];

	// Microsoft Graph presets: object endpoints + field mappings (Graph JSON → asset).
	type Preset = {
		kind: string;
		name: string;
		source_object: string;
		url: string;
		identity: string;
		match_fallbacks: string[];
		fields: FieldMap[];
		type_resolution?: { by?: string; map?: Record<string, string>; default?: string };
	};
	const PRESETS: Record<'intune' | 'defender' | 'ops_manager' | 'azure_arc', Preset> = {
		intune: {
			kind: 'intune',
			name: 'Microsoft Intune devices',
			source_object: 'managedDevice',
			url: 'https://graph.microsoft.com/v1.0/deviceManagement/managedDevices',
			identity: 'id',
			match_fallbacks: ['serialNumber', 'azureADDeviceId'],
			fields: [
				{ source: 'deviceName', target: 'name' },
				{ source: 'serialNumber', target: 'serial' },
				{ source: 'id', target: 'external_id' },
				{ source: 'lastSyncDateTime', target: 'last_seen', transform: 'parse_date' },
				{ source: 'userPrincipalName', target: 'assigned_email' },
				{ source: 'userDisplayName', target: 'assigned_name' },
				{ source: 'operatingSystem', target: 'attr.os' },
				{ source: 'osVersion', target: 'attr.os_version' },
				{ source: 'model', target: 'attr.model' },
				{ source: 'manufacturer', target: 'attr.manufacturer' },
				{ source: 'totalStorageSpaceInBytes', target: 'attr.storage_total_bytes' },
				{ source: 'freeStorageSpaceInBytes', target: 'attr.storage_free_bytes' },
				{ source: 'physicalMemoryInBytes', target: 'attr.memory_bytes' },
				{ source: 'wiFiMacAddress', target: 'attr.wifi_mac' },
				{ source: 'ethernetMacAddress', target: 'attr.ethernet_mac' },
				{ source: 'userDisplayName', target: 'attr.primary_user' },
				{ source: 'complianceState', target: 'attr.compliance' },
				{ source: 'managementState', target: 'attr.management_state' },
				{ source: 'managedDeviceOwnerType', target: 'attr.ownership' },
				{ source: 'deviceEnrollmentType', target: 'attr.enrollment_type' },
				{ source: 'enrolledDateTime', target: 'attr.enrolled_at', transform: 'parse_date' },
				{ source: 'isEncrypted', target: 'attr.encrypted' },
				{ source: 'azureADDeviceId', target: 'attr.azure_ad_device_id' },
				{ source: 'deviceCategoryDisplayName', target: 'attr.category' }
			]
		},
		defender: {
			kind: 'defender',
			name: 'Microsoft Defender machines',
			source_object: 'machine',
			url: 'https://graph.microsoft.com/v1.0/security/machines',
			identity: 'id',
			match_fallbacks: ['computerDnsName'],
			fields: [
				{ source: 'computerDnsName', target: 'name' },
				{ source: 'id', target: 'external_id' },
				{ source: 'lastSeen', target: 'last_seen', transform: 'parse_date' },
				{ source: 'osPlatform', target: 'attr.os' },
				{ source: 'version', target: 'attr.os_version' },
				{ source: 'lastIpAddress', target: 'attr.ip' },
				{ source: 'riskScore', target: 'attr.risk_score' },
				{ source: 'exposureLevel', target: 'attr.exposure' },
				{ source: 'healthStatus', target: 'attr.health' }
			]
		},
		azure_arc: {
			kind: 'azure_arc',
			name: 'Azure Arc servers (Hybrid Compute)',
			source_object: 'hybridComputeMachine',
			url: 'https://management.azure.com/subscriptions/{{subscription_id}}/providers/Microsoft.HybridCompute/machines?api-version=2024-07-10',
			identity: 'id',
			match_fallbacks: ['properties.vmId', 'properties.vmUuid', 'name', 'properties.machineFqdn'],
			fields: [
				{ source: 'name', target: 'name' },
				{ source: 'properties.displayName', target: 'attr.display_name' },
				{ source: 'id', target: 'external_id' },
				{ source: 'properties.lastStatusChange', target: 'last_seen', transform: 'parse_date' },
				{ source: 'properties.status', target: 'attr.arc_status' },
				{ source: 'properties.machineFqdn', target: 'attr.fqdn' },
				{ source: 'properties.osName', target: 'attr.os' },
				{ source: 'properties.osSku', target: 'attr.os_sku' },
				{ source: 'properties.osVersion', target: 'attr.os_version' },
				{ source: 'properties.osType', target: 'attr.os_type' },
				{ source: 'properties.vmId', target: 'attr.vm_id' },
				{ source: 'properties.vmUuid', target: 'attr.vm_uuid' },
				{ source: 'properties.agentVersion', target: 'attr.arc_agent_version' },
				{ source: 'location', target: 'attr.azure_location' },
				{ source: 'properties.domainName', target: 'attr.domain' },
				{ source: 'properties.mssqlDiscovered', target: 'attr.mssql_discovered' },
				{ source: 'properties.detectedProperties.manufacturer', target: 'attr.manufacturer' },
				{ source: 'properties.detectedProperties.model', target: 'attr.model' },
				{ source: 'properties.detectedProperties.coreCount', target: 'attr.cpu_cores' },
				{ source: 'properties.detectedProperties.logicalCoreCount', target: 'attr.cpu_logical_cores' },
				{ source: 'properties.detectedProperties.totalPhysicalMemoryInGigabytes', target: 'attr.memory_gb' },
				{ source: 'properties.detectedProperties.hypervisorType', target: 'attr.hypervisor' },
				{ source: 'properties.detectedProperties.cloudprovider', target: 'attr.cloud_provider' },
				{ source: 'properties.firmwareProfile.serialNumber', target: 'serial' },
				{ source: 'properties.networkProfile.networkInterfaces.0.ipAddresses.0.address', target: 'attr.ip' },
				{ source: 'properties.networkProfile.networkInterfaces.0.macAddress', target: 'attr.mac' },
				{ source: 'properties.licenseProfile.licenseStatus', target: 'attr.license_status' }
			]
		},
		ops_manager: {
			kind: 'ops_manager',
			name: 'ManageEngine OpManager devices',
			source_object: 'device',
			url: '',
			identity: 'id',
			match_fallbacks: ['ipaddress', 'deviceName', 'moid'],
			type_resolution: {
				by: 'category',
				map: { Desktop: 'desktop', Server: 'server', Switch: 'switch', Router: 'router', Laptop: 'laptop' },
				default: 'server'
			},
			fields: [
				{ source: 'displayName', target: 'name' },
				{ source: 'id', target: 'external_id' },
				{ source: 'ipaddress', target: 'attr.ip' },
				{ source: 'type', target: 'attr.device_type' },
				{ source: 'vendorName', target: 'attr.manufacturer' },
				{ source: 'category', target: 'attr.category' },
				{ source: 'statusStr', target: 'attr.monitor_status' },
				{ source: 'probeName', target: 'attr.probe' },
				{ source: 'probeDisplayName', target: 'attr.probe_display' },
				{ source: 'mapName', target: 'attr.network_map' },
				{ source: 'addedTime', target: 'attr.added_at', transform: 'parse_date' },
				{ source: 'isSNMP', target: 'attr.snmp' },
				{ source: 'interfaceCount', target: 'attr.interface_count' },
				{ source: 'deviceName', target: 'attr.opmanager_device_name' },
				{ source: 'moid', target: 'attr.moid' },
				{ source: 'statusNum', target: 'attr.status_code' }
			]
		}
	};

	function startPreset(which: 'intune' | 'defender' | 'ops_manager' | 'azure_arc') {
		mode = which;
		const p = PRESETS[which];
		const defaultType =
			which === 'ops_manager' || which === 'azure_arc' ? 'server' : 'laptop';
		form = { key: which + '_devices', name: p.name, kind: p.kind, default_asset_type: defaultType };
		if (which === 'azure_arc') {
			pull = {
				...pull,
				scope: 'https://management.azure.com/.default'
			};
			arc = { subscription_id: '' };
		}
		showForm = true;
	}

	async function load() {
		loading = true;
		error = '';
		try {
			connectors = (await apiGet<Connector[]>('/api/integration/connectors')) ?? [];
			if (can($me, 'asset.read')) {
				assetTypes = (await apiGet<AssetType[]>('/api/metadata/asset-types')) ?? [];
			}
			try {
				connectorHealth = (await apiGet<ConnectorHealthRow[]>('/api/reports/connector-health')) ?? [];
			} catch {
				connectorHealth = [];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}
	onMount(load);

	async function openConnector(c: Connector) {
		sel = c;
		runs = [];
		mappings = [];
		testResult = null;
		try {
			runs = (await apiGet<SyncRun[]>(`/api/integration/connectors/${c.id}/runs`)) ?? [];
			mappings = (await apiGet<Mapping[]>(`/api/integration/connectors/${c.id}/mappings`)) ?? [];
		} catch {
			/* ignore */
		}
	}

	async function create() {
		saving = true;
		error = '';
		try {
			if (mode === 'push') {
				const body = {
					key: form.key,
					name: form.name,
					kind: form.kind,
					config: form.default_asset_type ? { default_asset_type: form.default_asset_type } : {}
				};
				const res = await apiPost<{ ingest_token: string; ingest_path: string }>(
					'/api/integration/connectors',
					body
				);
				newToken = res.ingest_token;
				newPath = res.ingest_path;
			} else {
				const p = PRESETS[mode];
				const scheduleMin = mode === 'ops_manager' ? ops.schedule_minutes : pull.schedule_minutes;
				const defaultType =
					form.default_asset_type ||
					p.type_resolution?.default ||
					(mode === 'ops_manager' || mode === 'azure_arc' ? 'server' : 'laptop');
				let objectUrl = mode === 'ops_manager' ? ops.base_url.trim() : p.url;
				if (mode === 'azure_arc') {
					const sub = arc.subscription_id.trim();
					if (!sub) throw new Error('Azure subscription ID is required');
					objectUrl = objectUrl.replace(/\{\{subscription_id\}\}/g, sub);
				}
				const config: Record<string, unknown> = {
					schedule_seconds: (Number(scheduleMin) || 60) * 60,
					default_asset_type: defaultType,
					discovered_state: 'in_use',
					auto_create_people: mode !== 'ops_manager' && mode !== 'azure_arc',
					update_location_on_sync: true,
					objects: [
						{
							source_object: p.source_object,
							url: objectUrl
						}
					]
				};
				if (mode === 'ops_manager') {
					config.auth = { type: 'api_key', in: 'query', param: 'apiKey' };
				} else {
					config.auth = {
						token_endpoint: `https://login.microsoftonline.com/${pull.tenant}/oauth2/v2.0/token`,
						client_id: pull.client_id,
						scope: pull.scope
					};
				}
				const typeResolution = p.type_resolution
					? { ...p.type_resolution, default: defaultType }
					: { default: defaultType };
				const res = await apiPost<{ connector: Connector }>('/api/integration/connectors', {
					key: form.key,
					name: form.name,
					kind: p.kind,
					direction: 'pull',
					config,
					pull_secret: mode === 'ops_manager' ? ops.api_key : pull.client_secret
				});
				if (res?.connector?.id) {
					await apiPost(`/api/integration/connectors/${res.connector.id}/mappings`, {
						source_object: p.source_object,
						target_entity: 'asset',
						identity: p.identity,
						match_fallbacks: p.match_fallbacks,
						type_resolution: typeResolution,
						fields: p.fields,
						filters: [],
						enabled: true,
						sort: 1
					});
				}
			}
			showForm = false;
			mode = 'push';
			form = { key: '', name: '', kind: 'generic', default_asset_type: '' };
			pull = { tenant: '', client_id: '', client_secret: '', scope: 'https://graph.microsoft.com/.default', schedule_minutes: '60' };
			arc = { subscription_id: '' };
			ops = { base_url: '', api_key: '', schedule_minutes: '60' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create';
		} finally {
			saving = false;
		}
	}

	async function runNow(c: Connector) {
		busy = true;
		error = '';
		try {
			await apiPost(`/api/integration/connectors/${c.id}/run`);
			runs = (await apiGet<SyncRun[]>(`/api/integration/connectors/${c.id}/runs`)) ?? [];
			await load();
		} catch (e) {
			error = `Run failed: ${e instanceof Error ? e.message : e}`;
		} finally {
			busy = false;
		}
	}

	async function reprocess(c: Connector) {
		busy = true;
		error = '';
		try {
			await apiPost(`/api/integration/connectors/${c.id}/reprocess`);
			runs = (await apiGet<SyncRun[]>(`/api/integration/connectors/${c.id}/runs`)) ?? [];
		} catch (e) {
			error = `Reprocess failed: ${e instanceof Error ? e.message : e}`;
		} finally {
			busy = false;
		}
	}

	async function testConn(c: Connector) {
		busy = true;
		testResult = null;
		error = '';
		try {
			testResult = await apiPost<Record<string, unknown>>(`/api/integration/connectors/${c.id}/test`);
		} catch (e) {
			error = `Test failed: ${e instanceof Error ? e.message : e}`;
		} finally {
			busy = false;
		}
	}

	async function toggleEnabled(c: Connector) {
		try {
			await apiPut(`/api/integration/connectors/${c.id}`, { ...c, enabled: !c.enabled });
			await load();
			if (sel?.id === c.id) sel = { ...sel, enabled: !c.enabled };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update';
		}
	}

	async function rotate(c: Connector) {
		if (!confirm(`Rotate the ingest token for "${c.name}"? The old token stops working immediately.`))
			return;
		try {
			const res = await apiPost<{ ingest_token: string }>(
				`/api/integration/connectors/${c.id}/rotate-secret`
			);
			newToken = res.ingest_token;
			newPath = '/ingest/' + c.key;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to rotate';
		}
	}

	async function remove(c: Connector) {
		if (!confirm(`Delete connector "${c.name}"? Its sync history is removed too.`)) return;
		try {
			await apiDelete(`/api/integration/connectors/${c.id}`);
			if (sel?.id === c.id) sel = null;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete';
		}
	}

	function fullUrl(path: string): string {
		return `${API_URL}${path}`;
	}
	function copy(text: string) {
		navigator.clipboard?.writeText(text);
	}
	function fmt(ts?: string | null): string {
		return ts ? new Date(ts).toLocaleString() : '—';
	}
	function statusClass(s?: string): string {
		if (s === 'success') return 'ok';
		if (s === 'partial') return 'warn';
		if (s === 'error') return 'bad';
		return 'muted';
	}

	const sample = `curl -X POST \\
  ${fullUrl('/ingest/<key>')} \\
  -H "X-ITAM-Token: <token>" \\
  -H "Content-Type: application/json" \\
  -d '{"devices":[{"external_id":"INTUNE-123","name":"LAPTOP-01","serial":"SN123","asset_type":"laptop","last_seen":"2026-06-10T12:00:00Z","attributes":{"os":"Windows 11"},"software":[{"name":"Google Chrome","version":"125"}]}]}'`;
</script>

<div class="page">
	<header class="head">
		<div>
			<h1>Integrations</h1>
			<p class="muted">Ingest devices from external sources. Connectors authenticate with a token and reconcile against existing assets by identity, serial, then asset tag.</p>
		</div>
		{#if manage}
			<div class="row">
				<button class="btn secondary" onclick={() => startPreset('intune')}>+ Intune</button>
				<button class="btn secondary" onclick={() => startPreset('defender')}>+ Defender</button>
				<button class="btn secondary" onclick={() => startPreset('azure_arc')}>+ Azure Arc</button>
				<button class="btn secondary" onclick={() => startPreset('ops_manager')}>+ OpManager</button>
				<button class="btn" onclick={() => { mode = 'push'; showForm = !showForm; }}>New connector</button>
			</div>
		{/if}
	</header>

	{#if error}<div class="banner bad">{error}</div>{/if}

	{#if connectorHealth.length}
		<div class="card" style="margin-bottom:16px">
			<h3 style="margin-top:0">Connector health (24h)</h3>
			<table>
				<thead><tr><th>Connector</th><th>Status</th><th>Runs</th><th>Errors</th><th>Last run</th><th>Last sync</th></tr></thead>
				<tbody>
					{#each connectorHealth as ch}
						<tr>
							<td>{ch.name} <span class="muted">({ch.kind})</span></td>
							<td><span class="badge" class:bad={ch.last_status === 'error' || ch.errors_24h > 0}>{ch.last_status || '—'}</span></td>
							<td>{ch.runs_24h}</td>
							<td>{ch.errors_24h}</td>
							<td class="muted">{ch.last_run_at ? new Date(ch.last_run_at).toLocaleString() : '—'}</td>
							<td class="muted">{ch.last_seen} seen · {ch.last_created} new · {ch.last_updated} upd</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	{#if newToken}
		<div class="banner ok token-banner">
			<div>
				<strong>Connector ready.</strong> Copy the ingest token now — it is shown only once.
				<div class="kv">
					<code>POST {fullUrl(newPath)}</code>
					<button class="btn secondary small" onclick={() => copy(fullUrl(newPath))}>Copy URL</button>
				</div>
				<div class="kv">
					<code>X-ITAM-Token: {newToken}</code>
					<button class="btn secondary small" onclick={() => copy(newToken)}>Copy token</button>
				</div>
			</div>
			<button class="btn secondary small" onclick={() => (newToken = '')}>Dismiss</button>
		</div>
	{/if}

	{#if showForm}
		<div class="card form">
			<div class="grid cols-2">
				<div class="field">
					<label for="cn-name">Name</label>
					<input id="cn-name" bind:value={form.name} placeholder="Intune devices" />
				</div>
				<div class="field">
					<label for="cn-key">Key</label>
					<input id="cn-key" bind:value={form.key} placeholder="intune_devices" />
				</div>
				{#if mode === 'push'}
					<div class="field">
						<label for="cn-kind">Kind</label>
						<select id="cn-kind" bind:value={form.kind}>
							{#each KINDS as k}<option value={k.id}>{k.label}</option>{/each}
						</select>
					</div>
				{/if}
				<div class="field">
					<label for="cn-type">Default asset type</label>
					<select id="cn-type" bind:value={form.default_asset_type}>
						<option value="">— required only if devices omit asset_type —</option>
						{#each assetTypes.filter((t) => !t.is_abstract) as t}
							<option value={t.key}>{t.name}</option>
						{/each}
					</select>
				</div>
			</div>

			{#if mode === 'intune' || mode === 'defender' || mode === 'azure_arc'}
				<div class="ms-box">
					<div class="ms-title">
						{PRESETS[mode].name} —
						{mode === 'azure_arc' ? 'Azure Resource Manager (app-only)' : 'Microsoft Graph (app-only / client credentials)'}
					</div>
					<div class="grid cols-2">
						<div class="field"><label>Directory (tenant) ID</label><input bind:value={pull.tenant} placeholder="00000000-0000-0000-0000-000000000000" /></div>
						<div class="field"><label>Application (client) ID</label><input bind:value={pull.client_id} placeholder="app registration client id" /></div>
						<div class="field"><label>Client secret</label><input type="password" bind:value={pull.client_secret} placeholder="stored encrypted; never shown again" /></div>
						<div class="field"><label>Scope</label><input bind:value={pull.scope} /></div>
						{#if mode === 'azure_arc'}
							<div class="field" style="grid-column: 1 / -1">
								<label>Azure subscription ID</label>
								<input bind:value={arc.subscription_id} placeholder="50838948-4faa-4d2c-b769-87a520d9f53b" />
							</div>
						{/if}
						<div class="field"><label>Poll every (minutes)</label><input bind:value={pull.schedule_minutes} /></div>
					</div>
					<p class="muted small" style="margin:8px 0 0">
						Pulls <code class="mono">{PRESETS[mode].source_object}</code> from
						{#if mode === 'azure_arc'}
							<code class="mono">management.azure.com</code> (Hybrid Compute machines in the subscription above)
						{:else}
							<code class="mono">{PRESETS[mode].url}</code>
						{/if}
						and creates a field mapping automatically.
						{#if mode === 'azure_arc'}
							Assign the service principal <strong>Reader</strong> on the subscription (or resource group) so it can list
							<code class="mono">Microsoft.HybridCompute/machines</code>. OAuth scope:
							<code class="mono">https://management.azure.com/.default</code>.
						{:else}
							Required Graph permission: {mode === 'intune' ? 'DeviceManagementManagedDevices.Read.All' : 'Machine.Read.All'} (application). Use <strong>Test connection</strong> after creating.
						{/if}
					</p>
				</div>
			{:else if mode === 'ops_manager'}
				<div class="ms-box">
					<div class="ms-title">{PRESETS.ops_manager.name} — REST API (API key)</div>
					<div class="grid cols-2">
						<div class="field" style="grid-column: 1 / -1">
							<label>Device list URL</label>
							<input bind:value={ops.base_url} placeholder="https://your-opmanager:8061/api/json/device/listDevices" />
						</div>
						<div class="field"><label>API key</label><input type="password" bind:value={ops.api_key} placeholder="stored encrypted; never shown again" /></div>
						<div class="field"><label>Poll every (minutes)</label><input bind:value={ops.schedule_minutes} /></div>
					</div>
					<p class="muted small" style="margin:8px 0 0">
						Pulls monitored devices from OpManager's <code class="mono">listDevices</code> JSON endpoint. The API key is appended as
						<code class="mono">?apiKey=…</code> at fetch time — do not include it in the URL above. Asset type is resolved from the
						<code class="mono">category</code> field (Desktop → desktop, Server → server, etc.).
					</p>
				</div>
			{/if}

			<div class="actions">
				<button
					class="btn"
					onclick={create}
					disabled={saving ||
						!form.name ||
						!form.key ||
						((mode === 'intune' || mode === 'defender' || mode === 'azure_arc') &&
							(!pull.tenant || !pull.client_id || !pull.client_secret || (mode === 'azure_arc' && !arc.subscription_id.trim()))) ||
						(mode === 'ops_manager' && (!ops.base_url.trim() || !ops.api_key))}
				>
					{saving ? 'Creating…' : mode === 'push' ? 'Create connector' : 'Create & map'}
				</button>
				<button class="btn secondary" onclick={() => { showForm = false; mode = 'push'; }}>Cancel</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<div class="card">
			<table>
				<thead>
					<tr><th>Connector</th><th>Kind</th><th>Status</th><th>Last run</th><th>Result</th>{#if manage}<th></th>{/if}</tr>
				</thead>
				<tbody>
					{#each connectors as c}
						<tr
							role="button"
							tabindex="0"
							onclick={() => openConnector(c)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openConnector(c))}
						>
							<td><strong>{c.name}</strong><div class="muted mono">{c.key}</div></td>
							<td><span class="badge">{c.kind}</span></td>
							<td>
								{#if c.enabled}<span class="dot ok"></span> Enabled{:else}<span class="dot"></span> <span class="muted">Disabled</span>{/if}
							</td>
							<td class="muted">{fmt(c.last_run_at)}</td>
							<td class={statusClass(c.last_status)}>{c.last_status || '—'}</td>
							{#if manage}
								<td style="text-align:right" onclick={(e) => e.stopPropagation()}>
									<button class="btn secondary small" onclick={() => toggleEnabled(c)}>{c.enabled ? 'Disable' : 'Enable'}</button>
									<button class="btn danger small" onclick={() => remove(c)}>Delete</button>
								</td>
							{/if}
						</tr>
					{/each}
					{#if connectors.length === 0}
						<tr><td colspan={manage ? 6 : 5} class="muted">No connectors yet. Create one to start ingesting devices.</td></tr>
					{/if}
				</tbody>
			</table>
		</div>

		<div class="card help">
			<h3>Push devices to a connector</h3>
			<p class="muted">Send a JSON payload to the connector's ingest URL with its token. Records are matched and created or updated automatically.</p>
			<pre>{sample}</pre>
			<p class="muted small">Authenticate with <code>X-ITAM-Token: &lt;token&gt;</code> or an HMAC <code>X-ITAM-Signature: sha256=&lt;hmac&gt;</code> of the raw body.</p>
		</div>
	{/if}
</div>

{#if sel}
	{@const cn = sel}
	<div class="backdrop" onclick={() => (sel = null)} role="presentation"></div>
	<aside class="drawer">
		<header class="drawer-head">
			<div>
				<h2>{cn.name}</h2>
				<div class="muted mono">{cn.key} · {cn.kind}</div>
			</div>
			<button class="btn secondary small" onclick={() => (sel = null)}>Close</button>
		</header>

		<div class="kv">
			<code>POST {fullUrl('/ingest/' + cn.key)}</code>
			<button class="btn secondary small" onclick={() => copy(fullUrl('/ingest/' + cn.key))}>Copy</button>
		</div>
		{#if manage}
			<div class="row" style="margin:10px 0; flex-wrap:wrap">
				{#if cn.direction === 'pull'}
					<button class="btn small" onclick={() => runNow(cn)} disabled={busy}>{busy ? 'Working…' : 'Run now'}</button>
					<button class="btn secondary small" onclick={() => testConn(cn)} disabled={busy}>Test connection</button>
					<button class="btn secondary small" onclick={() => reprocess(cn)} disabled={busy}>Reprocess</button>
				{:else}
					<button class="btn secondary small" onclick={() => rotate(cn)}>Rotate token</button>
				{/if}
				<button class="btn secondary small" onclick={() => toggleEnabled(cn)}>{cn.enabled ? 'Disable' : 'Enable'}</button>
			</div>
		{/if}

		{#if testResult}
			{@const ok = testResult.ok === true}
			<div class="banner {ok ? 'ok' : 'bad'}" style="margin-top:6px">
				<strong>{ok ? 'Success' : 'Failed'}</strong> ({String(testResult.stage)}): {String(testResult.message)}
				{#if testResult.sample_fields}
					<div class="muted small" style="margin-top:6px">Sample fields: <span class="mono">{(testResult.sample_fields as string[]).slice(0, 12).join(', ')}</span></div>
				{/if}
			</div>
		{/if}

		{#if cn.direction === 'pull'}
			<h3 style="margin-top:18px">Field mappings</h3>
			{#if mappings.length === 0}
				<p class="muted small">No mappings. Records will land in raw storage but won't become assets.</p>
			{:else}
				{#each mappings as m}
					<div class="map-card">
						<div class="row" style="justify-content:space-between">
							<strong class="mono">{m.source_object} → {m.target_entity}</strong>
							<span class="badge">{m.fields.length} field(s)</span>
						</div>
						<div class="muted small" style="margin-top:4px">id: <span class="mono">{m.identity}</span>{#if m.match_fallbacks?.length} · fallback: <span class="mono">{m.match_fallbacks.join(', ')}</span>{/if}</div>
						<div class="map-fields">
							{#each m.fields as f}
								<span class="chip mono">{f.source} → {f.target}{#if f.transform} <span class="muted">({f.transform})</span>{/if}</span>
							{/each}
						</div>
					</div>
				{/each}
			{/if}
		{/if}

		<h3 style="margin-top:18px">Recent sync runs</h3>
		<table>
			<thead><tr><th>When</th><th>Status</th><th>Seen</th><th>New</th><th>Upd</th><th>Err</th></tr></thead>
			<tbody>
				{#each runs as run}
					<tr>
						<td class="muted">{fmt(run.started_at)}</td>
						<td class={statusClass(run.status)}>{run.status}</td>
						<td>{run.seen}</td>
						<td>{run.created}</td>
						<td>{run.updated}</td>
						<td class={run.errors ? 'bad' : 'muted'}>{run.errors}</td>
					</tr>
				{/each}
				{#if runs.length === 0}<tr><td colspan="6" class="muted">No sync runs yet.</td></tr>{/if}
			</tbody>
		</table>
	</aside>
{/if}

<style>
	.page { padding: 22px 26px; max-width: 1100px; }
	.head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 16px; }
	.head h1 { margin: 0 0 4px; }
	.head p { margin: 0; max-width: 640px; }
	.card { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 16px; }
	.card.form { padding: 18px; }
	.grid { display: grid; gap: 12px; }
	.cols-2 { grid-template-columns: 1fr 1fr; }
	.field label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 4px; }
	.field input, .field select { width: 100%; }
	.actions { display: flex; gap: 8px; margin-top: 14px; }
	table { width: 100%; border-collapse: collapse; }
	th, td { text-align: left; padding: 9px 10px; border-bottom: 1px solid var(--border); font-size: 14px; }
	tbody tr { cursor: pointer; }
	tbody tr:hover { background: var(--surface-2); }
	.badge { background: var(--surface-2); border: 1px solid var(--border); border-radius: 6px; padding: 1px 8px; font-size: 12px; }
	.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #888; margin-right: 4px; }
	.dot.ok { background: var(--success); }
	.muted { color: var(--muted); }
	.mono, .small { font-size: 12px; }
	.mono { font-family: ui-monospace, monospace; }
	.ok { color: var(--success); }
	.warn { color: #c98a00; }
	.bad { color: var(--danger); }
	.banner { padding: 12px 14px; border-radius: 10px; margin-bottom: 14px; border: 1px solid var(--border); }
	.banner.bad { background: color-mix(in srgb, var(--danger) 12%, transparent); border-color: var(--danger); }
	.banner.ok { background: color-mix(in srgb, var(--success) 12%, transparent); border-color: var(--success); }
	.token-banner { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; }
	.kv { display: flex; gap: 8px; align-items: center; margin-top: 6px; flex-wrap: wrap; }
	.kv code { background: var(--bg); padding: 4px 8px; border-radius: 6px; font-size: 12px; word-break: break-all; }
	.help pre { background: var(--bg); padding: 12px; border-radius: 8px; overflow-x: auto; font-size: 12px; line-height: 1.5; }
	.backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.4); z-index: 40; }
	.drawer { position: fixed; top: 0; right: 0; bottom: 0; width: 520px; max-width: 92vw; background: var(--surface); border-left: 1px solid var(--border); box-shadow: -8px 0 24px rgba(0, 0, 0, 0.18); padding: 20px; overflow-y: auto; z-index: 41; }
	.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 14px; }
	.drawer-head h2 { margin: 0 0 2px; }
	.row { display: flex; gap: 8px; }
	.btn.small { padding: 4px 10px; font-size: 12px; }
	.ms-box { margin-top: 14px; padding: 14px; border: 1px solid var(--border); border-radius: 10px; background: var(--bg); }
	.ms-title { font-weight: 600; margin-bottom: 10px; font-size: 13px; }
	.map-card { border: 1px solid var(--border); border-radius: 10px; padding: 12px; margin-bottom: 10px; }
	.map-fields { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
	.chip { background: var(--surface-2); border: 1px solid var(--border); border-radius: 6px; padding: 2px 7px; font-size: 11px; }
</style>
