<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { CostAnalytics, AssetCostRow, BudgetRow, DepreciationRow, AssetType, OrgUnit } from '$lib/types';

	let tab = $state<'overview' | 'assets' | 'budgets' | 'depreciation'>('overview');
	let error = $state('');
	let loading = $state(true);

	const manage = $derived(can($me, 'cost.manage'));

	// Overview
	let analytics = $state<CostAnalytics | null>(null);

	// Assets TCO
	let assetRows = $state<AssetCostRow[]>([]);
	let assetSort = $state<'net_cost' | 'purchase_cost' | 'repair_cost' | 'book_value'>('net_cost');
	let assetTypeId = $state('');
	let assetTypes = $state<AssetType[]>([]);

	// Budgets
	let budgetYear = $state(new Date().getFullYear());
	let budgetRows = $state<BudgetRow[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let newBudget = $state({ org_unit_id: '', amount: 0 });

	// Depreciation
	let depRows = $state<DepreciationRow[]>([]);
	let depEdits = $state<Record<number, number>>({});

	const ccy = $derived(analytics?.currency ?? 'USD');

	function money(n: number, c = ccy): string {
		return `${(n ?? 0).toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })} ${c}`;
	}
	function money2(n: number, c = ccy): string {
		return `${(n ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${c}`;
	}
	function maxAmt(list: { amount: number }[]): number {
		return list.reduce((m, x) => Math.max(m, Math.abs(x.amount)), 0) || 1;
	}
	function depth(path: string): number {
		return Math.max(0, (path?.split('.').length ?? 1) - 1);
	}

	async function loadOverview() {
		analytics = await apiGet<CostAnalytics>('/api/costs/summary');
	}
	async function loadAssets() {
		const q = new URLSearchParams({ sort: assetSort, limit: '100' });
		if (assetTypeId) q.set('type_id', assetTypeId);
		assetRows = (await apiGet<AssetCostRow[]>(`/api/costs/assets?${q}`)) ?? [];
	}
	async function loadBudgets() {
		const res = await apiGet<{ year: number; rows: BudgetRow[] }>(`/api/costs/budgets?year=${budgetYear}`);
		budgetRows = res?.rows ?? [];
	}
	async function loadDepreciation() {
		depRows = (await apiGet<DepreciationRow[]>('/api/costs/depreciation')) ?? [];
	}

	onMount(async () => {
		try {
			await loadOverview();
			assetTypes = (await apiGet<AssetType[]>('/api/metadata/asset-types')) ?? [];
			if (can($me, 'hierarchy.read')) orgUnits = (await apiGet<OrgUnit[]>('/api/org-units')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load cost data';
		} finally {
			loading = false;
		}
	});

	// Lazy-load per tab on first visit.
	let loaded = $state<Record<string, boolean>>({ overview: true });
	async function go(t: typeof tab) {
		tab = t;
		if (loaded[t]) return;
		error = '';
		try {
			if (t === 'assets') await loadAssets();
			else if (t === 'budgets') await loadBudgets();
			else if (t === 'depreciation') await loadDepreciation();
			loaded[t] = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	}

	async function applyAssetFilters() {
		try {
			await loadAssets();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	}

	const selectableTypes = $derived(assetTypes.filter((t) => !t.is_abstract));

	async function saveBudget() {
		if (!newBudget.org_unit_id || !Number(newBudget.amount)) {
			error = 'Pick an org unit and enter an amount';
			return;
		}
		error = '';
		try {
			await apiPost('/api/costs/budgets', {
				org_unit_id: newBudget.org_unit_id,
				period_year: budgetYear,
				amount: Number(newBudget.amount)
			});
			newBudget = { org_unit_id: '', amount: 0 };
			await loadBudgets();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
		}
	}
	async function deleteBudget(id: string) {
		if (!id || !confirm('Remove this budget?')) return;
		try {
			await apiDelete(`/api/costs/budgets/${id}`);
			await loadBudgets();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}

	async function saveDep(row: DepreciationRow) {
		const months = depEdits[row.id] ?? row.useful_life_months;
		try {
			await apiPut(`/api/costs/depreciation/${row.id}`, { useful_life_months: Number(months) || 0 });
			row.useful_life_months = Number(months) || 0;
			delete depEdits[row.id];
			depEdits = { ...depEdits };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Save failed';
		}
	}

	const years = (() => {
		const y = new Date().getFullYear();
		return [y + 1, y, y - 1, y - 2];
	})();

	const composition = $derived(
		analytics
			? [
					{ label: 'Purchases', amount: analytics.totals.capex },
					{ label: 'Repairs', amount: analytics.totals.repair },
					{ label: 'Upgrades', amount: analytics.totals.upgrade },
					{ label: 'Other', amount: analytics.totals.other }
				]
			: []
	);

	function variance(b: BudgetRow): number {
		return b.budget - b.actual;
	}
	function usedPct(b: BudgetRow): number {
		if (!b.budget) return b.actual > 0 ? 100 : 0;
		return Math.min(100, Math.round((b.actual / b.budget) * 100));
	}
</script>

<div class="topbar">
	<h1>Costs</h1>
</div>

{#if error}<p class="error">{error}</p>{/if}

<div class="tabs">
	<button type="button" class="tab" class:active={tab === 'overview'} onclick={() => go('overview')}>Overview</button>
	<button type="button" class="tab" class:active={tab === 'assets'} onclick={() => go('assets')}>Asset TCO</button>
	<button type="button" class="tab" class:active={tab === 'budgets'} onclick={() => go('budgets')}>Budgets</button>
	<button type="button" class="tab" class:active={tab === 'depreciation'} onclick={() => go('depreciation')}>Depreciation</button>
</div>

{#if loading}
	<p class="muted">Loading…</p>
{:else if tab === 'overview' && analytics}
	<div class="stats">
		<div class="stat">
			<div class="stat-label">Net lifetime cost (TCO)</div>
			<div class="stat-value">{money(analytics.totals.net)}</div>
			<div class="stat-sub">after {money(analytics.totals.proceeds)} disposal proceeds</div>
		</div>
		<div class="stat">
			<div class="stat-label">Capital (purchases)</div>
			<div class="stat-value">{money(analytics.totals.capex)}</div>
			<div class="stat-sub">repairs {money(analytics.totals.repair)} · upgrades {money(analytics.totals.upgrade)}</div>
		</div>
		<div class="stat">
			<div class="stat-label">Book value (now)</div>
			<div class="stat-value">{money(analytics.depreciation.book_value)}</div>
			<div class="stat-sub">of {money(analytics.depreciation.purchase_value)} purchase value</div>
		</div>
		<div class="stat">
			<div class="stat-label">Accumulated depreciation</div>
			<div class="stat-value">{money(analytics.depreciation.depreciation)}</div>
			<div class="stat-sub">straight-line, per asset type</div>
		</div>
	</div>

	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">Monthly spend (last 24 months)</h3>
		{#if analytics.monthly.length === 0}
			<p class="muted">No dated cost entries yet.</p>
		{:else}
			{@const mmax = Math.max(...analytics.monthly.map((m) => m.amount), 1)}
			<div class="spark">
				{#each analytics.monthly as m}
					<div class="spark-col" title="{m.month}: {money2(m.amount)}">
						<div class="spark-bar" style="height:{Math.max(2, (m.amount / mmax) * 100)}%"></div>
						<span class="spark-x">{m.month.slice(2)}</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<div class="grid cols-2">
		<div class="card">
			<h3 style="margin-top:0">Cost by asset type</h3>
			{#if analytics.by_type.length === 0}<p class="muted">No data.</p>{:else}
				{@const mx = maxAmt(analytics.by_type)}
				<div class="bars">
					{#each analytics.by_type as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill" style="width:{(Math.abs(b.amount) / mx) * 100}%"></span></span>
							<span class="bar-val">{money(b.amount)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
		<div class="card">
			<h3 style="margin-top:0">Cost by site</h3>
			{#if analytics.by_site.length === 0}<p class="muted">No located costs.</p>{:else}
				{@const mx = maxAmt(analytics.by_site)}
				<div class="bars">
					{#each analytics.by_site as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill site" style="width:{(Math.abs(b.amount) / mx) * 100}%"></span></span>
							<span class="bar-val">{money(b.amount)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
		<div class="card">
			<h3 style="margin-top:0">Spend by vendor</h3>
			{#if analytics.by_vendor.length === 0}<p class="muted">No data.</p>{:else}
				{@const mx = maxAmt(analytics.by_vendor)}
				<div class="bars">
					{#each analytics.by_vendor as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill vendor" style="width:{(Math.abs(b.amount) / mx) * 100}%"></span></span>
							<span class="bar-val">{money(b.amount)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
		<div class="card">
			<h3 style="margin-top:0">Cost composition</h3>
			{#if composition.length}
				{@const mx = maxAmt(composition)}
				<div class="bars">
					{#each composition as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill" style="width:{(b.amount / mx) * 100}%"></span></span>
							<span class="bar-val">{money(b.amount)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{:else if tab === 'assets'}
	<div class="card">
		<div class="toolbar" style="margin-bottom:12px">
			<label>
				Sort by
				<select bind:value={assetSort} onchange={applyAssetFilters}>
					<option value="net_cost">Net lifetime cost</option>
					<option value="purchase_cost">Purchase cost</option>
					<option value="repair_cost">Repair cost</option>
					<option value="book_value">Book value</option>
				</select>
			</label>
			<label>
				Type
				<select bind:value={assetTypeId} onchange={applyAssetFilters}>
					<option value="">All types</option>
					{#each selectableTypes as t}<option value={String(t.id)}>{t.name}</option>{/each}
				</select>
			</label>
		</div>
		<table>
			<thead>
				<tr><th>Asset</th><th>Type</th><th>Location</th><th>Purchase</th><th>Repairs</th><th>Net TCO</th><th>Book value</th><th>Age</th></tr>
			</thead>
			<tbody>
				{#each assetRows as a}
					<tr
						role="button"
						tabindex="0"
						style="cursor:pointer"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
					>
						<td>{a.name}<div class="muted" style="font-size:11px">{a.asset_tag}</div></td>
						<td class="muted">{a.type_name}</td>
						<td class="muted">{a.location_name}</td>
						<td>{money2(a.purchase_cost)}</td>
						<td>{a.repair_cost ? money2(a.repair_cost) : '—'}</td>
						<td><strong>{money2(a.net_cost)}</strong></td>
						<td class="muted">{money2(a.book_value)}</td>
						<td class="muted">{a.age_months != null ? `${a.age_months} mo` : '—'}</td>
					</tr>
				{/each}
				{#if assetRows.length === 0}<tr><td colspan="8" class="muted">No assets with cost data.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'budgets'}
	<div class="card" style="margin-bottom:16px">
		<div class="row" style="justify-content:space-between; align-items:center">
			<h3 style="margin:0">Cost centers &amp; budgets</h3>
			<label>
				Fiscal year
				<select bind:value={budgetYear} onchange={loadBudgets}>
					{#each years as y}<option value={y}>{y}</option>{/each}
				</select>
			</label>
		</div>
		<p class="muted" style="margin-top:6px">
			Cost centers are org units. Actuals roll up each unit's subtree from asset cost entries dated in {budgetYear}
			(an asset contributes via its owning org unit).
		</p>

		{#if manage}
			<div class="row" style="gap:8px; align-items:flex-end; margin-bottom:12px; flex-wrap:wrap">
				<label style="min-width:240px">
					Org unit
					<select bind:value={newBudget.org_unit_id}>
						<option value="">— select —</option>
						{#each orgUnits as o}
							<option value={o.id}>{'\u00A0'.repeat(depth(o.path) * 2)}{o.name}</option>
						{/each}
					</select>
				</label>
				<label>
					Budget ({budgetYear})
					<input type="number" step="100" bind:value={newBudget.amount} />
				</label>
				<button class="btn" type="button" onclick={saveBudget}>Set budget</button>
			</div>
		{/if}

		<table>
			<thead>
				<tr><th>Cost center</th><th>Budget</th><th>Actual</th><th>Variance</th><th style="width:160px">Used</th>{#if manage}<th></th>{/if}</tr>
			</thead>
			<tbody>
				{#each budgetRows as b}
					<tr>
						<td style="padding-left:{12 + depth(b.path) * 16}px">{b.org_unit_name}</td>
						<td>{b.budget ? money2(b.budget) : '—'}</td>
						<td>{money2(b.actual)}</td>
						<td class:over={variance(b) < 0} class:under={variance(b) >= 0 && b.budget > 0}>
							{b.budget ? money2(variance(b)) : '—'}
						</td>
						<td>
							{#if b.budget}
								<span class="bar-track" style="display:inline-block; width:110px; vertical-align:middle">
									<span class="bar-fill" class:danger={usedPct(b) >= 100} style="width:{usedPct(b)}%"></span>
								</span>
								<span class="muted" style="font-size:11px"> {usedPct(b)}%</span>
							{:else}
								<span class="muted">no budget</span>
							{/if}
						</td>
						{#if manage}
							<td>{#if b.budget_id}<button class="btn danger small" type="button" onclick={() => deleteBudget(b.budget_id)}>Remove</button>{/if}</td>
						{/if}
					</tr>
				{/each}
				{#if budgetRows.length === 0}<tr><td colspan={manage ? 6 : 5} class="muted">No budgets or spend for {budgetYear}.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'depreciation'}
	<div class="card">
		<h3 style="margin-top:0">Depreciation settings</h3>
		<p class="muted" style="margin-top:0">
			Straight-line depreciation per asset type. Useful life of 0 means an asset of that type is not depreciated
			(book value stays at purchase cost). Book value = purchase cost × (1 − age ÷ useful life), floored at 0.
		</p>
		<table>
			<thead><tr><th>Asset type</th><th>Assets</th><th style="width:200px">Useful life (months)</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each depRows as d}
					<tr>
						<td>{d.name}<div class="muted" style="font-size:11px">{d.key}</div></td>
						<td class="muted">{d.asset_count}</td>
						<td>
							{#if manage}
								<input type="number" min="0" step="1" value={depEdits[d.id] ?? d.useful_life_months} oninput={(e) => (depEdits[d.id] = Number((e.target as HTMLInputElement).value))} style="width:120px" />
								<span class="muted" style="font-size:11px"> {((depEdits[d.id] ?? d.useful_life_months) / 12).toFixed(1)} yr</span>
							{:else}
								{d.useful_life_months} ({(d.useful_life_months / 12).toFixed(1)} yr)
							{/if}
						</td>
						{#if manage}
							<td>
								{#if (depEdits[d.id] ?? d.useful_life_months) !== d.useful_life_months}
									<button class="btn small" type="button" onclick={() => saveDep(d)}>Save</button>
								{/if}
							</td>
						{/if}
					</tr>
				{/each}
				{#if depRows.length === 0}<tr><td colspan={manage ? 4 : 3} class="muted">No asset types.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{/if}

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
		gap: 14px;
		margin-bottom: 16px;
	}
	.stat {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 16px;
	}
	.stat-label {
		font-size: 12px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.stat-value {
		font-size: 26px;
		font-weight: 700;
		margin-top: 6px;
	}
	.stat-sub {
		font-size: 12px;
		color: var(--muted);
		margin-top: 4px;
	}
	.bars {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.bar-row {
		display: grid;
		grid-template-columns: 130px 1fr 110px;
		align-items: center;
		gap: 10px;
		font-size: 13px;
	}
	.bar-label {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		text-transform: capitalize;
	}
	.bar-track {
		background: var(--surface-2, rgba(127, 127, 127, 0.12));
		border-radius: 5px;
		height: 14px;
		overflow: hidden;
	}
	.bar-fill {
		display: block;
		height: 100%;
		background: #6aa6ff;
		border-radius: 5px;
	}
	.bar-fill.site { background: #f59e0b; }
	.bar-fill.vendor { background: #a78bfa; }
	.bar-fill.danger { background: #ef4444; }
	.bar-val {
		text-align: right;
		color: var(--muted);
		font-variant-numeric: tabular-nums;
	}
	.spark {
		display: flex;
		align-items: flex-end;
		gap: 3px;
		height: 120px;
		padding-top: 6px;
	}
	.spark-col {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: flex-end;
		height: 100%;
		min-width: 0;
	}
	.spark-bar {
		width: 100%;
		max-width: 22px;
		background: #34d399;
		border-radius: 3px 3px 0 0;
	}
	.spark-x {
		font-size: 9px;
		color: var(--muted);
		margin-top: 3px;
		white-space: nowrap;
	}
	td.over { color: #ef4444; font-variant-numeric: tabular-nums; }
	td.under { color: #34d399; font-variant-numeric: tabular-nums; }
	.toolbar label,
	.row label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--muted);
	}
</style>
