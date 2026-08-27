<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost } from '$lib/api';
	import { me, can } from '$lib/me';
	import { currencies, loadCurrencies, defaultCurrency } from '$lib/currencies';
	import type { PurchaseOrder, Vendor, AssetType } from '$lib/types';

	let pos = $state<PurchaseOrder[]>([]);
	let vendors = $state<Vendor[]>([]);
	let assetTypes = $state<AssetType[]>([]);
	let loading = $state(true);
	let error = $state('');

	const manage = $derived(can($me, 'procurement.manage'));
	const selectableTypes = $derived(assetTypes.filter((t) => !t.is_abstract));

	let showPOForm = $state(false);
	let po = $state<{
		po_number: string;
		vendor_id: string;
		currency: string;
		ordered_at: string;
		expected_at: string;
		notes: string;
		lines: { asset_type_id: string; description: string; quantity: number; unit_cost: number; warranty_months: number }[];
	}>({
		po_number: '',
		vendor_id: '',
		currency: 'USD',
		ordered_at: '',
		expected_at: '',
		notes: '',
		lines: [{ asset_type_id: '', description: '', quantity: 1, unit_cost: 0, warranty_months: 0 }]
	});

	async function load() {
		loading = true;
		error = '';
		try {
			pos = (await apiGet<PurchaseOrder[]>('/api/procurement/purchase-orders')) ?? [];
			vendors = (await apiGet<Vendor[]>('/api/procurement/vendors')) ?? [];
			assetTypes = (await apiGet<AssetType[]>('/api/metadata/asset-types')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		load();
		loadCurrencies().then((list) => {
			if (!po.currency || po.currency === 'USD') po.currency = defaultCurrency(list);
		});
	});

	const vendorName = (id: string | null) => vendors.find((v) => v.id === id)?.name ?? '—';

	function addLine() {
		po.lines = [...po.lines, { asset_type_id: '', description: '', quantity: 1, unit_cost: 0, warranty_months: 0 }];
	}
	function removeLine(i: number) {
		po.lines = po.lines.filter((_, idx) => idx !== i);
	}
	const poTotal = $derived(po.lines.reduce((s, l) => s + Number(l.quantity || 0) * Number(l.unit_cost || 0), 0));

	async function savePO(e: Event) {
		e.preventDefault();
		error = '';
		try {
			await apiPost('/api/procurement/purchase-orders', {
				po_number: po.po_number,
				vendor_id: po.vendor_id || null,
				currency: po.currency,
				ordered_at: po.ordered_at || undefined,
				expected_at: po.expected_at || undefined,
				notes: po.notes,
				lines: po.lines
					.filter((l) => l.description)
					.map((l) => ({
						asset_type_id: l.asset_type_id ? Number(l.asset_type_id) : null,
						description: l.description,
						quantity: Number(l.quantity) || 1,
						unit_cost: Number(l.unit_cost) || 0,
						warranty_months: Number(l.warranty_months) || 0
					}))
			});
			showPOForm = false;
			po = {
				po_number: '',
				vendor_id: '',
				currency: 'USD',
				ordered_at: '',
				expected_at: '',
				notes: '',
				lines: [{ asset_type_id: '', description: '', quantity: 1, unit_cost: 0, warranty_months: 0 }]
			};
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}


	function statusColor(s: string): string {
		return (
			{
				draft: '#6b7280',
				approved: '#3b82f6',
				ordered: '#8b5cf6',
				partially_received: '#f59e0b',
				received: '#22c55e',
				closed: '#111827',
				cancelled: '#ef4444'
			}[s] ?? '#6b7280'
		);
	}
</script>

<div class="topbar">
	<h1>Purchase orders</h1>
	{#if manage}
		<button class="btn" onclick={() => (showPOForm = !showPOForm)}>+ New purchase order</button>
	{/if}
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if showPOForm && manage}
		<div class="card" style="margin-bottom:16px">
			<h3 style="margin-top:0">New purchase order</h3>
			<form onsubmit={savePO}>
				<div class="grid cols-2">
					<div class="field"><label>PO number *</label><input bind:value={po.po_number} required /></div>
					<div class="field">
						<label>Vendor</label>
						<select bind:value={po.vendor_id}>
							<option value="">—</option>
							{#each vendors as v}<option value={v.id}>{v.name}</option>{/each}
						</select>
					</div>
					<div class="field"><label>Currency</label>
						<select bind:value={po.currency}>
							{#each $currencies as c}<option value={c.code}>{c.code} — {c.name}</option>{/each}
						</select>
					</div>
					<div class="field"><label>Ordered date</label><input type="date" bind:value={po.ordered_at} /></div>
					<div class="field"><label>Expected date</label><input type="date" bind:value={po.expected_at} /></div>
				</div>

				<label style="margin-top:8px; display:block">Lines</label>
				<table>
					<thead><tr><th>Asset type</th><th>Description</th><th>Qty</th><th>Unit cost</th><th>Warranty (mo)</th><th></th></tr></thead>
					<tbody>
						{#each po.lines as l, i}
							<tr>
								<td>
									<select bind:value={l.asset_type_id}>
										<option value="">—</option>
										{#each selectableTypes as t}<option value={String(t.id)}>{t.name}</option>{/each}
									</select>
								</td>
								<td><input bind:value={l.description} placeholder="e.g. Dell Latitude 5440" /></td>
								<td style="width:80px"><input type="number" min="1" bind:value={l.quantity} /></td>
								<td style="width:120px"><input type="number" step="0.01" bind:value={l.unit_cost} /></td>
								<td style="width:110px"><input type="number" min="0" step="1" bind:value={l.warranty_months} placeholder="0" /></td>
								<td><button class="btn danger small" type="button" onclick={() => removeLine(i)}>✕</button></td>
							</tr>
						{/each}
					</tbody>
				</table>
				<button class="btn secondary small" type="button" onclick={addLine}>+ Add line</button>
				<div style="margin-top:8px"><span class="muted">Estimated total: </span><strong>{poTotal.toFixed(2)} {po.currency}</strong></div>

				<div class="field" style="margin-top:8px"><label>Notes</label><input bind:value={po.notes} /></div>
				<div class="row">
					<button class="btn" type="submit">Create PO</button>
					<button class="btn secondary" type="button" onclick={() => (showPOForm = false)}>Cancel</button>
				</div>
			</form>
		</div>
	{/if}

	<div class="card">
		{#if loading}
			<p class="muted">Loading…</p>
		{:else}
			<table>
				<thead><tr><th>PO #</th><th>Vendor</th><th>Status</th><th>Expected</th><th>Created</th></tr></thead>
				<tbody>
					{#each pos as p}
						<tr
							style="cursor:pointer"
							role="button"
							tabindex="0"
							onclick={() => (window.location.href = `/procurement/${p.id}`)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), (window.location.href = `/procurement/${p.id}`))}
						>
							<td><a href={`/procurement/${p.id}`}>{p.po_number}</a></td>
							<td>{p.vendor?.name ?? vendorName(p.vendor_id)}</td>
							<td><span class="badge" style="border-color:{statusColor(p.status)}">{p.status}</span></td>
							<td class="muted">{p.expected_at ? new Date(p.expected_at).toLocaleDateString() : '—'}</td>
							<td class="muted">{new Date(p.created_at).toLocaleDateString()}</td>
						</tr>
					{/each}
					{#if pos.length === 0}<tr><td colspan="5" class="muted">No purchase orders yet.</td></tr>{/if}
				</tbody>
			</table>
		{/if}
	</div>
