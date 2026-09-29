<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Person, OrgUnit, Asset, Paginated } from '$lib/types';

	let people = $state<Person[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showForm = $state(false);
	let editId = $state<string | null>(null);
	let assetsFor = $state<Person | null>(null);
	let heldAssets = $state<Asset[]>([]);
	let assetsLoading = $state(false);

	let f = $state({
		first_name: '',
		last_name: '',
		email: '',
		title: '',
		employee_no: '',
		org_unit_id: '',
		manager_id: '',
		is_active: true
	});

	const byId = $derived(new Map(people.map((p) => [p.id, p])));
	const ouById = $derived(new Map(orgUnits.map((o) => [o.id, o])));
	let q = $state('');
	let fOrg = $state('');
	let fStatus = $state('');
	let fHolding = $state('');

	const filtered = $derived.by(() => {
		const term = q.trim().toLowerCase();
		return people.filter((p) => {
			if (term) {
				const hay = [p.first_name, p.last_name, `${p.first_name} ${p.last_name}`, p.email, p.title, p.employee_no, fullName(p.manager_id)]
					.join(' ')
					.toLowerCase();
				if (!hay.includes(term)) return false;
			}
			if (fOrg === '__none') {
				if (p.org_unit_id) return false;
			} else if (fOrg && p.org_unit_id !== fOrg) return false;
			if (fStatus && String(p.is_active) !== fStatus) return false;
			if (fHolding === 'yes' && !(p.asset_count ?? 0)) return false;
			if (fHolding === 'no' && (p.asset_count ?? 0)) return false;
			return true;
		});
	});
	const hasFilters = $derived(!!(q || fOrg || fStatus || fHolding));
	function clearFilters() {
		q = '';
		fOrg = '';
		fStatus = '';
		fHolding = '';
	}
	const canReadAssets = $derived(can($me, 'asset.read'));

	async function load() {
		loading = true;
		error = '';
		try {
			people = await apiGet<Person[]>('/api/people');
			orgUnits = await apiGet<OrgUnit[]>('/api/org-units');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function showAssets(p: Person) {
		assetsFor = p;
		heldAssets = [];
		assetsLoading = true;
		try {
			const res = await apiGet<Paginated<Asset>>(`/api/assets?assigned_person_id=${p.id}&page_size=100`);
			heldAssets = res.items ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load assets';
		} finally {
			assetsLoading = false;
		}
	}

	function openCreate() {
		editId = null;
		f = {
			first_name: '',
			last_name: '',
			email: '',
			title: '',
			employee_no: '',
			org_unit_id: '',
			manager_id: '',
			is_active: true
		};
		showForm = true;
	}

	function openEdit(p: Person) {
		editId = p.id;
		f = {
			first_name: p.first_name,
			last_name: p.last_name,
			email: p.email ?? '',
			title: p.title ?? '',
			employee_no: p.employee_no ?? '',
			org_unit_id: p.org_unit_id ?? '',
			manager_id: p.manager_id ?? '',
			is_active: p.is_active
		};
		showForm = true;
	}

	async function save(e: Event) {
		e.preventDefault();
		error = '';
		const payload = {
			first_name: f.first_name,
			last_name: f.last_name,
			email: f.email || null,
			title: f.title,
			employee_no: f.employee_no || null,
			org_unit_id: f.org_unit_id || null,
			manager_id: f.manager_id || null,
			is_active: f.is_active
		};
		try {
			if (editId) await apiPut(`/api/people/${editId}`, payload);
			else await apiPost('/api/people', payload);
			showForm = false;
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}

	async function remove(p: Person) {
		if (!confirm(`Delete ${p.first_name} ${p.last_name}?`)) return;
		try {
			await apiDelete(`/api/people/${p.id}`);
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete failed';
		}
	}

	function fullName(id: string | null): string {
		if (!id) return '—';
		const p = byId.get(id);
		return p ? `${p.first_name} ${p.last_name}` : '—';
	}

	const manage = $derived(can($me, 'hierarchy.manage'));
</script>

<div class="topbar">
	<h1>People</h1>
	{#if manage}
		<button class="btn" onclick={openCreate}>+ New person</button>
	{/if}
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if showForm}
	<div class="modal-backdrop" role="presentation">
		<div class="modal" role="dialog" aria-modal="true" aria-label={editId ? 'Edit person' : 'New person'} style="max-width:640px">
		<h3 style="margin-top:0">{editId ? 'Edit person' : 'New person'}</h3>
		<form onsubmit={save}>
			<div class="grid cols-2">
				<div class="field"><label>First name *</label><input bind:value={f.first_name} required /></div>
				<div class="field"><label>Last name *</label><input bind:value={f.last_name} required /></div>
				<div class="field"><label>Email</label><input type="email" bind:value={f.email} /></div>
				<div class="field"><label>Title</label><input bind:value={f.title} /></div>
				<div class="field"><label>Employee no.</label><input bind:value={f.employee_no} /></div>
				<div class="field">
					<label>Org unit</label>
					<select bind:value={f.org_unit_id}>
						<option value="">—</option>
						{#each orgUnits as o}<option value={o.id}>{o.name}</option>{/each}
					</select>
				</div>
				<div class="field">
					<label>Manager (supervisor)</label>
					<select bind:value={f.manager_id}>
						<option value="">—</option>
						{#each people.filter((p) => p.id !== editId) as p}
							<option value={p.id}>{p.first_name} {p.last_name}</option>
						{/each}
					</select>
				</div>
				<div class="field">
					<label>Status</label>
					<select value={String(f.is_active)} onchange={(e) => (f.is_active = e.currentTarget.value === 'true')}>
						<option value="true">Active</option>
						<option value="false">Inactive</option>
					</select>
				</div>
			</div>
			<div class="row">
				<button class="btn" type="submit">Save</button>
				<button class="btn secondary" type="button" onclick={() => (showForm = false)}>Cancel</button>
			</div>
		</form>
		</div>
	</div>
{/if}

<div class="card" style="margin-bottom:12px; padding:12px">
	<div class="filters">
		<div class="field"><label>Search</label><input type="search" placeholder="Name, email, title, employee no…" bind:value={q} /></div>
		<div class="field">
			<label>Org unit</label>
			<select bind:value={fOrg}>
				<option value="">All</option>
				<option value="__none">No org unit</option>
				{#each orgUnits as o}<option value={o.id}>{o.name}</option>{/each}
			</select>
		</div>
		<div class="field">
			<label>Status</label>
			<select bind:value={fStatus}>
				<option value="">Any</option>
				<option value="true">Active</option>
				<option value="false">Inactive</option>
			</select>
		</div>
		<div class="field">
			<label>Assets</label>
			<select bind:value={fHolding}>
				<option value="">Any</option>
				<option value="yes">Holding assets</option>
				<option value="no">Holding none</option>
			</select>
		</div>
		<div class="filter-meta">
			<span class="muted" style="font-size:12px">{filtered.length} of {people.length}</span>
			{#if hasFilters}<button class="btn secondary small" onclick={clearFilters}>Clear</button>{/if}
		</div>
	</div>
</div>

<div class="split" class:open={!!assetsFor}>
<div class="tbl-col">
<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<div class="table-scroll">
		<table>
			<thead>
				<tr><th>Name</th><th>Title</th><th>Org unit</th><th>Manager</th><th>Assets</th><th>Status</th><th></th></tr>
			</thead>
			<tbody>
				{#each filtered as p}
					<tr>
						<td>{p.first_name} {p.last_name}<div class="muted" style="font-size:12px">{p.email || ''}</div></td>
						<td>{p.title || '—'}</td>
						<td>{p.org_unit_id ? (ouById.get(p.org_unit_id)?.name ?? '—') : '—'}</td>
						<td>{fullName(p.manager_id)}</td>
						<td>
							{#if canReadAssets}
								<button class="btn secondary small" onclick={() => showAssets(p)}>
									{p.asset_count ?? 0} held
								</button>
							{:else}
								—
							{/if}
						</td>
						<td>
							<span class="badge" style="border-color:{p.is_active ? 'var(--success)' : 'var(--danger)'}">
								{p.is_active ? 'Active' : 'Inactive'}
							</span>
						</td>
						<td style="text-align:right">
							{#if manage}
								<button class="btn secondary small" onclick={() => openEdit(p)}>Edit</button>
								<button class="btn danger small" onclick={() => remove(p)}>Delete</button>
							{/if}
						</td>
					</tr>
				{/each}
				{#if filtered.length === 0}
					<tr><td colspan="7" class="muted">{people.length === 0 ? 'No people yet.' : 'No people match the filters.'}</td></tr>
				{/if}
			</tbody>
		</table>
		</div>
	{/if}
</div>
</div>
{#if assetsFor}
	<aside class="card side">
		<div class="row" style="justify-content:space-between; align-items:center">
			<h3 style="margin:0">Assets held by {assetsFor.first_name} {assetsFor.last_name}</h3>
			<button class="btn secondary small" onclick={() => (assetsFor = null)}>Close</button>
		</div>
		{#if assetsLoading}
			<p class="muted">Loading…</p>
		{:else if heldAssets.length === 0}
			<p class="muted">No assets currently assigned.</p>
		{:else}
			<div class="side-scroll"><table style="margin-top:12px">
				<thead><tr><th>Tag</th><th>Name</th><th>Type</th><th>State</th></tr></thead>
				<tbody>
					{#each heldAssets as a}
						<tr
							class="click"
							role="button"
							tabindex="0"
							onclick={() => goto(`/assets/${a.id}`)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
						>
							<td>{a.asset_tag}</td>
							<td>{a.name}</td>
							<td>{a.asset_type?.name ?? '—'}</td>
							<td>{a.current_state?.label ?? '—'}</td>
							</tr>
					{/each}
				</tbody>
			</table></div>
		{/if}
	</aside>
{/if}
</div>

<style>
	.table-scroll {
		max-height: calc(100vh - 340px);
		min-height: 240px;
		overflow: auto;
	}
	.table-scroll :global(thead th) {
		position: sticky;
		top: 0;
		background: var(--surface);
		z-index: 1;
	}
	.filters {
		display: grid;
		grid-template-columns: 2fr 1.5fr 1fr 1fr auto;
		gap: 10px;
		align-items: end;
	}
	.filters :global(.field) { margin: 0; }
	.filter-meta { display: flex; gap: 8px; align-items: center; padding-bottom: 8px; white-space: nowrap; }
	.split { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; align-items: start; }
	.split.open { grid-template-columns: minmax(0, 1fr) minmax(340px, 440px); }
	.side { position: sticky; top: 12px; margin: 0; }
	.tbl-col { min-width: 0; }
	.side h3 { font-size: 15px; }
	.side :global(td), .side :global(th) { white-space: nowrap; }
	.side-scroll { max-height: calc(100vh - 340px); overflow: auto; }
	.table-scroll :global(td), .table-scroll :global(th) { padding-top: 6px; padding-bottom: 6px; }
	@media (max-width: 900px) {
		.filters { grid-template-columns: 1fr 1fr; }
		.split.open { grid-template-columns: minmax(0, 1fr); }
	}
</style>
