<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type {
		AssetType,
		FieldDefinition,
		DataType,
		Unit,
		Lifecycle,
		RelationshipType,
		AutomationRule,
		Currency,
		PortProfile
	} from '$lib/types';
	import { BASEMAPS, DEFAULT_BASEMAP } from '$lib/basemaps';
	import { basemapId as basemapStore } from '$lib/mapconfig';

	let tab = $state<'types' | 'lifecycles' | 'relationships' | 'rules' | 'currencies' | 'ports' | 'maps' | 'sso'>('types');
	let error = $state('');

	const manage = $derived(can($me, 'metadata.manage'));
	const manageSettings = $derived(can($me, 'settings.manage'));

	// ---- GoTrue SSO status (read-only; Azure is configured in deploy env) ----
	let ssoEnabled = $state(false);
	let ssoLoading = $state(true);

	async function loadSsoStatus() {
		ssoLoading = true;
		try {
			const res = await fetch(`${import.meta.env.PUBLIC_API_URL ?? 'http://localhost:5607'}/auth/sso/status`);
			if (res.ok) {
				const data = await res.json();
				ssoEnabled = data?.enabled === true;
			}
		} catch {
			ssoEnabled = false;
		} finally {
			ssoLoading = false;
		}
	}

	// shared lookups
	let types = $state<AssetType[]>([]);
	let dataTypes = $state<DataType[]>([]);
	let units = $state<Unit[]>([]);

	function depth(path: string): number {
		return Math.max(0, path.split('.').length - 1);
	}

	// ---- asset types & fields ----
	let selectedType = $state<AssetType | null>(null);
	let fields = $state<FieldDefinition[]>([]);

	let showTypeForm = $state(false);
	let typeEditId = $state<number | null>(null);
	let tf = $state({ key: '', name: '', parent_id: '', is_abstract: false, lifecycle_id: '', icon: '', sort: 0 });

	let showFieldForm = $state(false);
	let fieldEditId = $state<number | null>(null);
	let ff = $state({
		key: '',
		label: '',
		data_type_id: '',
		required: false,
		unit_id: '',
		enum_options: '',
		help_text: '',
		sort: 0
	});

	let lifecycles = $state<Lifecycle[]>([]);
	let selectedLifecycle = $state<Lifecycle | null>(null);

	let relTypes = $state<RelationshipType[]>([]);
	let showRelForm = $state(false);
	let rf = $state({ key: '', label: '', inverse_label: '', cardinality: 'one_to_many', sort: 0 });

	let rules = $state<AutomationRule[]>([]);
	let showRuleForm = $state(false);
	let af = $state({ key: '', name: '', on_subject: '', condition: '{}', action: '{}', enabled: true, sort: 0 });

	let curList = $state<Currency[]>([]);
	let showCurForm = $state(false);
	let cf = $state({ code: '', name: '', symbol: '', is_default: false, enabled: true, sort: 100 });

	// ---- port profiles ----
	let portProfiles = $state<PortProfile[]>([]);
	let showPpForm = $state(false);
	let ppf = $state({ asset_type_id: '', label: '', port_type: 'ethernet', speed: '', name_prefix: '', start_index: 1, count: 1, sort: 0 });

	async function loadPortProfiles() {
		portProfiles = (await apiGet<PortProfile[]>('/api/metadata/port-profiles')) ?? [];
	}

	function typeName(id: number): string {
		return types.find((t) => t.id === id)?.name ?? `#${id}`;
	}

	async function savePortProfile() {
		if (!ppf.asset_type_id || !ppf.label) return;
		error = '';
		try {
			await apiPost('/api/metadata/port-profiles', {
				asset_type_id: Number(ppf.asset_type_id),
				label: ppf.label,
				port_type: ppf.port_type,
				speed: ppf.speed || null,
				name_prefix: ppf.name_prefix,
				start_index: Number(ppf.start_index),
				count: Number(ppf.count),
				sort: Number(ppf.sort)
			});
			showPpForm = false;
			ppf = { asset_type_id: '', label: '', port_type: 'ethernet', speed: '', name_prefix: '', start_index: 1, count: 1, sort: 0 };
			await loadPortProfiles();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function deletePortProfile(p: PortProfile) {
		if (!confirm(`Delete port profile "${p.label}"?`)) return;
		try {
			await apiDelete(`/api/metadata/port-profiles/${p.id}`);
			await loadPortProfiles();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function loadCommon() {
		types = await apiGet<AssetType[]>('/api/metadata/asset-types');
		dataTypes = await apiGet<DataType[]>('/api/metadata/data-types');
		units = await apiGet<Unit[]>('/api/metadata/units');
		lifecycles = await apiGet<Lifecycle[]>('/api/metadata/lifecycles');
		relTypes = await apiGet<RelationshipType[]>('/api/metadata/relationship-types');
		rules = await apiGet<AutomationRule[]>('/api/metadata/automation-rules');
		curList = await apiGet<Currency[]>('/api/metadata/currencies?all=true');
		await loadPortProfiles();
		await loadMapSetting();
	}

	// ---- map basemap setting ----
	let basemap = $state(DEFAULT_BASEMAP);
	let basemapSaved = $state('');
	async function loadMapSetting() {
		try {
			const rows = await apiGet<{ key: string; value: unknown }[]>('/api/settings');
			const row = rows?.find((r) => r.key === 'map.basemap');
			if (row && row.value) basemap = String(row.value);
			const rr = rows?.find((r) => r.key === 'field.nearby_radius_km');
			if (rr && rr.value != null) nearbyRadius = Number(rr.value) || nearbyRadius;
		} catch {
			/* settings may be unreadable; keep default */
		}
	}

	// ---- field app: "Near me" radius ----
	let nearbyRadius = $state(25);
	let radiusSaved = $state('');
	async function saveRadius() {
		radiusSaved = '';
		try {
			await apiPut('/api/settings/field.nearby_radius_km', { value: Number(nearbyRadius) });
			radiusSaved = 'Saved — the field app will use this radius.';
			setTimeout(() => (radiusSaved = ''), 4000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save radius';
		}
	}
	async function saveBasemap() {
		basemapSaved = '';
		try {
			await apiPut('/api/settings/map.basemap', { value: basemap });
			basemapStore.set(basemap);
			basemapSaved = 'Saved — new maps will use this basemap.';
			setTimeout(() => (basemapSaved = ''), 4000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save basemap';
		}
	}

	async function saveCurrency() {
		if (!cf.code.trim() || !cf.name.trim()) {
			error = 'Code and name are required';
			return;
		}
		try {
			await apiPost('/api/metadata/currencies', {
				code: cf.code.trim().toUpperCase(),
				name: cf.name.trim(),
				symbol: cf.symbol.trim(),
				is_default: cf.is_default,
				enabled: cf.enabled,
				sort: Number(cf.sort) || 100
			});
			cf = { code: '', name: '', symbol: '', is_default: false, enabled: true, sort: 100 };
			showCurForm = false;
			curList = await apiGet<Currency[]>('/api/metadata/currencies?all=true');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save currency';
		}
	}

	function editCurrency(c: Currency) {
		cf = { code: c.code, name: c.name, symbol: c.symbol, is_default: c.is_default, enabled: c.enabled, sort: c.sort };
		showCurForm = true;
	}

	async function toggleCurrency(c: Currency, enabled: boolean) {
		try {
			await apiPost('/api/metadata/currencies', { ...c, enabled });
			curList = await apiGet<Currency[]>('/api/metadata/currencies?all=true');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function makeDefault(c: Currency) {
		try {
			await apiPost('/api/metadata/currencies', { ...c, is_default: true, enabled: true });
			curList = await apiGet<Currency[]>('/api/metadata/currencies?all=true');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	onMount(async () => {
		try {
			await Promise.all([loadCommon(), loadSsoStatus()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	});

	async function selectType(t: AssetType) {
		selectedType = t;
		fields = await apiGet<FieldDefinition[]>(`/api/metadata/asset-types/${t.id}/fields`);
	}

	function openTypeCreate(parent: AssetType | null) {
		typeEditId = null;
		tf = {
			key: '',
			name: '',
			parent_id: parent ? String(parent.id) : '',
			is_abstract: false,
			lifecycle_id: '',
			icon: '',
			sort: 0
		};
		showTypeForm = true;
	}
	function openTypeEdit(t: AssetType) {
		typeEditId = t.id;
		tf = {
			key: t.key,
			name: t.name,
			parent_id: t.parent_id ? String(t.parent_id) : '',
			is_abstract: t.is_abstract,
			lifecycle_id: t.lifecycle_id ? String(t.lifecycle_id) : '',
			icon: t.icon ?? '',
			sort: t.sort
		};
		showTypeForm = true;
	}
	async function saveType(e: Event) {
		e.preventDefault();
		error = '';
		try {
			const body = {
				name: tf.name,
				is_abstract: tf.is_abstract,
				lifecycle_id: tf.lifecycle_id ? Number(tf.lifecycle_id) : null,
				icon: tf.icon,
				sort: Number(tf.sort)
			};
			if (typeEditId) {
				await apiPut(`/api/metadata/asset-types/${typeEditId}`, body);
			} else {
				await apiPost('/api/metadata/asset-types', {
					...body,
					key: tf.key,
					parent_id: tf.parent_id ? Number(tf.parent_id) : null
				});
			}
			showTypeForm = false;
			await loadCommon();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}
	async function removeType(t: AssetType) {
		if (!confirm(`Delete asset type "${t.name}"?`)) return;
		try {
			await apiDelete(`/api/metadata/asset-types/${t.id}`);
			if (selectedType?.id === t.id) selectedType = null;
			await loadCommon();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete failed';
		}
	}

	function openFieldCreate() {
		fieldEditId = null;
		ff = { key: '', label: '', data_type_id: '', required: false, unit_id: '', enum_options: '', help_text: '', sort: 0 };
		showFieldForm = true;
	}
	function openFieldEdit(fd: FieldDefinition) {
		fieldEditId = fd.id;
		ff = {
			key: fd.key,
			label: fd.label,
			data_type_id: String(fd.data_type_id),
			required: fd.required,
			unit_id: fd.unit_id ? String(fd.unit_id) : '',
			enum_options: Array.isArray(fd.enum_options) ? fd.enum_options.map((o) => String(o)).join(', ') : '',
			help_text: fd.help_text ?? '',
			sort: fd.sort
		};
		showFieldForm = true;
	}
	async function saveField(e: Event) {
		e.preventDefault();
		if (!selectedType) return;
		error = '';
		const enumArr = ff.enum_options
			? ff.enum_options.split(',').map((s) => s.trim()).filter(Boolean)
			: [];
		const body = {
			label: ff.label,
			data_type_id: Number(ff.data_type_id),
			required: ff.required,
			unit_id: ff.unit_id ? Number(ff.unit_id) : null,
			enum_options: enumArr,
			help_text: ff.help_text,
			sort: Number(ff.sort)
		};
		try {
			if (fieldEditId) {
				await apiPut(`/api/metadata/field-definitions/${fieldEditId}`, body);
			} else {
				await apiPost('/api/metadata/field-definitions', {
					...body,
					asset_type_id: selectedType.id,
					key: ff.key
				});
			}
			showFieldForm = false;
			await selectType(selectedType);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}
	async function removeField(fd: FieldDefinition) {
		if (!confirm(`Delete field "${fd.label}"?`)) return;
		try {
			await apiDelete(`/api/metadata/field-definitions/${fd.id}`);
			if (selectedType) await selectType(selectedType);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Delete failed';
		}
	}

	async function selectLifecycle(lc: Lifecycle) {
		selectedLifecycle = await apiGet<Lifecycle>(`/api/metadata/lifecycles/${lc.id}`);
	}

	async function saveRel(e: Event) {
		e.preventDefault();
		error = '';
		try {
			await apiPost('/api/metadata/relationship-types', {
				key: rf.key,
				label: rf.label,
				inverse_label: rf.inverse_label,
				cardinality: rf.cardinality,
				sort: Number(rf.sort)
			});
			showRelForm = false;
			rf = { key: '', label: '', inverse_label: '', cardinality: 'one_to_many', sort: 0 };
			await loadCommon();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}

	async function saveRule(e: Event) {
		e.preventDefault();
		error = '';
		let condition: Record<string, unknown> = {};
		let action: Record<string, unknown> = {};
		try {
			condition = JSON.parse(af.condition || '{}');
			action = JSON.parse(af.action || '{}');
		} catch {
			error = 'Condition and action must be valid JSON';
			return;
		}
		try {
			await apiPost('/api/metadata/automation-rules', {
				key: af.key,
				name: af.name,
				on_subject: af.on_subject,
				condition,
				action,
				enabled: af.enabled,
				sort: Number(af.sort)
			});
			showRuleForm = false;
			af = { key: '', name: '', on_subject: '', condition: '{}', action: '{}', enabled: true, sort: 0 };
			await loadCommon();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Save failed';
		}
	}

	function dataTypeLabel(id: number): string {
		return dataTypes.find((d) => d.id === id)?.label ?? String(id);
	}
</script>

<div class="topbar"><h1>Metadata</h1></div>

<div class="tabs">
	<button type="button" class="tab" class:active={tab === 'types'} onclick={() => (tab = 'types')}>Asset Types & Fields</button>
	<button type="button" class="tab" class:active={tab === 'lifecycles'} onclick={() => (tab = 'lifecycles')}>Lifecycles</button>
	<button type="button" class="tab" class:active={tab === 'relationships'} onclick={() => (tab = 'relationships')}>Relationship Types</button>
	<button type="button" class="tab" class:active={tab === 'rules'} onclick={() => (tab = 'rules')}>Automation Rules</button>
	<button type="button" class="tab" class:active={tab === 'currencies'} onclick={() => (tab = 'currencies')}>Currencies</button>
	<button type="button" class="tab" class:active={tab === 'ports'} onclick={() => (tab = 'ports')}>Port Profiles</button>
	<button type="button" class="tab" class:active={tab === 'maps'} onclick={() => (tab = 'maps')}>Maps</button>
	<button type="button" class="tab" class:active={tab === 'sso'} onclick={() => (tab = 'sso')}>SSO</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if tab === 'types'}
	<div class="grid cols-2">
		<div class="card">
			<div class="row" style="justify-content:space-between">
				<h3 style="margin-top:0">Asset type tree</h3>
				{#if manage}<button class="btn small" onclick={() => openTypeCreate(null)}>+ Root type</button>{/if}
			</div>
			<table>
				<tbody>
					{#each types as t}
						<tr class:active-row={selectedType?.id === t.id}>
							<td style="padding-left:{12 + depth(t.path) * 18}px" onclick={() => selectType(t)}>
								{t.name}
								{#if t.is_abstract}<span class="badge" style="margin-left:6px">abstract</span>{/if}
							</td>
							<td style="text-align:right; white-space:nowrap">
								{#if manage}
									<button class="btn secondary small" onclick={() => openTypeCreate(t)}>+ Child</button>
									<button class="btn secondary small" onclick={() => openTypeEdit(t)}>Edit</button>
									<button class="btn danger small" onclick={() => removeType(t)}>Del</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>

			{#if showTypeForm}
				<form onsubmit={saveType} style="margin-top:14px; border-top:1px solid var(--border); padding-top:14px">
					<h4 style="margin-top:0">{typeEditId ? 'Edit type' : 'New type'}</h4>
					{#if !typeEditId}
						<div class="field"><label>Key * (lowercase_underscore)</label><input bind:value={tf.key} required pattern="[a-z0-9_]+" /></div>
						<div class="field">
							<label>Parent</label>
							<select bind:value={tf.parent_id}>
								<option value="">— (root)</option>
								{#each types as t}<option value={String(t.id)}>{'— '.repeat(depth(t.path))}{t.name}</option>{/each}
							</select>
						</div>
					{/if}
					<div class="field"><label>Name *</label><input bind:value={tf.name} required /></div>
					<div class="field">
						<label>Lifecycle</label>
						<select bind:value={tf.lifecycle_id}>
							<option value="">— (inherit)</option>
							{#each lifecycles as lc}<option value={String(lc.id)}>{lc.name}</option>{/each}
						</select>
					</div>
					<div class="field">
						<label><input type="checkbox" style="width:auto" bind:checked={tf.is_abstract} /> Abstract (cannot hold assets directly)</label>
					</div>
					<div class="row">
						<button class="btn" type="submit">Save</button>
						<button class="btn secondary" type="button" onclick={() => (showTypeForm = false)}>Cancel</button>
					</div>
				</form>
			{/if}
		</div>

		<div class="card">
			<div class="row" style="justify-content:space-between">
				<h3 style="margin-top:0">Fields {selectedType ? `· ${selectedType.name}` : ''}</h3>
				{#if manage && selectedType}<button class="btn small" onclick={openFieldCreate}>+ Field</button>{/if}
			</div>
			{#if !selectedType}
				<p class="muted">Select an asset type to see its fields (inherited fields included).</p>
			{:else}
				<table>
					<thead><tr><th>Label</th><th>Key</th><th>Type</th><th>Req</th><th></th></tr></thead>
					<tbody>
						{#each fields as fd}
							<tr>
								<td>{fd.label}</td>
								<td class="muted">{fd.key}</td>
								<td>{dataTypeLabel(fd.data_type_id)}{fd.unit ? ` (${fd.unit.symbol})` : ''}</td>
								<td>{fd.required ? '✓' : ''}</td>
								<td style="text-align:right; white-space:nowrap">
									{#if manage && fd.asset_type_id === selectedType.id}
										<button class="btn secondary small" onclick={() => openFieldEdit(fd)}>Edit</button>
										<button class="btn danger small" onclick={() => removeField(fd)}>Del</button>
									{:else}
										<span class="muted" style="font-size:12px">inherited</span>
									{/if}
								</td>
							</tr>
						{/each}
						{#if fields.length === 0}<tr><td colspan="5" class="muted">No fields.</td></tr>{/if}
					</tbody>
				</table>

				{#if showFieldForm}
					<form onsubmit={saveField} style="margin-top:14px; border-top:1px solid var(--border); padding-top:14px">
						<h4 style="margin-top:0">{fieldEditId ? 'Edit field' : 'New field'}</h4>
						{#if !fieldEditId}
							<div class="field"><label>Key * (lowercase_underscore)</label><input bind:value={ff.key} required pattern="[a-z0-9_]+" /></div>
						{/if}
						<div class="field"><label>Label *</label><input bind:value={ff.label} required /></div>
						<div class="field">
							<label>Data type *</label>
							<select bind:value={ff.data_type_id} required>
								<option value="">Select…</option>
								{#each dataTypes as d}<option value={String(d.id)}>{d.label}</option>{/each}
							</select>
						</div>
						<div class="field">
							<label>Unit</label>
							<select bind:value={ff.unit_id}>
								<option value="">—</option>
								{#each units as u}<option value={String(u.id)}>{u.label} ({u.symbol})</option>{/each}
							</select>
						</div>
						<div class="field"><label>Enum options (comma separated, for Choice type)</label><input bind:value={ff.enum_options} /></div>
						<div class="field"><label>Help text</label><input bind:value={ff.help_text} /></div>
						<div class="field"><label><input type="checkbox" style="width:auto" bind:checked={ff.required} /> Required</label></div>
						<div class="row">
							<button class="btn" type="submit">Save</button>
							<button class="btn secondary" type="button" onclick={() => (showFieldForm = false)}>Cancel</button>
						</div>
					</form>
				{/if}
			{/if}
		</div>
	</div>
{:else if tab === 'lifecycles'}
	<div class="grid cols-2">
		<div class="card">
			<h3 style="margin-top:0">Lifecycles</h3>
			<table>
				<tbody>
					{#each lifecycles as lc}
						<tr class:active-row={selectedLifecycle?.id === lc.id}>
							<td onclick={() => selectLifecycle(lc)}>{lc.name}<div class="muted" style="font-size:12px">{lc.description}</div></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<div class="card">
			{#if !selectedLifecycle}
				<p class="muted">Select a lifecycle to inspect its states and transitions.</p>
			{:else}
				<h3 style="margin-top:0">{selectedLifecycle.name}</h3>
				<h4>States</h4>
				<div class="pill-actions">
					{#each selectedLifecycle.states ?? [] as st}
						<span class="badge" style="border-color:{st.color}">
							{st.label}{#if st.is_initial} · start{/if}{#if st.is_terminal} · end{/if}
						</span>
					{/each}
				</div>
				<h4 style="margin-top:16px">Transitions</h4>
				<table>
					<thead><tr><th>Action</th><th>From → To</th><th>Permission</th></tr></thead>
					<tbody>
						{#each selectedLifecycle.transitions ?? [] as tr}
							{@const states = selectedLifecycle.states ?? []}
							<tr>
								<td>{tr.label}</td>
								<td class="muted">
									{tr.from_state_id ? (states.find((s) => s.id === tr.from_state_id)?.label ?? '?') : 'any'}
									→ {states.find((s) => s.id === tr.to_state_id)?.label ?? '?'}
								</td>
								<td class="muted">{tr.required_permission || '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	</div>
{:else if tab === 'relationships'}
	<div class="topbar" style="margin-bottom:12px">
		<h3 style="margin:0">Relationship types</h3>
		{#if manage}<button class="btn small" onclick={() => (showRelForm = !showRelForm)}>+ New</button>{/if}
	</div>
	{#if showRelForm}
		<div class="card" style="margin-bottom:16px">
			<form onsubmit={saveRel}>
				<div class="grid cols-2">
					<div class="field"><label>Key * (lowercase_underscore)</label><input bind:value={rf.key} required pattern="[a-z0-9_]+" /></div>
					<div class="field">
						<label>Cardinality</label>
						<select bind:value={rf.cardinality}>
							<option value="one_to_one">one to one</option>
							<option value="one_to_many">one to many</option>
							<option value="many_to_many">many to many</option>
						</select>
					</div>
					<div class="field"><label>Label *</label><input bind:value={rf.label} required /></div>
					<div class="field"><label>Inverse label</label><input bind:value={rf.inverse_label} /></div>
				</div>
				<div class="row">
					<button class="btn" type="submit">Save</button>
					<button class="btn secondary" type="button" onclick={() => (showRelForm = false)}>Cancel</button>
				</div>
			</form>
		</div>
	{/if}
	<div class="card">
		<table>
			<thead><tr><th>Label</th><th>Inverse</th><th>Key</th><th>Cardinality</th></tr></thead>
			<tbody>
				{#each relTypes as rt}
					<tr><td>{rt.label}</td><td>{rt.inverse_label || '—'}</td><td class="muted">{rt.key}</td><td>{rt.cardinality}</td></tr>
				{/each}
				{#if relTypes.length === 0}<tr><td colspan="4" class="muted">None.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'rules'}
	<div class="topbar" style="margin-bottom:12px">
		<h3 style="margin:0">Automation rules</h3>
		{#if manage}<button class="btn small" onclick={() => (showRuleForm = !showRuleForm)}>+ New</button>{/if}
	</div>
	{#if showRuleForm}
		<div class="card" style="margin-bottom:16px">
			<form onsubmit={saveRule}>
				<div class="grid cols-2">
					<div class="field"><label>Key * (lowercase_underscore)</label><input bind:value={af.key} required pattern="[a-z0-9_]+" /></div>
					<div class="field"><label>Name *</label><input bind:value={af.name} required /></div>
					<div class="field"><label>On subject *</label><input bind:value={af.on_subject} placeholder="itam.asset.state_changed" required /></div>
					<div class="field"><label><input type="checkbox" style="width:auto" bind:checked={af.enabled} /> Enabled</label></div>
				</div>
				<div class="field"><label>Condition (JSON)</label><textarea bind:value={af.condition} rows="3"></textarea></div>
				<div class="field"><label>Action (JSON)</label><textarea bind:value={af.action} rows="3"></textarea></div>
				<div class="row">
					<button class="btn" type="submit">Save</button>
					<button class="btn secondary" type="button" onclick={() => (showRuleForm = false)}>Cancel</button>
				</div>
			</form>
		</div>
	{/if}
	<div class="card">
		<table>
			<thead><tr><th>Name</th><th>On subject</th><th>Enabled</th><th>Action</th></tr></thead>
			<tbody>
				{#each rules as ru}
					<tr>
						<td>{ru.name}<div class="muted" style="font-size:12px">{ru.key}</div></td>
						<td class="muted">{ru.on_subject}</td>
						<td>{ru.enabled ? '✓' : '—'}</td>
						<td class="muted" style="font-size:12px">{JSON.stringify(ru.action)}</td>
					</tr>
				{/each}
				{#if rules.length === 0}<tr><td colspan="4" class="muted">None.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'currencies'}
	<div class="row" style="justify-content:space-between; align-items:center; margin-bottom:12px">
		<div>
			<h3 style="margin:0">Currencies</h3>
			<p class="muted" style="margin:4px 0 0">The default currency is the reporting currency used across costs, budgets and analytics. Disable a currency to hide it from pickers.</p>
		</div>
		{#if manage}<button class="btn small" onclick={() => { showCurForm = !showCurForm; cf = { code: '', name: '', symbol: '', is_default: false, enabled: true, sort: 100 }; }}>+ New</button>{/if}
	</div>
	{#if showCurForm}
		<div class="card" style="margin-bottom:16px">
			<form onsubmit={(e) => { e.preventDefault(); saveCurrency(); }}>
				<div class="grid cols-2">
					<div class="field"><label>Code * (ISO 4217)</label><input bind:value={cf.code} maxlength="3" placeholder="USD" required /></div>
					<div class="field"><label>Name *</label><input bind:value={cf.name} placeholder="US Dollar" required /></div>
					<div class="field"><label>Symbol</label><input bind:value={cf.symbol} placeholder="$" /></div>
					<div class="field"><label>Sort</label><input type="number" bind:value={cf.sort} /></div>
					<div class="field"><label><input type="checkbox" style="width:auto" bind:checked={cf.enabled} /> Enabled</label></div>
					<div class="field"><label><input type="checkbox" style="width:auto" bind:checked={cf.is_default} /> Default (reporting) currency</label></div>
				</div>
				<div class="row">
					<button class="btn" type="submit">Save</button>
					<button class="btn secondary" type="button" onclick={() => (showCurForm = false)}>Cancel</button>
				</div>
			</form>
		</div>
	{/if}
	<div class="card">
		<table>
			<thead><tr><th>Code</th><th>Name</th><th>Symbol</th><th>Default</th><th>Enabled</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each curList as c}
					<tr>
						<td><strong>{c.code}</strong></td>
						<td>{c.name}</td>
						<td>{c.symbol || '—'}</td>
						<td>{#if c.is_default}<span class="badge">default</span>{:else if manage}<button class="link" onclick={() => makeDefault(c)}>set default</button>{:else}—{/if}</td>
						<td>{c.enabled ? '✓' : '—'}</td>
						{#if manage}
							<td style="text-align:right; white-space:nowrap">
								<button class="link" onclick={() => editCurrency(c)}>edit</button>
								{#if !c.is_default}
									<button class="link" onclick={() => toggleCurrency(c, !c.enabled)}>{c.enabled ? 'disable' : 'enable'}</button>
								{/if}
							</td>
						{/if}
					</tr>
				{/each}
				{#if curList.length === 0}<tr><td colspan={manage ? 6 : 5} class="muted">None.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'ports'}
	<div class="row" style="justify-content:space-between; align-items:center; margin-bottom:12px">
		<div>
			<h3 style="margin:0">Port Profiles</h3>
			<p class="muted" style="margin:4px 0 0">Templates that auto-generate physical ports for an asset type. On an asset's Ports tab, "Generate from profile" creates ports named <code>{'{prefix}{n}'}</code> from each block.</p>
		</div>
		{#if manage}<button class="btn small" onclick={() => { showPpForm = !showPpForm; ppf = { asset_type_id: '', label: '', port_type: 'ethernet', speed: '', name_prefix: '', start_index: 1, count: 1, sort: 0 }; }}>+ New</button>{/if}
	</div>
	{#if showPpForm}
		<div class="card" style="margin-bottom:16px">
			<form onsubmit={(e) => { e.preventDefault(); savePortProfile(); }}>
				<div class="grid cols-2">
					<div class="field">
						<label>Asset type *</label>
						<select bind:value={ppf.asset_type_id} required>
							<option value="">Select…</option>
							{#each types.filter((t) => !t.is_abstract) as t}<option value={t.id}>{t.name}</option>{/each}
						</select>
					</div>
					<div class="field"><label>Block label *</label><input bind:value={ppf.label} placeholder="Access ports" required /></div>
					<div class="field">
						<label>Port type</label>
						<select bind:value={ppf.port_type}>
							<option value="ethernet">ethernet</option>
							<option value="sfp">sfp</option>
							<option value="sfp_plus">sfp_plus</option>
							<option value="qsfp">qsfp</option>
							<option value="passthrough">passthrough</option>
							<option value="power">power</option>
							<option value="console">console</option>
							<option value="usb">usb</option>
							<option value="other">other</option>
						</select>
					</div>
					<div class="field"><label>Speed</label><input bind:value={ppf.speed} placeholder="1G, 10G…" /></div>
					<div class="field"><label>Name prefix</label><input bind:value={ppf.name_prefix} placeholder="Gi1/0/" /></div>
					<div class="field"><label>Start index</label><input type="number" bind:value={ppf.start_index} /></div>
					<div class="field"><label>Count</label><input type="number" min="1" max="1024" bind:value={ppf.count} /></div>
					<div class="field"><label>Sort</label><input type="number" bind:value={ppf.sort} /></div>
				</div>
				<div class="row">
					<button class="btn" type="submit">Save</button>
					<button class="btn secondary" type="button" onclick={() => (showPpForm = false)}>Cancel</button>
				</div>
			</form>
		</div>
	{/if}
	<div class="card">
		<table>
			<thead><tr><th>Asset type</th><th>Block</th><th>Type</th><th>Speed</th><th>Naming</th><th>Count</th>{#if manage}<th></th>{/if}</tr></thead>
			<tbody>
				{#each portProfiles as p}
					<tr>
						<td><strong>{typeName(p.asset_type_id)}</strong></td>
						<td>{p.label}</td>
						<td><span class="badge">{p.port_type}</span></td>
						<td class="muted">{p.speed || '—'}</td>
						<td class="muted">{p.name_prefix}{p.start_index}…{p.name_prefix}{p.start_index + p.count - 1}</td>
						<td>{p.count}</td>
						{#if manage}<td style="text-align:right"><button class="link" onclick={() => deletePortProfile(p)}>delete</button></td>{/if}
					</tr>
				{/each}
				{#if portProfiles.length === 0}<tr><td colspan={manage ? 7 : 6} class="muted">No port profiles defined.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'maps'}
	<div class="row" style="margin-bottom:12px">
		<div>
			<h3 style="margin:0">Maps</h3>
			<p class="muted" style="margin:4px 0 0">Default basemap for the Locations and Assets maps. Users can still switch live via the layer control on each map.</p>
		</div>
	</div>
	<div class="card" style="max-width:560px">
		<div class="field">
			<label>Default basemap</label>
			<select bind:value={basemap} disabled={!manage}>
				{#each BASEMAPS as b}<option value={b.id}>{b.label}</option>{/each}
			</select>
		</div>
		<p class="muted" style="font-size:12px; margin:8px 0 0">
			Google layers use the public <code>mt0–mt3</code> tile servers; Carto &amp; Esri need no key. OSM is the classic fallback.
		</p>
		{#if manage}
			<div class="row" style="margin-top:14px; align-items:center; gap:10px">
				<button class="btn" onclick={saveBasemap}>Save basemap</button>
				{#if basemapSaved}<span class="muted" style="color:var(--success)">{basemapSaved}</span>{/if}
			</div>
		{/if}
	</div>

	<div class="row" style="margin:20px 0 12px">
		<div>
			<h3 style="margin:0">Mobile field app</h3>
			<p class="muted" style="margin:4px 0 0">Controls the "Near me" feature: GPS only snaps to a store/site within this radius.</p>
		</div>
	</div>
	<div class="card" style="max-width:560px">
		<div class="field">
			<label>"Near me" radius (km)</label>
			<input type="number" min="0.1" step="0.1" bind:value={nearbyRadius} disabled={!manage} />
		</div>
		<p class="muted" style="font-size:12px; margin:8px 0 0">
			If no location with coordinates falls within this distance, the app tells the user nothing is nearby instead of jumping to a far-away site.
		</p>
		{#if manage}
			<div class="row" style="margin-top:14px; align-items:center; gap:10px">
				<button class="btn" onclick={saveRadius}>Save radius</button>
				{#if radiusSaved}<span class="muted" style="color:var(--success)">{radiusSaved}</span>{/if}
			</div>
		{/if}
	</div>
{:else if tab === 'sso'}
	<div class="row" style="margin-bottom:12px">
		<div>
			<h3 style="margin:0">Microsoft Entra ID (Azure AD) Single Sign-On</h3>
			<p class="muted" style="margin:4px 0 0">
				All authentication — email/password and Microsoft — goes through <strong>GoTrue</strong>.
				Configure Azure in <code>deploy/.env</code> and restart the <code>auth</code> container.
			</p>
		</div>
	</div>
	<div class="card" style="max-width:640px">
		<div class="field">
			<label>Status</label>
			{#if ssoLoading}
				<p class="muted" style="margin:0">Checking GoTrue…</p>
			{:else if ssoEnabled}
				<p style="margin:0"><span class="badge success">Microsoft sign-in enabled</span></p>
			{:else}
				<p style="margin:0"><span class="badge warning">Not configured</span></p>
			{/if}
		</div>
		<h4 style="margin:20px 0 8px">Setup (deploy environment)</h4>
		<ol class="muted" style="margin:0; padding-left:20px; font-size:13px; line-height:1.6">
			<li>Create an Entra app registration with redirect URI <code>http://localhost:5606/callback</code> (GoTrue, not the API).</li>
			<li>Set in <code>deploy/.env</code>: <code>AZURE_ENABLED=true</code>, <code>AZURE_CLIENT_ID</code>, <code>AZURE_CLIENT_SECRET</code>, <code>AZURE_URL</code>.</li>
			<li>Add client origins to <code>GOTRUE_URI_ALLOW_LIST</code> (web, Flutter web, <code>itam://sso-callback</code> for Android).</li>
			<li>Restart auth: <code>docker compose restart auth</code></li>
		</ol>
		<p class="muted" style="font-size:12px; margin:16px 0 0">
			See <code>docs/azure-ad-sso.md</code>. The first user matching <code>ADMIN_EMAIL</code> becomes superuser on first login.
		</p>
	</div>
{/if}

<style>
	.active-row > td { background: var(--surface-2); }
	.link {
		background: none;
		border: none;
		color: var(--accent, #3b82f6);
		cursor: pointer;
		font: inherit;
		padding: 0 6px;
		text-decoration: underline;
	}
</style>
