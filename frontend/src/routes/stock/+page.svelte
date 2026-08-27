<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Asset, Paginated } from '$lib/types';

	let items = $state<Asset[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state('');

	let qText = $state('');
	let typeFilter = $state('');
	let locFilter = $state('');

	const canDeploy = $derived(can($me, 'asset.transition'));

	async function load() {
		loading = true;
		error = '';
		try {
			items = (await apiGet<Paginated<Asset>>('/api/assets?state=in_stock&page_size=200')).items ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load stock';
		} finally {
			loading = false;
		}
	}
	onMount(load);

	function typeName(a: Asset): string {
		return a.asset_type?.name ?? '—';
	}
	function locName(a: Asset): string {
		return a.location?.name ?? 'Unassigned';
	}
	function daysIn(a: Asset): number {
		const t = a.created_at ? new Date(a.created_at).getTime() : 0;
		if (!t) return 0;
		return Math.max(0, Math.floor((Date.now() - t) / 86400000));
	}

	const filtered = $derived(
		items.filter((a) => {
			if (typeFilter && typeName(a) !== typeFilter) return false;
			if (locFilter && locName(a) !== locFilter) return false;
			if (qText) {
				const s = qText.toLowerCase();
				if (!`${a.asset_tag} ${a.name} ${a.serial ?? ''}`.toLowerCase().includes(s)) return false;
			}
			return true;
		})
	);

	function groupCounts(getter: (a: Asset) => string) {
		const m = new Map<string, number>();
		for (const a of items) {
			const k = getter(a);
			m.set(k, (m.get(k) ?? 0) + 1);
		}
		return [...m.entries()].sort((x, y) => y[1] - x[1]);
	}
	const byType = $derived(groupCounts(typeName));
	const byLoc = $derived(groupCounts(locName));
	const typeOptions = $derived(byType.map(([k]) => k));
	const locOptions = $derived(byLoc.map(([k]) => k));
	const maxType = $derived(Math.max(1, ...byType.map(([, n]) => n)));
	const maxLoc = $derived(Math.max(1, ...byLoc.map(([, n]) => n)));
	const aging = $derived(items.filter((a) => daysIn(a) >= 90).length);

	async function deploy(a: Asset, ev: Event) {
		ev.stopPropagation();
		if (busy) return;
		if (!confirm(`Deploy ${a.asset_tag} · ${a.name}? This moves it out of stock (In Stock → Deployed).`)) return;
		busy = a.id;
		try {
			await apiPost(`/api/assets/${a.id}/transition`, { transition_key: 'deploy' });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Deploy failed';
		} finally {
			busy = '';
		}
	}
</script>

<div class="topbar"><h1>Stock</h1><span class="muted">Assets currently <strong>In Stock</strong> — available to deploy or assign.</span></div>
{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else}
	<div class="stats">
		<div class="stat"><div class="lbl">In stock</div><div class="val">{items.length}</div></div>
		<div class="stat"><div class="lbl">Asset types</div><div class="val">{byType.length}</div></div>
		<div class="stat"><div class="lbl">Locations</div><div class="val">{byLoc.length}</div></div>
		<div class="stat"><div class="lbl">Aging ≥ 90d</div><div class="val" class:warn={aging > 0}>{aging}</div></div>
	</div>

	<div class="grid cols-2">
		<div class="card">
			<h3>By asset type</h3>
			{#each byType as [k, n]}
				<button class="bar" class:active={typeFilter === k} onclick={() => (typeFilter = typeFilter === k ? '' : k)}>
					<span class="bk">{k}</span>
					<span class="bt"><span class="fill" style="width:{(n / maxType) * 100}%"></span></span>
					<span class="bn">{n}</span>
				</button>
			{/each}
			{#if byType.length === 0}<p class="muted">Nothing in stock.</p>{/if}
		</div>
		<div class="card">
			<h3>By location</h3>
			{#each byLoc as [k, n]}
				<button class="bar" class:active={locFilter === k} onclick={() => (locFilter = locFilter === k ? '' : k)}>
					<span class="bk">{k}</span>
					<span class="bt"><span class="fill loc" style="width:{(n / maxLoc) * 100}%"></span></span>
					<span class="bn">{n}</span>
				</button>
			{/each}
			{#if byLoc.length === 0}<p class="muted">Nothing in stock.</p>{/if}
		</div>
	</div>

	<div class="card">
		<div class="row" style="margin-bottom:10px; flex-wrap:wrap; gap:8px">
			<input placeholder="Search tag / name / serial" bind:value={qText} style="flex:1; min-width:180px" />
			<select bind:value={typeFilter}>
				<option value="">All types</option>
				{#each typeOptions as t}<option value={t}>{t}</option>{/each}
			</select>
			<select bind:value={locFilter}>
				<option value="">All locations</option>
				{#each locOptions as l}<option value={l}>{l}</option>{/each}
			</select>
			{#if typeFilter || locFilter || qText}
				<button class="btn small secondary" onclick={() => { typeFilter = ''; locFilter = ''; qText = ''; }}>Clear</button>
			{/if}
		</div>
		<table>
			<thead>
				<tr><th>Tag</th><th>Name</th><th>Type</th><th>Location</th><th>Vendor</th><th>In stock</th>{#if canDeploy}<th></th>{/if}</tr>
			</thead>
			<tbody>
				{#each filtered as a}
					<tr
						class="clickable"
						role="button"
						tabindex="0"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && goto(`/assets/${a.id}`)}
					>
						<td class="mono">{a.asset_tag}</td>
						<td><strong>{a.name}</strong></td>
						<td class="muted">{typeName(a)}</td>
						<td class="muted">{locName(a)}</td>
						<td class="muted">{a.vendor || '—'}</td>
						<td class:warn={daysIn(a) >= 90}>{daysIn(a)}d</td>
						{#if canDeploy}
							<td style="text-align:right">
								<button class="btn small" disabled={busy === a.id} onclick={(e) => deploy(a, e)}>
									{busy === a.id ? '…' : 'Deploy'}
								</button>
							</td>
						{/if}
					</tr>
				{/each}
				{#if filtered.length === 0}
					<tr><td colspan={canDeploy ? 7 : 6} class="muted">No assets match.</td></tr>
				{/if}
			</tbody>
		</table>
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
	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 12px;
		margin-bottom: 16px;
	}
	@media (max-width: 720px) {
		.stats {
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
		font-size: 26px;
		font-weight: 700;
		margin-top: 4px;
	}
	.val.warn,
	td.warn {
		color: var(--danger);
	}
	.grid.cols-2 {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
		margin-bottom: 16px;
	}
	@media (max-width: 860px) {
		.grid.cols-2 {
			grid-template-columns: 1fr;
		}
	}
	.card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px;
		margin-bottom: 16px;
	}
	.card h3 {
		margin: 0 0 10px;
		font-size: 14px;
	}
	.bar {
		display: grid;
		grid-template-columns: 130px 1fr 36px;
		align-items: center;
		gap: 8px;
		width: 100%;
		background: none;
		border: none;
		padding: 4px 2px;
		cursor: pointer;
		font: inherit;
		color: inherit;
		text-align: left;
		border-radius: 6px;
	}
	.bar:hover {
		background: var(--surface-2);
	}
	.bar.active {
		background: color-mix(in srgb, var(--accent, #3b82f6) 16%, transparent);
	}
	.bk {
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.bt {
		background: var(--surface-2);
		height: 10px;
		border-radius: 5px;
		overflow: hidden;
	}
	.fill {
		display: block;
		height: 100%;
		background: #8b5cf6;
	}
	.fill.loc {
		background: #06b6d4;
	}
	.bn {
		font-size: 12px;
		text-align: right;
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
	}
	tr.clickable {
		cursor: pointer;
	}
	tr.clickable:hover {
		background: var(--surface-2);
	}
	.mono {
		font-family: ui-monospace, monospace;
		font-size: 12px;
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
</style>
