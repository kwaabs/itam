<script lang="ts">
	import { goto } from '$app/navigation';
	import { apiGet } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Paginated, Asset, AssetType, Location, Person, SetupHealthReport, Me } from '$lib/types';

	let assetsTotal = $state(0);
	let typesTotal = $state(0);
	let locationsTotal = $state(0);
	let peopleTotal = $state(0);
	let recent = $state<Asset[]>([]);
	let setup = $state<SetupHealthReport | null>(null);
	let loading = $state(true);

	async function loadDashboard(user: Me) {
		loading = true;
		try {
			if (can(user, 'asset.read')) {
				const a = await apiGet<Paginated<Asset>>('/api/assets?page_size=8');
				assetsTotal = a.total;
				recent = a.items;
			}
			if (can(user, 'metadata.read')) {
				typesTotal = (await apiGet<AssetType[]>('/api/metadata/asset-types')).filter(
					(t) => !t.is_abstract
				).length;
			}
			if (can(user, 'hierarchy.read')) {
				locationsTotal = (await apiGet<Location[]>('/api/locations')).length;
				peopleTotal = (await apiGet<Person[]>('/api/people')).length;
			}
			if (can(user, 'report.read')) {
				try {
					setup = await apiGet<SetupHealthReport>('/api/reports/setup-health');
				} catch {
					setup = null;
				}
			}
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		const user = $me;
		if (!user) return;
		void loadDashboard(user);
	});
</script>

<div class="topbar">
	<h1>Dashboard</h1>
</div>

{#if setup && setup.score < 100}
	<div class="card card-warn" style="margin-bottom:16px">
		<div class="row" style="justify-content:space-between; align-items:center">
			<div>
				<h3 style="margin:0">Setup health — {setup.score}%</h3>
				<p class="muted" style="margin:4px 0 0">Complete these items for cleaner sync, custody, and reports.</p>
			</div>
			<a class="btn secondary small" href="/reports">Data quality</a>
		</div>
		<ul class="checklist" style="margin:12px 0 0; padding:0; list-style:none">
			{#each setup.items as item}
				<li class:ok={item.ok} class:warn={!item.ok}>
					<button class="linklike" onclick={() => goto(item.href)}>
						{item.ok ? '✓' : '○'} {item.label}
						{#if !item.ok && item.count != null && item.count > 0}<span class="muted"> ({item.count})</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	</div>
{/if}

<div class="grid cols-4">
	<div class="card stat stat-indigo"><span class="value">{assetsTotal}</span><span class="label">Assets</span></div>
	<div class="card stat stat-violet"><span class="value">{typesTotal}</span><span class="label">Asset Types</span></div>
	<div class="card stat stat-cyan"><span class="value">{locationsTotal}</span><span class="label">Locations</span></div>
	<div class="card stat stat-emerald"><span class="value">{peopleTotal}</span><span class="label">People</span></div>
</div>

<div class="card card-accent" style="margin-top:20px">
	<h3 style="margin-top:0">Recent assets</h3>
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if recent.length === 0}
		<p class="muted">No assets yet.</p>
	{:else}
		<table>
			<thead><tr><th>Name</th><th>Tag</th><th>Type</th><th>State</th></tr></thead>
			<tbody>
				{#each recent as a}
					<tr
						class="click"
						role="button"
						tabindex="0"
						onclick={() => goto(`/assets/${a.id}`)}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/assets/${a.id}`))}
					>
						<td>{a.name}</td>
						<td class="muted">{a.asset_tag}</td>
						<td class="muted">{a.asset_type?.name ?? '—'}</td>
						<td class="muted">{a.current_state?.name ?? '—'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<style>
	.checklist li { padding: 4px 0; }
	.checklist li.warn .linklike { color: var(--warning); }
	.checklist li.ok .linklike { color: var(--muted, #9aa0b4); }
	.linklike {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
		font: inherit;
		padding: 0;
		text-align: left;
	}
</style>
