<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet } from '$lib/api';
	import type { Topology, TopoNode, Location } from '$lib/types';

	let topo = $state<Topology>({ nodes: [], links: [] });
	let locations = $state<Location[]>([]);
	let scope = $state('');
	let overlay = $state<'none' | 'location' | 'vlan' | 'subnet'>('none');
	let showPorts = $state(false);
	let loading = $state(true);
	let error = $state('');
	let hover = $state<string | null>(null);

	const palette = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#a855f7', '#06b6d4', '#ec4899', '#84cc16', '#f97316', '#14b8a6'];

	function catOf(node: TopoNode): string {
		if (overlay === 'location') return node.location || '—';
		if (overlay === 'vlan') return node.vlans && node.vlans.length ? node.vlans.map((v) => `VLAN ${v}`).join(', ') : '—';
		if (overlay === 'subnet') return node.subnets && node.subnets.length ? node.subnets.join(', ') : '—';
		return '';
	}
	const legend = $derived.by(() => {
		if (overlay === 'none') return [] as { cat: string; color: string }[];
		const cats = Array.from(new Set(topo.nodes.map(catOf)));
		return cats.map((c, i) => ({ cat: c, color: c === '—' ? '#6b7280' : palette[i % palette.length] }));
	});
	function colorFor(node: TopoNode): string {
		if (overlay === 'none') return '';
		const c = catOf(node);
		return legend.find((l) => l.cat === c)?.color ?? '#6b7280';
	}

	const W = 900;
	const H = 600;

	// Layout mode: force-directed (organic) or ELK-style layered hierarchy.
	let layout = $state<'force' | 'hier'>('force');
	let hierDir = $state<'TB' | 'LR'>('TB');
	// Viewbox dimensions — grow for hierarchical layouts that exceed the base canvas.
	let vbW = $state(W);
	let vbH = $state(H);

	let svgEl = $state<SVGSVGElement | null>(null);

	// Live positions published from the simulation, keyed by node id.
	let nodePos = $state<Record<string, { x: number; y: number }>>({});

	type Sim = { id: string; x: number; y: number; vx: number; vy: number };
	let sim: Sim[] = [];
	let edges: { a: number; b: number }[] = [];
	let raf = 0;
	let iter = 0;
	let running = false;
	let dragId = $state<string | null>(null);

	function tick() {
		const kRep = 9000;
		const kSpring = 0.025;
		const rest = 120;
		const center = 0.012;
		const damp = 0.86;

		for (let i = 0; i < sim.length; i++) {
			const a = sim[i];
			for (let j = i + 1; j < sim.length; j++) {
				const b = sim[j];
				const dx = a.x - b.x;
				const dy = a.y - b.y;
				const d2 = dx * dx + dy * dy || 0.01;
				const d = Math.sqrt(d2);
				const f = kRep / d2;
				a.vx += (dx / d) * f;
				a.vy += (dy / d) * f;
				b.vx -= (dx / d) * f;
				b.vy -= (dy / d) * f;
			}
		}
		for (const e of edges) {
			const a = sim[e.a];
			const b = sim[e.b];
			const dx = b.x - a.x;
			const dy = b.y - a.y;
			const d = Math.sqrt(dx * dx + dy * dy) || 0.01;
			const f = kSpring * (d - rest);
			a.vx += (dx / d) * f;
			a.vy += (dy / d) * f;
			b.vx -= (dx / d) * f;
			b.vy -= (dy / d) * f;
		}
		for (const s of sim) {
			if (s.id === dragId) {
				s.vx = 0;
				s.vy = 0;
				continue;
			}
			s.vx += (W / 2 - s.x) * center;
			s.vy += (H / 2 - s.y) * center;
			s.vx *= damp;
			s.vy *= damp;
			s.x = Math.max(40, Math.min(W - 40, s.x + s.vx));
			s.y = Math.max(40, Math.min(H - 40, s.y + s.vy));
		}
		const snap: Record<string, { x: number; y: number }> = {};
		for (const s of sim) snap[s.id] = { x: s.x, y: s.y };
		nodePos = snap;

		iter++;
		if (iter < 600 || dragId) {
			raf = requestAnimationFrame(tick);
		} else {
			running = false;
		}
	}

	function ensureRunning() {
		if (!running) {
			running = true;
			raf = requestAnimationFrame(tick);
		}
	}

	// ELK-style layered layout: assign nodes to layers by BFS distance from the
	// highest-degree node in each component, reduce crossings with a few
	// barycenter sweeps, then position layers along the chosen direction. Gives
	// the clean top-down / left-right hierarchy you get from Mermaid/ELK.
	function computeHierarchy() {
		const ids = topo.nodes.map((n) => n.id);
		const idSet = new Set(ids);
		const adj = new Map<string, string[]>();
		for (const id of ids) adj.set(id, []);
		for (const l of topo.links) {
			if (idSet.has(l.a_asset_id) && idSet.has(l.b_asset_id)) {
				adj.get(l.a_asset_id)!.push(l.b_asset_id);
				adj.get(l.b_asset_id)!.push(l.a_asset_id);
			}
		}
		const deg = (id: string) => adj.get(id)!.length;

		const layerOf = new Map<string, number>();
		const visited = new Set<string>();
		// Seed each connected component from its most-connected node so cores sit on top.
		for (const seed of [...ids].sort((a, b) => deg(b) - deg(a))) {
			if (visited.has(seed)) continue;
			layerOf.set(seed, 0);
			visited.add(seed);
			const queue = [seed];
			while (queue.length) {
				const cur = queue.shift()!;
				const cl = layerOf.get(cur)!;
				for (const nb of adj.get(cur)!) {
					if (!visited.has(nb)) {
						visited.add(nb);
						layerOf.set(nb, cl + 1);
						queue.push(nb);
					}
				}
			}
		}

		const maxLayer = Math.max(0, ...layerOf.values());
		const layers: string[][] = Array.from({ length: maxLayer + 1 }, () => []);
		for (const id of ids) layers[layerOf.get(id) ?? 0].push(id);

		const posInLayer = new Map<string, number>();
		layers.forEach((arr) => arr.forEach((id, i) => posInLayer.set(id, i)));
		const bary = (id: string, neighborLayer: string[]) => {
			const set = new Set(neighborLayer);
			const nbs = adj.get(id)!.filter((n) => set.has(n));
			if (!nbs.length) return posInLayer.get(id) ?? 0;
			return nbs.reduce((s, n) => s + (posInLayer.get(n) ?? 0), 0) / nbs.length;
		};
		for (let sweep = 0; sweep < 4; sweep++) {
			for (let li = 1; li < layers.length; li++) {
				layers[li].sort((x, y) => bary(x, layers[li - 1]) - bary(y, layers[li - 1]));
				layers[li].forEach((id, i) => posInLayer.set(id, i));
			}
			for (let li = layers.length - 2; li >= 0; li--) {
				layers[li].sort((x, y) => bary(x, layers[li + 1]) - bary(y, layers[li + 1]));
				layers[li].forEach((id, i) => posInLayer.set(id, i));
			}
		}

		const gapLayer = 120; // distance between layers
		const gapNode = 90; // distance between nodes within a layer
		const pad = 60;
		const widest = Math.max(1, ...layers.map((a) => a.length));
		const crossSize = pad * 2 + (widest - 1) * gapNode;
		const mainSize = pad * 2 + (layers.length - 1) * gapLayer;
		const snap: Record<string, { x: number; y: number }> = {};
		layers.forEach((arr, li) => {
			const main = pad + li * gapLayer;
			const rowSpan = (arr.length - 1) * gapNode;
			const start = (crossSize - rowSpan) / 2;
			arr.forEach((id, i) => {
				const cross = start + i * gapNode;
				snap[id] = hierDir === 'TB' ? { x: cross, y: main } : { x: main, y: cross };
			});
		});

		vbW = Math.max(W, hierDir === 'TB' ? crossSize : mainSize);
		vbH = Math.max(H, hierDir === 'TB' ? mainSize : crossSize);
		nodePos = snap;
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const q = scope ? `?location_id=${scope}` : '';
			topo = (await apiGet<Topology>(`/api/dcim/topology${q}`)) ?? { nodes: [], links: [] };
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		try {
			locations = (await apiGet<Location[]>('/api/locations')) ?? [];
		} catch {
			locations = [];
		}
		await load();
	});

	onDestroy(() => cancelAnimationFrame(raf));

	// Re-seed and (re)start the force simulation whenever the graph changes.
	$effect(() => {
		const nodes = topo.nodes;
		const links = topo.links;
		const mode = layout;
		const dir = hierDir;
		void dir;
		cancelAnimationFrame(raf);
		running = false;
		if (!nodes.length) {
			nodePos = {};
			vbW = W;
			vbH = H;
			return;
		}
		if (mode === 'hier') {
			computeHierarchy();
			return;
		}
		vbW = W;
		vbH = H;
		const n = nodes.length;
		sim = nodes.map((nd, i) => {
			const a = (i / n) * Math.PI * 2;
			return { id: nd.id, x: W / 2 + Math.cos(a) * 180, y: H / 2 + Math.sin(a) * 180, vx: 0, vy: 0 };
		});
		const idx = new Map(sim.map((s, i) => [s.id, i] as const));
		edges = links
			.map((l) => ({ a: idx.get(l.a_asset_id), b: idx.get(l.b_asset_id) }))
			.filter((e): e is { a: number; b: number } => e.a != null && e.b != null);
		iter = 0;
		ensureRunning();
		return () => cancelAnimationFrame(raf);
	});

	const cableColor: Record<string, string> = {
		cat5e: '#22c55e',
		cat6: '#22c55e',
		cat6a: '#16a34a',
		'fiber-lc': '#f59e0b',
		fiber: '#f59e0b',
		dac: '#06b6d4',
		power: '#ef4444',
		other: '#9aa3b2'
	};

	function degree(id: string): number {
		return topo.links.filter((l) => l.a_asset_id === id || l.b_asset_id === id).length;
	}
	function isLit(id: string): boolean {
		if (!hover) return false;
		if (id === hover) return true;
		return topo.links.some(
			(l) =>
				(l.a_asset_id === hover && l.b_asset_id === id) ||
				(l.b_asset_id === hover && l.a_asset_id === id)
		);
	}
	function label(node: TopoNode): string {
		return node.asset_tag || node.name;
	}

	// --- drag ---------------------------------------------------------------
	function toSvg(e: PointerEvent): { x: number; y: number } {
		if (!svgEl) return { x: 0, y: 0 };
		const r = svgEl.getBoundingClientRect();
		return { x: ((e.clientX - r.left) / r.width) * vbW, y: ((e.clientY - r.top) / r.height) * vbH };
	}
	function nodeDown(e: PointerEvent, id: string) {
		e.stopPropagation();
		dragId = id;
		(e.currentTarget as Element).setPointerCapture?.(e.pointerId);
		if (layout === 'force') {
			iter = 0; // re-energise so neighbours settle around the dragged node
			ensureRunning();
		}
	}
	function nodeMove(e: PointerEvent) {
		if (!dragId) return;
		const p = toSvg(e);
		const x = Math.max(20, Math.min(vbW - 20, p.x));
		const y = Math.max(20, Math.min(vbH - 20, p.y));
		const s = sim.find((it) => it.id === dragId);
		if (s) {
			s.x = x;
			s.y = y;
		}
		nodePos = { ...nodePos, [dragId]: { x, y } };
	}
	function nodeUp() {
		dragId = null;
	}
