<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Location, LocationKind, LocationAlias } from '$lib/types';
	import SitesMap from '$lib/components/SitesMap.svelte';

	let locations = $state<Location[]>([]);
	let kinds = $state<LocationKind[]>([]);
	let loading = $state(true);
	let error = $state('');

	let view = $state<'list' | 'map'>('list');
	let showForm = $state(false);
	let editId = $state<string | null>(null);
	let selected = $state<Location | null>(null);

	const children = $derived(selected ? locations.filter((l) => l.parent_id === selected!.id) : []);

	type Form = {
		key: string;
		name: string;
		kind: string;
		parent_id: string;
		dr_role: string;
		tier: string;
		timezone: string;
		address: string;
		latitude: string;
		longitude: string;
	};
	const blank = (parentId = ''): Form => ({
		key: '',
		name: '',
		kind: 'site',
		parent_id: parentId,
		dr_role: '',
		tier: '',
		timezone: '',
		address: '',
		latitude: '',
		longitude: ''
	});
	let f = $state<Form>(blank());

	let aliases = $state<LocationAlias[]>([]);
	let aliasForm = $state({ alias: '', location_id: '', note: '' });

	// Fallback kind list if the metadata endpoint is empty.
	const FALLBACK = ['region', 'site', 'building', 'floor', 'room', 'zone', 'row', 'rack', 'slot'];
	const kindKeys = $derived(kinds.length ? kinds.map((k) => k.key) : FALLBACK);
	const showGeo = $derived(kinds.find((k) => k.key === f.kind)?.geo ?? ['region', 'site', 'building'].includes(f.kind));
	const mappable = $derived(locations.filter((l) => l.latitude != null && l.longitude != null));

	const editingNode = $derived(editId ? (locations.find((l) => l.id === editId) ?? null) : null);
	// When editing, a node can't be moved under itself or its own descendants.
	const parentOptions = $derived(
		editingNode
			? locations.filter((l) => l.id !== editingNode.id && !l.path.startsWith(editingNode.path + '.'))
			: locations
	);

	function depth(path: string): number {
		return Math.max(0, path.split('.').length - 1);
	}

	async function loadAliases() {
		if (!can($me, 'hierarchy.read')) return;
		try {
			aliases = (await apiGet<LocationAlias[]>('/api/location-aliases')) ?? [];
		} catch {
			aliases = [];
		}
	}

	async function load() {
		loading = true;
		error = '';
		try {
			locations = await apiGet<Location[]>('/api/locations');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		try {
			kinds = (await apiGet<LocationKind[]>('/api/metadata/location-kinds')) ?? [];
		} catch {
			kinds = [];
		}
		await load();
		await loadAliases();
	});

	function openCreate(parent: Location | null) {
		editId = null;
		f = blank(parent ? parent.id : '');
		if (parent) {
			// default a child's kind to the next level down where possible
			const idx = kindKeys.indexOf(parent.kind);
			if (idx >= 0 && idx + 1 < kindKeys.length) f.kind = kindKeys[idx + 1];
		}
		showForm = true;
	}

	function openEdit(l: Location) {
		editId = l.id;
		f = {
			key: l.key,
			name: l.name,
			kind: l.kind,
			parent_id: l.parent_id ?? '',
			dr_role: l.dr_role ?? '',
			tier: l.tier ?? '',
			timezone: l.timezone ?? '',
			address: l.address ?? '',
			latitude: l.latitude != null ? String(l.latitude) : '',
			longitude: l.longitude != null ? String(l.longitude) : ''
		};
		showForm = true;
	}

	function geoPayload() {
		return {
			kind: f.kind,
			dr_role: f.dr_role || null,
			tier: f.tier || null,
			timezone: f.timezone || null,
			address: f.address || null,
			latitude: f.latitude !== '' ? Number(f.latitude) : null,
			longitude: f.longitude !== '' ? Number(f.longitude) : null
		};
	}

	async function save(e: Event) {
		e.preventDefault();
		error = '';
		try {
			if (editId) {
				await apiPut(`/api/locations/${editId}`, { name: f.name, ...geoPayload() });
				const origParent = editingNode?.parent_id ?? '';
				if (f.parent_id !== origParent) {
					await apiPost(`/api/locations/${editId}/move`, { parent_id: f.parent_id || null });
				}
			} else {
				await apiPost('/api/locations', {
					key: f.key,
					name: f.name,
					parent_id: f.parent_id || null,
					...geoPayload()
				});
			}
			showForm = false;
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}

	async function remove(l: Location) {
		if (!confirm(`Delete location "${l.name}"?`)) return;
		try {
			await apiDelete(`/api/locations/${l.id}`);
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete failed';
		}
	}

	async function saveAlias(e: Event) {
		e.preventDefault();
		error = '';
		try {
			await apiPost('/api/location-aliases', {
				alias: aliasForm.alias.trim().toLowerCase(),
				location_id: aliasForm.location_id,
				note: aliasForm.note.trim()
			});
			aliasForm = { alias: '', location_id: '', note: '' };
			await loadAliases();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save alias failed';
		}
	}

	async function removeAlias(alias: string) {
		if (!confirm(`Remove alias "${alias}"?`)) return;
		try {
			await apiDelete(`/api/location-aliases/${encodeURIComponent(alias)}`);
			await loadAliases();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete alias failed';
		}
	}

	const manage = $derived(can($me, 'hierarchy.manage'));
</script>

<div class="topbar">
	<h1>Locations</h1>
	<div class="row">
		<div class="seg">
			<button class="seg-btn" class:active={view === 'list'} onclick={() => (view = 'list')}>Tree</button>
			<button class="seg-btn" class:active={view === 'map'} onclick={() => (view = 'map')}>Map</button>
		</div>
		{#if manage}
			<button class="btn" onclick={() => openCreate(null)}>+ New location</button>
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if showForm}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">{editId ? 'Edit location' : 'New location'}</h3>
		<form onsubmit={save}>
			<div class="grid cols-2">
				{#if !editId}
					<div class="field">
						<label>Key * <span class="muted">(lowercase, digits, underscore)</span></label>
						<input bind:value={f.key} required pattern="[a-z0-9_]+" />
					</div>
				{/if}
				<div class="field">
					<label>Parent {#if editId}<span class="muted">(move under another location)</span>{/if}</label>
					<select bind:value={f.parent_id}>
						<option value="">— (root)</option>
						{#each parentOptions as l}
							<option value={l.id}>{'— '.repeat(depth(l.path))}{l.name}</option>
						{/each}
					</select>
				</div>
				<div class="field"><label>Name *</label><input bind:value={f.name} required /></div>
				<div class="field">
					<label>Kind</label>
					<select bind:value={f.kind}>
						{#each kindKeys as k}<option value={k}>{k}</option>{/each}
					</select>
				</div>
			</div>

			<div class="grid cols-2">
				<div class="field">
					<label>DR role</label>
					<select bind:value={f.dr_role}>
						<option value="">—</option>
						<option value="primary">primary</option>
						<option value="dr">dr</option>
						<option value="edge">edge</option>
						<option value="colo">colo</option>
					</select>
				</div>
				<div class="field">
					<label>Tier</label>
					<input bind:value={f.tier} placeholder="tier-3" />
				</div>
				<div class="field">
					<label>Timezone</label>
					<input bind:value={f.timezone} placeholder="Europe/London" />
				</div>
				<div class="field">
					<label>Address</label>
					<input bind:value={f.address} />
				</div>
			</div>

			{#if showGeo}
				<div class="grid cols-2">
					<div class="field">
						<label>Latitude <span class="muted">(decimal)</span></label>
						<input type="number" step="any" min="-90" max="90" bind:value={f.latitude} placeholder="51.5072" />
					</div>
					<div class="field">
						<label>Longitude <span class="muted">(decimal)</span></label>
						<input type="number" step="any" min="-180" max="180" bind:value={f.longitude} placeholder="-0.1276" />
					</div>
				</div>
			{/if}

			<div class="row">
				<button class="btn" type="submit">Save</button>
				<button class="btn secondary" type="button" onclick={() => (showForm = false)}>Cancel</button>
			</div>
		</form>
	</div>
{/if}

{#if view === 'list'}
	<div class="card">
		{#if loading}
			<p class="muted">Loading…</p>
		{:else}
			<table>
				<thead>
					<tr><th>Name</th><th>Kind</th><th>DR / Tier</th><th>Geo</th><th>Path</th><th></th></tr>
				</thead>
				<tbody>
					{#each locations as l}
						<tr
							role="button"
							tabindex="0"
							onclick={() => (selected = l)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), (selected = l))}
						>
							<td style="padding-left:{12 + depth(l.path) * 20}px">{l.name}</td>
							<td><span class="badge">{l.kind}</span></td>
							<td class="muted">
								{l.dr_role ?? '—'}{l.tier ? ` · ${l.tier}` : ''}
							</td>
							<td class="muted">
								{#if l.latitude != null && l.longitude != null}
									{l.latitude.toFixed(3)}, {l.longitude.toFixed(3)}
								{:else}—{/if}
							</td>
							<td class="muted">{l.path}</td>
							<td style="text-align:right; white-space:nowrap">
								{#if manage}
									<button class="btn secondary small" onclick={(e) => { e.stopPropagation(); openCreate(l); }}>+ Child</button>
									<button class="btn secondary small" onclick={(e) => { e.stopPropagation(); openEdit(l); }}>Edit</button>
									<button class="btn danger small" onclick={(e) => { e.stopPropagation(); remove(l); }}>Delete</button>
								{/if}
							</td>
						</tr>
					{/each}
					{#if locations.length === 0}
						<tr><td colspan="6" class="muted">No locations yet.</td></tr>
					{/if}
				</tbody>
			</table>
		{/if}
	</div>
{:else}
	<div class="card">
		{#if loading}
			<p class="muted">Loading…</p>
		{:else if mappable.length === 0}
			<p class="muted">No locations have coordinates yet. Edit a site/building and add latitude &amp; longitude.</p>
		{:else}
			<SitesMap sites={mappable} onopen={(id) => openEdit(locations.find((l) => l.id === id)!)} />
			<p class="muted" style="margin-top:8px">
				{mappable.length} located · double-click a pin to edit. Map tiles load from OpenStreetMap (needs internet in the browser).
			</p>
		{/if}
	</div>
{/if}

{#if can($me, 'hierarchy.read')}
	<div class="card" style="margin-top:16px">
		<h3 style="margin-top:0">External name aliases</h3>
		<p class="muted" style="margin-top:0">
			Map OpManager map/probe names or legacy site codes to ITAM locations. Used during connector sync and CSV import when the source label does not match a location key.
		</p>
		{#if manage}
			<form class="grid cols-3" style="margin-top:12px; align-items:end" onsubmit={saveAlias}>
				<div class="field">
					<label>Alias *</label>
					<input bind:value={aliasForm.alias} required placeholder="e.g. hq-floor2 or probe-east" />
				</div>
				<div class="field">
					<label>Location *</label>
					<select bind:value={aliasForm.location_id} required>
						<option value="">Select location…</option>
						{#each locations as l}
							<option value={l.id}>{l.name}</option>
						{/each}
					</select>
				</div>
				<div class="field">
					<label>Note</label>
					<input bind:value={aliasForm.note} placeholder="Optional — source system" />
				</div>
				<div class="field" style="grid-column:1/-1">
					<button class="btn small" type="submit">Add alias</button>
				</div>
			</form>
		{/if}
		<table style="margin-top:12px">
			<thead><tr><th>Alias</th><th>Location</th><th>Note</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each aliases as a}
					<tr>
						<td><code>{a.alias}</code></td>
						<td>{a.location?.name ?? locations.find((l) => l.id === a.location_id)?.name ?? '—'}</td>
						<td class="muted">{a.note || '—'}</td>
						{#if manage}
							<td><button class="btn danger small" onclick={() => removeAlias(a.alias)}>Remove</button></td>
						{/if}
					</tr>
				{/each}
				{#if aliases.length === 0}
					<tr><td colspan={manage ? 4 : 3} class="muted">No aliases yet.</td></tr>
				{/if}
			</tbody>
		</table>
	</div>
{/if}

{#if selected}
	<div class="drawer-backdrop" role="button" tabindex="0" aria-label="Close panel" onclick={() => (selected = null)} onkeydown={(e) => e.key === 'Escape' && (selected = null)}></div>
	<aside class="drawer">
		<button class="btn secondary small drawer-close" onclick={() => (selected = null)}>Close</button>
		<h2>{selected.name}</h2>
		<div class="row" style="gap:8px; margin-bottom:14px">
			<span class="badge">{selected.kind}</span>
			{#if selected.dr_role}<span class="badge">{selected.dr_role}</span>{/if}
			{#if selected.tier}<span class="badge">{selected.tier}</span>{/if}
		</div>

		<div class="kv">
			<div class="k">Key</div><div class="muted">{selected.key}</div>
			<div class="k">Path</div><div class="muted">{selected.path}</div>
			<div class="k">Timezone</div><div>{selected.timezone || '—'}</div>
			<div class="k">Address</div><div>{selected.address || '—'}</div>
			<div class="k">Coordinates</div>
			<div>
				{#if selected.latitude != null && selected.longitude != null}
					{selected.latitude}, {selected.longitude}
				{:else}—{/if}
			</div>
		</div>

		<h3 style="margin-bottom:6px">Children ({children.length})</h3>
		{#if children.length}
			<ul style="margin:0; padding-left:18px">
				{#each children as c}
					<li>
						<button class="linklike" onclick={() => (selected = c)}>{c.name}</button>
						<span class="muted"> · {c.kind}</span>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="muted" style="margin-top:0">No child locations.</p>
		{/if}

		{#if manage}
			<div class="row" style="margin-top:18px; flex-wrap:wrap">
				<button class="btn small" onclick={() => { const s = selected!; selected = null; openEdit(s); }}>Edit</button>
				<button class="btn secondary small" onclick={() => { const s = selected!; selected = null; openCreate(s); }}>+ Add child</button>
				{#if selected.latitude != null}
					<button class="btn secondary small" onclick={() => { selected = null; view = 'map'; }}>Show on map</button>
				{/if}
				<button class="btn danger small" onclick={() => { const s = selected!; selected = null; remove(s); }}>Delete</button>
			</div>
		{/if}
	</aside>
{/if}

<style>
	.linklike {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
		font: inherit;
		padding: 0;
	}
	.seg {
		display: inline-flex;
		border: 1px solid var(--border, #2a2a3a);
		border-radius: 8px;
		overflow: hidden;
	}
	.seg-btn {
		background: transparent;
		color: var(--muted, #9aa0b4);
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
</style>
