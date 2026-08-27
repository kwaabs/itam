<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Subnet, Vlan, IPAddress, Location, DHCPScope } from '$lib/types';

	let tab = $state<'subnets' | 'vlans' | 'ips'>('subnets');
	let subnets = $state<Subnet[]>([]);
	let vlans = $state<Vlan[]>([]);
	let locations = $state<Location[]>([]);
	let allIPs = $state<IPAddress[]>([]);
	let ipQuery = $state('');
	let loading = $state(true);
	let error = $state('');

	const manage = $derived(can($me, 'ipam.manage'));

	// subnet drawer
	let selected = $state<Subnet | null>(null);
	let drawerIPs = $state<IPAddress[]>([]);
	let drawerScopes = $state<DHCPScope[]>([]);
	let drawerLoading = $state(false);

	let showSubnetForm = $state(false);
	let editSubnetId = $state('');
	let sf = $state({ cidr: '', name: '', gateway: '', vlan_id: '', location_id: '', description: '' });
	let showVlanForm = $state(false);
	let editVlanId = $state('');
	let vf = $state({ vlan_id: '', name: '', description: '', location_id: '' });
	let ipForm = $state({ address: '', status: 'allocated', dns_name: '', mac: '', description: '' });
	let showScopeForm = $state(false);
	let scopeForm = $state({ name: '', range_start: '', range_end: '', gateway: '', dns: '', domain: '', lease_hours: '24' });
	let saving = $state(false);

	const filteredIPs = $derived(
		allIPs.filter((ip) => {
			if (!ipQuery) return true;
			const q = ipQuery.toLowerCase();
			return `${ip.address} ${ip.dns_name ?? ''} ${ip.mac ?? ''} ${ip.asset?.name ?? ''}`.toLowerCase().includes(q);
		})
	);

	async function loadAllIPs() {
		try {
			allIPs = (await apiGet<IPAddress[]>('/api/ipam/ips')) ?? [];
		} catch {
			allIPs = [];
		}
	}

	async function load() {
		loading = true;
		error = '';
		try {
			subnets = (await apiGet<Subnet[]>('/api/ipam/subnets')) ?? [];
			vlans = (await apiGet<Vlan[]>('/api/ipam/vlans')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		try {
			locations = (await apiGet<Location[]>('/api/locations')) ?? [];
		} catch {
			locations = [];
		}
		await load();
		await loadAllIPs();
	});

	function pct(s: Subnet): number {
		if (!s.capacity || s.capacity < 0) return 0;
		return Math.min(100, Math.round(((s.used_count ?? 0) / s.capacity) * 100));
	}

	async function openSubnet(s: Subnet) {
		selected = s;
		drawerLoading = true;
		showScopeForm = false;
		ipForm = { address: '', status: 'allocated', dns_name: '', mac: '', description: '' };
		try {
			const d = await apiGet<{ addresses: IPAddress[] }>(`/api/ipam/subnets/${s.id}`);
			drawerIPs = d.addresses ?? [];
		} catch {
			drawerIPs = [];
		}
		try {
			drawerScopes = (await apiGet<DHCPScope[]>(`/api/ipam/subnets/${s.id}/dhcp-scopes`)) ?? [];
		} catch {
			drawerScopes = [];
		}
		drawerLoading = false;
	}

	async function refreshDrawer() {
		if (selected) {
			await load();
			const fresh = subnets.find((x) => x.id === selected!.id);
			if (fresh) await openSubnet(fresh);
		}
	}

	function newSubnetForm() {
		editSubnetId = '';
		sf = { cidr: '', name: '', gateway: '', vlan_id: '', location_id: '', description: '' };
		showSubnetForm = !showSubnetForm;
	}

	function editSubnet(s: Subnet, e: Event) {
		e.stopPropagation();
		editSubnetId = s.id;
		sf = {
			cidr: s.cidr,
			name: s.name,
			gateway: s.gateway ?? '',
			vlan_id: s.vlan_id ?? '',
			location_id: s.location_id ?? '',
			description: s.description ?? ''
		};
		showSubnetForm = true;
	}

	async function createSubnet() {
		if (!sf.cidr || !sf.name) return;
		saving = true;
		error = '';
		try {
			const body = {
				cidr: sf.cidr,
				name: sf.name,
				gateway: sf.gateway || null,
				vlan_id: sf.vlan_id || null,
				location_id: sf.location_id || null,
				description: sf.description || null
			};
			if (editSubnetId) {
				await apiPut(`/api/ipam/subnets/${editSubnetId}`, body);
			} else {
				await apiPost('/api/ipam/subnets', body);
			}
			showSubnetForm = false;
			editSubnetId = '';
			sf = { cidr: '', name: '', gateway: '', vlan_id: '', location_id: '', description: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}

	async function deleteSubnet(s: Subnet) {
		if (!confirm(`Delete subnet ${s.cidr}? IP records will be unlinked.`)) return;
		try {
			await apiDelete(`/api/ipam/subnets/${s.id}`);
			if (selected?.id === s.id) selected = null;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}

	function newVlanForm() {
		editVlanId = '';
		vf = { vlan_id: '', name: '', description: '', location_id: '' };
		showVlanForm = !showVlanForm;
	}

	function editVlan(v: Vlan) {
		editVlanId = v.id;
		vf = {
			vlan_id: String(v.vlan_id),
			name: v.name,
			description: v.description ?? '',
			location_id: v.location_id ?? ''
		};
		showVlanForm = true;
	}

	async function createVlan() {
		if (!vf.vlan_id || !vf.name) return;
		saving = true;
		error = '';
		try {
			const body = {
				vlan_id: Number(vf.vlan_id),
				name: vf.name,
				description: vf.description || null,
				location_id: vf.location_id || null
			};
			if (editVlanId) {
				await apiPut(`/api/ipam/vlans/${editVlanId}`, body);
			} else {
				await apiPost('/api/ipam/vlans', body);
			}
			showVlanForm = false;
			editVlanId = '';
			vf = { vlan_id: '', name: '', description: '', location_id: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}

	async function deleteVlan(v: Vlan) {
		if (!confirm(`Delete VLAN ${v.vlan_id} (${v.name})?`)) return;
		try {
			await apiDelete(`/api/ipam/vlans/${v.id}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}

	async function nextFree() {
		if (!selected) return;
		try {
			const d = await apiGet<{ address: string }>(`/api/ipam/subnets/${selected.id}/next-free`);
			ipForm.address = d.address;
		} catch (e) {
			error = e instanceof Error ? e.message : 'No free address';
		}
	}

	async function addIP() {
		if (!selected || !ipForm.address) return;
		saving = true;
		error = '';
		try {
			await apiPost('/api/ipam/ips', {
				address: ipForm.address,
				subnet_id: selected.id,
				status: ipForm.status,
				dns_name: ipForm.dns_name || null,
				mac: ipForm.mac || null,
				description: ipForm.description || null
			});
			ipForm = { address: '', status: 'allocated', dns_name: '', mac: '', description: '' };
			await refreshDrawer();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add IP';
		} finally {
			saving = false;
		}
	}

	// Toggle an address between reserved and allocated.
	async function toggleReserve(ip: IPAddress) {
		const next = ip.status === 'reserved' ? 'allocated' : 'reserved';
		try {
			await apiPut(`/api/ipam/ips/${ip.id}`, { ...ip, status: next });
			await refreshDrawer();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function releaseIP(ip: IPAddress) {
		if (!confirm(`Release ${ip.address}?`)) return;
		try {
			await apiDelete(`/api/ipam/ips/${ip.id}`);
			await refreshDrawer();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function addScope() {
		if (!selected || !scopeForm.name || !scopeForm.range_start || !scopeForm.range_end) return;
		saving = true;
		error = '';
		try {
			await apiPost('/api/ipam/dhcp-scopes', {
				subnet_id: selected.id,
				name: scopeForm.name,
				range_start: scopeForm.range_start,
				range_end: scopeForm.range_end,
				gateway: scopeForm.gateway || null,
				dns: scopeForm.dns || null,
				domain: scopeForm.domain || null,
				lease_hours: Number(scopeForm.lease_hours) || 24,
				enabled: true
			});
			scopeForm = { name: '', range_start: '', range_end: '', gateway: '', dns: '', domain: '', lease_hours: '24' };
			showScopeForm = false;
			drawerScopes = (await apiGet<DHCPScope[]>(`/api/ipam/subnets/${selected.id}/dhcp-scopes`)) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add scope';
		} finally {
			saving = false;
		}
	}

	async function deleteScope(sc: DHCPScope) {
		if (!confirm(`Delete DHCP scope "${sc.name}"?`)) return;
		try {
			await apiDelete(`/api/ipam/dhcp-scopes/${sc.id}`);
			if (selected) drawerScopes = (await apiGet<DHCPScope[]>(`/api/ipam/subnets/${selected.id}/dhcp-scopes`)) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	function vlanLabel(id: string | null): string {
		const v = vlans.find((x) => x.id === id);
		return v ? `${v.vlan_id} · ${v.name}` : '—';
	}
</script>

<div class="topbar">
	<h1>IPAM</h1>
	<div class="row">
		<div class="seg">
			<button class="seg-btn" class:active={tab === 'subnets'} onclick={() => (tab = 'subnets')}>Subnets</button>
			<button class="seg-btn" class:active={tab === 'vlans'} onclick={() => (tab = 'vlans')}>VLANs</button>
			<button class="seg-btn" class:active={tab === 'ips'} onclick={() => (tab = 'ips')}>IP addresses</button>
		</div>
		{#if manage}
			{#if tab === 'subnets'}
				<button class="btn" onclick={newSubnetForm}>+ Subnet</button>
			{:else if tab === 'vlans'}
				<button class="btn" onclick={newVlanForm}>+ VLAN</button>
			{/if}
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if showSubnetForm && tab === 'subnets'}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">{editSubnetId ? 'Edit subnet' : 'New subnet'}</h3>
		<div class="grid cols-3">
			<label class="field">CIDR <input bind:value={sf.cidr} placeholder="10.20.0.0/24" disabled={!!editSubnetId} /></label>
			<label class="field">Name <input bind:value={sf.name} placeholder="Office LAN" /></label>
			<label class="field">Gateway <input bind:value={sf.gateway} placeholder="10.20.0.1" /></label>
			<label class="field">
				VLAN
				<select bind:value={sf.vlan_id}>
					<option value="">—</option>
					{#each vlans as v}<option value={v.id}>{v.vlan_id} · {v.name}</option>{/each}
				</select>
			</label>
			<label class="field">
				Location
				<select bind:value={sf.location_id}>
					<option value="">—</option>
					{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
				</select>
			</label>
			<label class="field">Description <input bind:value={sf.description} /></label>
		</div>
		<button class="btn" onclick={createSubnet} disabled={saving}>{saving ? 'Saving…' : editSubnetId ? 'Save' : 'Create'}</button>
	</div>
{/if}

{#if showVlanForm && tab === 'vlans'}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">{editVlanId ? 'Edit VLAN' : 'New VLAN'}</h3>
		<div class="grid cols-3">
			<label class="field">VLAN ID <input type="number" min="1" max="4094" bind:value={vf.vlan_id} disabled={!!editVlanId} /></label>
			<label class="field">Name <input bind:value={vf.name} placeholder="Voice" /></label>
			<label class="field">
				Location
				<select bind:value={vf.location_id}>
					<option value="">—</option>
					{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
				</select>
			</label>
		</div>
		<label class="field">Description <input bind:value={vf.description} /></label>
		<button class="btn" onclick={createVlan} disabled={saving}>{saving ? 'Saving…' : editVlanId ? 'Save' : 'Create'}</button>
	</div>
{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if tab === 'subnets'}
		<table>
			<thead><tr><th>CIDR</th><th>Name</th><th>VLAN</th><th>Location</th><th>Utilization</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each subnets as s}
					<tr
						role="button"
						tabindex="0"
						onclick={() => openSubnet(s)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openSubnet(s))}
					>
						<td><strong>{s.cidr}</strong></td>
						<td>{s.name}</td>
						<td class="muted">{vlanLabel(s.vlan_id)}</td>
						<td class="muted">{s.location?.name ?? '—'}</td>
						<td>
							<div class="util">
								<div class="util-bar"><span style="width:{pct(s)}%"></span></div>
								<span class="muted util-txt">
									{s.used_count ?? 0}{s.capacity && s.capacity > 0 ? ` / ${s.capacity}` : ''}
								</span>
							</div>
						</td>
						{#if manage}
							<td style="text-align:right; white-space:nowrap">
								<button class="btn secondary small" onclick={(e) => editSubnet(s, e)}>Edit</button>
								<button class="btn danger small" onclick={(e) => { e.stopPropagation(); deleteSubnet(s); }}>Delete</button>
							</td>
						{/if}
					</tr>
				{/each}
				{#if subnets.length === 0}<tr><td colspan="6" class="muted">No subnets yet.</td></tr>{/if}
			</tbody>
		</table>
	{:else if tab === 'vlans'}
		<table>
			<thead><tr><th>VLAN</th><th>Name</th><th>Location</th><th>Description</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each vlans as v}
					<tr>
						<td><strong>{v.vlan_id}</strong></td>
						<td>{v.name}</td>
						<td class="muted">{v.location?.name ?? '—'}</td>
						<td class="muted">{v.description || '—'}</td>
						{#if manage}
							<td style="text-align:right; white-space:nowrap">
								<button class="btn secondary small" onclick={() => editVlan(v)}>Edit</button>
								<button class="btn danger small" onclick={() => deleteVlan(v)}>Delete</button>
							</td>
						{/if}
					</tr>
				{/each}
				{#if vlans.length === 0}<tr><td colspan="5" class="muted">No VLANs yet.</td></tr>{/if}
			</tbody>
		</table>
	{:else}
		<div class="row" style="margin-bottom:10px; gap:8px; align-items:center">
			<input bind:value={ipQuery} placeholder="Search IP, DNS, MAC or asset…" style="max-width:320px" />
			<span class="muted">{filteredIPs.length} of {allIPs.length}</span>
		</div>
		<table>
			<thead><tr><th>Address</th><th>Status</th><th>DNS</th><th>MAC</th><th>Subnet</th><th>Asset</th></tr></thead>
			<tbody>
				{#each filteredIPs as ip}
					<tr>
						<td><strong>{ip.address}</strong></td>
						<td><span class="badge">{ip.status}</span></td>
						<td class="muted">{ip.dns_name || '—'}</td>
						<td class="muted">{ip.mac || '—'}</td>
						<td class="muted">{ip.subnet?.cidr ?? '—'}</td>
						<td class="muted">{ip.asset ? '' : (ip.description || '—')}{#if ip.asset}<a href={`/assets/${ip.asset_id}`}>{ip.asset.name}</a>{/if}</td>
					</tr>
				{/each}
				{#if filteredIPs.length === 0}<tr><td colspan="6" class="muted">No IP addresses.</td></tr>{/if}
			</tbody>
		</table>
	{/if}
</div>

{#if selected}
	<div class="drawer-backdrop" role="button" tabindex="0" aria-label="Close panel" onclick={() => (selected = null)} onkeydown={(e) => e.key === 'Escape' && (selected = null)}></div>
	<aside class="drawer">
		<button class="btn secondary small drawer-close" onclick={() => (selected = null)}>Close</button>
		<h2>{selected.cidr}</h2>
		<div class="muted" style="margin-bottom:14px">{selected.name}</div>
		<div class="kv">
			<div class="k">VLAN</div><div>{vlanLabel(selected.vlan_id)}</div>
			<div class="k">Gateway</div><div>{selected.gateway || '—'}</div>
			<div class="k">Location</div><div>{selected.location?.name ?? '—'}</div>
			<div class="k">Used</div><div>{selected.used_count ?? 0}{selected.capacity && selected.capacity > 0 ? ` of ${selected.capacity}` : ''}</div>
		</div>

		<h3 style="margin-bottom:6px">Addresses</h3>
		{#if drawerLoading}
			<p class="muted">Loading…</p>
		{:else}
			<table>
				<thead><tr><th>Address</th><th>Status</th><th>Assigned</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each drawerIPs as ip}
						<tr>
							<td><strong>{ip.address}</strong>{#if ip.dns_name}<div class="muted" style="font-size:12px">{ip.dns_name}</div>{/if}</td>
							<td><span class="badge">{ip.status}</span></td>
							<td class="muted">{ip.asset?.name ?? ip.description ?? '—'}</td>
							{#if manage}
								<td style="text-align:right; white-space:nowrap">
									<button class="btn secondary small" onclick={() => toggleReserve(ip)}>{ip.status === 'reserved' ? 'Unreserve' : 'Reserve'}</button>
									<button class="btn secondary small" onclick={() => releaseIP(ip)}>Release</button>
								</td>
							{/if}
						</tr>
					{/each}
					{#if drawerIPs.length === 0}<tr><td colspan="4" class="muted">No addresses allocated.</td></tr>{/if}
				</tbody>
			</table>
		{/if}

		{#if manage}
			<h3 style="margin-bottom:6px; margin-top:18px">Allocate address</h3>
			<div class="row" style="align-items:flex-end; gap:8px">
				<label class="field" style="flex:1; margin-bottom:0">Address <input bind:value={ipForm.address} placeholder="10.20.0.5" /></label>
				<button class="btn secondary small" onclick={nextFree}>Next free</button>
			</div>
			<div class="grid cols-2" style="margin-top:8px">
				<label class="field">
					Status
					<select bind:value={ipForm.status}>
						<option value="allocated">allocated</option>
						<option value="reserved">reserved</option>
						<option value="deprecated">deprecated</option>
					</select>
				</label>
				<label class="field">DNS name <input bind:value={ipForm.dns_name} placeholder="host.example.com" /></label>
			</div>
			<div class="grid cols-2">
				<label class="field">MAC <input bind:value={ipForm.mac} placeholder="00:11:22:33:44:55" /></label>
				<label class="field">Note <input bind:value={ipForm.description} /></label>
			</div>
			<button class="btn" onclick={addIP} disabled={saving || !ipForm.address}>{saving ? 'Saving…' : 'Allocate'}</button>
		{/if}

		<div class="row" style="justify-content:space-between; align-items:center; margin-top:22px">
			<h3 style="margin:0">DHCP scopes</h3>
			{#if manage}
				<button class="btn secondary small" onclick={() => (showScopeForm = !showScopeForm)}>+ Scope</button>
			{/if}
		</div>
		{#if showScopeForm && manage}
			<div class="card" style="margin-top:8px">
				<div class="grid cols-2">
					<label class="field">Name <input bind:value={scopeForm.name} placeholder="Default pool" /></label>
					<label class="field">Lease (hours) <input type="number" min="1" bind:value={scopeForm.lease_hours} /></label>
					<label class="field">Range start <input bind:value={scopeForm.range_start} placeholder="10.20.0.100" /></label>
					<label class="field">Range end <input bind:value={scopeForm.range_end} placeholder="10.20.0.200" /></label>
					<label class="field">Gateway <input bind:value={scopeForm.gateway} placeholder="10.20.0.1" /></label>
					<label class="field">DNS <input bind:value={scopeForm.dns} placeholder="10.20.0.2, 1.1.1.1" /></label>
					<label class="field">Domain <input bind:value={scopeForm.domain} placeholder="corp.local" /></label>
				</div>
				<button class="btn small" onclick={addScope} disabled={saving || !scopeForm.name || !scopeForm.range_start || !scopeForm.range_end}>{saving ? 'Saving…' : 'Add scope'}</button>
			</div>
		{/if}
		<table style="margin-top:8px">
			<thead><tr><th>Name</th><th>Range</th><th>Lease</th><th>Usage</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each drawerScopes as sc}
					<tr>
						<td><strong>{sc.name}</strong>{#if !sc.enabled}<span class="badge" style="margin-left:6px">disabled</span>{/if}</td>
						<td class="muted">{sc.range_start} – {sc.range_end}</td>
						<td class="muted">{sc.lease_hours}h</td>
						<td>
							{#if sc.pool_size}
								{@const pct = Math.min(100, Math.round(((sc.used ?? 0) / sc.pool_size) * 100))}
								<span class="usage" title="{sc.used ?? 0} of {sc.pool_size} addresses used">
									<span class="usage-track"><span class="usage-fill" class:hot={pct >= 80} style="width:{pct}%"></span></span>
									<span class="usage-num">{sc.used ?? 0}/{sc.pool_size}</span>
								</span>
							{:else}<span class="muted">—</span>{/if}
						</td>
						{#if manage}<td style="text-align:right"><button class="btn danger small" onclick={() => deleteScope(sc)}>Delete</button></td>{/if}
					</tr>
				{/each}
				{#if drawerScopes.length === 0}<tr><td colspan={manage ? 5 : 4} class="muted">No DHCP scopes.</td></tr>{/if}
			</tbody>
		</table>
	</aside>
{/if}

<style>
	.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
	.seg-btn { background: transparent; color: var(--muted); border: none; padding: 6px 14px; cursor: pointer; font: inherit; }
	.seg-btn.active { background: var(--primary); color: #fff; font-weight: 600; }
	.util { display: flex; align-items: center; gap: 8px; }
	.util-bar { flex: 1; max-width: 160px; height: 7px; background: var(--surface-2); border-radius: 999px; overflow: hidden; }
	.util-bar span { display: block; height: 100%; background: var(--primary); border-radius: 999px; }
	.util-txt { font-size: 12px; white-space: nowrap; }
	.usage { display: inline-flex; align-items: center; gap: 6px; }
	.usage-track { width: 70px; height: 7px; background: var(--surface-2); border-radius: 999px; overflow: hidden; }
	.usage-fill { display: block; height: 100%; background: var(--primary); border-radius: 999px; }
	.usage-fill.hot { background: #ef4444; }
	.usage-num { font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; }
</style>
