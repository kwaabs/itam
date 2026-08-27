<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import { currencies, loadCurrencies, defaultCurrency } from '$lib/currencies';
	import DynamicForm from '$lib/components/DynamicForm.svelte';
	import AssetCodes from '$lib/components/AssetCodes.svelte';
	import { browser } from '$app/environment';
	import { orgBreadcrumb, kindLabel, depth } from '$lib/org';
	import { showInfrastructure, mountLabel, powerLabel, isInfrastructureAsset } from '$lib/dcim';
	import type {
		Asset,
		AssetPlacement,
		FieldDefinition,
		LifecycleTransition,
		Location,
		OrgUnit,
		OrgUnitKind,
		Person,
		Port,
		Rack,
		Installation,
		Software,
		IPAddress,
		Subnet,
		CableHop
	} from '$lib/types';

	const id = $page.params.id;

	let asset = $state<Asset | null>(null);
	let fields = $state<FieldDefinition[]>([]);
	let transitions = $state<LifecycleTransition[]>([]);
	let locations = $state<Location[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);
	let orgKinds = $state<OrgUnitKind[]>([]);
	let people = $state<Person[]>([]);
	let history = $state<{ lifecycle: any[]; assignments: any[] }>({ lifecycle: [], assignments: [] });
	let relationships = $state<any[]>([]);
	let timeline = $state<any[]>([]);
	let costs = $state<any[]>([]);
	let netCost = $state(0);
	let costCurrency = $state('USD');

	let tab = $state<'overview' | 'custody' | 'timeline' | 'attributes' | 'costs' | 'relationships' | 'ports' | 'software' | 'infrastructure'>('overview');

	let ports = $state<Port[]>([]);
	let placement = $state<AssetPlacement | null>(null);
	let placementError = $state('');
	let racks = $state<Rack[]>([]);
	let mountForm = $state({ rack_id: '', position: 1, u_height: 1, face: 'front' });
	const showInfra = $derived(showInfrastructure(asset, placement));
	const infraManage = $derived(can($me, 'dcim.manage'));
	const infraView = $derived.by((): AssetPlacement => {
		if (placement) return placement;
		const attrs = asset?.attributes ?? {};
		const ru = attrs.rack_units;
		const pw = attrs.power_watts ?? attrs.output_watts;
		const wt = attrs.weight_kg;
		return {
			port_count: ports.length,
			rack_units: typeof ru === 'number' ? ru : typeof ru === 'string' && ru !== '' ? Number(ru) : null,
			power_watts: typeof pw === 'number' ? pw : typeof pw === 'string' && pw !== '' ? Number(pw) : null,
			weight_kg: typeof wt === 'number' ? wt : typeof wt === 'string' && wt !== '' ? Number(wt) : null
		};
	});
	const isPatch = $derived(asset?.asset_type?.key === 'patch_panel');
	const frontPorts = $derived(ports.filter((p) => /^F/i.test(p.name)));
	const rearPorts = $derived(ports.filter((p) => /^R/i.test(p.name)));
	let ips = $state<IPAddress[]>([]);
	let subnets = $state<Subnet[]>([]);
	let newIP = $state<{ address: string; port_id: string; dns_name: string }>({ address: '', port_id: '', dns_name: '' });

	let installs = $state<Installation[]>([]);
	let catalog = $state<Software[]>([]);
	let newInstall = $state<{ software_id: string; version: string }>({ software_id: '', version: '' });
	let newPort = $state<{ name: string; port_type: string; speed: string }>({
		name: '',
		port_type: 'ethernet',
		speed: ''
	});
	// Cabling: which local port we are connecting, plus the available remote ports.
	let connectFor = $state<Port | null>(null);
	let freePorts = $state<Port[]>([]);
	let connectTo = $state('');
	let connectCable = $state('cat6');
	let connectLabel = $state('');
	let connectLength = $state('');
	// Cable trace result drawer.
	let traceFor = $state<Port | null>(null);
	let traceHops = $state<CableHop[]>([]);

	let newCost = $state<{ kind: string; amount: string; currency: string; vendor: string; reference: string; note: string }>({
		kind: 'repair',
		amount: '',
		currency: 'USD',
		vendor: '',
		reference: '',
		note: ''
	});
	let error = $state('');
	let busy = $state(false);

	let assignPerson = $state('');
	let assignLocationTo = $state('');
	const assignDefaultLocationHint = $derived.by(() => {
		if (!assignPerson || assignLocationTo) return '';
		const person = people.find((p) => p.id === assignPerson);
		if (!person?.org_unit_id) return '';
		const ou = orgUnits.find((u) => u.id === person.org_unit_id);
		if (!ou?.default_location_id) return '';
		const loc = locations.find((l) => l.id === ou.default_location_id);
		return loc ? `If left blank, assign uses ${ou.name}'s default location: ${loc.name}.` : '';
	});
	const ingestMatchLabel = $derived.by(() => {
		const m = asset?.attributes?.ingest_match_method;
		const c = asset?.attributes?.ingest_match_confidence;
		if (!m) return '';
		return `Discovery match: ${m}${c ? ` (${c} confidence)` : ''}`;
	});
	const openUnacked = $derived(
		(history.assignments ?? []).some((a) => a.kind === 'assign' && !a.returned_at && !a.acknowledged_at)
	);
	let ownerOrgUnit = $state('');
	let transferTo = $state('');
	let transferReason = $state('');

	let txModal = $state<{ key: string; label: string } | null>(null);
	let txFields = $state<any[]>([]);
	let txValues = $state<Record<string, unknown>>({});
	let txNote = $state('');

	let editing = $state(false);
	let editValues = $state<Record<string, unknown>>({});
	let editLat = $state('');
	let editLng = $state('');

	async function loadAll() {
		asset = await apiGet<Asset>(`/api/assets/${id}`);
		fields = await apiGet<FieldDefinition[]>(`/api/metadata/asset-types/${asset.asset_type_id}/fields`);
		transitions = await apiGet<LifecycleTransition[]>(`/api/assets/${id}/transitions`);
		const h = await apiGet<{ lifecycle: any[]; assignments: any[] }>(`/api/assets/${id}/history`);
		history = {
			lifecycle: h?.lifecycle ?? [],
			assignments: h?.assignments ?? []
		};
		timeline = (await apiGet<any[]>(`/api/assets/${id}/timeline`)) ?? [];
		relationships = (await apiGet<any[]>(`/api/assets/${id}/relationships`)) ?? [];
		if (can($me, 'cost.read')) {
			const c = await apiGet<{ items: any[]; net_cost: number; currency: string }>(`/api/assets/${id}/costs`);
			costs = c.items ?? [];
			netCost = c.net_cost ?? 0;
			costCurrency = c.currency ?? 'USD';
		}
		if (can($me, 'hierarchy.read')) {
			locations = await apiGet<Location[]>('/api/locations');
			orgUnits = (await apiGet<OrgUnit[]>('/api/org-units')) ?? [];
			orgKinds = (await apiGet<OrgUnitKind[]>('/api/metadata/org-unit-kinds')) ?? [];
			people = await apiGet<Person[]>('/api/people');
		}
		ownerOrgUnit = asset?.owner_org_unit_id ?? '';
		if (can($me, 'dcim.read')) {
			ports = (await apiGet<Port[]>(`/api/dcim/ports?asset_id=${id}`)) ?? [];
			placementError = '';
			try {
				placement = await apiGet<AssetPlacement>(`/api/dcim/assets/${id}/placement`);
			} catch (e) {
				placement = null;
				placementError = e instanceof Error ? e.message : 'Could not load placement';
			}
			if (can($me, 'dcim.manage') && isInfrastructureAsset(asset)) {
				racks = (await apiGet<Rack[]>('/api/dcim/racks')) ?? [];
			}
		}
		if (can($me, 'ipam.read')) {
			ips = (await apiGet<IPAddress[]>(`/api/ipam/ips?asset_id=${id}`)) ?? [];
			subnets = (await apiGet<Subnet[]>('/api/ipam/subnets')) ?? [];
		}
		if (can($me, 'software.read')) {
			installs = (await apiGet<Installation[]>(`/api/software/installations?asset_id=${id}`)) ?? [];
			catalog = (await apiGet<Software[]>('/api/software')) ?? [];
		}
	}

	// Human-readable timeline details: drop internal id fields, empty values and
	// anything already shown in the summary, then render the rest as label/value.
	const TL_HIDE = new Set([
		'from_state_id', 'to_state_id', 'from_location_id', 'to_location_id',
		'holder_person_id', 'holder_org_unit_id', 'transition', 'via'
	]);
	function tlLabel(k: string): string {
		return k.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
	}
	function tlValue(v: unknown): string {
		if (v === null || v === undefined || v === '') return '';
		if (typeof v === 'object') return Object.entries(v as Record<string, unknown>)
			.map(([k, val]) => `${tlLabel(k)}: ${val}`)
			.join(', ');
		return String(v);
	}
	function tlRows(data: Record<string, unknown> | null | undefined): { label: string; value: string }[] {
		if (!data) return [];
		const out: { label: string; value: string }[] = [];
		for (const [k, v] of Object.entries(data)) {
			if (TL_HIDE.has(k)) continue;
			const val = tlValue(v);
			if (!val) continue;
			out.push({ label: tlLabel(k), value: val });
		}
		return out;
	}

	function addIP() {
		if (!newIP.address.trim()) return;
		act(async () => {
			await apiPost('/api/ipam/ips', {
				address: newIP.address.trim(),
				asset_id: id,
				port_id: newIP.port_id || null,
				dns_name: newIP.dns_name || null,
				status: 'allocated'
			});
			newIP = { address: '', port_id: '', dns_name: '' };
		});
	}

	function removeIP(ip: IPAddress) {
		if (!confirm(`Release ${ip.address}?`)) return;
		act(() => apiDelete(`/api/ipam/ips/${ip.id}`));
	}

	function addInstall() {
		if (!newInstall.software_id) return;
		act(async () => {
			let versionId: string | null = null;
			if (newInstall.version.trim()) {
				try {
					const v = await apiPost<{ id: string }>(`/api/software/${newInstall.software_id}/versions`, {
						version: newInstall.version.trim()
					});
					versionId = v.id;
				} catch {
					// version may already exist; ignore and install without it
				}
			}
			await apiPost('/api/software/installations', {
				software_id: newInstall.software_id,
				asset_id: id,
				version_id: versionId,
				source: 'manual'
			});
			newInstall = { software_id: '', version: '' };
		});
	}

	function removeInstall(inst: Installation) {
		act(() => apiDelete(`/api/software/installations/${inst.id}`));
	}

	function portIPs(portId: string): IPAddress[] {
		return ips.filter((ip) => ip.port_id === portId);
	}

	function addPort() {
		if (!newPort.name.trim()) return;
		act(async () => {
			await apiPost('/api/dcim/ports', {
				asset_id: id,
				name: newPort.name.trim(),
				port_type: newPort.port_type,
				speed: newPort.speed || undefined,
				sort: ports.length
			});
			newPort = { name: '', port_type: 'ethernet', speed: '' };
		});
	}

	function deletePort(p: Port) {
		if (!confirm(`Delete port ${p.name}?`)) return;
		act(() => apiDelete(`/api/dcim/ports/${p.id}`));
	}

	async function openConnect(p: Port) {
		error = '';
		connectFor = p;
		connectTo = '';
		connectCable = 'cat6';
		connectLabel = '';
		connectLength = '';
		try {
			freePorts = (await apiGet<Port[]>(`/api/dcim/ports/free?exclude_asset_id=${id}`)) ?? [];
		} catch {
			freePorts = [];
		}
	}

	function createConnection() {
		if (!connectFor || !connectTo) return;
		act(async () => {
			await apiPost('/api/dcim/connections', {
				a_port_id: connectFor!.id,
				b_port_id: connectTo,
				cable_type: connectCable,
				label: connectLabel || undefined,
				length_m: connectLength ? Number(connectLength) : undefined
			});
			connectFor = null;
		});
	}

	async function generatePorts() {
		if (!confirm('Generate ports from this asset type’s port profile? Existing ports are kept.')) return;
		try {
			const res = await apiPost<{ created: number }>(`/api/dcim/assets/${id}/generate-ports`, {});
			ports = (await apiGet<Port[]>(`/api/dcim/ports?asset_id=${id}`)) ?? [];
			error = res.created > 0 ? '' : 'No new ports were generated (they may already exist).';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not generate ports';
		}
	}

	async function traceCable(p: Port) {
		error = '';
		traceFor = p;
		traceHops = [];
		try {
			const res = await apiGet<{ hops: CableHop[] }>(`/api/dcim/ports/${p.id}/trace`);
			traceHops = res.hops ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Trace failed';
		}
	}

	function disconnect(p: Port) {
		if (!p.connection) return;
		if (!confirm(`Disconnect ${p.name} from ${p.connection.asset_tag}/${p.connection.port_name}?`)) return;
		act(() => apiDelete(`/api/dcim/connections/${p.connection!.connection_id}`));
	}

	function suggestMountHeight() {
		const ru = infraView.rack_units ?? asset?.attributes?.rack_units;
		if (typeof ru === 'number' && ru > 0) mountForm.u_height = ru;
		else if (typeof ru === 'string' && Number(ru) > 0) mountForm.u_height = Number(ru);
	}

	function doMount() {
		if (!mountForm.rack_id) return;
		act(async () => {
			await apiPost(`/api/dcim/racks/${mountForm.rack_id}/mounts`, {
				asset_id: id,
				position: mountForm.position,
				u_height: mountForm.u_height,
				face: mountForm.face
			});
			mountForm = { rack_id: '', position: 1, u_height: 1, face: 'front' };
		});
	}

	function doUnmount() {
		if (!placement?.mount) return;
		if (!confirm('Unmount this device from the rack?')) return;
		act(() => apiDelete(`/api/dcim/mounts/${placement!.mount!.id}`));
	}

	onMount(() => {
		loadAll();
		loadCurrencies().then((list) => {
			if (!newCost.currency || newCost.currency === 'USD') newCost.currency = defaultCurrency(list);
		});
	});

	async function act(fn: () => Promise<unknown>) {
		error = '';
		busy = true;
		try {
			await fn();
			await loadAll();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Action failed';
		} finally {
			busy = false;
		}
	}

	async function runTransition(t: LifecycleTransition) {
		error = '';
		let fields: any[] = [];
		try {
			fields = (await apiGet<any[]>(`/api/metadata/transitions/${t.id}/fields`)) ?? [];
		} catch {
			fields = [];
		}
		if (fields.length === 0) {
			act(() => apiPost(`/api/assets/${id}/transition`, { transition_key: t.key }));
			return;
		}
		txFields = fields;
		txValues = {};
		txNote = '';
		txModal = { key: t.key, label: t.label };
	}

	function addCost() {
		const amount = Number(newCost.amount);
		if (!amount) return;
		act(async () => {
			await apiPost(`/api/assets/${id}/costs`, {
				kind: newCost.kind,
				amount,
				currency: newCost.currency || undefined,
				vendor: newCost.vendor || undefined,
				reference: newCost.reference || undefined,
				note: newCost.note || undefined
			});
			newCost = { kind: 'repair', amount: '', currency: newCost.currency, vendor: '', reference: '', note: '' };
		});
	}

	function submitTransition() {
		const key = txModal?.key;
		if (!key) return;
		act(async () => {
			await apiPost(`/api/assets/${id}/transition`, {
				transition_key: key,
				note: txNote || undefined,
				data: txValues
			});
			txModal = null;
		});
	}
	function doAssign() {
		if (!assignPerson) return;
		const body: Record<string, unknown> = { holder_person_id: assignPerson };
		if (assignLocationTo) body.to_location_id = assignLocationTo;
		act(async () => {
			await apiPost(`/api/assets/${id}/assign`, body);
			assignLocationTo = '';
		});
	}
	function doReturn() {
		act(() => apiPost(`/api/assets/${id}/return`, {}));
	}
	function doAcknowledge() {
		act(() => apiPost(`/api/assets/${id}/acknowledge`, {}));
	}
	function custodyLabel(a: import('$lib/types').Assignment): string {
		if (a.kind === 'transfer') {
			const from = a.from_location?.name ?? '—';
			const to = a.to_location?.name ?? '—';
			return `Moved ${from} → ${to}`;
		}
		if (a.kind === 'return') return 'Returned';
		if (a.holder_person) return `Assigned to ${a.holder_person.first_name} ${a.holder_person.last_name}`.trim();
		if (a.holder_org_unit) return `Assigned to ${a.holder_org_unit.name}`;
		return a.kind;
	}
	function doSaveOwner() {
		act(async () => {
			await apiPut(`/api/assets/${id}`, {
				name: asset?.name,
				serial: asset?.serial,
				location_id: asset?.location_id,
				owner_org_unit_id: ownerOrgUnit || null,
				assigned_person_id: asset?.assigned_person_id,
				vendor: asset?.vendor,
				notes: asset?.notes,
				attributes: asset?.attributes ?? {}
			});
		});
	}
	function doTransfer() {
		if (!transferTo) return;
		act(async () => {
			await apiPost(`/api/assets/${id}/transfer`, {
				to_location_id: transferTo,
				reason: transferReason || undefined
			});
			transferTo = '';
			transferReason = '';
		});
	}
	function doDelete() {
		if (!confirm('Delete this asset?')) return;
		act(async () => {
			await apiDelete(`/api/assets/${id}`);
			goto('/assets');
		});
	}

	function startEdit() {
		editValues = { ...(asset?.attributes ?? {}) };
		editLat = asset?.latitude != null ? String(asset.latitude) : '';
		editLng = asset?.longitude != null ? String(asset.longitude) : '';
		editing = true;
	}
	function saveEdit() {
		act(async () => {
			await apiPut(`/api/assets/${id}`, {
				name: asset?.name,
				serial: asset?.serial,
				location_id: asset?.location_id,
				owner_org_unit_id: asset?.owner_org_unit_id,
				assigned_person_id: asset?.assigned_person_id,
				vendor: asset?.vendor,
				notes: asset?.notes,
				attributes: editValues,
				latitude: editLat !== '' ? Number(editLat) : null,
				longitude: editLng !== '' ? Number(editLng) : null
			});
			editing = false;
		});
	}

	function fieldValue(f: FieldDefinition): string {
		const v = asset?.attributes?.[f.key];
		if (v === undefined || v === null || v === '') return '—';
		return String(v) + (f.unit ? ` ${f.unit.symbol}` : '');
	}

	function hasValue(f: FieldDefinition): boolean {
		const v = asset?.attributes?.[f.key];
		return v !== undefined && v !== null && v !== '';
	}

	// Spec sheet ordering: populated fields first, each sorted by their defined sort.
	const specFields = $derived(
		[...fields].sort((a, b) => {
			const av = hasValue(a) ? 0 : 1;
			const bv = hasValue(b) ? 0 : 1;
			return av - bv || (a.sort ?? 0) - (b.sort ?? 0);
		})
	);
	const filledSpecCount = $derived(fields.filter(hasValue).length);

	// Deep link encoded in the QR (falls back to the bare tag during SSR).
	const assetUrl = $derived(browser ? `${window.location.origin}/assets/${id}` : `/assets/${id}`);

	const orgChain = $derived(orgBreadcrumb(orgUnits, asset?.owner_org_unit ?? null));
