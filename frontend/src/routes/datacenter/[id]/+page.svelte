<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type { Rack, Paginated, Asset, RackPower, PowerFeed, Sensor } from '$lib/types';

	const id = $page.params.id;
	let rack = $state<Rack | null>(null);
	let assets = $state<Asset[]>([]);
	let loading = $state(true);
	let error = $state('');

	let mount = $state({ asset_id: '', position: 1, u_height: 1, face: 'front' });

	// power + environment
	let power = $state<RackPower | null>(null);
	let feeds = $state<PowerFeed[]>([]);
	let sensors = $state<Sensor[]>([]);
	const manage = $derived(can($me, 'dcim.manage'));
	let pduForm = $state({ name: '', feed_id: '', capacity_w: '' });
	let senForm = $state({ name: '', metric: 'temperature', unit: '°C', max_threshold: '', min_threshold: '' });
	let readingFor = $state<string | null>(null);
	let readingVal = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			rack = await apiGet<Rack>(`/api/dcim/racks/${id}`);
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	async function loadPowerEnv() {
		try {
			power = await apiGet<RackPower>(`/api/dcim/racks/${id}/power`);
			sensors = (await apiGet<Sensor[]>(`/api/dcim/sensors?rack_id=${id}`)) ?? [];
			feeds = ((await apiGet<{ feed: PowerFeed }[]>('/api/dcim/power/feeds')) ?? []).map((x) => x.feed);
		} catch {
			/* ignore */
		}
	}

	function feedName(fid: string | null): string {
		return feeds.find((f) => f.id === fid)?.name ?? '—';
	}

	async function addPDU() {
		if (!pduForm.name.trim()) return;
		try {
			await apiPost(`/api/dcim/racks/${id}/pdus`, {
				name: pduForm.name,
				feed_id: pduForm.feed_id || null,
				capacity_w: pduForm.capacity_w ? Number(pduForm.capacity_w) : null
			});
			pduForm = { name: '', feed_id: '', capacity_w: '' };
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add PDU';
		}
	}

	async function delPDU(pduId: string) {
		try {
			await apiDelete(`/api/dcim/pdus/${pduId}`);
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete PDU';
		}
	}

	async function setRackCapacity(v: string) {
		if (!rack) return;
		try {
			await apiPut(`/api/dcim/racks/${id}`, { ...rack, power_capacity_w: v ? Number(v) : null });
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update capacity';
		}
	}

	async function addSensor() {
		if (!senForm.name.trim()) return;
		try {
			await apiPost('/api/dcim/sensors', {
				name: senForm.name,
				metric: senForm.metric,
				unit: senForm.unit,
				rack_id: id,
				location_id: rack?.location_id ?? null,
				max_threshold: senForm.max_threshold ? Number(senForm.max_threshold) : null,
				min_threshold: senForm.min_threshold ? Number(senForm.min_threshold) : null
			});
			senForm = { name: '', metric: 'temperature', unit: '°C', max_threshold: '', min_threshold: '' };
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add sensor';
		}
	}

	async function delSensor(sid: string) {
		try {
			await apiDelete(`/api/dcim/sensors/${sid}`);
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete sensor';
		}
	}

	async function recordReading(sid: string) {
		if (!readingVal) return;
		try {
			await apiPost(`/api/dcim/sensors/${sid}/readings`, { value: Number(readingVal), source: 'manual' });
			readingFor = null;
			readingVal = '';
			await loadPowerEnv();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to record reading';
		}
	}

	onMount(async () => {
		try {
			assets = (await apiGet<Paginated<Asset>>('/api/assets?page_size=200')).items ?? [];
		} catch {
			/* ignore */
		}
		load();
	});

	const minU = $derived(rack ? rack.starting_unit : 1);
	const maxU = $derived(rack ? rack.starting_unit + rack.u_height - 1 : 1);
	const mounts = $derived(rack?.mounts ?? []);
	const mountedHere = $derived(new Set(mounts.map((m) => m.asset_id)));

	// Build the elevation rows, top-of-rack first.
	type Slot =
		| { kind: 'device'; u: number; mount: NonNullable<Rack['mounts']>[number] }
		| { kind: 'empty'; u: number };

	const slots = $derived.by<Slot[]>(() => {
		if (!rack) return [];
		const order: number[] = [];
		if (rack.desc_units) for (let u = minU; u <= maxU; u++) order.push(u);
		else for (let u = maxU; u >= minU; u--) order.push(u);

		const occupied = new Set<number>();
		const startAt = new Map<number, Slot>();
		for (const m of mounts) {
			for (let i = 0; i < m.u_height; i++) occupied.add(m.position + i);
			const top = rack.desc_units ? m.position : m.position + m.u_height - 1;
			startAt.set(top, { kind: 'device', u: top, mount: m });
		}
		const out: Slot[] = [];
		for (const u of order) {
			if (startAt.has(u)) out.push(startAt.get(u)!);
			else if (!occupied.has(u)) out.push({ kind: 'empty', u });
		}
		return out;
	});

	function pickEmpty(u: number) {
		mount.position = u;
	}

	function suggestHeight(assetId: string) {
		const a = assets.find((x) => x.id === assetId);
		const ru = a?.attributes?.['rack_units'];
		if (typeof ru === 'number' && ru > 0) mount.u_height = ru;
	}

	async function doMount() {
		if (!mount.asset_id) {
			error = 'Choose an asset to mount';
			return;
		}
		error = '';
		try {
			await apiPost(`/api/dcim/racks/${id}/mounts`, {
				asset_id: mount.asset_id,
				position: Number(mount.position),
				u_height: Number(mount.u_height) || 1,
				face: mount.face
			});
			mount = { asset_id: '', position: 1, u_height: 1, face: 'front' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Mount failed';
		}
	}

	async function unmount(mountId: string) {
		error = '';
		try {
			await apiDelete(`/api/dcim/mounts/${mountId}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unmount failed';
		}
	}

	const ROW = 32;
</script>

<div class="topbar">
	<div>
		<h1 style="margin-bottom:4px">{rack?.name ?? 'Rack'}</h1>
		{#if rack?.location}
			<div class="muted">{rack.location.name}</div>
		{/if}
		{#if rack?.asset_id}
			<div class="muted small" style="margin-top:4px">
				Linked asset · <a href="/assets/{rack.asset_id}">open asset record</a>
			</div>
		{/if}
	</div>
	<button
		class="btn secondary"
		onclick={() => goto(rack?.location_id ? `/datacenter?loc=${rack.location_id}` : '/datacenter')}
	>
		← Back to floor
	</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if rack}
	<div class="dc-grid">
		<div class="card elev-card">
			<h3 style="margin-top:0">Elevation · {rack.u_height}U {rack.location ? `· ${rack.location.name}` : ''}</h3>
			<div class="rack">
				{#each slots as s}
					{#if s.kind === 'device'}
						<div class="u-row device" style="height:{Math.max(s.mount.u_height * ROW, ROW)}px">
							<span class="u-label">U{s.mount.position}{s.mount.u_height > 1 ? `–${s.mount.position + s.mount.u_height - 1}` : ''}</span>
							<button class="dev" onclick={() => goto(`/assets/${s.mount.asset_id}`)} title="Open asset">
								{#if s.mount.u_height === 1}
									<span class="dev-name dev-one-line">
										{s.mount.asset?.name ?? s.mount.asset?.asset_tag ?? 'device'}
										<span class="dev-meta-inline"> · {s.mount.asset?.asset_tag} · {s.mount.face}</span>
									</span>
								{:else}
									<span class="dev-name">{s.mount.asset?.name ?? s.mount.asset?.asset_tag ?? 'device'}</span>
									<span class="dev-meta">{s.mount.asset?.asset_tag} · {s.mount.face}</span>
								{/if}
							</button>
							{#if can($me, 'dcim.manage')}
								<button class="x" title="Unmount" onclick={() => unmount(s.mount.id)}>✕</button>
							{/if}
						</div>
					{:else}
						<div class="u-row empty" style="height:{ROW}px" onclick={() => pickEmpty(s.u)} onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), pickEmpty(s.u))} role="button" tabindex="0">
							<span class="u-label">U{s.u}</span>
							<span class="free">{can($me, 'dcim.manage') ? 'click to fill' : 'empty'}</span>
						</div>
					{/if}
				{/each}
			</div>
		</div>

		<div class="dc-side">
			{#if can($me, 'dcim.manage')}
				<div class="card mount-card">
					<h3 style="margin-top:0">Mount a device</h3>
					<div class="mount-form">
						<label class="field">
							Asset
							<select bind:value={mount.asset_id} onchange={() => suggestHeight(mount.asset_id)}>
								<option value="">— choose —</option>
								{#each assets.filter((a) => !mountedHere.has(a.id)) as a}
									<option value={a.id}>{a.asset_tag} · {a.name}</option>
								{/each}
							</select>
						</label>
						<label class="field">Position (U) <input type="number" min={minU} max={maxU} bind:value={mount.position} /></label>
						<label class="field">Height (U) <input type="number" min="1" bind:value={mount.u_height} /></label>
						<label class="field">
							Face
							<select bind:value={mount.face}>
								<option value="front">Front</option>
								<option value="rear">Rear</option>
								<option value="full">Full depth</option>
							</select>
						</label>
						<button class="btn" onclick={doMount}>Mount</button>
					</div>
					<p class="muted small" style="margin:10px 0 0">Overlapping units are rejected. Mounting records the move on the asset timeline.</p>
				</div>
			{/if}

			<div class="dc-pe">
				<div class="card">
				<h3 style="margin-top:0">Rack power</h3>
				{#if rack?.location}
					<p class="muted small" style="margin:0 0 10px">
						<a href="/datacenter?loc={rack.location_id}#power-env">Room feeds → {rack.location.name}</a>
					</p>
				{/if}
				{#if power}
					{@const cap = power.capacity_w}
					{@const pct = cap > 0 ? Math.min(100, (power.draw_w / cap) * 100) : 0}
					<div class="pwr-head">
						<div class="pwr-num"><strong>{power.draw_w.toFixed(0)} W</strong> draw{cap > 0 ? ` of ${cap} W` : ''}</div>
						{#if cap > 0}<div class="pwr-pct" class:hot={pct >= 90} class:warn={pct >= 75 && pct < 90}>{pct.toFixed(0)}%</div>{/if}
					</div>
					<div class="bar"><span style="width:{pct}%" class:hot={pct >= 90} class:warn={pct >= 75 && pct < 90}></span></div>
					<p class="muted small" style="margin:6px 0 0">
						Draw is summed from each mounted device's <code>power_watts</code>. Capacity prefers total PDU capacity{rack?.power_capacity_w ? `; rack design = ${rack.power_capacity_w} W` : ''}.
					</p>

					{#if manage}
						<label class="field" style="margin-top:10px">Rack design capacity (W)
							<input type="number" value={rack?.power_capacity_w ?? ''} onchange={(e) => setRackCapacity((e.target as HTMLInputElement).value)} placeholder="e.g. 7200" />
						</label>
					{/if}

					<h4 style="margin:14px 0 6px">PDUs</h4>
					<table class="mini">
						<thead><tr><th>Name</th><th>Feed</th><th>Capacity</th>{#if manage}<th></th>{/if}</tr></thead>
						<tbody>
							{#each power.pdus as p}
								<tr>
									<td>{p.name}</td>
									<td class="muted">{p.feed?.name ?? feedName(p.feed_id)}</td>
									<td>{p.capacity_w ? `${p.capacity_w} W` : '—'}</td>
									{#if manage}<td style="text-align:right"><button class="link-danger" onclick={() => delPDU(p.id)}>remove</button></td>{/if}
								</tr>
							{/each}
							{#if power.pdus.length === 0}<tr><td colspan={manage ? 4 : 3} class="muted">No PDUs yet.</td></tr>{/if}
						</tbody>
					</table>
					{#if manage}
						<div class="row" style="margin-top:8px">
							<input placeholder="PDU name" bind:value={pduForm.name} style="flex:1" />
							<select bind:value={pduForm.feed_id}>
								<option value="">No feed</option>
								{#each feeds as f}<option value={f.id}>{f.name}</option>{/each}
							</select>
							<input type="number" placeholder="W" bind:value={pduForm.capacity_w} style="width:90px" />
							<button class="btn small" onclick={addPDU}>Add</button>
						</div>
						{#if feeds.length === 0}<p class="muted small" style="margin-top:6px">Tip: create power feeds on the room page under Power &amp; environment.</p>{/if}
					{/if}
				{:else}
					<p class="muted">No power data.</p>
				{/if}
			</div>

			<div class="card">
				<h3 style="margin-top:0">Rack sensors</h3>
				<p class="muted small" style="margin:0 0 8px">Sensors mounted on this rack. Room-level sensors are on the location page.</p>
				<table class="mini">
					<thead><tr><th>Sensor</th><th>Metric</th><th>Reading</th><th>Status</th>{#if manage}<th></th>{/if}</tr></thead>
					<tbody>
						{#each sensors as sen}
							<tr>
								<td>{sen.name}</td>
								<td class="muted">{sen.metric}</td>
								<td>{sen.last_value != null ? `${sen.last_value}${sen.unit ?? ''}` : '—'}</td>
								<td><span class="pill {sen.status}">{sen.status ?? 'unknown'}</span></td>
								{#if manage}
									<td style="text-align:right; white-space:nowrap">
										{#if readingFor === sen.id}
											<input type="number" bind:value={readingVal} style="width:70px" placeholder="value" />
											<button class="btn small" onclick={() => recordReading(sen.id)}>Save</button>
										{:else}
											<button class="link" onclick={() => { readingFor = sen.id; readingVal = ''; }}>+ reading</button>
										{/if}
										<button class="link-danger" onclick={() => delSensor(sen.id)}>remove</button>
									</td>
								{/if}
							</tr>
						{/each}
						{#if sensors.length === 0}<tr><td colspan={manage ? 5 : 4} class="muted">No sensors on this rack.</td></tr>{/if}
					</tbody>
				</table>
				{#if manage}
					<div class="row" style="margin-top:8px; flex-wrap:wrap">
						<input placeholder="Sensor name" bind:value={senForm.name} style="flex:1; min-width:120px" />
						<select bind:value={senForm.metric}>
							<option value="temperature">temperature</option>
							<option value="humidity">humidity</option>
							<option value="airflow">airflow</option>
							<option value="power">power</option>
							<option value="water">water</option>
						</select>
						<input placeholder="unit" bind:value={senForm.unit} style="width:64px" />
						<input type="number" placeholder="min" bind:value={senForm.min_threshold} style="width:70px" />
						<input type="number" placeholder="max" bind:value={senForm.max_threshold} style="width:70px" />
						<button class="btn small" onclick={addSensor}>Add</button>
					</div>
					<p class="muted small" style="margin-top:6px">A reading past min/max emits <code>itam.env.threshold</code> — route it to a channel under Notifications to get alerted.</p>
				{/if}
			</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.dc-grid {
		display: grid;
		grid-template-columns: minmax(300px, 400px) minmax(0, 1fr);
		gap: 16px;
		align-items: start;
	}
	.dc-side {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-width: 0;
	}
	.dc-pe {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 16px;
		align-items: start;
	}
	.mount-form {
		display: grid;
		grid-template-columns: minmax(160px, 1fr) 72px 72px 100px auto;
		gap: 10px;
		align-items: end;
	}
	.mount-form .field {
		margin: 0;
	}
	.mount-form .btn {
		height: 38px;
		white-space: nowrap;
	}
	.elev-card {
		position: sticky;
		top: 12px;
	}
	.rack {
		border: 2px solid var(--rack-elev-border);
		border-radius: 6px;
		padding: 6px;
		background: var(--rack-elev-bg);
	}
	.u-row {
		display: flex;
		align-items: center;
		gap: 10px;
		border-radius: 4px;
		padding: 0 10px;
		box-sizing: border-box;
		min-height: 32px;
	}
	.u-label {
		flex: none;
		width: 52px;
		font-size: 11px;
		font-family: ui-monospace, monospace;
		font-weight: 600;
		color: var(--rack-elev-u-label);
		letter-spacing: -0.02em;
	}
	.u-row.empty {
		color: var(--rack-elev-empty);
		font-size: 12px;
		cursor: pointer;
		border: 1px dashed var(--rack-elev-empty-border);
		margin: 2px 0;
	}
	.u-row.empty:hover {
		border-color: var(--rack-elev-empty-hover-border);
		color: var(--rack-elev-empty-hover);
	}
	.u-row.empty .free {
		flex: 1;
		text-align: center;
		font-style: italic;
		line-height: 1.2;
	}
	.u-row.device {
		background: var(--rack-elev-device-bg);
		border: 1px solid var(--rack-elev-device-border);
		margin: 2px 0;
		padding-top: 4px;
		padding-bottom: 4px;
		align-items: center;
	}
	.dev {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		justify-content: center;
		gap: 2px;
		min-width: 0;
		background: none;
		border: none;
		color: var(--rack-elev-dev-text);
		cursor: pointer;
		text-align: left;
		padding: 0;
		font: inherit;
		line-height: 1.25;
		text-decoration: none;
		appearance: none;
	}
	.dev:hover .dev-name {
		color: var(--primary);
	}
	.dev-name {
		font-size: 13px;
		font-weight: 600;
		line-height: 1.2;
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dev-meta {
		font-size: 11px;
		line-height: 1.2;
		color: var(--rack-elev-dev-meta);
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-decoration: none;
	}
	.dev-one-line {
		font-weight: 600;
	}
	.dev-meta-inline {
		font-weight: 400;
		color: var(--rack-elev-dev-meta);
	}
	.x {
		flex: none;
		align-self: center;
		background: none;
		border: none;
		color: #f87171;
		cursor: pointer;
		font-size: 14px;
		line-height: 1;
		padding: 2px 4px;
	}
	.x:hover {
		color: #ef4444;
	}
	@media (max-width: 960px) {
		.dc-grid {
			grid-template-columns: 1fr;
		}
		.elev-card {
			position: static;
		}
		.mount-form {
			grid-template-columns: 1fr 1fr;
		}
		.mount-form .btn {
			grid-column: 1 / -1;
		}
		.dc-pe {
			grid-template-columns: 1fr;
		}
	}

	.pwr-head {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		margin-bottom: 6px;
	}
	.pwr-pct {
		font-weight: 700;
		color: var(--success);
	}
	.pwr-pct.warn {
		color: #c98a00;
	}
	.pwr-pct.hot {
		color: var(--danger);
	}
	.bar {
		height: 10px;
		background: var(--surface-2);
		border-radius: 6px;
		overflow: hidden;
	}
	.bar span {
		display: block;
		height: 100%;
		background: var(--success);
		border-radius: 6px;
		transition: width 0.2s ease;
	}
	.bar span.warn {
		background: #c98a00;
	}
	.bar span.hot {
		background: var(--danger);
	}
	table.mini {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	table.mini th,
	table.mini td {
		text-align: left;
		padding: 6px 8px;
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
	.pill {
		font-size: 11px;
		padding: 1px 8px;
		border-radius: 10px;
		text-transform: uppercase;
		background: var(--surface-2);
		color: var(--muted);
	}
	.pill.ok {
		background: color-mix(in srgb, var(--success) 18%, transparent);
		color: var(--success);
	}
	.pill.warn {
		background: color-mix(in srgb, #c98a00 22%, transparent);
		color: #c98a00;
	}
	.pill.breach {
		background: color-mix(in srgb, var(--danger) 20%, transparent);
		color: var(--danger);
	}
	.link {
		background: none;
		border: none;
		color: var(--primary);
		cursor: pointer;
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
