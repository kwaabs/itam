<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import { currencies, loadCurrencies, defaultCurrency } from '$lib/currencies';
	import type {
		Software,
		SoftwareVersion,
		Installation,
		License,
		LicenseAssignment,
		Person,
		Asset,
		Paginated
	} from '$lib/types';

	let tab = $state<'catalog' | 'licenses'>('catalog');
	let software = $state<Software[]>([]);
	let licenses = $state<License[]>([]);
	let loading = $state(true);
	let error = $state('');
	const manage = $derived(can($me, 'software.manage'));

	// catalog drawer
	let selSw = $state<Software | null>(null);
	let swVersions = $state<SoftwareVersion[]>([]);
	let swInstalls = $state<Installation[]>([]);
	let verForm = $state({ version: '' });

	// license drawer
	let selLic = $state<License | null>(null);
	let licAsg = $state<LicenseAssignment[]>([]);
	let people = $state<Person[]>([]);
	let assets = $state<Asset[]>([]);
	let asgForm = $state({ kind: 'asset', asset_id: '', person_id: '', notes: '' });

	let showSwForm = $state(false);
	let swForm = $state({ name: '', publisher: '', category: 'application', description: '' });
	let showLicForm = $state(false);
	let licForm = $state({ name: '', software_id: '', license_type: 'subscription', seats: '', expiry_date: '', purchase_cost: '', currency: 'USD' });
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			software = (await apiGet<Software[]>('/api/software')) ?? [];
			licenses = (await apiGet<License[]>('/api/software/licenses')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}
	onMount(() => {
		load();
		loadCurrencies().then((list) => {
			if (!licForm.currency || licForm.currency === 'USD') licForm.currency = defaultCurrency(list);
		});
	});

	// ---- compliance helpers ----
	function over(l: License): boolean {
		return !!l.seats && l.seats > 0 && (l.seats_used ?? 0) > l.seats;
	}
	function daysTo(date?: string | null): number | null {
		if (!date) return null;
		return Math.ceil((new Date(date).getTime() - Date.now()) / 86400000);
	}
	function expiryClass(l: License): string {
		const d = daysTo(l.expiry_date);
		if (d === null) return '';
		if (d < 0) return 'bad';
		if (d <= 30) return 'warn';
		return '';
	}
	const problems = $derived(
		licenses.filter((l) => over(l) || (daysTo(l.expiry_date) !== null && daysTo(l.expiry_date)! <= 30))
	);

	// ---- catalog ----
	async function openSw(s: Software) {
		selSw = s;
		verForm = { version: '' };
		try {
			const d = await apiGet<{ software: Software; installations: Installation[] }>(`/api/software/${s.id}`);
			swVersions = d.software.versions ?? [];
			swInstalls = d.installations ?? [];
		} catch {
			swVersions = [];
			swInstalls = [];
		}
	}
	async function createSw() {
		if (!swForm.name) return;
		saving = true;
		try {
			await apiPost('/api/software', swForm);
			showSwForm = false;
			swForm = { name: '', publisher: '', category: 'application', description: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}
	async function deleteSw(s: Software) {
		if (!confirm(`Delete ${s.name}? Installs and version records will be removed.`)) return;
		try {
			await apiDelete(`/api/software/${s.id}`);
			if (selSw?.id === s.id) selSw = null;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}
	async function addVersion() {
		if (!selSw || !verForm.version) return;
		saving = true;
		try {
			await apiPost(`/api/software/${selSw.id}/versions`, { version: verForm.version });
			await openSw(selSw);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}
	async function delVersion(v: SoftwareVersion) {
		try {
			await apiDelete(`/api/software/versions/${v.id}`);
			if (selSw) await openSw(selSw);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	// ---- licenses ----
	async function openLic(l: License) {
		selLic = l;
		asgForm = { kind: 'asset', asset_id: '', person_id: '', notes: '' };
		try {
			const d = await apiGet<{ assignments: LicenseAssignment[] }>(`/api/software/licenses/${l.id}`);
			licAsg = d.assignments ?? [];
		} catch {
			licAsg = [];
		}
		if (people.length === 0) people = (await apiGet<Person[]>('/api/people')) ?? [];
		if (assets.length === 0) {
			const p = await apiGet<Paginated<Asset>>('/api/assets?page_size=200');
			assets = p.items ?? [];
		}
	}
	async function createLic() {
		if (!licForm.name) return;
		saving = true;
		try {
			await apiPost('/api/software/licenses', {
				name: licForm.name,
				software_id: licForm.software_id || null,
				license_type: licForm.license_type,
				seats: licForm.seats !== '' ? Number(licForm.seats) : null,
				expiry_date: licForm.expiry_date || null,
				purchase_cost: licForm.purchase_cost !== '' ? Number(licForm.purchase_cost) : null,
				currency: licForm.currency || undefined
			});
			showLicForm = false;
			licForm = { name: '', software_id: '', license_type: 'subscription', seats: '', expiry_date: '', purchase_cost: '', currency: licForm.currency };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}
	async function deleteLic(l: License) {
		if (!confirm(`Delete license ${l.name}?`)) return;
		try {
			await apiDelete(`/api/software/licenses/${l.id}`);
			if (selLic?.id === l.id) selLic = null;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}
	async function refreshLic() {
		if (selLic) {
			await load();
			const fresh = licenses.find((x) => x.id === selLic!.id) ?? selLic;
			await openLic(fresh);
		}
	}
	async function assignSeat() {
		if (!selLic) return;
		if (asgForm.kind === 'asset' && !asgForm.asset_id) return;
		if (asgForm.kind === 'person' && !asgForm.person_id) return;
		saving = true;
		try {
			await apiPost(`/api/software/licenses/${selLic.id}/assignments`, {
				asset_id: asgForm.kind === 'asset' ? asgForm.asset_id : null,
				person_id: asgForm.kind === 'person' ? asgForm.person_id : null,
				notes: asgForm.notes || null
			});
			asgForm = { kind: asgForm.kind, asset_id: '', person_id: '', notes: '' };
			await refreshLic();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		} finally {
			saving = false;
		}
	}
	async function unassign(a: LicenseAssignment) {
		try {
			await apiDelete(`/api/software/license-assignments/${a.id}`);
			await refreshLic();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	function seatLabel(l: License): string {
		const used = l.seats_used ?? 0;
		return l.seats && l.seats > 0 ? `${used} / ${l.seats}` : `${used} / ∞`;
	}
</script>

<div class="topbar">
	<h1>Software</h1>
	<div class="row">
		<div class="seg">
			<button class="seg-btn" class:active={tab === 'catalog'} onclick={() => (tab = 'catalog')}>Catalog</button>
			<button class="seg-btn" class:active={tab === 'licenses'} onclick={() => (tab = 'licenses')}>Licenses</button>
		</div>
		{#if manage}
			{#if tab === 'catalog'}
				<button class="btn" onclick={() => (showSwForm = !showSwForm)}>+ Software</button>
			{:else}
				<button class="btn" onclick={() => (showLicForm = !showLicForm)}>+ License</button>
			{/if}
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if tab === 'licenses' && problems.length}
	<div class="card banner" style="margin-bottom:16px">
		<strong>{problems.length} license(s) need attention</strong>
		<span class="muted"> — over-allocated seats or expiring within 30 days.</span>
	</div>
{/if}

{#if showSwForm && tab === 'catalog'}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">New software</h3>
		<div class="grid cols-3">
			<label class="field">Name <input bind:value={swForm.name} placeholder="Adobe Photoshop" /></label>
			<label class="field">Publisher <input bind:value={swForm.publisher} placeholder="Adobe" /></label>
			<label class="field">
				Category
				<select bind:value={swForm.category}>
					<option value="application">application</option>
					<option value="os">os</option>
					<option value="driver">driver</option>
					<option value="utility">utility</option>
					<option value="firmware">firmware</option>
				</select>
			</label>
		</div>
		<label class="field">Description <input bind:value={swForm.description} /></label>
		<button class="btn" onclick={createSw} disabled={saving}>{saving ? 'Saving…' : 'Create'}</button>
	</div>
{/if}

{#if showLicForm && tab === 'licenses'}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">New license</h3>
		<div class="grid cols-3">
			<label class="field">Name <input bind:value={licForm.name} placeholder="M365 E3" /></label>
			<label class="field">
				Software
				<select bind:value={licForm.software_id}>
					<option value="">—</option>
					{#each software as s}<option value={s.id}>{s.name}</option>{/each}
				</select>
			</label>
			<label class="field">
				Type
				<select bind:value={licForm.license_type}>
					<option value="subscription">subscription</option>
					<option value="perpetual">perpetual</option>
					<option value="volume">volume</option>
					<option value="oem">oem</option>
					<option value="open_source">open_source</option>
				</select>
			</label>
			<label class="field">Seats <input type="number" min="0" bind:value={licForm.seats} placeholder="blank = unlimited" /></label>
			<label class="field">Expiry <input type="date" bind:value={licForm.expiry_date} /></label>
			<label class="field">Cost <input type="number" step="0.01" bind:value={licForm.purchase_cost} /></label>
			<label class="field">Currency
				<select bind:value={licForm.currency}>
					{#each $currencies as c}<option value={c.code}>{c.code} — {c.name}</option>{/each}
				</select>
			</label>
		</div>
		<button class="btn" onclick={createLic} disabled={saving}>{saving ? 'Saving…' : 'Create'}</button>
	</div>
{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if tab === 'catalog'}
		<table>
			<thead><tr><th>Name</th><th>Publisher</th><th>Category</th><th>Installs</th><th>Licenses</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each software as s}
					<tr
						role="button"
						tabindex="0"
						onclick={() => openSw(s)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openSw(s))}
					>
						<td><strong>{s.name}</strong></td>
						<td class="muted">{s.publisher || '—'}</td>
						<td><span class="badge">{s.category}</span></td>
						<td>{s.install_count ?? 0}</td>
						<td>{s.license_count ?? 0}</td>
						{#if manage}<td style="text-align:right"><button class="btn danger small" onclick={(e) => { e.stopPropagation(); deleteSw(s); }}>Delete</button></td>{/if}
					</tr>
				{/each}
				{#if software.length === 0}<tr><td colspan="6" class="muted">No software yet.</td></tr>{/if}
			</tbody>
		</table>
	{:else}
		<table>
			<thead><tr><th>License</th><th>Software</th><th>Type</th><th>Seats</th><th>Expiry</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each licenses as l}
					<tr
						role="button"
						tabindex="0"
						onclick={() => openLic(l)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openLic(l))}
					>
						<td><strong>{l.name}</strong></td>
						<td class="muted">{l.software?.name ?? '—'}</td>
						<td><span class="badge">{l.license_type}</span></td>
						<td><span class:seat-over={over(l)}>{seatLabel(l)}</span></td>
						<td class={expiryClass(l)}>{l.expiry_date ? new Date(l.expiry_date).toLocaleDateString() : '—'}</td>
						{#if manage}<td style="text-align:right"><button class="btn danger small" onclick={(e) => { e.stopPropagation(); deleteLic(l); }}>Delete</button></td>{/if}
					</tr>
				{/each}
				{#if licenses.length === 0}<tr><td colspan="6" class="muted">No licenses yet.</td></tr>{/if}
			</tbody>
		</table>
	{/if}
</div>

{#if selSw}
	<div class="drawer-backdrop" role="button" tabindex="0" aria-label="Close panel" onclick={() => (selSw = null)} onkeydown={(e) => e.key === 'Escape' && (selSw = null)}></div>
	<aside class="drawer">
		<button class="btn secondary small drawer-close" onclick={() => (selSw = null)}>Close</button>
		<h2>{selSw.name}</h2>
		<div class="muted" style="margin-bottom:14px">{selSw.publisher || ''} · {selSw.category}</div>

		<h3 style="margin-bottom:6px">Versions</h3>
		<div class="chips">
			{#each swVersions as v}
				<span class="chip">{v.version}{#if manage}<button class="chip-x" onclick={() => delVersion(v)}>×</button>{/if}</span>
			{/each}
			{#if swVersions.length === 0}<span class="muted">None</span>{/if}
		</div>
		{#if manage}
			<div class="row" style="margin-top:8px">
				<input bind:value={verForm.version} placeholder="e.g. 24.1" />
				<button class="btn small" onclick={addVersion} disabled={saving || !verForm.version}>Add</button>
			</div>
		{/if}

		<h3 style="margin-bottom:6px; margin-top:18px">Installed on ({swInstalls.length})</h3>
		<table>
			<tbody>
				{#each swInstalls as inst}
					<tr
						role="button"
						tabindex="0"
						onclick={() => inst.asset && goto(`/assets/${inst.asset_id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), inst.asset && goto(`/assets/${inst.asset_id}`))}
					>
						<td>{inst.asset?.name ?? inst.asset_id}</td>
						<td class="muted" style="text-align:right">{inst.version?.version ?? ''} · {inst.source}</td>
					</tr>
				{/each}
				{#if swInstalls.length === 0}<tr><td class="muted">Not installed anywhere.</td></tr>{/if}
			</tbody>
		</table>
	</aside>
{/if}

{#if selLic}
	<div class="drawer-backdrop" role="button" tabindex="0" aria-label="Close panel" onclick={() => (selLic = null)} onkeydown={(e) => e.key === 'Escape' && (selLic = null)}></div>
	<aside class="drawer">
		<button class="btn secondary small drawer-close" onclick={() => (selLic = null)}>Close</button>
		<h2>{selLic.name}</h2>
		<div class="row" style="gap:8px; margin-bottom:14px">
			<span class="badge">{selLic.license_type}</span>
			<span class="badge" class:seat-over={over(selLic)}>Seats {seatLabel(selLic)}</span>
			{#if selLic.expiry_date}<span class="badge {expiryClass(selLic)}">Expires {new Date(selLic.expiry_date).toLocaleDateString()}</span>{/if}
		</div>
		<div class="kv">
			<div class="k">Software</div><div>{selLic.software?.name ?? '—'}</div>
			<div class="k">Key</div><div class="muted">{selLic.license_key || '—'}</div>
			<div class="k">Cost</div><div>{selLic.purchase_cost != null ? `${selLic.purchase_cost} ${selLic.currency}` : '—'}</div>
		</div>

		<h3 style="margin-bottom:6px; margin-top:14px">Assignments ({licAsg.length})</h3>
		<table>
			<tbody>
				{#each licAsg as a}
					<tr>
						<td>
							{#if a.asset}{a.asset.name}{:else if a.person}{a.person.first_name} {a.person.last_name}{:else}—{/if}
						</td>
						<td class="muted" style="text-align:right">
							{#if manage}<button class="btn secondary small" onclick={() => unassign(a)}>Remove</button>{/if}
						</td>
					</tr>
				{/each}
				{#if licAsg.length === 0}<tr><td class="muted">No seats assigned.</td></tr>{/if}
			</tbody>
		</table>

		{#if manage}
			<h3 style="margin-bottom:6px; margin-top:16px">Assign a seat</h3>
			<div class="seg" style="margin-bottom:8px">
				<button class="seg-btn" class:active={asgForm.kind === 'asset'} onclick={() => (asgForm.kind = 'asset')}>To asset</button>
				<button class="seg-btn" class:active={asgForm.kind === 'person'} onclick={() => (asgForm.kind = 'person')}>To person</button>
			</div>
			{#if asgForm.kind === 'asset'}
				<select bind:value={asgForm.asset_id}>
					<option value="">Select asset…</option>
					{#each assets as a}<option value={a.id}>{a.name} ({a.asset_tag})</option>{/each}
				</select>
			{:else}
				<select bind:value={asgForm.person_id}>
					<option value="">Select person…</option>
					{#each people as p}<option value={p.id}>{p.first_name} {p.last_name}</option>{/each}
				</select>
			{/if}
			<button class="btn small" style="margin-top:8px" onclick={assignSeat} disabled={saving}>Assign</button>
		{/if}
	</aside>
{/if}

<style>
	.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
	.seg-btn { background: transparent; color: var(--muted); border: none; padding: 6px 14px; cursor: pointer; font: inherit; }
	.seg-btn.active { background: var(--primary); color: #fff; font-weight: 600; }
	.banner { border-left: 3px solid var(--warning); }
	.warn { color: var(--warning); font-weight: 600; }
	.bad { color: var(--danger); font-weight: 600; }
	.seat-over { color: var(--danger); font-weight: 700; }
	.chips { display: flex; flex-wrap: wrap; gap: 6px; }
	.chip { background: var(--surface-2); border: 1px solid var(--border); border-radius: 999px; padding: 3px 10px; font-size: 13px; display: inline-flex; align-items: center; gap: 4px; }
	.chip-x { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 14px; padding: 0; line-height: 1; }
</style>
