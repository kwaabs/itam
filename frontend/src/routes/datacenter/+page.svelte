<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import RackFloorPlan from '$lib/components/RackFloorPlan.svelte';
	import type { Rack, Location, LocationKind, PowerFeed, Sensor } from '$lib/types';

	type FeedRow = { feed: PowerFeed; connected_w: number };

	let locations = $state<Location[]>([]);
	let racks = $state<Rack[]>([]);
	let kinds = $state<LocationKind[]>([]);
	let feeds = $state<FeedRow[]>([]);
	let roomSensors = $state<Sensor[]>([]);
	let loading = $state(true);
	let powerLoading = $state(false);
	let error = $state('');
	let feedForm = $state({ name: '', source: 'utility', capacity_w: '', voltage: '', phase: '', redundancy: '' });

	const manage = $derived(can($me, 'dcim.manage'));
	// Drill-down position is driven by the URL so back/forward and deep links work.
	const currentId = $derived($page.url.searchParams.get('loc'));
	const current = $derived(currentId ? (locations.find((l) => l.id === currentId) ?? null) : null);

	function navigate(id: string | null) {
		goto(id ? `/datacenter?loc=${id}` : '/datacenter', { keepFocus: true, noScroll: true });
	}

	async function loadAll() {
		loading = true;
		error = '';
		try {
			locations = (await apiGet<Location[]>('/api/locations')) ?? [];
			racks = (await apiGet<Rack[]>('/api/dcim/racks')) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	async function loadPowerEnv(locId: string) {
		powerLoading = true;
		try {
			feeds = (await apiGet<FeedRow[]>(`/api/dcim/power/feeds?location_id=${locId}`)) ?? [];
			roomSensors = (await apiGet<Sensor[]>(`/api/dcim/sensors?location_id=${locId}`)) ?? [];
		} catch {
			feeds = [];
			roomSensors = [];
		} finally {
			powerLoading = false;
		}
	}

	async function addFeed() {
		if (!feedForm.name.trim() || !current) return;
		try {
			await apiPost('/api/dcim/power/feeds', {
				name: feedForm.name,
				source: feedForm.source,
				capacity_w: feedForm.capacity_w ? Number(feedForm.capacity_w) : null,
				voltage: feedForm.voltage ? Number(feedForm.voltage) : null,
				phase: feedForm.phase || null,
				redundancy: feedForm.redundancy || null,
				location_id: current.id
			});
			feedForm = { name: '', source: 'utility', capacity_w: '', voltage: '', phase: '', redundancy: '' };
			await loadPowerEnv(current.id);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add feed';
		}
	}

	async function delFeed(id: string) {
		if (!current) return;
		try {
			await apiDelete(`/api/dcim/power/feeds/${id}`);
			await loadPowerEnv(current.id);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete feed';
		}
	}

	$effect(() => {
		if (currentId) loadPowerEnv(currentId);
		else {
			feeds = [];
			roomSensors = [];
		}
	});

	const roomBreaches = $derived(roomSensors.filter((s) => s.status === 'breach'));
	const roomWarns = $derived(roomSensors.filter((s) => s.status === 'warn'));

	onMount(async () => {
		try {
			kinds = (await apiGet<LocationKind[]>('/api/metadata/location-kinds')) ?? [];
		} catch {
			kinds = [];
		}
		await loadAll();
	});

	function childrenOf(id: string | null): Location[] {
		return locations.filter((l) => l.parent_id === id);
	}
	function isUnder(node: Location, anc: Location): boolean {
		return node.path === anc.path || node.path.startsWith(anc.path + '.');
	}
	function subtreeRackCount(loc: Location): number {
		return racks.filter((r) => r.location && isUnder(r.location, loc)).length;
	}
	function racksAt(id: string): Rack[] {
		return racks.filter((r) => r.location_id === id);
	}
	function racksUnder(loc: Location): Rack[] {
		return racks.filter((r) => r.location && isUnder(r.location, loc));
	}

	const roots = $derived(childrenOf(null));
	const childLocs = $derived(current ? childrenOf(current.id) : []);
	const racksHere = $derived(current ? racksAt(current.id) : []);
	// Aggregated layout for intermediate levels (site/floor/room): every rack in
	// the subtree, grouped/labeled by the immediate sub-location it sits under.
	const subtreeRacks = $derived(current ? racksUnder(current) : []);
	const aggGroups = $derived.by(() =>
		current
			? childLocs
					.map((c) => ({ id: c.id, label: c.name, rackIds: racksUnder(c).map((r) => r.id) }))
					.filter((g) => g.rackIds.length)
			: []
	);
	const showAgg = $derived(!!current && racksHere.length === 0 && subtreeRacks.length > 0);
	const crumbs = $derived(
		current
			? locations
					.filter((l) => isUnder(current, l))
					.sort((a, b) => a.path.length - b.path.length)
			: []
	);

	// Which sub-kinds can be created under the current node (next levels down).
	const childKindOptions = $derived.by(() => {
		const order = kinds.length ? kinds.map((k) => k.key) : ['region', 'site', 'building', 'floor', 'room', 'zone', 'row'];
		if (!current) return order;
		const idx = order.indexOf(current.kind);
		return idx >= 0 ? order.slice(idx + 1) : order;
	});

	// --- inline management ---------------------------------------------------
	let showAddLoc = $state(false);
	let showAddRack = $state(false);
	let locForm = $state({ name: '', kind: '' });
	let rackForm = $state({ name: '', u_height: 42 });
	let saving = $state(false);

	function slugify(s: string): string {
		return s.toLowerCase().trim().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
	}

	async function addLocation() {
		if (!locForm.name) return;
		saving = true;
		error = '';
		try {
			const created = await apiPost<Location>('/api/locations', {
				key: slugify(locForm.name) + '_' + Math.random().toString(36).slice(2, 6),
				name: locForm.name,
				kind: locForm.kind || childKindOptions[0] || 'room',
				parent_id: current ? current.id : null
			});
			showAddLoc = false;
			locForm = { name: '', kind: '' };
			await loadAll();
			navigate(created.id);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add';
		} finally {
			saving = false;
		}
	}

	async function addRack() {
		if (!rackForm.name || !current) return;
		saving = true;
		error = '';
		try {
			const r = await apiPost<Rack>('/api/dcim/racks', {
				name: rackForm.name,
				location_id: current.id,
				u_height: Number(rackForm.u_height) || 42
			});
			showAddRack = false;
			rackForm = { name: '', u_height: 42 };
			goto(`/datacenter/${r.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add rack';
		} finally {
			saving = false;
		}
	}

	async function persistPosition(id: string, x: number, y: number, rotation: number) {
		try {
			await apiPost(`/api/dcim/racks/${id}/position`, { pos_x: x, pos_y: y, rotation });
			const r = racks.find((rk) => rk.id === id);
			if (r) {
				r.pos_x = x;
				r.pos_y = y;
				r.rotation = rotation;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save position';
		}
	}

	// --- rack delete / move --------------------------------------------------
	let moveRack = $state<Rack | null>(null);
	let moveDest = $state('');

	function depthOf(path: string): number {
		return Math.max(0, path.split('.').length - 1);
	}
	// Possible destinations: any location except the rack's current one.
	const moveTargets = $derived(
		moveRack
			? locations
					.filter((l) => l.id !== moveRack!.location_id)
					.slice()
					.sort((a, b) => a.path.localeCompare(b.path))
			: []
	);

	async function deleteRack(r: Rack) {
		const mounts = (r.mounts ?? []).length;
		const warn = mounts
			? ` ${mounts} mounted asset(s) will be unmounted (the assets themselves are kept).`
			: '';
		if (!confirm(`Delete rack "${r.name}"?${warn}`)) return;
		error = '';
		try {
			await apiDelete(`/api/dcim/racks/${r.id}`);
			await loadAll();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Delete failed';
		}
	}

	function openMoveRack(r: Rack) {
		moveRack = r;
		moveDest = '';
	}

	async function confirmMoveRack() {
		if (!moveRack || !moveDest) return;
		saving = true;
		error = '';
		try {
			await apiPost(`/api/dcim/racks/${moveRack.id}/move`, { location_id: moveDest });
			moveRack = null;
			await loadAll();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Move failed';
		} finally {
			saving = false;
		}
	}

	function kindIcon(kind: string): string {
		switch (kind) {
			case 'region': return '🌍';
			case 'site': return '🏢';
			case 'building': return '🏬';
			case 'floor': return '🗂';
			case 'room': return '🚪';
			case 'zone': return '🧭';
			case 'row': return '↔';
			default: return '📍';
		}
	}
</script>

<div class="topbar">
	<div>
		<h1 style="margin-bottom:4px">Data Center</h1>
		<div class="crumbs">
			<button class="crumb" onclick={() => navigate(null)}>Data Centers</button>
			{#each crumbs as c}
				<span class="sep">/</span>
				{#if c.id === currentId}
					<span class="crumb current">{c.name}</span>
				{:else}
					<button class="crumb" onclick={() => navigate(c.id)}>{c.name}</button>
				{/if}
			{/each}
		</div>
	</div>
	<div class="row">
		{#if manage && current}
			{#if childKindOptions.length}
				<button class="btn secondary" onclick={() => (showAddLoc = !showAddLoc)}>+ Sub-location</button>
			{/if}
			<button class="btn" onclick={() => (showAddRack = !showAddRack)}>+ Rack here</button>
		{/if}
	</div>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if showAddLoc}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">Add sub-location under {current?.name}</h3>
		<div class="grid cols-2">
			<label class="field">Name <input bind:value={locForm.name} placeholder="Floor 2 / Hall A / Row 3" /></label>
			<label class="field">
				Kind
				<select bind:value={locForm.kind}>
					{#each childKindOptions as k}<option value={k}>{k}</option>{/each}
				</select>
			</label>
		</div>
		<button class="btn" onclick={addLocation} disabled={saving}>{saving ? 'Saving…' : 'Create'}</button>
	</div>
{/if}

{#if showAddRack}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">Add rack to {current?.name}</h3>
		<div class="grid cols-2">
			<label class="field">Name <input bind:value={rackForm.name} placeholder="Rack B2" /></label>
			<label class="field">Height (U) <input type="number" min="1" max="60" bind:value={rackForm.u_height} /></label>
		</div>
		<button class="btn" onclick={addRack} disabled={saving}>{saving ? 'Saving…' : 'Create rack'}</button>
	</div>
{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if !current}
	<!-- Level 0: data center sites -->
	{#if roots.length === 0}
		<div class="card"><p class="muted">No locations yet. Create sites under Locations.</p></div>
	{:else}
		<div class="grid cols-3">
			{#each roots as site}
				<button class="node-card" onclick={() => navigate(site.id)}>
					<div class="node-ico">{kindIcon(site.kind)}</div>
					<div class="node-body">
						<div class="node-name">{site.name}</div>
						<div class="muted node-sub">
							{site.kind}{site.dr_role ? ` · ${site.dr_role}` : ''}
						</div>
						<div class="node-meta">{subtreeRackCount(site)} racks</div>
					</div>
				</button>
			{/each}
		</div>
	{/if}
{:else}
	<!-- Inside a node: show child locations and/or racks -->
	{#if childLocs.length}
		<h3 class="section-h">Sub-locations</h3>
		<div class="grid cols-3" style="margin-bottom:20px">
			{#each childLocs as child}
				<button class="node-card" onclick={() => navigate(child.id)}>
					<div class="node-ico">{kindIcon(child.kind)}</div>
					<div class="node-body">
						<div class="node-name">{child.name}</div>
						<div class="muted node-sub">{child.kind}</div>
						<div class="node-meta">{subtreeRackCount(child)} racks</div>
					</div>
				</button>
			{/each}
		</div>
	{/if}

	{#if racksHere.length}
		<h3 class="section-h">Floor layout · {racksHere.length} rack(s)</h3>
		<div class="card" style="margin-bottom:16px">
			<RackFloorPlan
				racks={racksHere}
				canManage={manage}
				onmove={persistPosition}
				onopen={(id) => goto(`/datacenter/${id}`)}
			/>
			<p class="muted" style="margin-top:8px">
				{#if manage}Drag to arrange · click to open{:else}Click a rack to open{/if}
			</p>
		</div>
		<div class="card">
			<table>
				<thead><tr><th>Rack</th><th>Height</th><th>Occupied</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each racksHere as r}
						{@const used = (r.mounts ?? []).reduce((n, m) => n + m.u_height, 0)}
						<tr
							role="button"
							tabindex="0"
							onclick={() => goto(`/datacenter/${r.id}`)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/datacenter/${r.id}`))}
						>
							<td>{r.name}</td>
							<td>{r.u_height}U</td>
							<td class="muted">{used}U used</td>
							{#if manage}
								<td style="text-align:right; white-space:nowrap">
									<button class="btn secondary small" onclick={(e) => { e.stopPropagation(); openMoveRack(r); }}>Move</button>
									<button class="btn danger small" onclick={(e) => { e.stopPropagation(); deleteRack(r); }}>Delete</button>
								</td>
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if showAgg}
		<h3 class="section-h">
			Layout · {subtreeRacks.length} rack(s) across {aggGroups.length}
			{aggGroups.length === 1 ? 'area' : 'areas'}
		</h3>
		<div class="card" style="margin-bottom:16px">
			<RackFloorPlan
				racks={subtreeRacks}
				groups={aggGroups}
				canManage={false}
				onopen={(id) => goto(`/datacenter/${id}`)}
			/>
			<p class="muted" style="margin-top:8px">
				Racks grouped by sub-location · click a rack to open · drill into a {childKindOptions[0] ?? 'zone'} to drag &amp; arrange
			</p>
		</div>
		<div class="card">
			<table>
				<thead><tr><th>Rack</th><th>Location</th><th>Height</th><th>Occupied</th></tr></thead>
				<tbody>
					{#each subtreeRacks as r}
						{@const used = (r.mounts ?? []).reduce((n, m) => n + m.u_height, 0)}
						<tr
							role="button"
							tabindex="0"
							onclick={() => goto(`/datacenter/${r.id}`)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), goto(`/datacenter/${r.id}`))}
						>
							<td>{r.name}</td>
							<td class="muted">{r.location?.name ?? '—'}</td>
							<td>{r.u_height}U</td>
							<td class="muted">{used}U used</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if childLocs.length === 0}
		<div class="card">
			<p class="muted">
				Nothing here yet.
				{#if manage}Add a sub-location or a rack to start building out {current.name}.{/if}
			</p>
		</div>
	{/if}

	<section id="power-env" class="pe-section">
		<h3 class="section-h">Power &amp; environment · {current.name}</h3>
		<p class="muted small" style="margin:-4px 0 12px">
			Room/zone-level feeds and sensors for this location. Rack PDUs and rack sensors are managed on each rack page.
		</p>
		{#if powerLoading}
			<p class="muted">Loading…</p>
		{:else}
			<div class="pe-grid">
				<div class="card">
					<h4 style="margin-top:0">Power feeds</h4>
					<table class="mini">
						<thead><tr><th>Feed</th><th>Source</th><th>Capacity</th><th>PDU load</th><th>Headroom</th>{#if manage}<th></th>{/if}</tr></thead>
						<tbody>
							{#each feeds as row}
								{@const cap = row.feed.capacity_w ?? 0}
								{@const head = cap - row.connected_w}
								<tr>
									<td><strong>{row.feed.name}</strong>{row.feed.redundancy ? ` · ${row.feed.redundancy}` : ''}</td>
									<td class="muted">{row.feed.source}</td>
									<td>{cap ? `${cap} W` : '—'}</td>
									<td>{row.connected_w} W</td>
									<td class={cap > 0 && head < 0 ? 'over' : ''}>{cap ? `${head} W` : '—'}</td>
									{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delFeed(row.feed.id)}>remove</button></td>{/if}
								</tr>
							{/each}
							{#if feeds.length === 0}
								<tr><td colspan={manage ? 6 : 5} class="muted">No feeds at this location.{#if childLocs.length} Usually defined at room/zone level — drill down if needed.{/if}</td></tr>
							{/if}
						</tbody>
					</table>
					{#if manage}
						<div class="row wrap" style="margin-top:10px">
							<input placeholder="Feed name (e.g. Feed A)" bind:value={feedForm.name} style="flex:1; min-width:140px" />
							<select bind:value={feedForm.source}>
								<option value="utility">utility</option>
								<option value="ups">ups</option>
								<option value="generator">generator</option>
								<option value="other">other</option>
							</select>
							<input type="number" placeholder="Capacity W" bind:value={feedForm.capacity_w} style="width:110px" />
							<input type="number" placeholder="Volts" bind:value={feedForm.voltage} style="width:80px" />
							<input placeholder="Phase" bind:value={feedForm.phase} style="width:70px" />
							<input placeholder="Redundancy" bind:value={feedForm.redundancy} style="width:110px" />
							<button class="btn small" onclick={addFeed}>Add feed</button>
						</div>
					{/if}
				</div>

				<div class="card">
					<div class="env-head">
						<h4 style="margin:0">Room sensors</h4>
						<div class="badges">
							<span class="pill breach">{roomBreaches.length} breach</span>
							<span class="pill warn">{roomWarns.length} warn</span>
							<span class="pill ok">{roomSensors.length} total</span>
						</div>
					</div>
					<table class="mini" style="margin-top:10px">
						<thead><tr><th>Sensor</th><th>Metric</th><th>Reading</th><th>Range</th><th>Status</th></tr></thead>
						<tbody>
							{#each roomSensors as sen}
								<tr>
									<td>{sen.name}</td>
									<td class="muted">{sen.metric}</td>
									<td>{sen.last_value != null ? `${sen.last_value}${sen.unit ?? ''}` : '—'}</td>
									<td class="muted">{sen.min_threshold ?? '−∞'} … {sen.max_threshold ?? '∞'}</td>
									<td><span class="pill {sen.status}">{sen.status ?? 'unknown'}</span></td>
								</tr>
							{/each}
							{#if roomSensors.length === 0}
								<tr><td colspan="5" class="muted">No room-level sensors here. Add temperature/humidity sensors at this location, or rack sensors on a rack page.</td></tr>
							{/if}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	</section>
{/if}

{#if moveRack}
	<div
		class="modal-backdrop"
		role="button"
		tabindex="0"
		aria-label="Close dialog"
		onclick={(e) => { if (e.target === e.currentTarget) moveRack = null; }}
		onkeydown={(e) => e.key === 'Escape' && (moveRack = null)}
	>
		<div class="modal">
			<h3 style="margin-top:0">Move rack “{moveRack.name}”</h3>
			<div class="field">
				<label>Destination location</label>
				<select bind:value={moveDest}>
					<option value="">Select location…</option>
					{#each moveTargets as l}
						<option value={l.id}>{'— '.repeat(depthOf(l.path))}{l.name} ({l.kind})</option>
					{/each}
				</select>
			</div>
			<p class="muted" style="margin-top:0">Floor position resets on the new layout.</p>
			<div class="modal-actions">
				<button class="btn secondary" onclick={() => (moveRack = null)}>Cancel</button>
				<button class="btn" onclick={confirmMoveRack} disabled={saving || !moveDest}>
					{saving ? 'Moving…' : 'Move rack'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.crumbs { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
	.crumb {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
		font: inherit;
		padding: 0;
	}
	.crumb.current { color: var(--text); cursor: default; }
	.crumbs .sep { color: var(--muted); }
	.section-h { margin: 8px 0 10px; }
	.node-card {
		display: flex;
		gap: 12px;
		align-items: center;
		text-align: left;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 16px;
		cursor: pointer;
		font: inherit;
		color: var(--text);
		transition: border-color 0.15s;
	}
	.node-card:hover { border-color: var(--primary); }
	.node-ico { font-size: 26px; line-height: 1; }
	.node-name { font-weight: 600; font-size: 15px; }
	.node-sub { font-size: 12px; text-transform: capitalize; }
	.node-meta { font-size: 12px; color: var(--muted); margin-top: 4px; }
	.pe-section { margin-top: 24px; padding-top: 8px; border-top: 1px solid var(--border); }
	.pe-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; align-items: start; }
	@media (max-width: 900px) { .pe-grid { grid-template-columns: 1fr; } }
	table.mini { width: 100%; border-collapse: collapse; font-size: 13px; }
	table.mini th, table.mini td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--border); }
	.row.wrap { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
	.over { color: var(--danger); font-weight: 600; }
	.env-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; }
	.badges { display: flex; gap: 6px; }
	.pill { font-size: 11px; padding: 1px 8px; border-radius: 10px; text-transform: uppercase; background: var(--surface-2); color: var(--muted); }
	.pill.ok { background: color-mix(in srgb, var(--success) 18%, transparent); color: var(--success); }
	.pill.warn { background: color-mix(in srgb, #c98a00 22%, transparent); color: #c98a00; }
	.pill.breach { background: color-mix(in srgb, var(--danger) 20%, transparent); color: var(--danger); }
	.link-danger { background: none; border: none; color: var(--danger); cursor: pointer; font-size: 12px; }
	.small { font-size: 12px; }
</style>