</script>

{#if !asset}
	<p class="muted">Loading…</p>
{:else}
	<div class="topbar">
		<div>
			<h1 style="margin-bottom:4px">{asset.name}</h1>
			<div class="muted">{asset.asset_tag} · {asset.asset_type?.name}</div>
		</div>
		<div class="row">
			{#if asset.current_state}
				<span class="badge" style="border-color:{asset.current_state.color}">{asset.current_state.label}</span>
			{/if}
			{#if can($me, 'asset.delete')}
				<button class="btn danger small" onclick={doDelete}>Delete</button>
			{/if}
		</div>
	</div>

	{#if error}<p class="error">{error}</p>{/if}

	{#if can($me, 'asset.transition') && transitions.length}
		<div class="card" style="margin-bottom:16px">
			<label>Lifecycle actions</label>
			<div class="pill-actions">
				{#each transitions as t}
					<button class="btn small" disabled={busy || t.allowed === false} onclick={() => runTransition(t)}>
						{t.label}
					</button>
				{/each}
			</div>
		</div>
	{/if}

	<div class="tabs">
		<button type="button" class="tab" class:active={tab === 'overview'} onclick={() => (tab = 'overview')}>Overview</button>
		<button type="button" class="tab" class:active={tab === 'custody'} onclick={() => (tab = 'custody')}>Custody</button>
		<button type="button" class="tab" class:active={tab === 'timeline'} onclick={() => (tab = 'timeline')}>Timeline</button>
		<button type="button" class="tab" class:active={tab === 'attributes'} onclick={() => (tab = 'attributes')}>Attributes</button>
		{#if can($me, 'cost.read')}
			<button type="button" class="tab" class:active={tab === 'costs'} onclick={() => (tab = 'costs')}>Costs</button>
		{/if}
		{#if can($me, 'dcim.read') && showInfra}
			<button type="button" class="tab" class:active={tab === 'infrastructure'} onclick={() => (tab = 'infrastructure')}>Infrastructure</button>
		{/if}
		{#if can($me, 'dcim.read')}
			<button type="button" class="tab" class:active={tab === 'ports'} onclick={() => (tab = 'ports')}>Ports</button>
		{/if}
		{#if can($me, 'software.read')}
			<button type="button" class="tab" class:active={tab === 'software'} onclick={() => (tab = 'software')}>Software</button>
		{/if}
		<button type="button" class="tab" class:active={tab === 'relationships'} onclick={() => (tab = 'relationships')}>Relationships</button>
	</div>

	{#if tab === 'overview'}
		<div class="grid cols-2">
			<div class="card">
				<h3 style="margin-top:0">Details</h3>
				<div class="kv">
					<div class="k">Serial</div><div>{asset.serial || '—'}</div>
					<div class="k">Location</div><div>{asset.location?.name ?? '—'}</div>
					<div class="k">Organization</div>
					<div>
						{#if orgChain.length}
							<div class="org-crumb muted small">
								{#each orgChain as ou, i}
									{#if i > 0}<span> › </span>{/if}
									<span>{ou.name}{ou.kind ? ` (${kindLabel(orgKinds, ou.kind)})` : ''}</span>
								{/each}
							</div>
						{:else}
							<span class="muted">—</span>
						{/if}
					</div>
					{#if can($me, 'asset.write')}
						<div class="k">Owner unit</div>
						<div>
							<div class="row">
								<select bind:value={ownerOrgUnit}>
									<option value="">— unassigned —</option>
									{#each orgUnits as ou}
										<option value={ou.id}>{'— '.repeat(depth(ou.path))}{ou.name}{ou.kind ? ` (${ou.kind})` : ''}</option>
									{/each}
								</select>
								<button class="btn small" onclick={doSaveOwner} disabled={busy}>Save</button>
							</div>
						</div>
					{:else}
						<div class="k">Owner</div><div>{asset.owner_org_unit?.name ?? '—'}</div>
					{/if}
					<div class="k">Assigned to</div>
					<div>{asset.assigned_to ? `${asset.assigned_to.first_name} ${asset.assigned_to.last_name}` : '—'}</div>
					<div class="k">Vendor</div><div>{asset.vendor || '—'}</div>
					<div class="k">Purchase cost</div><div>{asset.purchase_cost ?? '—'}</div>
					<div class="k">Purchase date</div><div>{asset.purchase_date ? new Date(asset.purchase_date).toLocaleDateString() : '—'}</div>
					<div class="k">Warranty</div>
					<div>
						{#if asset.warranty_expiry}
							{new Date(asset.warranty_expiry).toLocaleDateString()}
							{#if new Date(asset.warranty_expiry) < new Date()}<span class="badge" style="margin-left:6px; color:#ef4444">expired</span>{/if}
						{:else}—{/if}
					</div>
					<div class="k">Last seen</div>
					<div>
						{#if asset.last_seen_at}
							{new Date(asset.last_seen_at).toLocaleString()}
							{#if asset.last_seen_location}· {asset.last_seen_location.name}{/if}
							{#if asset.last_seen_source}<span class="muted"> ({asset.last_seen_source})</span>{/if}
						{:else}—{/if}
					</div>
					{#if ingestMatchLabel}
						<div class="k">Discovery match</div>
						<div><span class="badge">{ingestMatchLabel}</span></div>
					{/if}
					<div class="k">Notes</div><div>{asset.notes || '—'}</div>
				</div>
			</div>

			{#if can($me, 'asset.assign')}
				<div class="card">
					<h3 style="margin-top:0">Custody</h3>
					<div class="field">
						<label>Assign to person</label>
						<div class="row">
							<select bind:value={assignPerson}>
								<option value="">Select person…</option>
								{#each people as p}<option value={p.id}>{p.first_name} {p.last_name}</option>{/each}
							</select>
							<button class="btn small" onclick={doAssign} disabled={busy}>Assign</button>
						</div>
					</div>
					<div class="field">
						<label>Also move to location <span class="muted">(optional — updates physical location on assign)</span></label>
						<select bind:value={assignLocationTo}>
							<option value="">Keep current location</option>
							{#each locations as l}
								<option value={l.id}>{l.name}</option>
							{/each}
						</select>
						{#if assignDefaultLocationHint}
							<p class="muted small" style="margin:6px 0 0">{assignDefaultLocationHint}</p>
						{/if}
					</div>
					<div class="field">
						<button class="btn secondary small" onclick={doReturn} disabled={busy}>Return to stock</button>
						{#if openUnacked}
							<button class="btn small" style="margin-left:8px" onclick={doAcknowledge} disabled={busy}>Acknowledge receipt</button>
						{/if}
					</div>
					<div class="field">
						<label>Move to location <span class="muted">(currently: {asset.location?.name ?? 'none'})</span></label>
						<select bind:value={transferTo}>
							<option value="">Select destination…</option>
							{#each locations as l}
								{#if l.id !== asset.location_id}<option value={l.id}>{l.name}</option>{/if}
							{/each}
						</select>
						<input
							type="text"
							style="margin-top:6px"
							bind:value={transferReason}
							placeholder="Reason (optional) — e.g. deployed to branch"
						/>
						<button class="btn small" style="margin-top:6px" onclick={doTransfer} disabled={busy || !transferTo}>
							Move asset
						</button>
					</div>
				</div>
			{/if}
		</div>

		{#if showInfra && tab === 'overview'}
			<div class="card" style="margin-top:16px">
				<div class="row" style="justify-content:space-between; align-items:baseline">
					<h3 style="margin-top:0">Data center</h3>
					<button type="button" class="linkish" onclick={() => (tab = 'infrastructure')}>View details →</button>
				</div>
				<div class="kv">
					{#if infraView.rack_record}
						<div class="k">Rack record</div>
						<div>
							<a href="/datacenter/{infraView.rack_record.id}">{infraView.rack_record.name}</a>
							<span class="muted"> · {infraView.rack_record.u_height}U</span>
							{#if infraView.rack_record.location}
								<span class="muted"> · {infraView.rack_record.location.name}</span>
							{/if}
						</div>
					{/if}
					{#if infraView.mount?.rack}
						<div class="k">Rack mount</div>
						<div>
							<a href="/datacenter/{infraView.mount.rack_id}">{infraView.mount.rack.name}</a>
							<span class="muted"> · {mountLabel(infraView.mount)}</span>
							{#if infraView.mount.rack.location}
								<span class="muted"> · {infraView.mount.rack.location.name}</span>
							{/if}
						</div>
					{:else if !infraView.rack_record}
						<div class="k">Rack mount</div><div class="muted">Not mounted</div>
					{/if}
					{#if infraView.rack_units != null}
						<div class="k">Height</div><div>{infraView.rack_units}U</div>
					{/if}
					{#if infraView.power_watts != null}
						<div class="k">Power draw</div><div>{powerLabel(infraView.power_watts)}</div>
					{/if}
					{#if infraView.port_count > 0}
						<div class="k">Ports</div>
						<div><button type="button" class="linkish" onclick={() => (tab = 'ports')}>{infraView.port_count} port(s)</button></div>
					{/if}
				</div>
			</div>
		{/if}

		{#if fields.length}
			<div class="card" style="margin-top:16px">
				<div class="row" style="justify-content:space-between; align-items:baseline">
					<h3 style="margin-top:0">Specifications</h3>
					<span class="muted small">{filledSpecCount} of {fields.length} filled</span>
				</div>
				<div class="specs">
					{#each specFields as f}
						<div class="spec" class:empty={!hasValue(f)}>
							<div class="spec-label">{f.label}{#if f.unit}<span class="muted"> ({f.unit.symbol})</span>{/if}</div>
							<div class="spec-value">{fieldValue(f)}</div>
							{#if f.help_text}<div class="spec-help muted">{f.help_text}</div>{/if}
						</div>
					{/each}
				</div>
				{#if can($me, 'asset.write')}
					<p class="muted small" style="margin:12px 0 0">Edit these in the <button type="button" class="linkish" onclick={() => (tab = 'attributes')}>Attributes</button> tab.</p>
				{/if}
			</div>
		{/if}

		<div class="card" style="margin-top:16px">
			<h3 style="margin-top:0">Identification</h3>
			<AssetCodes tag={asset.asset_tag} url={assetUrl} name={asset.name} />
			<p class="muted" style="margin-top:10px">
				QR opens this asset page · barcode encodes the tag <strong>{asset.asset_tag}</strong> (Code 128).
			</p>
		</div>
	{:else if tab === 'attributes'}
		<div class="card">
			<div class="row" style="justify-content:space-between">
				<h3 style="margin-top:0">Attributes</h3>
				{#if can($me, 'asset.write')}
					{#if editing}
						<div class="row">
							<button class="btn small" onclick={saveEdit} disabled={busy}>Save</button>
							<button class="btn secondary small" onclick={() => (editing = false)}>Cancel</button>
						</div>
					{:else}
						<button class="btn secondary small" onclick={startEdit}>Edit</button>
					{/if}
				{/if}
			</div>
			{#if editing}
				<DynamicForm {fields} bind:values={editValues} />
				<div class="field" style="margin-top:8px">
					<label>Coordinates <span class="muted">(optional — overrides location on the map)</span></label>
					<div class="grid cols-2">
						<input type="number" step="any" min="-90" max="90" bind:value={editLat} placeholder="latitude" />
						<input type="number" step="any" min="-180" max="180" bind:value={editLng} placeholder="longitude" />
					</div>
				</div>
			{:else if fields.length === 0}
				<p class="muted">No custom fields for this type.</p>
			{:else}
				<div class="kv">
					{#each fields as f}
						<div class="k">{f.label}</div><div>{fieldValue(f)}</div>
					{/each}
				</div>
			{/if}
		</div>
	{:else if tab === 'custody'}
		<div class="card">
			<h3 style="margin-top:0">Custody history</h3>
			<p class="muted" style="margin-top:4px">Assignments, returns, and location moves for this asset.</p>
			{#if !(history.assignments?.length)}
				<p class="muted">No custody events yet.</p>
			{:else}
				<table>
					<thead>
						<tr><th>When</th><th>Event</th><th>Location</th><th>Reason</th><th>Status</th><th>Ack</th></tr>
					</thead>
					<tbody>
						{#each history.assignments as a}
							<tr>
								<td class="muted">{new Date(a.assigned_at).toLocaleString()}</td>
								<td>{custodyLabel(a)}</td>
								<td>
									{#if a.kind === 'transfer' || (a.from_location || a.to_location)}
										{a.from_location?.name ?? '—'} → {a.to_location?.name ?? '—'}
									{:else}
										{a.to_location?.name ?? a.from_location?.name ?? '—'}
									{/if}
								</td>
								<td>{a.reason || '—'}</td>
								<td>{a.returned_at ? `Closed ${new Date(a.returned_at).toLocaleDateString()}` : a.kind === 'assign' && !a.returned_at ? 'Open' : '—'}</td>
								<td class="muted">{a.acknowledged_at ? new Date(a.acknowledged_at).toLocaleDateString() : a.kind === 'assign' && !a.returned_at ? '—' : 'n/a'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{:else if tab === 'timeline'}
		<div class="card">
			<h3 style="margin-top:0">Timeline</h3>
			{#if timeline.length === 0}
				<p class="muted">No events yet.</p>
			{:else}
				<ul class="timeline">
					{#each timeline as ev}
						<li>
							<span class="dot" data-kind={ev.kind}></span>
							<div class="tl-body">
								<div class="tl-head">
									<span class="tl-summary">{ev.summary || ev.kind}</span>
									<span class="tl-kind">{ev.kind}</span>
									<span class="muted tl-when">{new Date(ev.occurred_at).toLocaleString()}</span>
								</div>
								{#if tlRows(ev.data).length}
									<dl class="tl-data">
										{#each tlRows(ev.data) as row}
											<dt>{row.label}</dt><dd>{row.value}</dd>
										{/each}
									</dl>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{:else if tab === 'costs'}
		<div class="card" style="margin-bottom:16px">
			<div class="row" style="justify-content:space-between; align-items:baseline">
				<h3 style="margin-top:0">Cost ledger</h3>
				<div><span class="muted">Net lifetime cost: </span><strong>{netCost.toFixed(2)} {costCurrency}</strong></div>
			</div>
			<table>
				<thead><tr><th>Date</th><th>Kind</th><th>Amount</th><th>Vendor</th><th>Reference</th><th>Note</th></tr></thead>
				<tbody>
					{#each costs as c}
						<tr>
							<td class="muted">{c.incurred_at ? new Date(c.incurred_at).toLocaleDateString() : '—'}</td>
							<td><span class="badge">{c.kind}</span></td>
							<td>{c.kind === 'disposal_proceeds' ? '-' : ''}{Number(c.amount).toFixed(2)} {c.currency}</td>
							<td>{c.vendor || '—'}</td>
							<td>{c.reference || '—'}</td>
							<td>{c.note || '—'}</td>
						</tr>
					{/each}
					{#if costs.length === 0}<tr><td colspan="6" class="muted">No costs recorded.</td></tr>{/if}
				</tbody>
			</table>
		</div>
		{#if can($me, 'cost.manage')}
			<div class="card">
				<h3 style="margin-top:0">Record a cost</h3>
				<div class="grid cols-2">
					<div class="field">
						<label>Kind</label>
						<select bind:value={newCost.kind}>
							<option value="purchase">Purchase</option>
							<option value="repair">Repair</option>
							<option value="upgrade">Upgrade</option>
							<option value="disposal_proceeds">Disposal proceeds</option>
							<option value="other">Other</option>
						</select>
					</div>
					<div class="field">
						<label>Amount</label>
						<input type="number" step="0.01" bind:value={newCost.amount} />
					</div>
					<div class="field">
						<label>Currency</label>
						<select bind:value={newCost.currency}>
							{#each $currencies as c}<option value={c.code}>{c.code} — {c.name}</option>{/each}
						</select>
					</div>
					<div class="field">
						<label>Vendor</label>
						<input type="text" bind:value={newCost.vendor} />
					</div>
					<div class="field">
						<label>Reference</label>
						<input type="text" bind:value={newCost.reference} placeholder="Invoice / RMA #" />
					</div>
				</div>
				<div class="field">
					<label>Note</label>
					<input type="text" bind:value={newCost.note} />
				</div>
				<button class="btn small" onclick={addCost} disabled={busy}>Add cost</button>
			</div>
		{/if}
	{:else if tab === 'infrastructure'}
		<p class="muted" style="margin:0 0 12px">
			Data-center context for this asset: rack placement, physical specs used for capacity planning, and links into DCIM.
		</p>
		{#if placementError}
			<p class="error" style="margin-bottom:12px">{placementError} — showing attributes and ports only.</p>
		{/if}
		<div class="grid cols-2">
			<div class="card">
				<h3 style="margin-top:0">Placement</h3>
				{#if infraView.rack_record}
					<p class="muted" style="margin-top:0">This asset is linked to a rack record in the data center.</p>
					<div class="kv">
						<div class="k">Rack</div>
						<div>
							<a href="/datacenter/{infraView.rack_record.id}">{infraView.rack_record.name}</a>
							<span class="muted"> · {infraView.rack_record.u_height}U</span>
						</div>
						<div class="k">Location</div><div>{infraView.rack_record.location?.name ?? '—'}</div>
						{#if infraView.rack_record.power_capacity_w}
							<div class="k">Power budget</div><div>{powerLabel(infraView.rack_record.power_capacity_w)}</div>
						{/if}
					</div>
					<button class="btn secondary small" style="margin-top:12px" onclick={() => goto(`/datacenter/${infraView.rack_record!.id}`)}>
						Open rack elevation
					</button>
				{:else if infraView.mount?.rack}
					<div class="kv">
						<div class="k">Rack</div>
						<div><a href="/datacenter/{infraView.mount.rack_id}">{infraView.mount.rack.name}</a></div>
						<div class="k">Location</div><div>{infraView.mount.rack.location?.name ?? '—'}</div>
						<div class="k">Position</div><div>{mountLabel(infraView.mount)}</div>
						<div class="k">Mounted</div><div>{new Date(infraView.mount.mounted_at).toLocaleString()}</div>
					</div>
					<div class="row" style="margin-top:12px">
						<button class="btn secondary small" onclick={() => goto(`/datacenter/${infraView.mount!.rack_id}`)}>
							Open rack elevation
						</button>
						{#if infraManage}
							<button class="btn danger small" onclick={doUnmount} disabled={busy}>Unmount</button>
						{/if}
					</div>
				{:else}
					<p class="muted" style="margin-top:0">This device is not mounted in a rack yet.</p>
					{#if infraManage && !infraView.rack_record}
						<label class="field">
							Rack
							<select bind:value={mountForm.rack_id} onchange={suggestMountHeight}>
								<option value="">— choose rack —</option>
								{#each racks as r}
									<option value={r.id}>{r.name}{r.location ? ` · ${r.location.name}` : ''}</option>
								{/each}
							</select>
						</label>
						<div class="grid cols-2">
							<label class="field">Position (U) <input type="number" min="1" bind:value={mountForm.position} /></label>
							<label class="field">Height (U) <input type="number" min="1" bind:value={mountForm.u_height} /></label>
						</div>
						<label class="field">
							Face
							<select bind:value={mountForm.face}>
								<option value="front">Front</option>
								<option value="rear">Rear</option>
								<option value="full">Full depth</option>
							</select>
						</label>
						<button class="btn small" onclick={doMount} disabled={busy || !mountForm.rack_id}>Mount in rack</button>
					{/if}
				{/if}
			</div>

			<div class="card">
				<h3 style="margin-top:0">DCIM specs</h3>
				<div class="kv">
					<div class="k">Rack units</div><div>{infraView.rack_units != null ? `${infraView.rack_units}U` : '—'}</div>
					<div class="k">Power draw</div><div>{powerLabel(infraView.power_watts ?? undefined)}</div>
					<div class="k">Weight</div><div>{infraView.weight_kg != null ? `${infraView.weight_kg} kg` : '—'}</div>
					<div class="k">Ports</div>
					<div>
						{#if infraView.port_count > 0}
							<button type="button" class="linkish" onclick={() => (tab = 'ports')}>{infraView.port_count} configured</button>
						{:else}
							<span class="muted">None</span>
							{#if infraManage}
								· <button type="button" class="linkish" onclick={() => (tab = 'ports')}>Add ports</button>
							{/if}
						{/if}
					</div>
				</div>
				<p class="muted small" style="margin:12px 0 0">
					Rack units and power are stored on the asset record and used for capacity planning.
					{#if can($me, 'asset.write')}
						Edit in <button type="button" class="linkish" onclick={() => (tab = 'attributes')}>Attributes</button>.
					{/if}
				</p>
			</div>
		</div>
	{:else if tab === 'ports'}
		{#if ports.length}
			<div class="card" style="margin-bottom:16px">
				{#if isPatch && frontPorts.length && rearPorts.length}
					<h3 style="margin-top:0">Patch panel layout</h3>
					<div class="pp-row"><span class="pp-side">Front</span>
						<div class="panel">
							{#each frontPorts as p}
								<div class="port-cell" class:up={!!p.connection}
									title={`${p.name} · ${p.port_type}${p.connection ? ` → ${p.connection.asset_tag || p.connection.asset_name} / ${p.connection.port_name}` : ' · free'}`}>
									<span class="port-led"></span><span class="port-name">{p.name}</span>
								</div>
							{/each}
						</div>
					</div>
					<div class="pp-row" style="margin-top:8px"><span class="pp-side">Rear</span>
						<div class="panel">
							{#each rearPorts as p}
								<div class="port-cell" class:up={!!p.connection}
									title={`${p.name} · ${p.port_type}${p.connection ? ` → ${p.connection.asset_tag || p.connection.asset_name} / ${p.connection.port_name}` : ' · free'}`}>
									<span class="port-led"></span><span class="port-name">{p.name}</span>
								</div>
							{/each}
						</div>
					</div>
				{:else}
					<h3 style="margin-top:0">Front panel</h3>
					<div class="panel">
						{#each ports as p}
							<div
								class="port-cell"
								class:up={!!p.connection}
								title={`${p.name} · ${p.port_type}${p.connection ? ` → ${p.connection.asset_tag || p.connection.asset_name} / ${p.connection.port_name}` : ' · free'}`}
							>
								<span class="port-led"></span>
								<span class="port-name">{p.name}</span>
							</div>
						{/each}
					</div>
				{/if}
				<div class="panel-legend muted">
					<span><span class="dot up"></span> connected</span>
					<span><span class="dot"></span> free</span>
				</div>
			</div>
		{/if}
		<div class="card" style="margin-bottom:16px">
			<div class="row" style="justify-content:space-between; align-items:center">
				<h3 style="margin-top:0">Ports &amp; cabling</h3>
				{#if can($me, 'dcim.manage')}
					<button class="btn secondary small" onclick={generatePorts} disabled={busy}>Generate from profile</button>
				{/if}
			</div>
			<table>
				<thead>
					<tr><th>Port</th><th>Type</th><th>Speed</th><th>Connected to</th><th></th></tr>
				</thead>
				<tbody>
					{#each ports as p}
						<tr>
							<td><strong>{p.name}</strong></td>
							<td><span class="badge">{p.port_type}</span></td>
							<td class="muted">{p.speed || '—'}</td>
							<td>
								{#if p.connection}
									<a href={`/assets/${p.connection.asset_id}`}>{p.connection.asset_tag || p.connection.asset_name}</a>
									<span class="muted"> / {p.connection.port_name}</span>
									<span class="badge" style="margin-left:6px">{p.connection.cable_type}</span>
								{:else}
									<span class="muted">— not connected —</span>
								{/if}
							</td>
							<td style="text-align:right; white-space:nowrap">
								{#if p.connection}
									<button class="btn secondary small" onclick={() => traceCable(p)} disabled={busy}>Trace</button>
								{/if}
								{#if can($me, 'dcim.manage')}
									{#if p.connection}
										<button class="btn secondary small" onclick={() => disconnect(p)} disabled={busy}>Disconnect</button>
									{:else}
										<button class="btn small" onclick={() => openConnect(p)} disabled={busy}>Connect</button>
										<button class="btn danger small" onclick={() => deletePort(p)} disabled={busy}>Delete</button>
									{/if}
								{/if}
							</td>
						</tr>
					{/each}
					{#if ports.length === 0}<tr><td colspan="5" class="muted">No ports defined.</td></tr>{/if}
				</tbody>
			</table>
		</div>
		{#if can($me, 'dcim.manage')}
			<div class="card">
				<h3 style="margin-top:0">Add a port</h3>
				<div class="grid cols-3">
					<div class="field">
						<label>Name</label>
						<input type="text" bind:value={newPort.name} placeholder="eth0, GE1/0/1, PSU…" />
					</div>
					<div class="field">
						<label>Type</label>
						<select bind:value={newPort.port_type}>
							<option value="ethernet">Ethernet</option>
							<option value="fiber">Fiber</option>
							<option value="sfp">SFP/SFP+</option>
							<option value="console">Console</option>
							<option value="power">Power</option>
							<option value="usb">USB</option>
							<option value="other">Other</option>
						</select>
					</div>
					<div class="field">
						<label>Speed</label>
						<input type="text" bind:value={newPort.speed} placeholder="1G, 10G, 240V…" />
					</div>
				</div>
				<button class="btn small" onclick={addPort} disabled={busy}>Add port</button>
			</div>
		{/if}

		{#if can($me, 'ipam.read')}
			<div class="card" style="margin-top:16px">
				<h3 style="margin-top:0">IP addresses</h3>
				<table>
					<thead><tr><th>Address</th><th>DNS</th><th>Port</th><th>Subnet</th>{#if can($me, 'ipam.manage')}<th></th>{/if}</tr></thead>
					<tbody>
						{#each ips as ip}
							<tr>
								<td><strong>{ip.address}</strong> <span class="badge">{ip.status}</span></td>
								<td class="muted">{ip.dns_name || '—'}</td>
								<td class="muted">{ports.find((p) => p.id === ip.port_id)?.name ?? '—'}</td>
								<td class="muted">{ip.subnet?.cidr ?? '—'}</td>
								{#if can($me, 'ipam.manage')}
									<td style="text-align:right"><button class="btn secondary small" onclick={() => removeIP(ip)} disabled={busy}>Release</button></td>
								{/if}
							</tr>
						{/each}
						{#if ips.length === 0}<tr><td colspan="5" class="muted">No IP addresses assigned.</td></tr>{/if}
					</tbody>
				</table>
				{#if can($me, 'ipam.manage')}
					<div class="grid cols-3" style="margin-top:12px">
						<div class="field"><label>Address</label><input bind:value={newIP.address} placeholder="10.10.0.20" /></div>
						<div class="field">
							<label>Bind to port</label>
							<select bind:value={newIP.port_id}>
								<option value="">— none —</option>
								{#each ports as p}<option value={p.id}>{p.name}</option>{/each}
							</select>
						</div>
						<div class="field"><label>DNS name</label><input bind:value={newIP.dns_name} placeholder="host.local" /></div>
					</div>
					<button class="btn small" onclick={addIP} disabled={busy || !newIP.address.trim()}>Assign IP</button>
				{/if}
			</div>
		{/if}
	{:else if tab === 'software'}
		<div class="card">
			<h3 style="margin-top:0">Installed software</h3>
			<table>
				<thead><tr><th>Title</th><th>Publisher</th><th>Version</th><th>Source</th>{#if can($me, 'software.manage')}<th></th>{/if}</tr></thead>
				<tbody>
					{#each installs as inst}
						<tr>
							<td><strong>{inst.software?.name ?? inst.software_id}</strong></td>
							<td class="muted">{inst.software?.publisher ?? '—'}</td>
							<td class="muted">{inst.version?.version ?? '—'}</td>
							<td><span class="badge">{inst.source}</span></td>
							{#if can($me, 'software.manage')}
								<td style="text-align:right"><button class="btn secondary small" onclick={() => removeInstall(inst)} disabled={busy}>Uninstall</button></td>
							{/if}
						</tr>
					{/each}
					{#if installs.length === 0}<tr><td colspan="5" class="muted">No software recorded on this asset.</td></tr>{/if}
				</tbody>
			</table>
			{#if can($me, 'software.manage')}
				<div class="grid cols-3" style="margin-top:12px">
					<div class="field">
						<label>Software</label>
						<select bind:value={newInstall.software_id}>
							<option value="">Select title…</option>
							{#each catalog as s}<option value={s.id}>{s.name}</option>{/each}
						</select>
					</div>
					<div class="field"><label>Version (optional)</label><input bind:value={newInstall.version} placeholder="e.g. 24.1" /></div>
				</div>
				<button class="btn small" onclick={addInstall} disabled={busy || !newInstall.software_id}>Record install</button>
			{/if}
		</div>
	{:else if tab === 'relationships'}
		<div class="card">
			<h3 style="margin-top:0">Relationships</h3>
			<table>
				<thead><tr><th>From</th><th>To</th><th>Type</th></tr></thead>
				<tbody>
					{#each relationships as r}
						<tr><td>{r.from_asset_id}</td><td>{r.to_asset_id}</td><td>{r.relationship_type_id}</td></tr>
					{/each}
					{#if relationships.length === 0}<tr><td colspan="3" class="muted">No relationships.</td></tr>{/if}
				</tbody>
			</table>
		</div>
	{/if}

	{#if connectFor}
		<div
			class="modal-backdrop"
			role="button"
			tabindex="0"
			aria-label="Close dialog"
			onclick={(e) => { if (e.target === e.currentTarget) connectFor = null; }}
			onkeydown={(e) => e.key === 'Escape' && (connectFor = null)}
		>
			<div class="modal">
				<h3>Connect {connectFor.name}</h3>
				<div class="field">
					<label>To port</label>
					<select bind:value={connectTo}>
						<option value="">Select a remote port…</option>
						{#each freePorts as fp}
							<option value={fp.id}>
								{fp.asset?.asset_tag ?? fp.asset?.name ?? fp.asset_id} / {fp.name} ({fp.port_type})
							</option>
						{/each}
					</select>
					{#if freePorts.length === 0}
						<p class="muted" style="margin-top:6px">No free ports available on other assets.</p>
					{/if}
				</div>
				<div class="field">
					<label>Cable type</label>
					<select bind:value={connectCable}>
						<option value="cat5e">Cat5e</option>
						<option value="cat6">Cat6</option>
						<option value="cat6a">Cat6a</option>
						<option value="fiber-lc">Fiber (LC)</option>
						<option value="dac">DAC</option>
						<option value="power">Power</option>
						<option value="other">Other</option>
					</select>
				</div>
				<div class="grid cols-2">
					<div class="field">
						<label>Cable label (optional)</label>
						<input type="text" bind:value={connectLabel} placeholder="e.g. A12-PATCH" />
					</div>
					<div class="field">
						<label>Length (m, optional)</label>
						<input type="number" min="0" step="0.5" bind:value={connectLength} placeholder="3" />
					</div>
				</div>
				<div class="modal-actions">
					<button class="btn secondary" onclick={() => (connectFor = null)}>Cancel</button>
					<button class="btn" onclick={createConnection} disabled={busy || !connectTo}>Connect</button>
				</div>
			</div>
		</div>
	{/if}

	{#if traceFor}
		<div
			class="modal-backdrop"
			role="button"
			tabindex="0"
			aria-label="Close dialog"
			onclick={(e) => { if (e.target === e.currentTarget) traceFor = null; }}
			onkeydown={(e) => e.key === 'Escape' && (traceFor = null)}
		>
			<div class="modal">
				<h3>Cable trace from {traceFor.name}</h3>
				{#if traceHops.length === 0}
					<p class="muted">No cable path found.</p>
				{:else}
					<ol class="trace">
						<li class="trace-start">{traceFor.name} <span class="muted">({asset?.asset_tag})</span></li>
						{#each traceHops as h}
							<li>
								<div class="trace-cable">
									<span class="badge">{h.cable_type}</span>
									{#if h.label}<span class="muted"> {h.label}</span>{/if}
									{#if h.length_m}<span class="muted"> · {h.length_m} m</span>{/if}
									{#if h.patch_panel}<span class="badge" style="margin-left:6px">patch panel</span>{/if}
								</div>
								<div class="trace-dest">
									→ <strong>{h.to_asset || h.to_asset_tag}</strong>
									<span class="muted"> / {h.to_port}</span>
								</div>
							</li>
						{/each}
					</ol>
				{/if}
				<div class="modal-actions">
					<button class="btn secondary" onclick={() => (traceFor = null)}>Close</button>
				</div>
			</div>
		</div>
	{/if}

	{#if txModal}
		<div
			class="modal-backdrop"
			role="button"
			tabindex="0"
			aria-label="Close dialog"
			onclick={(e) => { if (e.target === e.currentTarget) txModal = null; }}
			onkeydown={(e) => e.key === 'Escape' && (txModal = null)}
		>
			<div class="modal">
				<h3>{txModal.label}</h3>
				<DynamicForm fields={txFields as any} bind:values={txValues} />
				<div class="field">
					<label>Note</label>
					<input type="text" bind:value={txNote} placeholder="Optional note" />
				</div>
				<div class="modal-actions">
					<button class="btn secondary" onclick={() => (txModal = null)}>Cancel</button>
					<button class="btn" onclick={submitTransition} disabled={busy}>Confirm</button>
				</div>
			</div>
		</div>
	{/if}
{/if}

<style>
	.panel {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		padding: 12px;
		background: var(--bg);
		border-radius: 8px;
	}
	.pp-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.pp-row .panel {
		flex: 1;
		flex-wrap: nowrap;
		overflow-x: auto;
	}
	.pp-side {
		width: 44px;
		font-size: 12px;
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
	}
	.specs {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 12px;
	}
	.spec {
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 10px 12px;
		background: var(--bg);
	}
	.spec.empty { opacity: 0.55; }
	.spec-label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.03em; }
	.spec-value { font-size: 15px; font-weight: 600; margin-top: 3px; word-break: break-word; }
	.spec-help { font-size: 11px; margin-top: 4px; }
	.linkish {
		background: none;
		border: none;
		padding: 0;
		color: var(--primary);
		cursor: pointer;
		font: inherit;
		text-decoration: underline;
	}
	.port-cell {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		min-width: 52px;
		padding: 6px 4px;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		font-size: 11px;
	}
	.port-led {
		width: 10px;
		height: 10px;
		border-radius: 2px;
		background: #4b5161;
		box-shadow: inset 0 0 2px rgba(0, 0, 0, 0.5);
	}
	.port-cell.up .port-led {
		background: var(--success);
		box-shadow: 0 0 6px var(--success);
	}
	.port-name {
		max-width: 60px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--muted);
	}
	.panel-legend {
		display: flex;
		gap: 16px;
		margin-top: 10px;
		font-size: 12px;
	}
	.panel-legend .dot {
		display: inline-block;
		width: 9px;
		height: 9px;
		border-radius: 2px;
		background: #4b5161;
		margin-right: 4px;
	}
	.panel-legend .dot.up {
		background: var(--success);
	}
	.trace {
		list-style: none;
		margin: 8px 0;
		padding: 0;
	}
	.trace li {
		padding: 8px 10px;
		border-left: 2px solid var(--border);
		margin-left: 8px;
	}
	.trace li.trace-start {
		font-weight: 600;
		border-left-color: var(--accent, #3b82f6);
	}
	.trace-cable {
		font-size: 12px;
	}
	.trace-dest {
		margin-top: 2px;
	}
</style>
