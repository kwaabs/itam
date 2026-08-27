<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { StoragePool, StorageVolume, Asset, Paginated } from '$lib/types';

	let assets = $state<Asset[]>([]);
	let arrays = $state<Asset[]>([]);
	let arrayId = $state('');
	let pools = $state<StoragePool[]>([]);
	let volumes = $state<StorageVolume[]>([]);
	let loading = $state(false);
	let error = $state('');
	const manage = $derived(can($me, 'storage.manage'));

	let pForm = $state({ name: '', raid: '', raw_gb: '', usable_gb: '' });
	let vForm = $state({ name: '', pool_id: '', capacity_gb: '', used_gb: '', protocol: 'iscsi', attached_asset_id: '' });

	onMount(async () => {
		try {
			const [all, arr] = await Promise.all([
				apiGet<Paginated<Asset>>('/api/assets?page_size=300'),
				apiGet<Paginated<Asset>>('/api/assets?type=storage&page_size=300')
			]);
			assets = all.items ?? [];
			arrays = arr.items ?? [];
		} catch {
			/* ignore */
		}
	});

	async function loadArray() {
		if (!arrayId) {
			pools = [];
			volumes = [];
			return;
		}
		loading = true;
		error = '';
		try {
			pools = (await apiGet<StoragePool[]>(`/api/storage/pools?array_asset_id=${arrayId}`)) ?? [];
			volumes = (await apiGet<StorageVolume[]>(`/api/storage/volumes?array_asset_id=${arrayId}`)) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	function assetName(id: string | null): string {
		const a = assets.find((x) => x.id === id);
		return a ? `${a.asset_tag} · ${a.name}` : '—';
	}

	const usable = $derived(pools.reduce((s, p) => s + (p.usable_gb ?? 0), 0));
	const allocated = $derived(volumes.reduce((s, v) => s + (v.capacity_gb ?? 0), 0));
	const used = $derived(volumes.reduce((s, v) => s + (v.used_gb ?? 0), 0));

	async function addPool() {
		if (!pForm.name.trim() || !arrayId) return;
		try {
			await apiPost('/api/storage/pools', {
				name: pForm.name,
				array_asset_id: arrayId,
				raid: pForm.raid || null,
				raw_gb: pForm.raw_gb ? Number(pForm.raw_gb) : null,
				usable_gb: pForm.usable_gb ? Number(pForm.usable_gb) : null
			});
			pForm = { name: '', raid: '', raw_gb: '', usable_gb: '' };
			await loadArray();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function addVolume() {
		if (!vForm.name.trim() || !arrayId) return;
		try {
			await apiPost('/api/storage/volumes', {
				name: vForm.name,
				array_asset_id: arrayId,
				pool_id: vForm.pool_id || null,
				capacity_gb: vForm.capacity_gb ? Number(vForm.capacity_gb) : null,
				used_gb: vForm.used_gb ? Number(vForm.used_gb) : null,
				protocol: vForm.protocol,
				attached_asset_id: vForm.attached_asset_id || null
			});
			vForm = { name: '', pool_id: '', capacity_gb: '', used_gb: '', protocol: 'iscsi', attached_asset_id: '' };
			await loadArray();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function del(kind: string, id: string) {
		try {
			await apiDelete(`/api/storage/${kind}/${id}`);
			await loadArray();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
</script>

<h1>Storage</h1>
{#if error}<p class="error">{error}</p>{/if}

<div class="card">
	<label class="field" style="max-width:420px">
		Storage array / NAS asset
		<select bind:value={arrayId} onchange={loadArray}>
			<option value="">— choose an array asset —</option>
			{#each arrays as a}<option value={a.id}>{a.asset_tag} · {a.name}</option>{/each}
		</select>
	</label>
	<p class="muted small">Storage arrays and NAS are assets (types added in step A). Pick one to manage its pools and volumes.</p>
</div>

{#if arrayId}
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<div class="cards">
			<div class="stat"><div class="lbl">Usable (pools)</div><div class="val">{usable.toFixed(0)} GB</div></div>
			<div class="stat">
				<div class="lbl">Allocated (volumes)</div>
				<div class="val">{allocated.toFixed(0)} GB</div>
				<div class="sub">{usable > 0 ? ((allocated / usable) * 100).toFixed(0) : 0}% of usable{allocated > usable && usable > 0 ? ' · over-provisioned' : ''}</div>
			</div>
			<div class="stat"><div class="lbl">Used</div><div class="val">{used.toFixed(0)} GB</div></div>
		</div>

		<div class="card">
			<h3 style="margin-top:0">Pools</h3>
			<table>
				<thead><tr><th>Name</th><th>RAID</th><th>Raw</th><th>Usable</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each pools as p}
						<tr>
							<td><strong>{p.name}</strong></td>
							<td class="muted">{p.raid || '—'}</td>
							<td>{p.raw_gb ? `${p.raw_gb} GB` : '—'}</td>
							<td>{p.usable_gb ? `${p.usable_gb} GB` : '—'}</td>
							{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => del('pools', p.id)}>remove</button></td>{/if}
						</tr>
					{/each}
					{#if pools.length === 0}<tr><td colspan={manage ? 5 : 4} class="muted">No pools.</td></tr>{/if}
				</tbody>
			</table>
			{#if manage}
				<div class="row" style="margin-top:8px; flex-wrap:wrap">
					<input placeholder="Pool name" bind:value={pForm.name} style="flex:1; min-width:120px" />
					<input placeholder="RAID" bind:value={pForm.raid} style="width:90px" />
					<input type="number" placeholder="Raw GB" bind:value={pForm.raw_gb} style="width:100px" />
					<input type="number" placeholder="Usable GB" bind:value={pForm.usable_gb} style="width:110px" />
					<button class="btn small" onclick={addPool}>Add pool</button>
				</div>
			{/if}
		</div>

		<div class="card">
			<h3 style="margin-top:0">Volumes / LUNs</h3>
			<table>
				<thead><tr><th>Name</th><th>Pool</th><th>Capacity</th><th>Used</th><th>Protocol</th><th>Attached to</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each volumes as v}
						<tr>
							<td><strong>{v.name}</strong></td>
							<td class="muted">{pools.find((p) => p.id === v.pool_id)?.name ?? '—'}</td>
							<td>{v.capacity_gb ? `${v.capacity_gb} GB` : '—'}</td>
							<td>{v.used_gb ? `${v.used_gb} GB` : '—'}</td>
							<td class="muted">{v.protocol || '—'}</td>
							<td class="muted">{v.attached_asset_id ? assetName(v.attached_asset_id) : '—'}</td>
							{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => del('volumes', v.id)}>remove</button></td>{/if}
						</tr>
					{/each}
					{#if volumes.length === 0}<tr><td colspan={manage ? 7 : 6} class="muted">No volumes.</td></tr>{/if}
				</tbody>
			</table>
			{#if manage}
				<div class="row" style="margin-top:8px; flex-wrap:wrap">
					<input placeholder="Volume name" bind:value={vForm.name} style="flex:1; min-width:120px" />
					<select bind:value={vForm.pool_id}>
						<option value="">No pool</option>
						{#each pools as p}<option value={p.id}>{p.name}</option>{/each}
					</select>
					<input type="number" placeholder="Cap GB" bind:value={vForm.capacity_gb} style="width:90px" />
					<input type="number" placeholder="Used GB" bind:value={vForm.used_gb} style="width:90px" />
					<select bind:value={vForm.protocol}>
						<option value="iscsi">iscsi</option>
						<option value="fc">fc</option>
						<option value="nfs">nfs</option>
						<option value="smb">smb</option>
						<option value="local">local</option>
					</select>
					<select bind:value={vForm.attached_asset_id}>
						<option value="">Unattached</option>
						{#each assets as a}<option value={a.id}>{a.asset_tag} · {a.name}</option>{/each}
					</select>
					<button class="btn small" onclick={addVolume}>Add volume</button>
				</div>
			{/if}
		</div>
	{/if}
{/if}

<style>
	.card {
		margin-bottom: 16px;
	}
	.cards {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
		margin-bottom: 16px;
	}
	@media (max-width: 720px) {
		.cards {
			grid-template-columns: 1fr;
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
		font-size: 22px;
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
		padding: 7px 8px;
		border-bottom: 1px solid var(--border);
	}
	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.btn.small {
		padding: 4px 10px;
		font-size: 12px;
	}
	.small {
		font-size: 12px;
	}
	.link-danger {
		background: none;
		border: none;
		color: var(--danger);
		cursor: pointer;
		font-size: 12px;
	}
</style>
