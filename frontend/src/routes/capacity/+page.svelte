<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet } from '$lib/api';
	import type { CapacityReport } from '$lib/types';

	let report = $state<CapacityReport | null>(null);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			report = await apiGet<CapacityReport>('/api/dcim/capacity');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	});

	function cls(pct: number): string {
		return pct >= 90 ? 'hot' : pct >= 75 ? 'warn' : 'ok';
	}
</script>

<div class="topbar">
	<h1>Capacity</h1>
	<button class="btn secondary" onclick={() => goto('/datacenter')}>Data Center →</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if report}
	{@const t = report.totals}
	<div class="cards">
		<div class="stat"><div class="lbl">Racks</div><div class="val">{t.rack_count}</div></div>
		<div class="stat">
			<div class="lbl">Space (U)</div>
			<div class="val">{t.u_used}/{t.u_total}</div>
			<div class="sub">{t.u_total ? ((t.u_used / t.u_total) * 100).toFixed(0) : 0}% used</div>
		</div>
		<div class="stat">
			<div class="lbl">Power</div>
			<div class="val">{t.power_draw_w.toFixed(0)} W</div>
			<div class="sub">of {t.power_capacity_w} W capacity</div>
		</div>
		<div class="stat">
			<div class="lbl">Weight</div>
			<div class="val">{t.weight_used_kg.toFixed(0)} kg</div>
			<div class="sub">mounted load</div>
		</div>
	</div>

	<div class="card">
		<h3 style="margin-top:0">Per-rack utilization</h3>
		<table>
			<thead>
				<tr><th>Rack</th><th>Location</th><th>Space</th><th>Power</th><th>Weight</th></tr>
			</thead>
			<tbody>
				{#each report.racks as r}
					<tr
						class="clickable"
						role="button"
						tabindex="0"
						onclick={() => goto(`/datacenter/${r.rack_id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/datacenter/${r.rack_id}`))}
					>
						<td><strong>{r.rack_name}</strong></td>
						<td class="muted">{r.location_name || '—'}</td>
						<td>
							<div class="meter"><span class={cls(r.u_pct)} style="width:{Math.min(100, r.u_pct)}%"></span></div>
							<span class="mlabel">{r.u_used}/{r.u_total}U</span>
						</td>
						<td>
							{#if r.power_capacity_w > 0}
								<div class="meter"><span class={cls(r.power_pct)} style="width:{Math.min(100, r.power_pct)}%"></span></div>
								<span class="mlabel">{r.power_draw_w.toFixed(0)}/{r.power_capacity_w}W</span>
							{:else}
								<span class="mlabel muted">{r.power_draw_w.toFixed(0)}W · no cap</span>
							{/if}
						</td>
						<td>
							{#if r.weight_capacity_kg}
								<div class="meter"><span class={cls(r.weight_pct)} style="width:{Math.min(100, r.weight_pct)}%"></span></div>
								<span class="mlabel">{r.weight_used_kg.toFixed(0)}/{r.weight_capacity_kg}kg</span>
							{:else}
								<span class="mlabel muted">{r.weight_used_kg.toFixed(0)}kg</span>
							{/if}
						</td>
					</tr>
				{/each}
				{#if report.racks.length === 0}<tr><td colspan="5" class="muted">No racks yet.</td></tr>{/if}
			</tbody>
		</table>
		<p class="muted small" style="margin-top:8px">Power uses each device's <code>power_watts</code>; weight uses <code>weight_kg</code>. Set rack power/weight budgets on the rack page.</p>
	</div>
{/if}

<style>
	.topbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
	}
	.cards {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 12px;
		margin-bottom: 16px;
	}
	@media (max-width: 720px) {
		.cards {
			grid-template-columns: repeat(2, 1fr);
		}
	}
	.stat {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 14px;
	}
	.stat .lbl {
		font-size: 12px;
		color: var(--muted);
		text-transform: uppercase;
	}
	.stat .val {
		font-size: 24px;
		font-weight: 700;
		margin-top: 4px;
	}
	.stat .sub {
		font-size: 12px;
		color: var(--muted);
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
		vertical-align: middle;
	}
	tr.clickable {
		cursor: pointer;
	}
	tr.clickable:hover td {
		background: var(--surface-2);
	}
	.meter {
		height: 8px;
		background: var(--surface-2);
		border-radius: 5px;
		overflow: hidden;
		min-width: 120px;
	}
	.meter span {
		display: block;
		height: 100%;
		border-radius: 5px;
	}
	.meter span.ok {
		background: var(--success);
	}
	.meter span.warn {
		background: #c98a00;
	}
	.meter span.hot {
		background: var(--danger);
	}
	.mlabel {
		font-size: 11px;
		color: var(--muted);
		font-family: ui-monospace, monospace;
	}
	.small {
		font-size: 12px;
	}
</style>