</script>

<div class="topbar">
	<h1>Network</h1>
	<div class="row" style="gap:8px">
		<select bind:value={layout} style="min-width:160px" title="Graph layout">
			<option value="force">Force-directed</option>
			<option value="hier">Hierarchical (ELK)</option>
		</select>
		{#if layout === 'hier'}
			<select bind:value={hierDir} style="min-width:120px" title="Hierarchy direction">
				<option value="TB">Top → bottom</option>
				<option value="LR">Left → right</option>
			</select>
		{/if}
		<select bind:value={overlay} style="min-width:150px">
			<option value="none">No overlay</option>
			<option value="location">Color by location</option>
			<option value="vlan">Color by VLAN</option>
			<option value="subnet">Color by subnet</option>
		</select>
		<select bind:value={scope} onchange={load} style="min-width:200px">
			<option value="">All sites</option>
			{#each locations as l}<option value={l.id}>{l.name}</option>{/each}
		</select>
	</div>
	<label class="port-toggle"><input type="checkbox" bind:checked={showPorts} /> Port labels</label>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if loading}
	<p class="muted">Loading…</p>
{:else if topo.nodes.length === 0}
	<div class="card">
		<p class="muted">
			No cabled devices to graph{scope ? ' in this scope' : ''}. Add ports to network devices and connect
			them (Asset → Ports → Connect) to see the topology here.
		</p>
	</div>
{:else}
	<div class="grid" style="grid-template-columns: 1fr 280px; align-items:start">
		<div class="card">
			<svg
				bind:this={svgEl}
				viewBox={`0 0 ${vbW} ${vbH}`}
				class="graph"
				onpointermove={nodeMove}
				onpointerup={nodeUp}
				onpointerleave={nodeUp}
				role="application"
				aria-label="Network topology"
			>
				{#each topo.links as l}
					{@const a = nodePos[l.a_asset_id]}
					{@const b = nodePos[l.b_asset_id]}
					{#if a && b}
						{@const lit = hover === l.a_asset_id || hover === l.b_asset_id}
						<line
							x1={a.x} y1={a.y} x2={b.x} y2={b.y}
							stroke={cableColor[l.cable_type] ?? '#9aa3b2'}
							stroke-width={lit ? 3 : 1.5}
							opacity={hover && !lit ? 0.12 : 0.8}
						>
							<title>{l.a_port} ↔ {l.b_port} ({l.cable_type})</title>
						</line>
						{#if showPorts || lit}
							{@const ax = a.x + (b.x - a.x) * 0.22}
							{@const ay = a.y + (b.y - a.y) * 0.22}
							{@const bx = a.x + (b.x - a.x) * 0.78}
							{@const by = a.y + (b.y - a.y) * 0.78}
							<text x={ax} y={ay} class="port-label" text-anchor="middle">{l.a_port}</text>
							<text x={bx} y={by} class="port-label" text-anchor="middle">{l.b_port}</text>
						{/if}
					{/if}
				{/each}
				{#each topo.nodes as node}
					{@const p = nodePos[node.id]}
					{#if p}
						{@const dim = hover && !isLit(node.id)}
						<g
							transform={`translate(${p.x},${p.y})`}
							class="node"
							class:dragging={dragId === node.id}
							opacity={dim ? 0.3 : 1}
							role="button"
							tabindex="0"
							onpointerdown={(e) => nodeDown(e, node.id)}
							onmouseenter={() => (hover = node.id)}
							onmouseleave={() => (hover = null)}
							ondblclick={() => goto(`/assets/${node.id}`)}
							onkeydown={(e) => e.key === 'Enter' && goto(`/assets/${node.id}`)}
						>
							<circle r={10 + Math.min(8, degree(node.id))} class="node-dot" style={overlay !== 'none' ? `fill:${colorFor(node)}` : ''} />
							<text y="-16" text-anchor="middle" class="node-label">{label(node)}</text>
						</g>
					{/if}
				{/each}
			</svg>
			<p class="muted" style="margin-top:8px">
				{topo.nodes.length} device(s), {topo.links.length} cable(s). Drag to rearrange · hover to
				highlight · double-click to open.
			</p>
			{#if overlay !== 'none' && legend.length}
				<div class="legend">
					{#each legend as item}
						<span class="legend-item"><span class="legend-dot" style={`background:${item.color}`}></span>{item.cat}</span>
					{/each}
				</div>
			{/if}
		</div>

		<div class="card">
			<h3 style="margin-top:0">Devices</h3>
			<table>
				<tbody>
					{#each [...topo.nodes].sort((a, b) => degree(b.id) - degree(a.id)) as node}
						<tr
							onclick={() => goto(`/assets/${node.id}`)}
							onmouseenter={() => (hover = node.id)}
							onmouseleave={() => (hover = null)}
						>
							<td>{label(node)}</td>
							<td class="muted" style="text-align:right">{degree(node.id)} link(s)</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.graph {
		display: block;
		width: 100%;
		height: auto;
		background: var(--bg);
		border-radius: 8px;
		touch-action: none;
	}
	.node { cursor: grab; }
	.node.dragging { cursor: grabbing; }
	.node-dot { fill: var(--primary); stroke: var(--surface); stroke-width: 2; transition: fill 0.15s; }
	.node:hover .node-dot { fill: var(--primary-hover); }
	.node-label { fill: var(--text); font-size: 12px; font-weight: 600; pointer-events: none; }
	.legend { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 10px; font-size: 12px; }
	.legend-item { display: inline-flex; align-items: center; gap: 5px; color: var(--muted); }
	.legend-dot { width: 10px; height: 10px; border-radius: 3px; display: inline-block; }
	.port-label { fill: var(--muted); font-size: 9px; pointer-events: none; paint-order: stroke; stroke: var(--bg); stroke-width: 2.5px; }
	.port-toggle { display: inline-flex; align-items: center; gap: 5px; font-size: 13px; color: var(--muted); cursor: pointer; }
</style>
