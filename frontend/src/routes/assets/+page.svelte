<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet } from '$lib/api';
	import { me, can } from '$lib/me';
	import AssetsMap from '$lib/components/AssetsMap.svelte';
	import QRCode from 'qrcode';
	import JsBarcode from 'jsbarcode';
	import type { Paginated, Asset, AssetType, Location, AssetMapPoint, OrgUnit } from '$lib/types';

	const PAGE_SIZE = 50;
	let assets = $state<Asset[]>([]);
	let total = $state(0);
	let page = $state(1);
	const totalPages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	let types = $state<AssetType[]>([]);
	let locations = $state<Location[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);

	let view = $state<'list' | 'map'>('list');
	let mapPoints = $state<AssetMapPoint[]>([]);
	let mapLoaded = $state(false);

	let q = $state('');
	let typeKey = $state('');
	let infraPreset = $state('');
	let locationId = $state('');
	let orgUnitId = $state('');
	let loading = $state(true);
	let error = $state('');

	async function loadMap() {
		try {
			mapPoints = (await apiGet<AssetMapPoint[]>('/api/assets/map')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load map';
		} finally {
			mapLoaded = true;
		}
	}

	function showMap() {
		view = 'map';
		if (!mapLoaded) loadMap();
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
			if (q) params.set('q', q);
			if (typeKey) params.set('type', typeKey);
			if (locationId) params.set('location_id', locationId);
			if (orgUnitId) {
				params.set('owner_org_unit_id', orgUnitId);
				params.set('subtree', 'true');
			}
			const res = await apiGet<Paginated<Asset>>(`/api/assets?${params.toString()}`);
			assets = res.items ?? [];
			total = res.total ?? 0;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	// Filtering changes the result set, so always jump back to the first page.
	function applyFilter() {
		page = 1;
		if (infraPreset) typeKey = infraPreset;
		load();
	}

	function setInfraPreset(key: string) {
		infraPreset = infraPreset === key ? '' : key;
		typeKey = infraPreset;
		applyFilter();
	}

	function goToPage(p: number) {
		const next = Math.min(Math.max(1, p), totalPages);
		if (next === page) return;
		page = next;
		load();
	}

	onMount(async () => {
		try {
			types = await apiGet<AssetType[]>('/api/metadata/asset-types');
			locations = await apiGet<Location[]>('/api/locations');
			orgUnits = (await apiGet<OrgUnit[]>('/api/org-units')) ?? [];
		} catch {
			/* ignore */
		}
		load();
	});

	// --- printable QR/barcode labels for the current (filtered) page ----------
	let printing = $state(false);

	function escapeHtml(s: string): string {
		return s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c] ?? c);
	}

	async function labelHtml(a: Asset): Promise<string> {
		const url = `${window.location.origin}/assets/${a.id}`;
		let qr = '';
		try {
			qr = await QRCode.toString(url, { type: 'svg', margin: 0, width: 88 });
		} catch {
			qr = '';
		}
		let bar = '';
		try {
			const el = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
			JsBarcode(el, a.asset_tag, { format: 'CODE128', displayValue: true, fontSize: 11, height: 30, width: 1.3, margin: 0 });
			bar = el.outerHTML;
		} catch {
			bar = '';
		}
		return `<div class="label"><div class="qr">${qr}</div><div class="meta"><div class="nm">${escapeHtml(a.name)}</div><div class="tg">${escapeHtml(a.asset_tag)}</div><div class="bar">${bar}</div></div></div>`;
	}

	async function printLabels() {
		if (!assets.length || printing) return;
		printing = true;
		try {
			const parts = await Promise.all(assets.map(labelHtml));
			const w = window.open('', '_blank');
			if (!w) return;
			w.document.write(`<!doctype html><html><head><title>Asset labels</title><style>
				*{box-sizing:border-box}
				body{font-family:system-ui,-apple-system,sans-serif;margin:0;padding:10px;background:#fff;color:#000}
				.sheet{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}
				.label{border:1px solid #000;border-radius:6px;padding:8px;display:flex;gap:8px;align-items:center;break-inside:avoid}
				.qr svg{width:72px;height:72px;display:block}
				.meta{min-width:0}
				.nm{font-weight:700;font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:150px}
				.tg{font-size:10px;color:#444;margin:1px 0 4px}
				.bar svg{max-width:150px}
				.toolbar{margin-bottom:10px}
				@media print{.toolbar{display:none}body{padding:0}}
			</style></head><body>
				<div class="toolbar"><button onclick="window.print()">Print ${assets.length} label(s)</button></div>
				<div class="sheet">${parts.join('')}</div>
			</body></html>`);
			w.document.close();
		} finally {
			printing = false;
		}
	}
</script>

