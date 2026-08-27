<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Vendor, VendorStatus } from '$lib/types';

	let vendors = $state<Vendor[]>([]);
	let statuses = $state<VendorStatus[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state('');

	let qText = $state('');
	let statusFilter = $state('');

	const canManage = $derived(can($me, 'procurement.manage'));

	let showForm = $state(false);
	let editing = $state<Vendor | null>(null);
	let form = $state({
		name: '',
		contact_email: '',
		contact_phone: '',
		website: '',
		notes: '',
		status: 'active'
	});

	async function load() {
		loading = true;
		error = '';
		try {
			[vendors, statuses] = await Promise.all([
				apiGet<Vendor[]>('/api/procurement/vendors?summary=true'),
				apiGet<VendorStatus[]>('/api/metadata/vendor-statuses')
			]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load vendors';
		} finally {
			loading = false;
		}
	}
	onMount(load);

	function statusMeta(key: string): VendorStatus | undefined {
		return statuses.find((s) => s.key === key);
	}
	function money(n: number | undefined): string {
		return (n ?? 0).toLocaleString(undefined, { maximumFractionDigits: 0 });
	}

	const filtered = $derived(
		vendors.filter((v) => {
			if (statusFilter && v.status !== statusFilter) return false;
			if (qText) {
				const s = qText.toLowerCase();
				if (!`${v.name} ${v.contact_email ?? ''} ${v.website ?? ''}`.toLowerCase().includes(s))
					return false;
			}
			return true;
		})
	);

	function openNew() {
		editing = null;
		form = {
			name: '',
			contact_email: '',
			contact_phone: '',
			website: '',
			notes: '',
			status: statuses[0]?.key ?? 'active'
		};
		showForm = true;
	}
	function openEdit(v: Vendor) {
		editing = v;
		form = {
			name: v.name,
			contact_email: v.contact_email ?? '',
			contact_phone: v.contact_phone ?? '',
			website: v.website ?? '',
			notes: v.notes ?? '',
			status: v.status ?? 'active'
		};
		showForm = true;
	}

	async function save() {
		if (!form.name.trim()) {
			error = 'Name is required';
			return;
		}
		busy = 'save';
		error = '';
		try {
			if (editing) {
				await apiPut(`/api/procurement/vendors/${editing.id}`, form);
			} else {
				await apiPost('/api/procurement/vendors', form);
			}
			showForm = false;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
		} finally {
			busy = '';
		}
	}

	async function setStatus(v: Vendor, status: string) {
		busy = v.id;
		error = '';
		try {
			await apiPut(`/api/procurement/vendors/${v.id}`, { ...v, status });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Update failed';
		} finally {
			busy = '';
		}
	}

	async function remove(v: Vendor) {
		if (!confirm(`Delete vendor "${v.name}"? This cannot be undone.`)) return;
		busy = v.id;
		error = '';
		try {
			await apiDelete(`/api/procurement/vendors/${v.id}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		} finally {
			busy = '';
		}
	}
</script>

<div class="topbar">
	<h1>Vendors</h1>
	<span class="muted">Suppliers and manufacturers — statuses are configurable in Admin.</span>
	{#if canManage}
		<button class="btn" style="margin-left:auto" onclick={openNew}>+ New vendor</button>
	{/if}
</div>
{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else}
	<div class="card">
		<div class="row" style="margin-bottom:10px; flex-wrap:wrap; gap:8px">
			<input placeholder="Search name / email / website" bind:value={qText} style="flex:1; min-width:200px" />
			<select bind:value={statusFilter}>
				<option value="">All statuses</option>
				{#each statuses as s}<option value={s.key}>{s.label}</option>{/each}
			</select>
			{#if statusFilter || qText}
				<button class="btn small secondary" onclick={() => { statusFilter = ''; qText = ''; }}>Clear</button>
			{/if}
		</div>
		<table>
			<thead>
				<tr>
					<th>Name</th><th>Contact</th><th>Status</th>
					<th style="text-align:right">POs</th><th style="text-align:right">Spend</th>
					<th style="text-align:right">Assets</th>
					{#if canManage}<th></th>{/if}
				</tr>
			</thead>
			<tbody>
				{#each filtered as v}
					{@const sm = statusMeta(v.status)}
					<tr>
						<td>
							<strong>{v.name}</strong>
							{#if v.website}<div><a class="muted small" href={v.website} target="_blank" rel="noreferrer">{v.website}</a></div>{/if}
						</td>
						<td class="muted small">
							{#if v.contact_email}<div>{v.contact_email}</div>{/if}
							{#if v.contact_phone}<div>{v.contact_phone}</div>{/if}
							{#if !v.contact_email && !v.contact_phone}—{/if}
						</td>
						<td>
							<span class="badge" style="--c:{sm?.color || '#6b7280'}">{sm?.label ?? v.status}</span>
						</td>
						<td style="text-align:right">{v.po_count ?? 0}</td>
						<td style="text-align:right">{money(v.total_spent)}</td>
						<td style="text-align:right">{v.asset_count ?? 0}</td>
						{#if canManage}
							<td class="actions">
								<button class="btn small secondary" disabled={busy === v.id} onclick={() => openEdit(v)}>Edit</button>
								{#if v.status === 'blacklisted'}
									<button class="btn small" disabled={busy === v.id} onclick={() => setStatus(v, 'active')}>Activate</button>
								{:else}
									<button class="btn small warn" disabled={busy === v.id} onclick={() => setStatus(v, 'blacklisted')}>Blacklist</button>
								{/if}
								<button class="btn small danger" disabled={busy === v.id} onclick={() => remove(v)}>Delete</button>
							</td>
						{/if}
					</tr>
				{/each}
				{#if filtered.length === 0}
					<tr><td colspan={canManage ? 7 : 6} class="muted">No vendors match.</td></tr>
				{/if}
			</tbody>
		</table>
	</div>
{/if}

{#if showForm}
	<div
		class="backdrop"
		role="button"
		tabindex="0"
		aria-label="Close"
		onclick={(e) => { if (e.target === e.currentTarget) showForm = false; }}
		onkeydown={(e) => e.key === 'Escape' && (showForm = false)}
	>
		<aside class="drawer">
			<div class="drawer-head">
				<h3>{editing ? 'Edit vendor' : 'New vendor'}</h3>
				<button class="btn small secondary" onclick={() => (showForm = false)}>Close</button>
			</div>
			<label>Name<input bind:value={form.name} placeholder="Acme Corp" /></label>
			<label>Status
				<select bind:value={form.status}>
					{#each statuses as s}<option value={s.key}>{s.label}</option>{/each}
				</select>
			</label>
			<label>Contact email<input bind:value={form.contact_email} type="email" /></label>
			<label>Contact phone<input bind:value={form.contact_phone} /></label>
			<label>Website<input bind:value={form.website} placeholder="https://…" /></label>
			<label>Notes<textarea bind:value={form.notes} rows="3"></textarea></label>
			<div class="drawer-actions">
				<button class="btn" disabled={busy === 'save'} onclick={save}>{busy === 'save' ? 'Saving…' : 'Save'}</button>
			</div>
		</aside>
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
		border-bottom: 1px solid var(--border);
		vertical-align: top;
	}
	.small {
		font-size: 12px;
	}
	.badge {
		display: inline-block;
		padding: 2px 8px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		color: var(--c);
		background: color-mix(in srgb, var(--c) 16%, transparent);
	}
	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
		flex-wrap: wrap;
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
	.btn.warn {
		background: #f59e0b;
		color: #1a1a1a;
	}
	.btn.danger {
		background: var(--danger);
		color: #fff;
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
		width: min(420px, 100%);
		height: 100%;
		background: var(--surface);
		border-left: 1px solid var(--border);
		padding: 18px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.drawer-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.drawer-head h3 {
		margin: 0;
	}
	.drawer label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 13px;
		color: var(--muted);
	}
	.drawer input,
	.drawer select,
	.drawer textarea {
		font: inherit;
		color: var(--text);
	}
	.drawer-actions {
		margin-top: auto;
	}
</style>
