<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiDownload } from '$lib/api';
	import { me, can } from '$lib/me';
	import type {
		ReportOverview, ReportBucket, WarrantyRow, OrgUnit, OrgUnitKind, OrgInventoryReport,
		AssetType, CustodyReport, DataQualityReport, DataQualityFixResult, Location, UnmappedLocationRow
	} from '$lib/types';
	import { depth, kindLabel } from '$lib/org';

	const canCost = $derived(can($me, 'cost.read'));
	const canHierarchy = $derived(can($me, 'hierarchy.read'));

	let overview = $state<ReportOverview | null>(null);
	let warranty = $state<WarrantyRow[]>([]);
	let warrantyDays = $state(90);
	let orgUnits = $state<OrgUnit[]>([]);
	let orgKinds = $state<OrgUnitKind[]>([]);
	let assetTypes = $state<AssetType[]>([]);
	let orgReportId = $state('');
	let orgReportType = $state('');
	let orgReport = $state<OrgInventoryReport | null>(null);
	let orgReportLoading = $state(false);
	let custody = $state<CustodyReport | null>(null);
	let dataQuality = $state<DataQualityReport | null>(null);
	let unmapped = $state<UnmappedLocationRow[]>([]);
	let locations = $state<Location[]>([]);
	let selectedGapIds = $state<Set<string>>(new Set());
	let bulkLocationId = $state('');
	let bulkOrgId = $state('');
	let fixBusy = $state(false);
	let fixMsg = $state('');
	let mergePair = $state<{ serial: string; survivor: string; merge: string } | null>(null);
	let aliasDraft = $state<Record<string, { location_id: string; note: string }>>({});
	let loading = $state(true);
	let error = $state('');

	const dayOptions = [30, 90, 180, 365];

	async function loadOverview() {
		try {
			overview = await apiGet<ReportOverview>('/api/reports/overview');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load reports';
		}
	}

	async function loadWarranty() {
		try {
			warranty = (await apiGet<WarrantyRow[]>(`/api/reports/warranty?days=${warrantyDays}`)) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load warranty data';
		}
	}

	function setDays(d: number) {
		warrantyDays = d;
		loadWarranty();
	}

	async function loadOrgInventory() {
		if (!orgReportId) {
			orgReport = null;
			return;
		}
		orgReportLoading = true;
		try {
			const params = new URLSearchParams({ org_unit_id: orgReportId, subtree: 'true' });
			if (orgReportType) params.set('type', orgReportType);
			orgReport = await apiGet<OrgInventoryReport>(`/api/reports/org-inventory?${params}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load org inventory';
		} finally {
			orgReportLoading = false;
		}
	}

	const canWrite = $derived(can($me, 'asset.write'));
	const canManageHierarchy = $derived(can($me, 'hierarchy.manage'));

	async function loadDataQuality() {
		try {
			dataQuality = await apiGet<DataQualityReport>('/api/reports/data-quality');
		} catch {
			dataQuality = null;
		}
	}

	async function loadUnmapped() {
		try {
			unmapped = (await apiGet<UnmappedLocationRow[]>('/api/reports/unmapped-locations')) ?? [];
		} catch {
			unmapped = [];
		}
	}

	async function bulkFix(action: string, assetIds: string[] = []) {
		fixBusy = true;
		fixMsg = '';
		error = '';
		try {
			const body: Record<string, unknown> = { action };
			if (assetIds.length) body.asset_ids = assetIds;
			if (action === 'set_location') body.location_id = bulkLocationId;
			if (action === 'set_org_unit') body.owner_org_unit_id = bulkOrgId;
			const res = await apiPost<DataQualityFixResult>('/api/reports/data-quality/fix', body);
			fixMsg = `Updated ${res.updated}, skipped ${res.skipped}${res.errors?.length ? ` · ${res.errors.length} errors` : ''}`;
			selectedGapIds = new Set();
			await loadDataQuality();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Bulk fix failed';
		} finally {
			fixBusy = false;
		}
	}

	async function mergeAssets() {
		if (!mergePair) return;
		fixBusy = true;
		error = '';
		try {
			await apiPost('/api/assets/merge', { survivor_id: mergePair.survivor, merge_id: mergePair.merge });
			mergePair = null;
			await loadDataQuality();
			fixMsg = 'Assets merged.';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Merge failed';
		} finally {
			fixBusy = false;
		}
	}

	async function createAlias(label: string) {
		const draft = aliasDraft[label];
		if (!draft?.location_id) return;
		try {
			await apiPost('/api/location-aliases', { alias: label, location_id: draft.location_id, note: draft.note || 'from unmapped report' });
			await loadUnmapped();
			fixMsg = `Alias "${label}" created.`;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Create alias failed';
		}
	}

	function toggleGap(id: string) {
		const next = new Set(selectedGapIds);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selectedGapIds = next;
	}

	onMount(async () => {
		await Promise.all([loadOverview(), loadWarranty()]);
		if (canHierarchy) {
			[orgUnits, orgKinds, assetTypes, locations] = await Promise.all([
				apiGet<OrgUnit[]>('/api/org-units'),
				apiGet<OrgUnitKind[]>('/api/metadata/org-unit-kinds'),
				apiGet<AssetType[]>('/api/metadata/asset-types'),
				apiGet<Location[]>('/api/locations')
			]);
		}
		try {
			custody = await apiGet<CustodyReport>('/api/reports/custody');
		} catch {
			/* report.read required */
		}
		await Promise.all([loadDataQuality(), loadUnmapped()]);
		loading = false;
	});

	function fmt(n: number): string {
		return n.toLocaleString();
	}
	function money(n: number, ccy = 'USD'): string {
		return `${n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${ccy}`;
	}
	function maxOf(b: ReportBucket[]): number {
		return b.reduce((m, x) => Math.max(m, x.count), 0) || 1;
	}
	function expiryClass(days: number): string {
		if (days < 0) return 'exp';
		if (days <= 30) return 'warn';
		return 'ok';
	}
</script>

<div class="topbar">
	<h1>Reports</h1>
	<div class="row">
		<button class="btn secondary small" onclick={() => apiDownload('/api/reports/export/assets', 'itam_assets.csv')}>Export assets</button>
		<button class="btn secondary small" onclick={() => apiDownload('/api/reports/export/custody', 'itam_custody.csv')}>Export custody</button>
		<button class="btn secondary small" onclick={() => apiDownload(`/api/reports/export/warranty?days=${warrantyDays}`, 'itam_warranty.csv')}>Export warranty</button>
		{#if canCost}
			<button class="btn secondary small" onclick={() => apiDownload('/api/reports/export/costs', 'itam_costs.csv')}>Export costs</button>
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if overview}
	<!-- Headline stats -->
	<div class="stats">
		<div class="stat">
			<div class="stat-label">Total assets</div>
			<div class="stat-value">{fmt(overview.total_assets)}</div>
		</div>
		<div class="stat">
			<div class="stat-label">Open purchase orders</div>
			<div class="stat-value">{fmt(overview.procurement.open_pos)}</div>
			<div class="stat-sub">{money(overview.procurement.open_value, overview.financial?.currency ?? 'USD')} on order</div>
		</div>
		<div class="stat">
			<div class="stat-label">Warranty expiring ≤ 90d</div>
			<div class="stat-value">{fmt(overview.warranty.next_90)}</div>
			<div class="stat-sub">{fmt(overview.warranty.expired)} already expired</div>
		</div>
		{#if overview.financial}
			<div class="stat">
				<div class="stat-label">Purchase value</div>
				<div class="stat-value">{money(overview.financial.purchase_value, overview.financial.currency)}</div>
				<div class="stat-sub">Lifetime cost {money(overview.financial.ledger_net, overview.financial.currency)}</div>
			</div>
		{/if}
	</div>

	<div class="grid cols-2">
		<div class="card">
			<h3 style="margin-top:0">Assets by lifecycle state</h3>
			{#if overview.by_state.length === 0}
				<p class="muted">No data.</p>
			{:else}
				{@const mx = maxOf(overview.by_state)}
				<div class="bars">
					{#each overview.by_state as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill" style="width:{(b.count / mx) * 100}%"></span></span>
							<span class="bar-val">{fmt(b.count)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="card">
			<h3 style="margin-top:0">Assets by type</h3>
			{#if overview.by_type.length === 0}
				<p class="muted">No data.</p>
			{:else}
				{@const mx = maxOf(overview.by_type)}
				<div class="bars">
					{#each overview.by_type as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill alt" style="width:{(b.count / mx) * 100}%"></span></span>
							<span class="bar-val">{fmt(b.count)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="card">
			<h3 style="margin-top:0">Assets by site</h3>
			{#if overview.by_site.length === 0}
				<p class="muted">No located assets.</p>
			{:else}
				{@const mx = maxOf(overview.by_site)}
				<div class="bars">
					{#each overview.by_site as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill site" style="width:{(b.count / mx) * 100}%"></span></span>
							<span class="bar-val">{fmt(b.count)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="card">
			<h3 style="margin-top:0">Purchase orders by status</h3>
			{#if overview.procurement.by_status.length === 0}
				<p class="muted">No purchase orders.</p>
			{:else}
				{@const mx = maxOf(overview.procurement.by_status)}
				<div class="bars">
					{#each overview.procurement.by_status as b}
						<div class="bar-row">
							<span class="bar-label">{b.label}</span>
							<span class="bar-track"><span class="bar-fill po" style="width:{(b.count / mx) * 100}%"></span></span>
							<span class="bar-val">{fmt(b.count)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>

	{#if canHierarchy}
		<div class="card" style="margin-top:16px">
			<h3 style="margin-top:0">Inventory by organization</h3>
			<p class="muted" style="margin-top:4px">
				Count assets assigned to an org unit and everything beneath it (region → district → division → unit).
			</p>
			<div class="toolbar" style="margin-top:12px">
				<select bind:value={orgReportId} onchange={loadOrgInventory}>
					<option value="">Select org unit…</option>
					{#each orgUnits as ou}
						<option value={ou.id}>{'— '.repeat(depth(ou.path))}{ou.name}{ou.kind ? ` (${ou.kind})` : ''}</option>
					{/each}
				</select>
				<select bind:value={orgReportType} onchange={loadOrgInventory}>
					<option value="">All types</option>
					{#each assetTypes.filter((t) => !t.is_abstract) as t}
						<option value={t.key}>{t.name}</option>
					{/each}
				</select>
			</div>
			{#if orgReportLoading}
				<p class="muted">Loading…</p>
			{:else if orgReport}
				<p style="margin:12px 0 4px">
					<strong>{fmt(orgReport.total)}</strong> asset(s) under <strong>{orgReport.org_unit.name}</strong>
				</p>
				{#if orgReport.breadcrumb.length > 1}
					<p class="muted small">
						{#each orgReport.breadcrumb as c, i}
							{#if i > 0} › {/if}{c.name}{c.kind ? ` (${kindLabel(orgKinds, c.kind)})` : ''}
						{/each}
					</p>
				{/if}
				{#if orgReport.by_type.length === 0}
					<p class="muted">No assets with an owner org unit in this subtree.</p>
				{:else}
					{@const mx = Math.max(...orgReport.by_type.map((b) => b.count), 1)}
					<div class="bars" style="margin-top:12px">
						{#each orgReport.by_type as b}
							<div class="bar-row">
								<span class="bar-label">{b.label}</span>
								<span class="bar-track"><span class="bar-fill alt" style="width:{(b.count / mx) * 100}%"></span></span>
								<span class="bar-val">{fmt(b.count)}</span>
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		</div>
	{/if}

	{#if custody}
		<div class="card" style="margin-top:16px">
			<h3 style="margin-top:0">Custody overview</h3>
			<div class="stats" style="margin-top:12px">
				<div class="stat">
					<div class="stat-label">Checked out</div>
					<div class="stat-value">{fmt(custody.assigned)}</div>
				</div>
				<div class="stat">
					<div class="stat-label">Available (unassigned)</div>
					<div class="stat-value">{fmt(custody.available)}</div>
				</div>
			</div>
			{#if custody.by_holder.length}
				<h4 style="margin:16px 0 8px">Top holders</h4>
				{@const mx = Math.max(...custody.by_holder.map((h) => h.count), 1)}
				<div class="bars">
					{#each custody.by_holder as h}
						<div class="bar-row">
							<span class="bar-label">{h.name || 'Unknown'}</span>
							<span class="bar-track"><span class="bar-fill" style="width:{(h.count / mx) * 100}%"></span></span>
							<span class="bar-val">{fmt(h.count)}</span>
						</div>
					{/each}
				</div>
			{/if}
			{#if custody.not_in_stock_after_return.length}
				<h4 style="margin:16px 0 8px">Needs attention — returned but not in stock</h4>
				<p class="muted small">These assets have no assignee but are still in a deployed/in-use state.</p>
				<div class="table-scroll">
					<table>
						<thead><tr><th>Tag</th><th>Name</th><th>State</th></tr></thead>
						<tbody>
							{#each custody.not_in_stock_after_return as a}
								<tr
									class="click"
									role="button"
									tabindex="0"
									onclick={() => goto(`/assets/${a.id}`)}
									onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
								>
									<td>{a.asset_tag}</td>
									<td>{a.name}</td>
									<td>{a.state_name}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			{#if custody.unacknowledged?.length}
				<h4 style="margin:16px 0 8px">Awaiting acknowledge</h4>
				<div class="table-scroll">
					<table>
						<thead><tr><th>Tag</th><th>Name</th><th>Status</th></tr></thead>
						<tbody>
							{#each custody.unacknowledged as a}
								<tr class="click" role="button" tabindex="0" onclick={() => goto(`/assets/${a.id}`)}>
									<td>{a.asset_tag}</td><td>{a.name}</td><td>{a.state_name}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			{#if custody.assigned_in_stock?.length}
				<h4 style="margin:16px 0 8px">Assigned but still in stock</h4>
				<div class="table-scroll">
					<table>
						<thead><tr><th>Tag</th><th>Name</th><th>State</th></tr></thead>
						<tbody>
							{#each custody.assigned_in_stock as a}
								<tr class="click" role="button" tabindex="0" onclick={() => goto(`/assets/${a.id}`)}>
									<td>{a.asset_tag}</td><td>{a.name}</td><td>{a.state_name}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}

	{#if dataQuality}
		<div class="card" style="margin-top:16px">
			<h3 style="margin-top:0">Data quality</h3>
			<p class="muted" style="margin-top:0">Fix gaps in bulk — set org from assignee, apply org default locations, merge duplicate serials.</p>
			{#if fixMsg}<p class="muted small">{fixMsg}</p>{/if}
			<div class="stats" style="margin-top:12px">
				<div class="stat">
					<div class="stat-label">Missing location</div>
					<div class="stat-value">{fmt(dataQuality.missing_location)}</div>
				</div>
				<div class="stat">
					<div class="stat-label">Missing org unit</div>
					<div class="stat-value">{fmt(dataQuality.missing_org_unit)}</div>
				</div>
				<div class="stat">
					<div class="stat-label">Missing serial</div>
					<div class="stat-value">{fmt(dataQuality.missing_serial)}</div>
				</div>
			</div>
			{#if canWrite}
				<div class="action-bar">
					<button class="btn secondary small" disabled={fixBusy} onclick={() => bulkFix('apply_assignee_org_all')}>Set org from assignee (all)</button>
					<button class="btn secondary small" disabled={fixBusy} onclick={() => bulkFix('apply_org_default_location_all')}>Apply org default location (all)</button>
				</div>
				<div class="bulk-actions">
					<div class="bulk-action">
						<label for="bulk-location">Bulk location for selected</label>
						<div class="bulk-action-row">
							<select id="bulk-location" bind:value={bulkLocationId}>
								<option value="">Select location…</option>
								{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
							</select>
							<button class="btn secondary small" disabled={fixBusy || !bulkLocationId || selectedGapIds.size === 0} onclick={() => bulkFix('set_location', [...selectedGapIds])}>Apply</button>
						</div>
					</div>
					<div class="bulk-action">
						<label for="bulk-org">Bulk org for selected</label>
						<div class="bulk-action-row">
							<select id="bulk-org" bind:value={bulkOrgId}>
								<option value="">Select org unit…</option>
								{#each orgUnits as u}<option value={u.id}>{' '.repeat(depth(u.path))}{u.name}</option>{/each}
							</select>
							<button class="btn secondary small" disabled={fixBusy || !bulkOrgId || selectedGapIds.size === 0} onclick={() => bulkFix('set_org_unit', [...selectedGapIds])}>Apply</button>
						</div>
					</div>
				</div>
			{/if}
			{#if (dataQuality.duplicate_groups ?? []).length}
				<h4 style="margin:16px 0 8px">Duplicate serials — merge</h4>
				{#each dataQuality.duplicate_groups ?? [] as grp}
					<div style="margin-bottom:12px; padding:10px; border:1px solid var(--border); border-radius:8px">
						<div class="row" style="justify-content:space-between; align-items:center">
							<code>{grp.serial}</code>
							{#if canWrite && grp.assets.length >= 2}
								<button class="btn secondary small" onclick={() => (mergePair = { serial: grp.serial, survivor: grp.assets[0].id, merge: grp.assets[1].id })}>Merge…</button>
							{/if}
						</div>
						<ul style="margin:8px 0 0; padding-left:18px">
							{#each grp.assets as a}
								<li><button class="linklike" onclick={() => goto(`/assets/${a.id}`)}>{a.asset_tag} · {a.name}</button></li>
							{/each}
						</ul>
					</div>
				{/each}
			{:else if dataQuality.duplicate_serials.length}
				<h4 style="margin:16px 0 8px">Duplicate serials</h4>
				<div class="table-scroll">
					<table>
						<thead><tr><th>Serial</th><th>Count</th></tr></thead>
						<tbody>
							{#each dataQuality.duplicate_serials as d}
								<tr><td><code>{d.serial}</code></td><td>{fmt(d.count)}</td></tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			{#if dataQuality.gaps.length}
				<h4 style="margin:16px 0 8px">Sample gaps (up to 50){#if canWrite} — select rows for bulk fix{/if}</h4>
				<div class="table-scroll">
					<table>
						<thead><tr>{#if canWrite}<th></th>{/if}<th>Tag</th><th>Name</th><th>Issue</th></tr></thead>
						<tbody>
							{#each dataQuality.gaps as g}
								<tr>
									{#if canWrite}
										<td><input type="checkbox" checked={selectedGapIds.has(g.id)} onchange={() => toggleGap(g.id)} /></td>
									{/if}
									<td><button class="linklike" onclick={() => goto(`/assets/${g.id}`)}>{g.asset_tag}</button></td>
									<td>{g.name}</td>
									<td class="muted">{g.issue}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<p class="muted" style="margin-top:12px">No obvious gaps in the sample window.</p>
			{/if}
		</div>
	{/if}

	{#if unmapped.length}
		<div class="card" style="margin-top:16px">
			<h3 style="margin-top:0">Unmapped ingest location labels</h3>
			<p class="muted" style="margin-top:0">Labels from discovery that do not match a location key, name, or alias.</p>
			<table>
				<thead><tr><th>Label</th><th>Source</th><th>Assets</th>{#if canManageHierarchy}<th>Map to location</th>{/if}</tr></thead>
				<tbody>
					{#each unmapped as row}
						<tr>
							<td><code>{row.label}</code></td>
							<td class="muted">{row.source}</td>
							<td>{fmt(row.asset_count)}</td>
							{#if canManageHierarchy}
								<td>
									<div class="row" style="gap:6px">
										<select
											value={aliasDraft[row.label]?.location_id ?? ''}
											onchange={(e) => {
												aliasDraft = { ...aliasDraft, [row.label]: { location_id: (e.currentTarget as HTMLSelectElement).value, note: aliasDraft[row.label]?.note ?? '' } };
											}}
										>
											<option value="">Select…</option>
											{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
										</select>
										<button class="btn small" disabled={!aliasDraft[row.label]?.location_id} onclick={() => createAlias(row.label)}>Add alias</button>
									</div>
								</td>
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	{#if mergePair}
		<div class="drawer-backdrop" role="button" tabindex="0" aria-label="Close" onclick={() => (mergePair = null)} onkeydown={(e) => e.key === 'Escape' && (mergePair = null)}></div>
		<aside class="drawer">
			<h3 style="margin-top:0">Merge duplicate serial</h3>
			<p class="muted"><code>{mergePair.serial}</code></p>
			<div class="field">
				<label>Keep (survivor)</label>
				<select bind:value={mergePair.survivor}>
					{#each (dataQuality?.duplicate_groups ?? []).find((g) => g.serial === mergePair!.serial)?.assets ?? [] as a}
						<option value={a.id}>{a.asset_tag} · {a.name}</option>
					{/each}
				</select>
			</div>
			<div class="field">
				<label>Merge into survivor (will be deleted)</label>
				<select bind:value={mergePair.merge}>
					{#each (dataQuality?.duplicate_groups ?? []).find((g) => g.serial === mergePair!.serial)?.assets ?? [] as a}
						{#if a.id !== mergePair!.survivor}<option value={a.id}>{a.asset_tag} · {a.name}</option>{/if}
					{/each}
				</select>
			</div>
			<div class="row" style="margin-top:12px">
				<button class="btn" disabled={fixBusy} onclick={mergeAssets}>Merge assets</button>
				<button class="btn secondary" onclick={() => (mergePair = null)}>Cancel</button>
			</div>
		</aside>
	{/if}

	<!-- Warranty expiry -->
	<div class="card" style="margin-top:16px">
		<div class="row" style="justify-content:space-between; align-items:center">
			<h3 style="margin:0">Warranty expiry</h3>
			<div class="seg" role="group" aria-label="Warranty window">
				{#each dayOptions as d}
					<button class="seg-btn" class:active={warrantyDays === d} onclick={() => setDays(d)}>{d}d</button>
				{/each}
			</div>
		</div>
		<p class="muted" style="margin-top:6px">Assets that have expired or expire within {warrantyDays} days, soonest first.</p>
		<table>
			<thead><tr><th>Asset</th><th>Tag</th><th>Type</th><th>Location</th><th>Expiry</th><th>Days left</th></tr></thead>
			<tbody>
				{#each warranty as a}
					<tr
						class="click"
						role="button"
						tabindex="0"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
					>
						<td>{a.name}</td>
						<td class="muted">{a.asset_tag}</td>
						<td class="muted">{a.type_name}</td>
						<td class="muted">{a.location_name}</td>
						<td>{new Date(a.warranty_expiry).toLocaleDateString()}</td>
						<td><span class="pill {expiryClass(a.days_left)}">{a.days_left < 0 ? `${-a.days_left}d ago` : `${a.days_left}d`}</span></td>
					</tr>
				{/each}
				{#if warranty.length === 0}
					<tr><td colspan="6" class="muted">No assets with warranty in this window.</td></tr>
				{/if}
			</tbody>
		</table>
	</div>
{/if}

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
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
		font-size: 28px;
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
		grid-template-columns: 130px 1fr 48px;
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
	.bar-fill.alt { background: #34d399; }
	.bar-fill.site { background: #f59e0b; }
	.bar-fill.po { background: #a78bfa; }
	.bar-val {
		text-align: right;
		color: var(--muted);
		font-variant-numeric: tabular-nums;
	}
	tr.click {
		cursor: pointer;
	}
	.pill {
		display: inline-block;
		padding: 1px 8px;
		border-radius: 999px;
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}
	.pill.ok { background: rgba(52, 211, 153, 0.15); color: #34d399; }
	.pill.warn { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
	.pill.exp { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	.seg-btn {
		background: transparent;
		color: var(--muted);
		border: none;
		padding: 5px 12px;
		cursor: pointer;
		font: inherit;
	}
	.seg-btn.active {
		background: #6aa6ff;
		color: #0d0f17;
		font-weight: 600;
	}
	.linklike {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
		font: inherit;
		padding: 0;
	}
	.drawer-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		z-index: 40;
	}
	.drawer {
		position: fixed;
		top: 0;
		right: 0;
		width: min(420px, 92vw);
		height: 100%;
		background: var(--surface, #12141c);
		border-left: 1px solid var(--border);
		padding: 20px;
		z-index: 41;
		overflow-y: auto;
	}
</style>