<div class="topbar">
	<h1>Assets</h1>
	<div class="row">
		<div class="seg">
			<button class="seg-btn" class:active={view === 'list'} onclick={() => (view = 'list')}>List</button>
			<button class="seg-btn" class:active={view === 'map'} onclick={showMap}>Map</button>
		</div>
		{#if view === 'list'}
			<button class="btn secondary" onclick={printLabels} disabled={printing || !assets.length}>
				{printing ? 'Preparing…' : 'Print labels'}
			</button>
		{/if}
		{#if can($me, 'asset.write')}
			<button class="btn" onclick={() => goto('/assets/new')}>+ New asset</button>
		{/if}
	</div>
</div>

{#if view === 'map'}
	{#if error}<p class="error">{error}</p>{/if}
	<div class="card">
		{#if !mapLoaded}
			<p class="muted">Loading map…</p>
		{:else if mapPoints.length === 0}
			<p class="muted">No assets have coordinates yet. Assets inherit coordinates from their location — add latitude/longitude to a site under Locations.</p>
		{:else}
			<AssetsMap points={mapPoints} />
			<p class="muted" style="margin-top:8px">
				{mapPoints.length} asset(s) plotted · grouped by coordinate · click a marker to list assets.
			</p>
		{/if}
	</div>
{:else}
<div class="toolbar">
	<input placeholder="Search tag or name…" bind:value={q} onkeydown={(e) => e.key === 'Enter' && applyFilter()} />
	<select bind:value={typeKey} onchange={() => { infraPreset = ''; applyFilter(); }}>
		<option value="">All types</option>
		{#each types.filter((t) => !t.is_abstract) as t}
			<option value={t.key}>{t.name}</option>
		{/each}
	</select>
	<select bind:value={locationId} onchange={applyFilter}>
		<option value="">All locations</option>
		{#each locations as l}
			<option value={l.id}>{l.name}</option>
		{/each}
	</select>
	<select bind:value={orgUnitId} onchange={applyFilter}>
		<option value="">All org units</option>
		{#each orgUnits as ou}
			<option value={ou.id}>{'— '.repeat(Math.max(0, ou.path.split('.').length - 1))}{ou.name}{ou.kind ? ` (${ou.kind})` : ''}</option>
		{/each}
	</select>
	<button class="btn secondary" onclick={applyFilter}>Filter</button>
</div>
{#if can($me, 'dcim.read')}
	<div class="toolbar" style="margin-top:-8px">
		<span class="muted small">Infrastructure:</span>
		<button class="btn secondary small" class:active={infraPreset === 'server'} onclick={() => setInfraPreset('server')}>Servers</button>
		<button class="btn secondary small" class:active={infraPreset === 'network'} onclick={() => setInfraPreset('network')}>Network</button>
		<button class="btn secondary small" class:active={infraPreset === 'storage'} onclick={() => setInfraPreset('storage')}>Storage</button>
		<button class="btn secondary small" class:active={infraPreset === 'power'} onclick={() => setInfraPreset('power')}>Power</button>
	</div>
{/if}

{#if error}<p class="error">{error}</p>{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<table>
			<thead>
				<tr><th>Tag</th><th>Name</th><th>Type</th><th>State</th><th>Location</th><th>Org unit</th><th>Assigned</th></tr>
			</thead>
			<tbody>
				{#each assets as a}
					<tr
						role="button"
						tabindex="0"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
					>
						<td>{a.asset_tag}</td>
						<td>{a.name}</td>
						<td>{a.asset_type?.name ?? '—'}</td>
						<td>
							{#if a.current_state}
								<span class="badge" style="border-color:{a.current_state.color}">{a.current_state.label}</span>
							{:else}—{/if}
						</td>
						<td>{a.location?.name ?? '—'}</td>
						<td>{a.owner_org_unit?.name ?? '—'}</td>
						<td>{a.assigned_to ? `${a.assigned_to.first_name} ${a.assigned_to.last_name}` : '—'}</td>
					</tr>
				{/each}
				{#if assets.length === 0}
					<tr><td colspan="6" class="muted">No assets found.</td></tr>
				{/if}
			</tbody>
		</table>
		<div class="pager">
			<span class="muted">
				{#if total === 0}No assets{:else}
					{(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
				{/if}
			</span>
			{#if totalPages > 1}
				<div class="row">
					<button class="btn secondary" onclick={() => goToPage(1)} disabled={page <= 1}>« First</button>
					<button class="btn secondary" onclick={() => goToPage(page - 1)} disabled={page <= 1}>‹ Prev</button>
					<span class="muted">Page {page} / {totalPages}</span>
					<button class="btn secondary" onclick={() => goToPage(page + 1)} disabled={page >= totalPages}>Next ›</button>
					<button class="btn secondary" onclick={() => goToPage(totalPages)} disabled={page >= totalPages}>Last »</button>
				</div>
			{/if}
		</div>
	{/if}
</div>
{/if}

<style>
	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	.seg-btn {
		background: transparent;
		color: var(--muted);
		border: none;
		padding: 6px 14px;
		cursor: pointer;
		font: inherit;
	}
	.seg-btn.active {
		background: #6aa6ff;
		color: #0d0f17;
		font-weight: 600;
	}
	.pager {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		margin-top: 12px;
		flex-wrap: wrap;
	}
	.pager .row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.pager .btn.secondary:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
