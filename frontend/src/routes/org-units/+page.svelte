<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import { depth, kindLabel } from '$lib/org';
	import type { OrgUnit, OrgUnitKind, Location } from '$lib/types';

	let units = $state<OrgUnit[]>([]);
	let kinds = $state<OrgUnitKind[]>([]);
	let locations = $state<Location[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showForm = $state(false);
	let editId = $state<string | null>(null);
	let f = $state({ key: '', name: '', kind: '', parent_id: '', default_location_id: '' });

	const editingNode = $derived(editId ? (units.find((u) => u.id === editId) ?? null) : null);
	const parentOptions = $derived(
		editingNode
			? units.filter((u) => u.id !== editingNode.id && !u.path.startsWith(editingNode.path + '.'))
			: units
	);

	async function load() {
		loading = true;
		error = '';
		try {
			[units, kinds] = await Promise.all([
				apiGet<OrgUnit[]>('/api/org-units'),
				apiGet<OrgUnitKind[]>('/api/metadata/org-unit-kinds')
			]);
			locations = (await apiGet<Location[]>('/api/locations')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function openCreate(parent: OrgUnit | null) {
		editId = null;
		f = { key: '', name: '', kind: '', parent_id: parent ? parent.id : '', default_location_id: '' };
		showForm = true;
	}

	function openEdit(u: OrgUnit) {
		editId = u.id;
		f = { key: u.key, name: u.name, kind: u.kind ?? '', parent_id: u.parent_id ?? '', default_location_id: u.default_location_id ?? '' };
		showForm = true;
	}

	async function save(e: Event) {
		e.preventDefault();
		error = '';
		try {
			if (editId) {
				await apiPut(`/api/org-units/${editId}`, {
					name: f.name,
					kind: f.kind || null,
					default_location_id: f.default_location_id || null
				});
				const origParent = editingNode?.parent_id ?? '';
				if (f.parent_id !== origParent) {
					await apiPost(`/api/org-units/${editId}/move`, { parent_id: f.parent_id || null });
				}
			} else {
				await apiPost('/api/org-units', {
					key: f.key,
					name: f.name,
					kind: f.kind || null,
					parent_id: f.parent_id || null,
					default_location_id: f.default_location_id || null
				});
			}
			showForm = false;
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}

	async function remove(u: OrgUnit) {
		if (!confirm(`Delete org unit "${u.name}"?`)) return;
		try {
			await apiDelete(`/api/org-units/${u.id}`);
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete failed';
		}
	}

	const locById = $derived(new Map(locations.map((l) => [l.id, l])));
	const manage = $derived(can($me, 'hierarchy.manage'));
</script>

<div class="topbar">
	<h1>Org Units</h1>
	{#if manage}
		<button class="btn" onclick={() => openCreate(null)}>+ New unit</button>
	{/if}
</div>

<p class="muted" style="margin:-8px 0 16px">
	Define your hierarchy (region → district → division → unit). Set a <strong>default location</strong> on units
	so assign/check-out can place assets automatically. Link assets via <strong>Owner (org unit)</strong> for roll-up reports.
</p>

{#if error}<p class="error">{error}</p>{/if}

{#if showForm}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">{editId ? 'Edit org unit' : 'New org unit'}</h3>
		<form onsubmit={save}>
			<div class="grid cols-2">
				{#if !editId}
					<div class="field">
						<label>Key * <span class="muted">(lowercase, digits, underscore)</span></label>
						<input bind:value={f.key} required pattern="[a-z0-9_]+" />
					</div>
				{/if}
				<div class="field">
					<label>Kind <span class="muted">(region, district, division, unit…)</span></label>
					<select bind:value={f.kind}>
						<option value="">—</option>
						{#each kinds as k}
							<option value={k.key}>{k.label}</option>
						{/each}
					</select>
				</div>
				<div class="field">
					<label>Parent {#if editId}<span class="muted">(move under another unit)</span>{/if}</label>
					<select bind:value={f.parent_id}>
						<option value="">— (root)</option>
						{#each parentOptions as u}
							<option value={u.id}>{'— '.repeat(depth(u.path))}{u.name}</option>
						{/each}
					</select>
				</div>
				<div class="field"><label>Name *</label><input bind:value={f.name} required /></div>
				<div class="field">
					<label>Default location <span class="muted">(used when assigning to people in this unit)</span></label>
					<select bind:value={f.default_location_id}>
						<option value="">—</option>
						{#each locations as l}
							<option value={l.id}>{l.name}</option>
						{/each}
					</select>
				</div>
			</div>
			<div class="row">
				<button class="btn" type="submit">Save</button>
				<button class="btn secondary" type="button" onclick={() => (showForm = false)}>Cancel</button>
			</div>
		</form>
	</div>
{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<table>
			<thead>
				<tr><th>Name</th><th>Kind</th><th>Default location</th><th>Key</th><th>Path</th><th></th></tr>
			</thead>
			<tbody>
				{#each units as u}
					<tr>
						<td style="padding-left:{12 + depth(u.path) * 20}px">{u.name}</td>
						<td class="muted">{kindLabel(kinds, u.kind)}</td>
						<td class="muted">{u.default_location_id ? (locById.get(u.default_location_id)?.name ?? '—') : '—'}</td>
						<td class="muted">{u.key}</td>
						<td class="muted">{u.path}</td>
						<td style="text-align:right">
							{#if manage}
								<button class="btn secondary small" onclick={() => openCreate(u)}>+ Child</button>
								<button class="btn secondary small" onclick={() => openEdit(u)}>Edit</button>
								<button class="btn danger small" onclick={() => remove(u)}>Delete</button>
							{/if}
						</td>
					</tr>
				{/each}
				{#if units.length === 0}
					<tr><td colspan="6" class="muted">No org units yet.</td></tr>
				{/if}
			</tbody>
		</table>
	{/if}
</div>
