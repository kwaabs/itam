<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Cluster, Host, VM, Asset, Paginated } from '$lib/types';

	let clusters = $state<Cluster[]>([]);
	let hosts = $state<Host[]>([]);
	let vms = $state<VM[]>([]);
	let assets = $state<Asset[]>([]);
	let servers = $state<Asset[]>([]);
	let loading = $state(true);
	let error = $state('');
	const manage = $derived(can($me, 'virt.manage'));

	let cForm = $state({ name: '', hypervisor: 'esxi' });
	let hForm = $state({ asset_id: '', cluster_id: '', hypervisor: 'esxi', cpu_cores: '', ram_gb: '' });
	let vForm = $state({ name: '', host_id: '', vcpus: '', ram_gb: '', disk_gb: '', power_state: 'running', guest_os: '' });

	async function load() {
		loading = true;
		error = '';
		try {
			clusters = (await apiGet<Cluster[]>('/api/virt/clusters')) ?? [];
			hosts = (await apiGet<Host[]>('/api/virt/hosts')) ?? [];
			vms = (await apiGet<VM[]>('/api/virt/vms')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		try {
			const [all, srv] = await Promise.all([
				apiGet<Paginated<Asset>>('/api/assets?page_size=300'),
				apiGet<Paginated<Asset>>('/api/assets?type=server&page_size=300')
			]);
			assets = all.items ?? [];
			servers = srv.items ?? [];
		} catch {
			/* ignore */
		}
		load();
	});

	function openAsset(id: string | null | undefined) {
		if (id) goto(`/assets/${id}`);
	}

	let detail = $state<VM | null>(null);

	function vmsOfHost(hid: string): VM[] {
		return vms.filter((v) => v.host_id === hid);
	}
	function alloc(hid: string) {
		const list = vmsOfHost(hid);
		return {
			vcpu: list.reduce((s, v) => s + (v.vcpus ?? 0), 0),
			ram: list.reduce((s, v) => s + (v.ram_gb ?? 0), 0),
			count: list.length
		};
	}
	function assetName(id: string | null): string {
		const a = assets.find((x) => x.id === id);
		return a ? `${a.asset_tag} · ${a.name}` : '—';
	}
	function clusterName(id: string | null): string {
		return clusters.find((c) => c.id === id)?.name ?? '—';
	}

	async function addCluster() {
		if (!cForm.name.trim()) return;
		try {
			await apiPost('/api/virt/clusters', { name: cForm.name, hypervisor: cForm.hypervisor });
			cForm = { name: '', hypervisor: 'esxi' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function addHost() {
		if (!hForm.asset_id) {
			error = 'Pick the physical server asset';
			return;
		}
		try {
			await apiPost('/api/virt/hosts', {
				asset_id: hForm.asset_id,
				cluster_id: hForm.cluster_id || null,
				hypervisor: hForm.hypervisor,
				cpu_cores: hForm.cpu_cores ? Number(hForm.cpu_cores) : null,
				ram_gb: hForm.ram_gb ? Number(hForm.ram_gb) : null
			});
			hForm = { asset_id: '', cluster_id: '', hypervisor: 'esxi', cpu_cores: '', ram_gb: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function addVM() {
		if (!vForm.name.trim()) return;
		try {
			await apiPost('/api/virt/vms', {
				name: vForm.name,
				host_id: vForm.host_id || null,
				vcpus: vForm.vcpus ? Number(vForm.vcpus) : null,
				ram_gb: vForm.ram_gb ? Number(vForm.ram_gb) : null,
				disk_gb: vForm.disk_gb ? Number(vForm.disk_gb) : null,
				power_state: vForm.power_state,
				guest_os: vForm.guest_os || null
			});
			vForm = { name: '', host_id: '', vcpus: '', ram_gb: '', disk_gb: '', power_state: 'running', guest_os: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function del(kind: string, id: string) {
		try {
			await apiDelete(`/api/virt/${kind}/${id}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	function pct(used: number, cap: number | null): number {
		return cap && cap > 0 ? Math.min(100, (used / cap) * 100) : 0;
	}
</script>

<h1>Virtualization</h1>
{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else}
	<div class="card">
		<h3 style="margin-top:0">Clusters</h3>
		<table>
			<thead><tr><th>Name</th><th>Hypervisor</th><th>Hosts</th><th>VMs</th><th>vCPU alloc</th><th>RAM alloc</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each clusters as c}
					{@const ch = hosts.filter((h) => h.cluster_id === c.id)}
					{@const cv = vms.filter((v) => v.cluster_id === c.id)}
					<tr>
						<td><strong>{c.name}</strong>{c.ha ? ' · HA' : ''}{c.drs ? ' · DRS' : ''}</td>
						<td class="muted">{c.hypervisor || '—'}</td>
						<td>{ch.length}</td>
						<td>{cv.length}</td>
						<td>{cv.reduce((s, v) => s + (v.vcpus ?? 0), 0)}</td>
						<td>{cv.reduce((s, v) => s + (v.ram_gb ?? 0), 0)} GB</td>
						{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => del('clusters', c.id)}>remove</button></td>{/if}
					</tr>
				{/each}
				{#if clusters.length === 0}<tr><td colspan={manage ? 7 : 6} class="muted">No clusters.</td></tr>{/if}
			</tbody>
		</table>
		{#if manage}
			<div class="row" style="margin-top:8px">
				<input placeholder="Cluster name" bind:value={cForm.name} style="flex:1" />
				<input placeholder="hypervisor" bind:value={cForm.hypervisor} style="width:120px" />
				<button class="btn small" onclick={addCluster}>Add cluster</button>
			</div>
		{/if}
	</div>

	<div class="card">
		<h3 style="margin-top:0">Hosts</h3>
		<table>
			<thead><tr><th>Host (asset)</th><th>Cluster</th><th>Cores</th><th>RAM</th><th>VMs</th><th>vCPU alloc</th><th>RAM alloc</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each hosts as h}
					{@const a = alloc(h.id)}
					<tr
						class="clickable"
						role="button"
						tabindex="0"
						onclick={() => openAsset(h.asset_id)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && openAsset(h.asset_id)}
					>
						<td>{h.asset ? `${h.asset.asset_tag} · ${h.asset.name}` : assetName(h.asset_id)}</td>
						<td class="muted">{clusterName(h.cluster_id)}</td>
						<td>{h.cpu_cores ?? '—'}</td>
						<td>{h.ram_gb ? `${h.ram_gb} GB` : '—'}</td>
						<td>{a.count}</td>
						<td>
							{a.vcpu}{h.cpu_cores ? ` / ${h.cpu_cores}` : ''}
							{#if h.cpu_cores}<div class="meter"><span class:hot={pct(a.vcpu, h.cpu_cores) >= 100} style="width:{pct(a.vcpu, h.cpu_cores)}%"></span></div>{/if}
						</td>
						<td>
							{a.ram}{h.ram_gb ? ` / ${h.ram_gb} GB` : ' GB'}
							{#if h.ram_gb}<div class="meter"><span class:hot={pct(a.ram, h.ram_gb) >= 100} style="width:{pct(a.ram, h.ram_gb)}%"></span></div>{/if}
						</td>
						{#if manage}<td style="text-align:right"><button class="link-danger" onclick={(e) => { e.stopPropagation(); del('hosts', h.id); }}>remove</button></td>{/if}
					</tr>
				{/each}
				{#if hosts.length === 0}<tr><td colspan={manage ? 8 : 7} class="muted">No hosts.</td></tr>{/if}
			</tbody>
		</table>
		{#if manage}
			<div class="row" style="margin-top:8px; flex-wrap:wrap">
				<select bind:value={hForm.asset_id} style="flex:1; min-width:160px">
					<option value="">— server asset —</option>
					{#each servers as a}<option value={a.id}>{a.asset_tag} · {a.name}</option>{/each}
				</select>
				<select bind:value={hForm.cluster_id}>
					<option value="">No cluster</option>
					{#each clusters as c}<option value={c.id}>{c.name}</option>{/each}
				</select>
				<input type="number" placeholder="cores" bind:value={hForm.cpu_cores} style="width:80px" />
				<input type="number" placeholder="RAM GB" bind:value={hForm.ram_gb} style="width:90px" />
				<button class="btn small" onclick={addHost}>Add host</button>
			</div>
		{/if}
	</div>

	<div class="card">
		<h3 style="margin-top:0">Virtual machines</h3>
		<table>
			<thead><tr><th>Name</th><th>Host</th><th>vCPU</th><th>RAM</th><th>Disk</th><th>State</th><th>Guest OS</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each vms as v}
					<tr
						class="clickable"
						role="button"
						tabindex="0"
						onclick={() => (detail = v)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (detail = v)}
					>
						<td><strong>{v.name}</strong>{v.ip ? ` · ${v.ip}` : ''}</td>
						<td class="muted">{v.host_id ? assetName(hosts.find((h) => h.id === v.host_id)?.asset_id ?? null) : '—'}</td>
						<td>{v.vcpus ?? '—'}</td>
						<td>{v.ram_gb ? `${v.ram_gb} GB` : '—'}</td>
						<td>{v.disk_gb ? `${v.disk_gb} GB` : '—'}</td>
						<td><span class="pill {v.power_state}">{v.power_state}</span></td>
						<td class="muted">{v.guest_os || '—'}</td>
						{#if manage}<td style="text-align:right"><button class="link-danger" onclick={(e) => { e.stopPropagation(); del('vms', v.id); }}>remove</button></td>{/if}
					</tr>
				{/each}
				{#if vms.length === 0}<tr><td colspan={manage ? 8 : 7} class="muted">No VMs.</td></tr>{/if}
			</tbody>
		</table>
		{#if manage}
			<div class="row" style="margin-top:8px; flex-wrap:wrap">
				<input placeholder="VM name" bind:value={vForm.name} style="flex:1; min-width:140px" />
				<select bind:value={vForm.host_id}>
					<option value="">No host</option>
					{#each hosts as h}<option value={h.id}>{assetName(h.asset_id)}</option>{/each}
				</select>
				<input type="number" placeholder="vCPU" bind:value={vForm.vcpus} style="width:70px" />
				<input type="number" placeholder="RAM GB" bind:value={vForm.ram_gb} style="width:90px" />
				<input type="number" placeholder="Disk GB" bind:value={vForm.disk_gb} style="width:90px" />
				<select bind:value={vForm.power_state}>
					<option value="running">running</option>
					<option value="stopped">stopped</option>
					<option value="suspended">suspended</option>
				</select>
				<button class="btn small" onclick={addVM}>Add VM</button>
			</div>
		{/if}
	</div>
{/if}

{#if detail}
	{@const dh = detail.host_id ? hosts.find((x) => x.id === detail?.host_id) : null}
	{@const dc = detail.cluster_id ? clusters.find((x) => x.id === detail?.cluster_id) : null}
	{@const attrs = Object.entries(detail.attributes ?? {})}
	<div
		class="backdrop"
		role="button"
		tabindex="0"
		aria-label="Close details"
		onclick={(e) => { if (e.target === e.currentTarget) detail = null; }}
		onkeydown={(e) => e.key === 'Escape' && (detail = null)}
	>
		<aside class="drawer">
			<div class="drawer-head">
				<div>
					<h3>{detail.name}</h3>
					<span class="pill {detail.power_state}">{detail.power_state}</span>
					{#if detail.guest_os}<span class="tagchip">{detail.guest_os}</span>{/if}
				</div>
				<button class="btn small secondary" onclick={() => (detail = null)}>Close</button>
			</div>

			<h4>Compute</h4>
			<div class="metrics">
				<div class="m"><div class="ml">vCPU</div><div class="mv">{detail.vcpus ?? '—'}</div></div>
				<div class="m"><div class="ml">RAM</div><div class="mv">{detail.ram_gb ? `${detail.ram_gb} GB` : '—'}</div></div>
				<div class="m"><div class="ml">Disk</div><div class="mv">{detail.disk_gb ? `${detail.disk_gb} GB` : '—'}</div></div>
			</div>

			<h4>Placement</h4>
			<dl class="kv">
				<dt>Host</dt>
				<dd>
					{#if dh}
						<button class="linklike" onclick={() => openAsset(dh.asset_id)}>{assetName(dh.asset_id)}</button>
						{#if dh.hypervisor}<span class="muted small"> · {dh.hypervisor}</span>{/if}
					{:else}—{/if}
				</dd>
				{#if dh}
					<dt>Host capacity</dt>
					<dd class="muted">{dh.cpu_cores ?? '?'} cores · {dh.ram_gb ? `${dh.ram_gb} GB` : '? RAM'}</dd>
				{/if}
				<dt>Cluster</dt>
				<dd>
					{dc ? dc.name : '—'}
					{#if dc?.hypervisor}<span class="muted small"> · {dc.hypervisor}</span>{/if}
					{#if dc?.ha}<span class="tagchip">HA</span>{/if}
					{#if dc?.drs}<span class="tagchip">DRS</span>{/if}
				</dd>
			</dl>

			<h4>Network</h4>
			<dl class="kv">
				<dt>IP</dt><dd>{detail.ip || '—'}</dd>
			</dl>

			{#if attrs.length}
				<h4>Attributes</h4>
				<dl class="kv">
					{#each attrs as [k, v]}
						<dt>{k}</dt><dd>{v === null || v === '' ? '—' : String(v)}</dd>
					{/each}
				</dl>
			{/if}

			<div class="drawer-actions">
				{#if detail.asset_id}
					<button class="btn small" onclick={() => openAsset(detail?.asset_id)}>Open asset record</button>
				{:else}
					<p class="muted small">This VM isn't tracked as an asset, so it has no lifecycle, cost or warranty record. Create/link an asset to manage those.</p>
				{/if}
			</div>
		</aside>
	</div>
{/if}

<style>
	.card {
		margin-bottom: 16px;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	th,
	td {
		text-align: left;
		padding: 7px 8px;
		border-bottom: 1px solid var(--border);
		vertical-align: middle;
	}
	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	tr.clickable {
		cursor: pointer;
	}
	tr.clickable:hover {
		background: var(--surface-2);
	}
	.btn.small {
		padding: 4px 10px;
		font-size: 12px;
	}
	.meter {
		height: 6px;
		background: var(--surface-2);
		border-radius: 4px;
		overflow: hidden;
		margin-top: 3px;
	}
	.meter span {
		display: block;
		height: 100%;
		background: var(--success);
	}
	.meter span.hot {
		background: var(--danger);
	}
	.pill {
		font-size: 11px;
		padding: 1px 8px;
		border-radius: 10px;
		background: var(--surface-2);
		color: var(--muted);
	}
	.pill.running {
		background: color-mix(in srgb, var(--success) 18%, transparent);
		color: var(--success);
	}
	.pill.stopped {
		background: color-mix(in srgb, var(--danger) 18%, transparent);
		color: var(--danger);
	}
	.link-danger {
		background: none;
		border: none;
		color: var(--danger);
		cursor: pointer;
		font-size: 12px;
	}
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		display: flex;
		justify-content: flex-end;
		z-index: 50;
	}
	.drawer {
		width: 380px;
		max-width: 90vw;
		height: 100%;
		background: var(--surface);
		border-left: 1px solid var(--border);
		padding: 18px;
		overflow-y: auto;
	}
	.drawer-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 10px;
		margin-bottom: 8px;
	}
	.drawer-head h3 {
		margin: 0 0 6px;
	}
	.drawer h4 {
		margin: 16px 0 8px;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted);
	}
	.metrics {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 8px;
	}
	.metrics .m {
		background: var(--surface-2);
		border-radius: 8px;
		padding: 10px;
		text-align: center;
	}
	.metrics .ml {
		font-size: 11px;
		color: var(--muted);
		text-transform: uppercase;
	}
	.metrics .mv {
		font-size: 18px;
		font-weight: 700;
		margin-top: 2px;
	}
	.kv {
		display: grid;
		grid-template-columns: 120px 1fr;
		row-gap: 8px;
		column-gap: 10px;
		margin: 0;
		font-size: 13px;
	}
	.kv dt {
		color: var(--muted);
	}
	.kv dd {
		margin: 0;
		word-break: break-word;
	}
	.tagchip {
		display: inline-block;
		font-size: 10px;
		padding: 1px 6px;
		border-radius: 8px;
		background: var(--surface-2);
		color: var(--muted);
		margin-left: 4px;
		vertical-align: middle;
	}
	.linklike {
		background: none;
		border: none;
		padding: 0;
		color: var(--accent, #3b82f6);
		cursor: pointer;
		font: inherit;
		text-decoration: underline;
	}
	.drawer-actions {
		margin-top: 18px;
		padding-top: 14px;
		border-top: 1px solid var(--border);
	}
	.btn.secondary {
		background: var(--surface-2);
		color: var(--text);
	}
</style>
