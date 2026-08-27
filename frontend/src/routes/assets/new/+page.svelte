<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { apiGet, apiPost } from '$lib/api';
	import DynamicForm from '$lib/components/DynamicForm.svelte';
	import type { AssetType, FieldDefinition, Location, OrgUnit, Asset } from '$lib/types';

	let types = $state<AssetType[]>([]);
	let locations = $state<Location[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let fields = $state<FieldDefinition[]>([]);

	let assetTag = $state('');
	let name = $state('');
	let serial = $state('');
	let assetTypeId = $state<number | ''>('');
	let locationId = $state('');
	let ownerOrgUnitId = $state('');
	let vendor = $state('');
	let notes = $state('');
	let attributes = $state<Record<string, unknown>>({});

	let error = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let busy = $state(false);

	onMount(async () => {
		types = (await apiGet<AssetType[]>('/api/metadata/asset-types')).filter((t) => !t.is_abstract);
		locations = await apiGet<Location[]>('/api/locations');
		orgUnits = await apiGet<OrgUnit[]>('/api/org-units');

		const pre = $page.url.searchParams.get('type');
		if (pre) {
			const match = types.find((t) => t.key === pre);
			if (match) {
				assetTypeId = match.id;
				await onTypeChange();
			}
		}
	});

	async function onTypeChange() {
		attributes = {};
		fields = [];
		if (assetTypeId === '') return;
		fields = await apiGet<FieldDefinition[]>(`/api/metadata/asset-types/${assetTypeId}/fields`);
	}

	async function submit(e: Event) {
		e.preventDefault();
		error = '';
		fieldErrors = {};
		busy = true;
		try {
			const created = await apiPost<Asset>('/api/assets', {
				asset_tag: assetTag,
				name,
				serial,
				asset_type_id: Number(assetTypeId),
				location_id: locationId || null,
				owner_org_unit_id: ownerOrgUnitId || null,
				vendor,
				notes,
				attributes
			});
			goto(`/assets/${created.id}`);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to create';
			try {
				const parsed = JSON.parse(msg);
				if (parsed.fields) fieldErrors = parsed.fields;
				error = parsed.error ?? msg;
			} catch {
				error = msg;
			}
		} finally {
			busy = false;
		}
	}
</script>

<div class="topbar">
	<h1>New asset</h1>
	<button class="btn secondary" onclick={() => goto('/assets')}>Cancel</button>
</div>

<form onsubmit={submit}>
	<div class="grid cols-2">
		<div class="card">
			<h3 style="margin-top:0">Core</h3>
			<div class="field">
				<label>Asset type *</label>
				<select bind:value={assetTypeId} onchange={onTypeChange} required>
					<option value="">Select a type…</option>
					{#each types as t}<option value={t.id}>{t.name}</option>{/each}
				</select>
			</div>
			<div class="field"><label>Asset tag *</label><input bind:value={assetTag} required /></div>
			<div class="field"><label>Name *</label><input bind:value={name} required /></div>
			<div class="field"><label>Serial number</label><input bind:value={serial} /></div>
			<div class="field">
				<label>Location</label>
				<select bind:value={locationId}>
					<option value="">—</option>
					{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
				</select>
			</div>
			<div class="field">
				<label>Owner (org unit)</label>
				<select bind:value={ownerOrgUnitId}>
					<option value="">—</option>
					{#each orgUnits as o}<option value={o.id}>{o.name}</option>{/each}
				</select>
			</div>
			<div class="field"><label>Vendor</label><input bind:value={vendor} /></div>
			<div class="field"><label>Notes</label><textarea bind:value={notes} rows="2"></textarea></div>
		</div>

		<div class="card">
			<h3 style="margin-top:0">Attributes</h3>
			{#if assetTypeId === ''}
				<p class="muted">Select an asset type to see its fields.</p>
			{:else if fields.length === 0}
				<p class="muted">This type has no custom fields.</p>
			{:else}
				<DynamicForm {fields} bind:values={attributes} />
			{/if}
			{#if Object.keys(fieldErrors).length}
				<div class="error">
					{#each Object.entries(fieldErrors) as [k, v]}<div>{k}: {v}</div>{/each}
				</div>
			{/if}
		</div>
	</div>

	{#if error}<p class="error">{error}</p>{/if}
	<div style="margin-top:16px">
		<button class="btn" type="submit" disabled={busy}>{busy ? 'Saving…' : 'Create asset'}</button>
	</div>
</form>
