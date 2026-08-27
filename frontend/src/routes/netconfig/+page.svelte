<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Asset, NetZone, NetInterface, NetRule, HAGroup, ConfigBackup, AddressObject, NetRoute, NATRule, BackupDiffLine } from '$lib/types';

	let devices = $state<Asset[]>([]);
	let sel = $state<Asset | null>(null);
	let zones = $state<NetZone[]>([]);
	let ifaces = $state<NetInterface[]>([]);
	let rules = $state<NetRule[]>([]);
	let backups = $state<ConfigBackup[]>([]);
	let haGroups = $state<HAGroup[]>([]);
	let addrObjs = $state<AddressObject[]>([]);
	let routes = $state<NetRoute[]>([]);
	let natRules = $state<NATRule[]>([]);
	let tab = $state<'rules' | 'interfaces' | 'zones' | 'addr' | 'routes' | 'nat' | 'backups'>('rules');
	let error = $state('');
	const manage = $derived(can($me, 'netcfg.manage'));

	let zForm = $state({ name: '', description: '' });
	let iForm = $state({ name: '', ip_cidr: '', zone_id: '', vlan: '' });
	let rForm = $state({ seq: '', name: '', action: 'allow', src_zone: '', dst_zone: '', source: '', destination: '', service: '', protocol: 'tcp', ports: '' });
	let aoForm = $state({ name: '', kind: 'host', value: '', description: '' });
	let rtForm = $state({ destination: '', next_hop: '', interface: '', metric: '', description: '' });
	let natForm = $state({ name: '', nat_type: 'source', orig_src: '', orig_dst: '', orig_service: '', trans_src: '', trans_dst: '', trans_service: '' });
	let bForm = $state({ version: '', note: '', content: '' });
	let viewing = $state<ConfigBackup | null>(null);
	let haForm = $state({ name: '', mode: 'active-passive', vip: '' });

	// backup diff
	let diffA = $state('');
	let diffB = $state('');
	let diffLines = $state<BackupDiffLine[]>([]);
	let diffStats = $state<{ added: number; removed: number } | null>(null);

	async function loadDevices() {
		try {
			devices = (await apiGet<Asset[]>('/api/netcfg/devices')) ?? [];
			haGroups = (await apiGet<HAGroup[]>('/api/netcfg/ha-groups')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	}
	onMount(loadDevices);

	async function select(d: Asset) {
		sel = d;
		await loadDevice();
	}
	async function loadDevice() {
		if (!sel) return;
		const id = sel.id;
		try {
			zones = (await apiGet<NetZone[]>(`/api/netcfg/assets/${id}/zones`)) ?? [];
			ifaces = (await apiGet<NetInterface[]>(`/api/netcfg/assets/${id}/interfaces`)) ?? [];
			rules = (await apiGet<NetRule[]>(`/api/netcfg/assets/${id}/rules`)) ?? [];
			backups = (await apiGet<ConfigBackup[]>(`/api/netcfg/assets/${id}/backups`)) ?? [];
			addrObjs = (await apiGet<AddressObject[]>(`/api/netcfg/assets/${id}/address-objects`)) ?? [];
			routes = (await apiGet<NetRoute[]>(`/api/netcfg/assets/${id}/routes`)) ?? [];
			natRules = (await apiGet<NATRule[]>(`/api/netcfg/assets/${id}/nat-rules`)) ?? [];
			diffA = ''; diffB = ''; diffLines = []; diffStats = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function addZone() {
		if (!sel || !zForm.name.trim()) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/zones`, zForm);
		zForm = { name: '', description: '' };
		await loadDevice();
	}
	async function addIface() {
		if (!sel || !iForm.name.trim()) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/interfaces`, {
			name: iForm.name,
			ip_cidr: iForm.ip_cidr || null,
			zone_id: iForm.zone_id || null,
			vlan: iForm.vlan ? Number(iForm.vlan) : null
		});
		iForm = { name: '', ip_cidr: '', zone_id: '', vlan: '' };
		await loadDevice();
	}
	async function addRule() {
		if (!sel) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/rules`, {
			seq: rForm.seq ? Number(rForm.seq) : rules.length + 1,
			name: rForm.name || null,
			action: rForm.action,
			src_zone: rForm.src_zone || null,
			dst_zone: rForm.dst_zone || null,
			source: rForm.source || null,
			destination: rForm.destination || null,
			service: rForm.service || null,
			protocol: rForm.protocol || null,
			ports: rForm.ports || null
		});
		rForm = { seq: '', name: '', action: 'allow', src_zone: '', dst_zone: '', source: '', destination: '', service: '', protocol: 'tcp', ports: '' };
		await loadDevice();
	}
	async function toggleRule(r: NetRule) {
		await apiPut(`/api/netcfg/assets/${sel!.id}/rules/${r.id}`, { ...r, enabled: !r.enabled });
		await loadDevice();
	}
	async function addBackup() {
		if (!sel || !bForm.content.trim()) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/backups`, bForm);
		bForm = { version: '', note: '', content: '' };
		await loadDevice();
	}
	async function viewBackup(b: ConfigBackup) {
		viewing = await apiGet<ConfigBackup>(`/api/netcfg/assets/${sel!.id}/backups/${b.id}`);
	}
	async function runDiff() {
		if (!sel || !diffA || !diffB) return;
		try {
			const res = await apiGet<{ added: number; removed: number; lines: BackupDiffLine[] }>(
				`/api/netcfg/assets/${sel.id}/backups/diff?a=${diffA}&b=${diffB}`
			);
			diffLines = res.lines ?? [];
			diffStats = { added: res.added, removed: res.removed };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Diff failed';
		}
	}
	async function registerIP(i: NetInterface) {
		if (!sel) return;
		try {
			await apiPost(`/api/netcfg/interfaces/${i.id}/register-ip`, {});
			await loadDevice();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to register IP';
		}
	}
	async function addAddrObj() {
		if (!sel || !aoForm.name.trim() || !aoForm.value.trim()) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/address-objects`, aoForm);
		aoForm = { name: '', kind: 'host', value: '', description: '' };
		await loadDevice();
	}
	async function addRoute() {
		if (!sel || !rtForm.destination.trim()) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/routes`, {
			destination: rtForm.destination,
			next_hop: rtForm.next_hop || null,
			interface: rtForm.interface || null,
			metric: rtForm.metric ? Number(rtForm.metric) : null,
			description: rtForm.description || null
		});
		rtForm = { destination: '', next_hop: '', interface: '', metric: '', description: '' };
		await loadDevice();
	}
	async function addNat() {
		if (!sel) return;
		await apiPost(`/api/netcfg/assets/${sel.id}/nat-rules`, {
			name: natForm.name || null,
			nat_type: natForm.nat_type,
			orig_src: natForm.orig_src || null,
			orig_dst: natForm.orig_dst || null,
			orig_service: natForm.orig_service || null,
			trans_src: natForm.trans_src || null,
			trans_dst: natForm.trans_dst || null,
			trans_service: natForm.trans_service || null,
			seq: natRules.length + 1
		});
		natForm = { name: '', nat_type: 'source', orig_src: '', orig_dst: '', orig_service: '', trans_src: '', trans_dst: '', trans_service: '' };
		await loadDevice();
	}
	async function delSub(kind: string, id: string | number) {
		if (!sel) return;
		await apiDelete(`/api/netcfg/assets/${sel.id}/${kind}/${id}`);
		await loadDevice();
	}
	async function addHA() {
		if (!haForm.name.trim()) return;
		await apiPost('/api/netcfg/ha-groups', haForm);
		haForm = { name: '', mode: 'active-passive', vip: '' };
		await loadDevices();
	}
	async function addHAMember(gid: string, assetId: string, role: string) {
		if (!assetId) return;
		await apiPost(`/api/netcfg/ha-groups/${gid}/members`, { asset_id: assetId, role });
		await loadDevices();
	}
	async function delHA(gid: string) {
		await apiDelete(`/api/netcfg/ha-groups/${gid}`);
		await loadDevices();
	}
</script>

<h1>Firewall / Network Config</h1>
{#if error}<p class="error">{error}</p>{/if}

<div class="layout">
	<aside class="devices">
		<h3>Devices</h3>
		{#each devices as d}
			<button class="dev" class:active={sel?.id === d.id} onclick={() => select(d)}>
				<span class="dn">{d.name}</span>
				<span class="dm">{d.asset_type?.name ?? ''} · {d.asset_tag}</span>
			</button>
		{/each}
		{#if devices.length === 0}<p class="muted small">No network/security devices. Create assets of type firewall, router, switch or load balancer.</p>{/if}

		<h3 style="margin-top:20px">HA groups</h3>
		{#each haGroups as g}
			<div class="ha">
				<div class="ha-head">
					<strong>{g.name}</strong> <span class="muted">{g.mode}{g.vip ? ` · VIP ${g.vip}` : ''}</span>
					{#if manage}<button class="link-danger" onclick={() => delHA(g.id)}>×</button>{/if}
				</div>
				{#each g.members ?? [] as m}
					<div class="ha-m">{m.asset?.name ?? m.asset_id} <span class="muted">{m.role}</span></div>
				{/each}
				{#if manage && sel}
					<button class="link" onclick={() => addHAMember(g.id, sel!.id, 'primary')}>+ add “{sel.name}” as primary</button>
					<button class="link" onclick={() => addHAMember(g.id, sel!.id, 'secondary')}>as secondary</button>
				{/if}
			</div>
		{/each}
		{#if manage}
			<div class="row" style="margin-top:8px; flex-wrap:wrap">
				<input placeholder="HA group name" bind:value={haForm.name} style="flex:1; min-width:110px" />
				<select bind:value={haForm.mode}>
					<option value="active-passive">active-passive</option>
					<option value="active-active">active-active</option>
				</select>
				<input placeholder="VIP" bind:value={haForm.vip} style="width:110px" />
				<button class="btn small" onclick={addHA}>Add</button>
			</div>
		{/if}
	</aside>

	<section class="detail">
		{#if !sel}
			<p class="muted">Select a device to view its configuration.</p>
		{:else}
			<div class="tabs">
				<button class:active={tab === 'rules'} onclick={() => (tab = 'rules')}>Policy ({rules.length})</button>
				<button class:active={tab === 'interfaces'} onclick={() => (tab = 'interfaces')}>Interfaces ({ifaces.length})</button>
				<button class:active={tab === 'zones'} onclick={() => (tab = 'zones')}>Zones ({zones.length})</button>
				<button class:active={tab === 'addr'} onclick={() => (tab = 'addr')}>Address objects ({addrObjs.length})</button>
				<button class:active={tab === 'routes'} onclick={() => (tab = 'routes')}>Routes ({routes.length})</button>
				<button class:active={tab === 'nat'} onclick={() => (tab = 'nat')}>NAT ({natRules.length})</button>
				<button class:active={tab === 'backups'} onclick={() => (tab = 'backups')}>Config backups ({backups.length})</button>
			</div>

			{#if tab === 'rules'}
				<table>
					<thead><tr><th>#</th><th>Name</th><th>Action</th><th>Src→Dst zone</th><th>Source</th><th>Destination</th><th>Service</th><th>On</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each rules as r}
							<tr class:disabled={!r.enabled}>
								<td>{r.seq}</td>
								<td>{r.name || '—'}</td>
								<td><span class="pill {r.action}">{r.action}</span></td>
								<td class="muted">{r.src_zone || '*'} → {r.dst_zone || '*'}</td>
								<td class="muted">{r.source || 'any'}</td>
								<td class="muted">{r.destination || 'any'}</td>
								<td class="muted">{r.service || (r.protocol ? `${r.protocol}/${r.ports || '*'}` : 'any')}</td>
								<td>{#if manage}<button class="link" onclick={() => toggleRule(r)}>{r.enabled ? 'yes' : 'no'}</button>{:else}{r.enabled ? 'yes' : 'no'}{/if}</td>
								{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delSub('rules', r.id)}>remove</button></td>{/if}
							</tr>
						{/each}
						{#if rules.length === 0}<tr><td colspan={manage ? 9 : 8} class="muted">No rules.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="#" bind:value={rForm.seq} style="width:48px" />
						<input placeholder="name" bind:value={rForm.name} style="width:110px" />
						<select bind:value={rForm.action}><option value="allow">allow</option><option value="deny">deny</option><option value="reject">reject</option></select>
						<input placeholder="src zone" bind:value={rForm.src_zone} style="width:90px" />
						<input placeholder="dst zone" bind:value={rForm.dst_zone} style="width:90px" />
						<input placeholder="source" bind:value={rForm.source} style="width:110px" />
						<input placeholder="destination" bind:value={rForm.destination} style="width:110px" />
						<input placeholder="service" bind:value={rForm.service} style="width:90px" />
						<input placeholder="ports" bind:value={rForm.ports} style="width:70px" />
						<button class="btn small" onclick={addRule}>Add rule</button>
					</div>
				{/if}
			{:else if tab === 'interfaces'}
				<table>
					<thead><tr><th>Name</th><th>IP / CIDR</th><th>Zone</th><th>VLAN</th><th>On</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each ifaces as i}
							<tr>
								<td><strong>{i.name}</strong></td>
								<td class="muted">{i.ip_cidr || '—'}</td>
								<td class="muted">{zones.find((z) => z.id === i.zone_id)?.name ?? '—'}</td>
								<td>{i.vlan ?? '—'}</td>
								<td>{i.enabled ? 'yes' : 'no'}</td>
								{#if manage}<td style="text-align:right; white-space:nowrap">
									{#if i.ip_cidr}<button class="link" title={i.ip_id ? 'Already in IPAM' : 'Create IPAM record'} onclick={() => registerIP(i)}>{i.ip_id ? 'in IPAM ✓' : 'register IP'}</button>{/if}
									<button class="link-danger" onclick={() => delSub('interfaces', i.id)}>remove</button>
								</td>{/if}
							</tr>
						{/each}
						{#if ifaces.length === 0}<tr><td colspan={manage ? 6 : 5} class="muted">No interfaces.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="name (e.g. ge-0/0/1)" bind:value={iForm.name} style="flex:1; min-width:130px" />
						<input placeholder="IP/CIDR" bind:value={iForm.ip_cidr} style="width:140px" />
						<select bind:value={iForm.zone_id}><option value="">no zone</option>{#each zones as z}<option value={z.id}>{z.name}</option>{/each}</select>
						<input type="number" placeholder="VLAN" bind:value={iForm.vlan} style="width:80px" />
						<button class="btn small" onclick={addIface}>Add</button>
					</div>
				{/if}
			{:else if tab === 'zones'}
				<table>
					<thead><tr><th>Zone</th><th>Description</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each zones as z}
							<tr><td><strong>{z.name}</strong></td><td class="muted">{z.description || '—'}</td>{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delSub('zones', z.id)}>remove</button></td>{/if}</tr>
						{/each}
						{#if zones.length === 0}<tr><td colspan={manage ? 3 : 2} class="muted">No zones.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px">
						<input placeholder="Zone name (e.g. trust, dmz)" bind:value={zForm.name} style="flex:1" />
						<input placeholder="description" bind:value={zForm.description} style="flex:1" />
						<button class="btn small" onclick={addZone}>Add zone</button>
					</div>
				{/if}
			{:else if tab === 'addr'}
				<table>
					<thead><tr><th>Name</th><th>Kind</th><th>Value</th><th>Description</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each addrObjs as a}
							<tr>
								<td><strong>{a.name}</strong></td>
								<td><span class="pill">{a.kind}</span></td>
								<td class="muted mono">{a.value}</td>
								<td class="muted">{a.description || '—'}</td>
								{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delSub('address-objects', a.id)}>remove</button></td>{/if}
							</tr>
						{/each}
						{#if addrObjs.length === 0}<tr><td colspan={manage ? 5 : 4} class="muted">No address objects.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="name" bind:value={aoForm.name} style="width:130px" />
						<select bind:value={aoForm.kind}><option value="host">host</option><option value="network">network</option><option value="range">range</option><option value="fqdn">fqdn</option><option value="group">group</option></select>
						<input placeholder="value (10.0.0.5, 10.0.0.0/24…)" bind:value={aoForm.value} style="flex:1; min-width:160px" />
						<input placeholder="description" bind:value={aoForm.description} style="width:140px" />
						<button class="btn small" onclick={addAddrObj}>Add</button>
					</div>
				{/if}
			{:else if tab === 'routes'}
				<table>
					<thead><tr><th>Destination</th><th>Next hop</th><th>Interface</th><th>Metric</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each routes as rt}
							<tr>
								<td><strong>{rt.destination}</strong></td>
								<td class="muted mono">{rt.next_hop || '—'}</td>
								<td class="muted">{rt.interface || '—'}</td>
								<td>{rt.metric ?? '—'}</td>
								{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delSub('routes', rt.id)}>remove</button></td>{/if}
							</tr>
						{/each}
						{#if routes.length === 0}<tr><td colspan={manage ? 5 : 4} class="muted">No routes.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="destination (CIDR or default)" bind:value={rtForm.destination} style="width:170px" />
						<input placeholder="next hop" bind:value={rtForm.next_hop} style="width:130px" />
						<input placeholder="interface" bind:value={rtForm.interface} style="width:110px" />
						<input type="number" placeholder="metric" bind:value={rtForm.metric} style="width:80px" />
						<button class="btn small" onclick={addRoute}>Add</button>
					</div>
				{/if}
			{:else if tab === 'nat'}
				<table>
					<thead><tr><th>#</th><th>Name</th><th>Type</th><th>Original (src/dst/svc)</th><th>Translated (src/dst/svc)</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each natRules as n}
							<tr>
								<td>{n.seq}</td>
								<td>{n.name || '—'}</td>
								<td><span class="pill">{n.nat_type}</span></td>
								<td class="muted mono">{n.orig_src || '*'} / {n.orig_dst || '*'} / {n.orig_service || '*'}</td>
								<td class="muted mono">{n.trans_src || '*'} / {n.trans_dst || '*'} / {n.trans_service || '*'}</td>
								{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delSub('nat-rules', n.id)}>remove</button></td>{/if}
							</tr>
						{/each}
						{#if natRules.length === 0}<tr><td colspan={manage ? 6 : 5} class="muted">No NAT rules.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="name" bind:value={natForm.name} style="width:100px" />
						<select bind:value={natForm.nat_type}><option value="source">source</option><option value="destination">destination</option><option value="static">static</option></select>
						<input placeholder="orig src" bind:value={natForm.orig_src} style="width:90px" />
						<input placeholder="orig dst" bind:value={natForm.orig_dst} style="width:90px" />
						<input placeholder="orig svc" bind:value={natForm.orig_service} style="width:80px" />
						<input placeholder="trans src" bind:value={natForm.trans_src} style="width:90px" />
						<input placeholder="trans dst" bind:value={natForm.trans_dst} style="width:90px" />
						<input placeholder="trans svc" bind:value={natForm.trans_service} style="width:80px" />
						<button class="btn small" onclick={addNat}>Add</button>
					</div>
				{/if}
			{:else if tab === 'backups'}
				<table>
					<thead><tr><th>Taken</th><th>Version</th><th>Source</th><th>Size</th><th>Hash</th><th></th></tr></thead>
					<tbody>
						{#each backups as b}
							<tr>
								<td>{new Date(b.taken_at).toLocaleString()}</td>
								<td>{b.version || '—'}</td>
								<td class="muted">{b.source}</td>
								<td>{b.size_bytes ?? 0} B</td>
								<td class="muted mono">{b.hash?.slice(0, 10) ?? ''}</td>
								<td style="text-align:right">
									<button class="link" onclick={() => viewBackup(b)}>view</button>
									{#if manage}<button class="link-danger" onclick={() => delSub('backups', b.id)}>remove</button>{/if}
								</td>
							</tr>
						{/each}
						{#if backups.length === 0}<tr><td colspan="6" class="muted">No config backups.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div style="margin-top:10px">
						<div class="row" style="margin-bottom:6px">
							<input placeholder="version / tag" bind:value={bForm.version} style="width:160px" />
							<input placeholder="note" bind:value={bForm.note} style="flex:1" />
						</div>
						<textarea bind:value={bForm.content} placeholder="Paste device running-config here…" rows="6" style="width:100%; font-family:ui-monospace,monospace"></textarea>
						<button class="btn small" style="margin-top:6px" onclick={addBackup}>Save backup snapshot</button>
					</div>
				{/if}
				{#if backups.length >= 2}
					<div style="margin-top:16px; border-top:1px solid var(--border); padding-top:12px">
						<strong>Compare snapshots</strong>
						<div class="row" style="margin-top:6px; flex-wrap:wrap">
							<select bind:value={diffA}>
								<option value="">From…</option>
								{#each backups as b}<option value={b.id}>{b.version || new Date(b.taken_at).toLocaleString()}</option>{/each}
							</select>
							<span class="muted">→</span>
							<select bind:value={diffB}>
								<option value="">To…</option>
								{#each backups as b}<option value={b.id}>{b.version || new Date(b.taken_at).toLocaleString()}</option>{/each}
							</select>
							<button class="btn small" onclick={runDiff} disabled={!diffA || !diffB || diffA === diffB}>Diff</button>
							{#if diffStats}<span class="muted">+{diffStats.added} / −{diffStats.removed}</span>{/if}
						</div>
						{#if diffLines.length}
							<pre class="diff">{#each diffLines as l}<span class="d-{l.op === '+' ? 'add' : l.op === '-' ? 'del' : 'eq'}">{l.op === '+' ? '+' : l.op === '-' ? '-' : ' '} {l.text}
</span>{/each}</pre>
						{/if}
					</div>
				{/if}
			{/if}
		{/if}
	</section>
</div>

{#if viewing}
	<div class="modal-bg" onclick={() => (viewing = null)} role="presentation"></div>
	<div class="modal">
		<div class="row" style="justify-content:space-between">
			<strong>Config · {viewing.version || new Date(viewing.taken_at).toLocaleString()}</strong>
			<button class="link" onclick={() => (viewing = null)}>close</button>
		</div>
		<pre>{viewing.content || '(empty)'}</pre>
	</div>
{/if}

<style>
	.layout {
		display: grid;
		grid-template-columns: 280px 1fr;
		gap: 16px;
		align-items: start;
	}
	@media (max-width: 860px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
	.devices,
	.detail {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 14px;
	}
	.devices h3 {
		margin: 0 0 8px;
		font-size: 13px;
		text-transform: uppercase;
		color: var(--muted);
	}
	.dev {
		display: block;
		width: 100%;
		text-align: left;
		background: none;
		border: 1px solid transparent;
		border-radius: 8px;
		padding: 7px 9px;
		cursor: pointer;
		color: var(--text);
	}
	.dev:hover {
		background: var(--surface-2);
	}
	.dev.active {
		border-color: var(--primary);
		background: color-mix(in srgb, var(--primary) 10%, transparent);
	}
	.dn {
		display: block;
		font-weight: 600;
		font-size: 13px;
	}
	.dm {
		display: block;
		font-size: 11px;
		color: var(--muted);
	}
	.tabs {
		display: flex;
		gap: 4px;
		border-bottom: 1px solid var(--border);
		margin-bottom: 12px;
		flex-wrap: wrap;
	}
	.tabs button {
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 6px 10px;
		cursor: pointer;
		color: var(--muted);
		font-size: 13px;
	}
	.tabs button.active {
		color: var(--text);
		border-bottom-color: var(--primary);
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12.5px;
	}
	th,
	td {
		text-align: left;
		padding: 6px 7px;
		border-bottom: 1px solid var(--border);
	}
	tr.disabled td {
		opacity: 0.5;
	}
	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.btn.small {
		padding: 4px 10px;
		font-size: 12px;
	}
	.small {
		font-size: 12px;
	}
	.mono {
		font-family: ui-monospace, monospace;
	}
	.ha {
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px;
		margin-bottom: 8px;
	}
	.ha-head {
		display: flex;
		gap: 6px;
		align-items: center;
		justify-content: space-between;
	}
	.ha-m {
		font-size: 12px;
		padding-left: 6px;
	}
	.pill {
		font-size: 11px;
		padding: 1px 8px;
		border-radius: 10px;
		background: var(--surface-2);
		color: var(--muted);
		text-transform: uppercase;
	}
	.pill.allow {
		background: color-mix(in srgb, var(--success) 18%, transparent);
		color: var(--success);
	}
	.pill.deny,
	.pill.reject {
		background: color-mix(in srgb, var(--danger) 18%, transparent);
		color: var(--danger);
	}
	.link {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
		font-size: 12px;
	}
	.link-danger {
		background: none;
		border: none;
		color: var(--danger);
		cursor: pointer;
		font-size: 12px;
	}
	.modal-bg {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		z-index: 40;
	}
	.modal {
		position: fixed;
		top: 6vh;
		left: 50%;
		transform: translateX(-50%);
		width: min(800px, 92vw);
		max-height: 85vh;
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px;
		z-index: 41;
	}
	pre {
		background: var(--surface-2);
		padding: 12px;
		border-radius: 8px;
		overflow: auto;
		font-size: 12px;
		white-space: pre-wrap;
	}
	pre.diff {
		max-height: 360px;
	}
	.diff .d-add {
		display: block;
		background: color-mix(in srgb, var(--success) 16%, transparent);
		color: var(--success);
	}
	.diff .d-del {
		display: block;
		background: color-mix(in srgb, var(--danger) 16%, transparent);
		color: var(--danger);
	}
	.diff .d-eq {
		display: block;
		color: var(--muted);
	}
</style>
