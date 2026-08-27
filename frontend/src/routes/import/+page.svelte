<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiUpload, apiDownload } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { AssetType } from '$lib/types';

	type RowResult = {
		line: number;
		ok: boolean;
		errors?: string[];
		result?: string;
		values: Record<string, string>;
	};
	type ImportResult = {
		target: string;
		mode: string;
		headers: string[];
		rows: RowResult[];
		summary: Record<string, number>;
	};

	const targets = [
		{ key: 'assets', label: 'Assets', perm: 'asset.write' },
		{ key: 'purchase-orders', label: 'Purchase Orders', perm: 'procurement.manage' },
		{ key: 'people', label: 'People', perm: 'hierarchy.manage' },
		{ key: 'locations', label: 'Locations / Data Centers', perm: 'hierarchy.manage' },
		{ key: 'org-units', label: 'Org Units', perm: 'hierarchy.manage' },
		{ key: 'subnets', label: 'Network — Subnets', perm: 'ipam.manage' },
		{ key: 'vlans', label: 'Network — VLANs', perm: 'ipam.manage' },
		{ key: 'ips', label: 'Network — IP Addresses', perm: 'ipam.manage' },
		{ key: 'software', label: 'Software', perm: 'software.manage' },
		{ key: 'licenses', label: 'Licenses', perm: 'software.manage' }
	];

	let target = $state('assets');
	let typeKey = $state('');
	let types = $state<AssetType[]>([]);
	let file = $state<File | null>(null);
	let result = $state<ImportResult | null>(null);
	let busy = $state(false);
	let error = $state('');
	let committed = $state(false);

	const allowedTargets = $derived(targets.filter((t) => can($me, t.perm)));

	onMount(async () => {
		try {
			types = await apiGet<AssetType[]>('/api/metadata/asset-types');
		} catch {
			/* ignore */
		}
	});

	function reset() {
		result = null;
		committed = false;
		error = '';
	}

	function onPickFile(e: Event) {
		const input = e.target as HTMLInputElement;
		file = input.files?.[0] ?? null;
		reset();
	}

	function summarizeRow(row: RowResult): string {
		const v = row.values;
		const first =
			v['asset_tag'] || v['po_number'] || v['cidr'] || v['key'] || v['vlan_id'] || v['email'] || '';
		const name = [v['first_name'], v['last_name']].filter(Boolean).join(' ');
		const second = v['name'] || v['description'] || name || '';
		return [first, second].filter(Boolean).join(' · ');
	}

	async function downloadTemplate(format: 'csv' | 'xlsx') {
		const p = new URLSearchParams();
		if (target === 'assets' && typeKey) p.set('type', typeKey);
		if (format === 'xlsx') p.set('format', 'xlsx');
		const qs = p.toString() ? `?${p.toString()}` : '';
		try {
			await apiDownload(`/api/import/${target}/template${qs}`, `itam_${target}_template.${format}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Download failed';
		}
	}

	async function run(mode: 'preview' | 'commit') {
		if (!file) {
			error = 'Choose a CSV or XLSX file first.';
			return;
		}
		busy = true;
		error = '';
		try {
			result = await apiUpload<ImportResult>(`/api/import/${target}?mode=${mode}`, file);
			committed = mode === 'commit';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Import failed';
		} finally {
			busy = false;
		}
	}

	const validCount = $derived(result?.summary?.valid ?? 0);
</script>

<div class="topbar">
	<h1>Bulk Import</h1>
</div>

{#if allowedTargets.length === 0}
	<div class="card"><p class="muted">You don't have permission to import data.</p></div>
{:else}
	<div class="card">
		<div class="toolbar">
			<label>
				What to import
				<select bind:value={target} onchange={reset}>
					{#each allowedTargets as t}
						<option value={t.key}>{t.label}</option>
					{/each}
				</select>
			</label>
			{#if target === 'assets'}
				<label>
					Tailor template to type (optional)
					<select bind:value={typeKey}>
						<option value="">Generic (no attributes)</option>
						{#each types.filter((t) => !t.is_abstract) as t}
							<option value={t.key}>{t.name}</option>
						{/each}
					</select>
				</label>
			{/if}
		</div>

		<div class="step">
			<span class="step-no">1</span>
			<div>
				<strong>Download a template</strong>
				<p class="muted">Headers match exactly what the importer expects. Fill it in, keep the header row, delete the example/comment rows.</p>
				<button class="btn secondary small" onclick={() => downloadTemplate('csv')}>CSV template</button>
				<button class="btn secondary small" onclick={() => downloadTemplate('xlsx')}>Excel template</button>
			</div>
		</div>

		<div class="step">
			<span class="step-no">2</span>
			<div>
				<strong>Upload your file</strong>
				<p class="muted">Accepts .csv or .xlsx.</p>
				<input type="file" accept=".csv,.xlsx" onchange={onPickFile} />
				{#if file}<span class="muted" style="margin-left:8px">{file.name}</span>{/if}
			</div>
		</div>

		<div class="step">
			<span class="step-no">3</span>
			<div>
				<strong>Validate, then commit</strong>
				<p class="muted">Preview checks every row without saving. Commit creates only the valid rows.</p>
				<button class="btn secondary" onclick={() => run('preview')} disabled={busy || !file}>
					{busy ? 'Working…' : 'Preview'}
				</button>
				<button
					class="btn"
					onclick={() => run('commit')}
					disabled={busy || !result || committed || validCount === 0}
				>
					Commit {validCount} valid row{validCount === 1 ? '' : 's'}
				</button>
			</div>
		</div>
	</div>

	{#if error}<p class="error">{error}</p>{/if}

	{#if result}
		<div class="card">
			<h3 style="margin-top:0">
				{committed ? 'Import complete' : 'Preview'}
			</h3>
			<div class="summary">
				<span class="pill">{result.summary.total} rows</span>
				<span class="pill ok">{result.summary.valid} valid</span>
				{#if result.summary.invalid > 0}<span class="pill bad">{result.summary.invalid} invalid</span>{/if}
				{#if result.summary.purchase_orders != null}<span class="pill">{result.summary.purchase_orders} PO(s)</span>{/if}
				{#if committed}<span class="pill ok">{result.summary.created} created</span>{/if}
			</div>

			{#if committed && validCount > 0}
				<p class="muted" style="margin-top:8px">Created records now have full lifecycle timelines and (for assets) purchase costs recorded.</p>
			{/if}

			<table style="margin-top:12px">
				<thead>
					<tr><th>Line</th><th>Status</th><th>Detail</th></tr>
				</thead>
				<tbody>
					{#each result.rows as row}
						<tr>
							<td>{row.line}</td>
							<td>
								{#if row.ok}
									<span class="badge" style="border-color:#22c55e;color:#16a34a">ok</span>
								{:else}
									<span class="badge" style="border-color:#ef4444;color:#dc2626">error</span>
								{/if}
							</td>
							<td>
								{#if row.errors && row.errors.length}
									<span style="color:#dc2626">{row.errors.join('; ')}</span>
								{:else}
									<span class="muted">{summarizeRow(row)}</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}

<style>
	.step {
		display: flex;
		gap: 12px;
		align-items: flex-start;
		padding: 14px 0;
		border-top: 1px solid var(--border, #e5e7eb);
	}
	.step:first-of-type {
		border-top: none;
	}
	.step-no {
		flex: none;
		width: 26px;
		height: 26px;
		border-radius: 50%;
		background: #eef2ff;
		color: #4f46e5;
		display: grid;
		place-items: center;
		font-size: 13px;
		font-weight: 600;
	}
	.step p {
		margin: 4px 0 8px;
	}
	.toolbar label {
		display: flex;
		flex-direction: column;
		font-size: 12px;
		color: var(--muted, #6b7280);
		gap: 4px;
	}
	.summary {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}
	.pill {
		border: 1px solid var(--border, #e5e7eb);
		border-radius: 999px;
		padding: 2px 10px;
		font-size: 12px;
	}
	.pill.ok {
		border-color: #22c55e;
		color: #16a34a;
	}
	.pill.bad {
		border-color: #ef4444;
		color: #dc2626;
	}
</style>
