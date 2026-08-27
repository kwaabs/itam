<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Person, OrgUnit, Asset, Paginated, CustodyReport } from '$lib/types';

	let people = $state<Person[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let custody = $state<CustodyReport | null>(null);
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
	const heldCount = $derived(new Map((custody?.by_holder ?? []).map((h) => [h.person_id, h.count])));
	const canReadAssets = $derived(can($me, 'asset.read'));

	async function load() {
		loading = true;
		error = '';
		try {
			people = await apiGet<Person[]>('/api/people');
			orgUnits = await apiGet<OrgUnit[]>('/api/org-units');
			if (can($me, 'report.read')) {
				custody = await apiGet<CustodyReport>('/api/reports/custody');
			}
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
	<div class="card" style="margin-bottom:16px">
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
{/if}

{#if assetsFor}
	<div class="card" style="margin-bottom:16px">
		<div class="row" style="justify-content:space-between; align-items:center">
			<h3 style="margin:0">Assets held by {assetsFor.first_name} {assetsFor.last_name}</h3>
			<button class="btn secondary small" onclick={() => (assetsFor = null)}>Close</button>
		</div>
		{#if assetsLoading}
			<p class="muted">Loading…</p>
		{:else if heldAssets.length === 0}
			<p class="muted">No assets currently assigned.</p>
		{:else}
			<table style="margin-top:12px">
				<thead><tr><th>Tag</th><th>Name</th><th>Type</th><th>State</th><th>Location</th></tr></thead>
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
							<td>{a.location?.name ?? '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<table>
			<thead>
				<tr><th>Name</th><th>Title</th><th>Org unit</th><th>Manager</th><th>Assets</th><th>Status</th><th></th></tr>
			</thead>
			<tbody>
				{#each people as p}
					<tr>
						<td>{p.first_name} {p.last_name}<div class="muted" style="font-size:12px">{p.email || ''}</div></td>
						<td>{p.title || '—'}</td>
						<td>{p.org_unit_id ? (ouById.get(p.org_unit_id)?.name ?? '—') : '—'}</td>
						<td>{fullName(p.manager_id)}</td>
						<td>
							{#if canReadAssets}
								<button class="btn secondary small" onclick={() => showAssets(p)}>
									{heldCount.get(p.id) ?? 0} held
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
				{#if people.length === 0}
					<tr><td colspan="7" class="muted">No people yet.</td></tr>
				{/if}
			</tbody>
		</table>
	{/if}
</div>
