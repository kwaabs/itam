<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Store, Location, Asset, Paginated } from '$lib/types';
	import SitesMap from '$lib/components/SitesMap.svelte';

	let stores = $state<Store[]>([]);
	let stock = $state<Asset[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state('');
	let expanded = $state<string | null>(null);

	const canManage = $derived(can($me, 'hierarchy.manage'));

	let showForm = $state(false);
	let editId = $state<string | null>(null);
	let form = $state({ name: '', address: '', departments: '', latitude: '', longitude: '' });

	const blankForm = () => ({ name: '', address: '', departments: '', latitude: '', longitude: '' });

	function newStore() {
		editId = null;
		form = blankForm();
		showForm = true;
	}

	function openEdit(s: Store) {
		editId = s.id;
		form = {
			name: s.name,
			address: s.address ?? '',
			departments: '',
			latitude: s.latitude != null ? String(s.latitude) : '',
			longitude: s.longitude != null ? String(s.longitude) : ''
		};
		showForm = true;
	}

	const locatedStores = $derived(stores.filter((s) => s.latitude != null && s.longitude != null));

	function ltreeKey(s: string): string {
		return (
			s
				.toLowerCase()
				.trim()
				.replace(/[^a-z0-9]+/g, '_')
				.replace(/^_+|_+$/g, '')
				.slice(0, 40) || 'store'
		);
	}

	async function load() {
		loading = true;
		error = '';
		try {
			[stores, stock] = await Promise.all([
				apiGet<Store[]>('/api/stores'),
				apiGet<Paginated<Asset>>('/api/assets?state=in_stock&page_size=500').then((p) => p.items ?? [])
			]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load stores';
		} finally {
			loading = false;
		}
	}
	onMount(load);

	function money(n: number): string {
		return (n ?? 0).toLocaleString(undefined, { maximumFractionDigits: 0 });
	}

	function stockFor(store: Store): Asset[] {
		const prefix = store.path + '.';
		return stock.filter((a) => {
			const p = a.location?.path;
			return p && (p === store.path || p.startsWith(prefix));
		});
	}

	async function save() {
		if (!form.name.trim()) {
			error = 'Name is required';
			return;
		}
		busy = 'save';
		error = '';
		try {
			const payload = {
				name: form.name.trim(),
				kind: 'store',
				address: form.address.trim() || null,
				latitude: form.latitude.trim() ? Number(form.latitude) : null,
				longitude: form.longitude.trim() ? Number(form.longitude) : null
			};
			if (editId) {
				await apiPut<Location>(`/api/locations/${editId}`, payload);
			} else {
				const created = await apiPost<Location>('/api/locations', {
					...payload,
					key: ltreeKey(form.name),
					parent_id: null
				});
				const depts = form.departments
					.split(',')
					.map((d) => d.trim())
					.filter(Boolean);
				for (const d of depts) {
					await apiPost('/api/locations', {
						key: ltreeKey(d),
						name: d,
						kind: 'department',
						parent_id: created.id
					});
				}
			}
			showForm = false;
			editId = null;
			form = blankForm();
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
		} finally {
			busy = '';
		}
	}
</script>

<div class="topbar">
	<h1>Stores</h1>
	<span class="muted">Stock-holding locations and their departments.</span>
	{#if canManage}
		<button class="btn" style="margin-left:auto" onclick={newStore}>+ New store</button>
	{/if}
</div>
{#if error}<p class="error">{error}</p>{/if}

{#if showForm}
	<div class="card form">
		<h3 style="margin:0 0 12px">{editId ? 'Edit store' : 'New store'}</h3>
		<div class="grid">
			<label>Store name<input bind:value={form.name} placeholder="Main Stockroom" /></label>
			<label>Store location
				<input bind:value={form.address} placeholder="e.g. Central Warehouse, 12 Dock Rd, Manchester" />
			</label>
			<label>Latitude (optional)
				<input bind:value={form.latitude} type="text" inputmode="decimal" placeholder="53.4808" />
			</label>
			<label>Longitude (optional)
				<input bind:value={form.longitude} type="text" inputmode="decimal" placeholder="-2.2426" />
			</label>
			{#if !editId}
				<label class="wide">Departments (optional, comma-separated)
					<input bind:value={form.departments} placeholder="Receiving, Spares, IT" />
				</label>
			{/if}
		</div>
		<div class="row" style="gap:8px; margin-top:10px">
			<button class="btn" disabled={busy === 'save'} onclick={save}>
				{busy === 'save' ? 'Saving…' : editId ? 'Save changes' : 'Create store'}
			</button>
			<button class="btn small secondary" onclick={() => { showForm = false; editId = null; }}>Cancel</button>
		</div>
	</div>
{/if}

{#if locatedStores.length > 0}
	<div class="card">
		<SitesMap sites={locatedStores} onopen={(id) => (expanded = id)} />
		<p class="muted" style="margin-top:8px">
			{locatedStores.length} store{locatedStores.length === 1 ? '' : 's'} on the map · double-click a pin to open.
		</p>
	</div>
{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if stores.length === 0}
	<div class="card"><p class="muted">No stores yet. Create one to start holding stock.</p></div>
{:else}
	<div class="store-grid">
		{#each stores as s}
			{@const items = stockFor(s)}
			<div class="card store">
				<button
					class="store-head"
					onclick={() => (expanded = expanded === s.id ? null : s.id)}
				>
					<div>
						<h3>{s.name}</h3>
						{#if s.address}<div class="muted small addr">{s.address}</div>{/if}
						<span class="muted small">{s.department_count} dept{s.department_count === 1 ? '' : 's'}</span>
					</div>
					<div class="metrics">
						<div class="m"><div class="mv">{s.in_stock_count}</div><div class="ml">in stock</div></div>
						<div class="m"><div class="mv">{money(s.in_stock_value)}</div><div class="ml">value</div></div>
					</div>
				</button>
				{#if expanded === s.id}
					{#if canManage}
						<div class="row" style="padding:8px 12px; gap:8px">
							<button class="btn small secondary" onclick={() => openEdit(s)}>Edit location / coordinates</button>
						</div>
					{/if}
					<table>
						<thead><tr><th>Tag</th><th>Name</th><th>Type</th><th>Location</th></tr></thead>
						<tbody>
							{#each items as a}
								<tr class="clickable" role="button" tabindex="0"
									onclick={() => goto(`/assets/${a.id}`)}
									onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && goto(`/assets/${a.id}`)}>
									<td class="mono">{a.asset_tag}</td>
									<td>{a.name}</td>
									<td class="muted">{a.asset_type?.name ?? '—'}</td>
									<td class="muted">{a.location?.name ?? '—'}</td>
								</tr>
							{/each}
							{#if items.length === 0}
								<tr><td colspan="4" class="muted">No stock held here.</td></tr>
							{/if}
						</tbody>
					</table>
				{/if}
			</div>
		{/each}
	</div>
{/if}

<style>
	.topbar {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex-wrap: wrap;
		margin-bottom: 14px;
	}
	.topbar h1 {
		margin: 0;
	}
	.card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px;
		margin-bottom: 16px;
	}
	.form .grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}
	.form label.wide {
		grid-column: 1 / -1;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 13px;
		color: var(--muted);
	}
	input {
		font: inherit;
		color: var(--text);
	}
	.store-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 16px;
	}
	.store {
		margin-bottom: 0;
		padding: 0;
		overflow: hidden;
	}
	.store-head {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		background: none;
		border: none;
		padding: 16px;
		cursor: pointer;
		color: inherit;
		text-align: left;
	}
	.store-head:hover {
		background: var(--surface-2);
	}
	.store-head h3 {
		margin: 0 0 2px;
		font-size: 15px;
	}
	.addr {
		margin-bottom: 2px;
	}
	.metrics {
		display: flex;
		gap: 18px;
	}
	.m {
		text-align: right;
	}
	.mv {
		font-size: 20px;
		font-weight: 700;
	}
	.ml {
		font-size: 11px;
		color: var(--muted);
		text-transform: uppercase;
	}
	.small {
		font-size: 12px;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	th,
	td {
		text-align: left;
		padding: 8px;
		border-top: 1px solid var(--border);
	}
	tr.clickable {
		cursor: pointer;
	}
	tr.clickable:hover {
		background: var(--surface-2);
	}
	.mono {
		font-family: ui-monospace, monospace;
		font-size: 12px;
	}
	.row {
		display: flex;
		align-items: center;
	}
	.btn.small {
		padding: 4px 10px;
		font-size: 12px;
	}
	.btn.secondary {
		background: var(--surface-2);
		color: var(--text);
	}
</style>
