<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Asset, AssetType, Paginated, OrgUnit } from '$lib/types';
	import { depth } from '$lib/org';

	const PAGE_SIZE = 50;

	let types = $state<AssetType[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let assets = $state<Asset[]>([]);
	let summaryAssets = $state<Asset[]>([]);
	let total = $state(0);
	let page = $state(1);
	const totalPages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	let q = $state('');
	let typeKey = $state('');
	let stateKey = $state('');
	let orgUnitId = $state('');
	let loading = $state(true);
	let error = $state('');

	const canWrite = $derived(can($me, 'asset.write'));

	// Concrete peripheral subtypes (monitor, keyboard, …), sorted.
	const peripheralTypes = $derived(
		types
			.filter((t) => !t.is_abstract && t.path.startsWith('hardware.peripheral.'))
			.sort((a, b) => a.sort - b.sort || a.name.localeCompare(b.name))
	);

	const summary = $derived.by(() => {
		const m = new Map<string, number>();
		for (const t of peripheralTypes) m.set(t.key, 0);
		for (const a of summaryAssets) {
			const k = a.asset_type?.key;
			if (k) m.set(k, (m.get(k) ?? 0) + 1);
		}
		return peripheralTypes.map((t) => ({ type: t, count: m.get(t.key) ?? 0 }));
	});

	async function loadSummary() {
		const params = new URLSearchParams({ type: 'peripheral', page_size: '500' });
		if (orgUnitId) {
			params.set('owner_org_unit_id', orgUnitId);
			params.set('subtree', 'true');
		}
		const res = await apiGet<Paginated<Asset>>(`/api/assets?${params}`);
		summaryAssets = res.items ?? [];
	}

	async function loadTypes() {
		types = (await apiGet<AssetType[]>('/api/metadata/asset-types')) ?? [];
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
			params.set('type', typeKey || 'peripheral');
			if (q) params.set('q', q);
			if (stateKey) params.set('state', stateKey);
			if (orgUnitId) {
				params.set('owner_org_unit_id', orgUnitId);
				params.set('subtree', 'true');
			}
			const res = await apiGet<Paginated<Asset>>(`/api/assets?${params}`);
			assets = res.items ?? [];
			total = res.total ?? 0;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load peripherals';
		} finally {
			loading = false;
		}
	}

	function applyFilter() {
		page = 1;
		load();
		loadSummary();
	}

	function goToPage(p: number) {
		const next = Math.min(Math.max(1, p), totalPages);
		if (next === page) return;
		page = next;
		load();
	}

	function newPeripheral(key: string) {
		goto(`/assets/new?type=${encodeURIComponent(key)}`);
	}

	onMount(async () => {
		try {
			await loadTypes();
			orgUnits = (await apiGet<OrgUnit[]>('/api/org-units')) ?? [];
			await Promise.all([loadSummary(), load()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
			loading = false;
		}
	});
</script>

<div class="topbar">
	<div>
		<h1>Peripherals</h1>
		<p class="muted" style="margin:4px 0 0">
			Monitors, keyboards, mice, projectors, printers, and other end-user hardware.
		</p>
	</div>
	<div class="row">
		<button class="btn secondary" onclick={() => goto('/stock')}>Stock room</button>
		{#if canWrite}
			<button class="btn" onclick={() => goto('/assets/new?type=monitor')}>+ Add peripheral</button>
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

<div class="summary-grid">
	{#each summary as s}
		<button
			type="button"
			class="summary-card"
			class:active={typeKey === s.type.key}
			onclick={() => {
				typeKey = typeKey === s.type.key ? '' : s.type.key;
				applyFilter();
			}}
		>
			<div class="summary-name">{s.type.name}</div>
			<div class="summary-count">{s.count}</div>
			{#if canWrite}
				<span
					class="summary-add"
					role="button"
					tabindex="0"
					onclick={(e) => {
						e.stopPropagation();
						newPeripheral(s.type.key);
					}}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.stopPropagation();
							newPeripheral(s.type.key);
						}
					}}
				>+ add</span>
			{/if}
		</button>
	{/each}
</div>

<div class="toolbar">
	<input placeholder="Search tag, name, serial…" bind:value={q} onkeydown={(e) => e.key === 'Enter' && applyFilter()} />
	<select bind:value={typeKey} onchange={applyFilter}>
		<option value="">All peripheral types</option>
		{#each peripheralTypes as t}
			<option value={t.key}>{t.name}</option>
		{/each}
	</select>
	<select bind:value={stateKey} onchange={applyFilter}>
		<option value="">All states</option>
		<option value="in_stock">In stock</option>
		<option value="deployed">Deployed / in use</option>
		<option value="in_use">In use</option>
		<option value="retired">Retired</option>
	</select>
	<select bind:value={orgUnitId} onchange={applyFilter}>
		<option value="">All org units</option>
		{#each orgUnits as ou}
			<option value={ou.id}>{'— '.repeat(depth(ou.path))}{ou.name}{ou.kind ? ` (${ou.kind})` : ''}</option>
		{/each}
	</select>
	<button class="btn secondary" onclick={applyFilter}>Filter</button>
</div>

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if assets.length === 0}
		<p class="muted">No peripherals found. Add stock via Procurement → receive, or register directly with <strong>+ Add peripheral</strong>.</p>
	{:else}
		<table>
			<thead>
				<tr><th>Tag</th><th>Name</th><th>Type</th><th>State</th><th>Location</th><th>Org unit</th><th>Assigned to</th><th>Attributes</th></tr>
			</thead>
			<tbody>
				{#each assets as a}
					<tr
						role="button"
						tabindex="0"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
					>
						<td><code>{a.asset_tag}</code></td>
						<td>{a.name}</td>
						<td>{a.asset_type?.name ?? '—'}</td>
						<td>{a.current_state?.label ?? '—'}</td>
						<td class="muted">{a.location?.name ?? '—'}</td>
						<td class="muted">{a.owner_org_unit?.name ?? '—'}</td>
						<td class="muted">{a.assigned_to?.email ?? '—'}</td>
						<td class="muted attrs">
							{#if a.attributes?.screen_size}{a.attributes.screen_size}"{/if}
							{#if a.attributes?.resolution} · {a.attributes.resolution}{/if}
							{#if a.attributes?.connectivity} · {a.attributes.connectivity}{/if}
							{#if a.attributes?.capacity_gb} · {a.attributes.capacity_gb} GB{/if}
							{#if a.attributes?.lumens} · {a.attributes.lumens} lm{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>

		{#if totalPages > 1}
			<div class="pager">
				<button class="btn secondary small" disabled={page <= 1} onclick={() => goToPage(1)}>First</button>
				<button class="btn secondary small" disabled={page <= 1} onclick={() => goToPage(page - 1)}>Prev</button>
				<span class="muted">Page {page} of {totalPages} · {total} total</span>
				<button class="btn secondary small" disabled={page >= totalPages} onclick={() => goToPage(page + 1)}>Next</button>
				<button class="btn secondary small" disabled={page >= totalPages} onclick={() => goToPage(totalPages)}>Last</button>
			</div>
		{/if}
	{/if}
</div>

<style>
	.summary-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
		gap: 10px;
		margin-bottom: 16px;
	}
	.summary-card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 12px;
		text-align: left;
		cursor: pointer;
		position: relative;
	}
	.summary-card:hover,
	.summary-card.active {
		border-color: var(--primary);
		background: var(--surface-2);
	}
	.summary-name {
		font-size: 12px;
		color: var(--muted);
		margin-bottom: 4px;
	}
	.summary-count {
		font-size: 22px;
		font-weight: 600;
	}
	.summary-add {
		position: absolute;
		top: 8px;
		right: 8px;
		font-size: 11px;
		color: var(--primary);
	}
	.attrs {
		font-size: 12px;
		max-width: 220px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.pager {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 14px;
		flex-wrap: wrap;
	}
</style>
