<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { apiGet, apiPost } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { PurchaseOrder, Location } from '$lib/types';

	const id = $page.params.id;

	let po = $state<PurchaseOrder | null>(null);
	let locations = $state<Location[]>([]);
	let error = $state('');
	let busy = $state(false);
	let message = $state('');

	const manage = $derived(can($me, 'procurement.manage'));

	let receiveLoc = $state('');
	let receiveQty = $state<Record<string, number>>({});

	async function load() {
		error = '';
		po = await apiGet<PurchaseOrder>(`/api/procurement/purchase-orders/${id}`);
		receiveLoc = po.location_id ?? '';
		const q: Record<string, number> = {};
		for (const l of po.lines ?? []) q[l.id] = Math.max(0, l.quantity - l.received_qty);
		receiveQty = q;
		if (can($me, 'hierarchy.read')) {
			locations = (await apiGet<Location[]>('/api/locations')) ?? [];
		}
	}

	onMount(load);

	async function act(fn: () => Promise<unknown>, ok = '') {
		error = '';
		message = '';
		busy = true;
		try {
			await fn();
			if (ok) message = ok;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Action failed';
		} finally {
			busy = false;
		}
	}

	const approve = () => act(() => apiPost(`/api/procurement/purchase-orders/${id}/approve`), 'PO approved');
	const cancel = () => {
		if (!confirm('Cancel this purchase order?')) return;
		act(() => apiPost(`/api/procurement/purchase-orders/${id}/cancel`), 'PO cancelled');
	};

	function receive() {
		const lines = Object.entries(receiveQty)
			.filter(([, q]) => Number(q) > 0)
			.map(([po_line_id, q]) => ({ po_line_id, quantity: Number(q) }));
		if (lines.length === 0) {
			error = 'Enter a quantity to receive on at least one line';
			return;
		}
		act(async () => {
			const res = await apiPost<{ created_count: number }>(
				`/api/procurement/purchase-orders/${id}/receive`,
				{ location_id: receiveLoc || null, lines }
			);
			message = `Received — ${res.created_count} asset(s) created`;
		});
	}

	const lineTotal = (q: number, c: number) => (Number(q) * Number(c)).toFixed(2);
	const canReceive = $derived(po && po.status !== 'cancelled' && po.status !== 'closed');
	const outstanding = $derived((po?.lines ?? []).some((l) => l.quantity - l.received_qty > 0));
</script>

{#if !po}
	<p class="muted">Loading…</p>
{:else}
	<div class="topbar">
		<div>
			<h1 style="margin-bottom:4px">{po.po_number}</h1>
			<div class="muted">{po.vendor?.name ?? '—'} · {po.currency}</div>
		</div>
		<div class="row">
			<span class="badge">{po.status}</span>
			{#if manage && po.status === 'draft'}<button class="btn small" onclick={approve} disabled={busy}>Approve</button>{/if}
			{#if manage && po.status !== 'cancelled' && po.status !== 'received'}<button class="btn danger small" onclick={cancel} disabled={busy}>Cancel</button>{/if}
			<a class="btn secondary small" href="/procurement">Back</a>
		</div>
	</div>

	{#if error}<p class="error">{error}</p>{/if}
	{#if message}<p style="color:var(--success)">{message}</p>{/if}

	<div class="grid cols-2" style="margin-bottom:16px">
		<div class="card">
			<h3 style="margin-top:0">Details</h3>
			<div class="kv">
				<div class="k">Vendor</div><div>{po.vendor?.name ?? '—'}</div>
				<div class="k">Status</div><div>{po.status}</div>
				<div class="k">Ordered</div><div>{po.ordered_at ? new Date(po.ordered_at).toLocaleDateString() : '—'}</div>
				<div class="k">Expected</div><div>{po.expected_at ? new Date(po.expected_at).toLocaleDateString() : '—'}</div>
				<div class="k">Deliver to</div><div>{po.location?.name ?? '—'}</div>
				<div class="k">Notes</div><div>{po.notes || '—'}</div>
			</div>
		</div>
	</div>

	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">Lines</h3>
		<table>
			<thead><tr><th>Description</th><th>Type</th><th>Qty</th><th>Received</th><th>Outstanding</th><th>Unit cost</th><th>Warranty</th><th>Line total</th></tr></thead>
			<tbody>
				{#each po.lines ?? [] as l}
					<tr>
						<td>{l.description}</td>
						<td>{l.asset_type?.name ?? '—'}</td>
						<td>{l.quantity}</td>
						<td>{l.received_qty}</td>
						<td>{l.quantity - l.received_qty}</td>
						<td>{Number(l.unit_cost).toFixed(2)}</td>
						<td class="muted">{l.warranty_months ? `${l.warranty_months} mo` : '—'}</td>
						<td>{lineTotal(l.quantity, l.unit_cost)} {po.currency}</td>
					</tr>
				{/each}
				{#if (po.lines ?? []).length === 0}<tr><td colspan="8" class="muted">No lines.</td></tr>{/if}
			</tbody>
		</table>
	</div>

	{#if manage && canReceive && outstanding}
		<div class="card">
			<h3 style="margin-top:0">Receive delivery</h3>
			<p class="muted" style="margin-top:0">Receiving creates one asset per unit in the asset register, in each item's initial lifecycle state, with the purchase cost recorded, the purchase date set to today, and the warranty expiry derived from each line's warranty term.</p>
			<div class="field" style="max-width:320px">
				<label>Deliver into location</label>
				<select bind:value={receiveLoc}>
					<option value="">— (use PO default)</option>
					{#each locations as loc}<option value={loc.id}>{loc.name}</option>{/each}
				</select>
			</div>
			<table>
				<thead><tr><th>Line</th><th>Outstanding</th><th>Receive now</th></tr></thead>
				<tbody>
					{#each po.lines ?? [] as l}
						{#if l.quantity - l.received_qty > 0}
							<tr>
								<td>{l.description}</td>
								<td>{l.quantity - l.received_qty}</td>
								<td style="width:120px">
									<input type="number" min="0" max={l.quantity - l.received_qty} bind:value={receiveQty[l.id]} />
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
			<button class="btn" onclick={receive} disabled={busy}>Receive</button>
		</div>
	{/if}
{/if}
