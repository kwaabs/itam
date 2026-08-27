<script lang="ts">
	import type { Rack } from '$lib/types';

	let {
		racks,
		groups,
		canManage = false,
		onmove,
		onopen
	}: {
		racks: Rack[];
		// When provided, racks are auto-arranged into labeled bands (one per
		// sub-location) instead of using their stored x/y. Read-only; used for
		// the aggregated site/floor/room layout views.
		groups?: { id: string; label: string; rackIds: string[] }[];
		canManage?: boolean;
		onmove?: (id: string, x: number, y: number, rotation: number) => void;
		onopen?: (id: string) => void;
	} = $props();

	const FLOOR_W = 1200;
	const FLOOR_H = 680;
	const RW = 54; // rack footprint width (px, top-down)
	const RH = 96; // rack footprint depth (px)
	const GRID = 40; // background grid + auto-placement step

	const grouped = $derived(!!(groups && groups.length));

	// Grouped (aggregated) layout: lay each sub-location out as a labeled band
	// with its racks in a wrapping grid. Heights grow to fit the content.
	const PADX = 20;
	const HEADER_H = 30;
	const CELL_W = RW + 26;
	const CELL_H = RH + 30;
	const BAND_GAP = 20;

	const layout = $derived.by(() => {
		const place: Record<string, { x: number; y: number }> = {};
		const bands: { label: string; x: number; y: number; w: number; h: number; count: number }[] = [];
		if (!grouped) return { place, bands, height: FLOOR_H };
		const maxPerRow = Math.max(1, Math.floor((FLOOR_W - PADX * 2 - 16) / CELL_W));
		let y = 16;
		for (const g of groups!) {
			const top = y;
			const n = g.rackIds.length;
			const rows = Math.max(1, Math.ceil(n / maxPerRow));
			g.rackIds.forEach((rid, i) => {
				const col = i % maxPerRow;
				const row = Math.floor(i / maxPerRow);
				place[rid] = { x: PADX + 14 + col * CELL_W, y: top + HEADER_H + row * CELL_H };
			});
			const h = HEADER_H + rows * CELL_H + 4;
			bands.push({ label: g.label, x: PADX / 2, y: top, w: FLOOR_W - PADX, h, count: n });
			y = top + h + BAND_GAP;
		}
		return { place, bands, height: Math.max(FLOOR_H, y + 6) };
	});

	const floorH = $derived(grouped ? layout.height : FLOOR_H);

	let svgEl: SVGSVGElement | null = $state(null);
	let selectedId = $state<string | null>(null);

	// Live overrides during/after a drag, keyed by rack id.
	let live = $state<Record<string, { x: number; y: number }>>({});
	let rot = $state<Record<string, number>>({});

	let dragId: string | null = null;
	let startX = 0;
	let startY = 0;
	let origX = 0;
	let origY = 0;
	let moved = false;

	// Zoom & pan are expressed through the SVG viewBox.
	let zoom = $state(1);
	let panX = $state(0);
	let panY = $state(0);
	let panning = false;
	let panStartX = 0;
	let panStartY = 0;
	let panOrigX = 0;
	let panOrigY = 0;

	const vbW = $derived(FLOOR_W / zoom);
	const vbH = $derived(floorH / zoom);
	const viewBox = $derived(`${panX} ${panY} ${vbW} ${vbH}`);

	// Fallback grid position (by index) for racks without saved coordinates.
	function fallback(i: number) {
		const perRow = Math.max(1, Math.floor((FLOOR_W - 80) / (RW + GRID)));
		const col = i % perRow;
		const row = Math.floor(i / perRow);
		return { x: 60 + col * (RW + GRID), y: 60 + row * (RH + GRID) };
	}

	function posOf(r: Rack, i: number): { x: number; y: number } {
		if (grouped) return layout.place[r.id] ?? fallback(i);
		if (live[r.id]) return live[r.id];
		if (r.pos_x != null && r.pos_y != null) return { x: r.pos_x, y: r.pos_y };
		return fallback(i);
	}

	function rotOf(r: Rack): number {
		return rot[r.id] ?? r.rotation ?? 0;
	}

	function toSvg(e: { clientX: number; clientY: number }): { x: number; y: number } {
		if (!svgEl) return { x: 0, y: 0 };
		const rect = svgEl.getBoundingClientRect();
		return {
			x: panX + (e.clientX - rect.left) * (vbW / rect.width),
			y: panY + (e.clientY - rect.top) * (vbH / rect.height)
		};
	}

	function down(e: PointerEvent, r: Rack, i: number) {
		e.stopPropagation();
		selectedId = r.id;
		if (grouped) {
			onopen?.(r.id);
			return;
		}
		if (!canManage) {
			onopen?.(r.id);
			return;
		}
		e.preventDefault();
		const p = toSvg(e);
		const cur = posOf(r, i);
		dragId = r.id;
		origX = cur.x;
		origY = cur.y;
		startX = p.x;
		startY = p.y;
		moved = false;
		(e.currentTarget as Element).setPointerCapture?.(e.pointerId);
	}

	function move(e: PointerEvent) {
		if (panning && svgEl) {
			const rect = svgEl.getBoundingClientRect();
			const dx = (e.clientX - panStartX) * (vbW / rect.width);
			const dy = (e.clientY - panStartY) * (vbH / rect.height);
			panX = clampPan(panOrigX - dx, FLOOR_W, vbW);
			panY = clampPan(panOrigY - dy, floorH, vbH);
			return;
		}
		if (!dragId) return;
		const p = toSvg(e);
		const nx = clamp(origX + (p.x - startX), 0, FLOOR_W - RW);
		const ny = clamp(origY + (p.y - startY), 0, FLOOR_H - RH);
		if (Math.abs(p.x - startX) > 3 || Math.abs(p.y - startY) > 3) moved = true;
		live = { ...live, [dragId]: { x: nx, y: ny } };
	}

	function up() {
		panning = false;
		if (!dragId) return;
		const id = dragId;
		dragId = null;
		if (moved && live[id]) {
			onmove?.(id, Math.round(live[id].x), Math.round(live[id].y), rot[id] ?? 0);
		} else if (!moved) {
			onopen?.(id);
		}
	}

	function openRack(id: string) {
		onopen?.(id);
	}

	function rackKeydown(e: KeyboardEvent, id: string) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			openRack(id);
		}
	}

	function clamp(v: number, lo: number, hi: number) {
		return Math.max(lo, Math.min(hi, v));
	}

	// --- zoom & pan ----------------------------------------------------------
	function clampPan(v: number, total: number, span: number): number {
		if (span >= total) return (total - span) / 2; // center when zoomed out past 1:1
		return Math.max(0, Math.min(total - span, v));
	}
	function applyZoom(nz: number, fx: number, fy: number) {
		nz = clamp(nz, 0.5, 6);
		if (nz === zoom) return;
		const nvbW = FLOOR_W / nz;
		const nvbH = floorH / nz;
		const rx = (fx - panX) / vbW;
		const ry = (fy - panY) / vbH;
		panX = clampPan(fx - rx * nvbW, FLOOR_W, nvbW);
		panY = clampPan(fy - ry * nvbH, floorH, nvbH);
		zoom = nz;
	}
	function zoomBy(factor: number) {
		applyZoom(zoom * factor, panX + vbW / 2, panY + vbH / 2);
	}
	function resetView() {
		zoom = 1;
		panX = 0;
		panY = 0;
	}
	function wheel(e: WheelEvent) {
		e.preventDefault();
		const p = toSvg(e);
		applyZoom(zoom * (e.deltaY < 0 ? 1.15 : 1 / 1.15), p.x, p.y);
	}
	function bgDown(e: PointerEvent) {
		selectedId = null;
		panning = true;
		panStartX = e.clientX;
		panStartY = e.clientY;
		panOrigX = panX;
		panOrigY = panY;
		(e.currentTarget as Element).setPointerCapture?.(e.pointerId);
	}

	function rotate(r: Rack, i: number) {
		const next = (rotOf(r) + 90) % 360;
		rot = { ...rot, [r.id]: next };
		const cur = posOf(r, i);
		onmove?.(r.id, Math.round(cur.x), Math.round(cur.y), next);
	}

	function used(r: Rack): number {
		return (r.mounts ?? []).reduce((n, m) => n + m.u_height, 0);
	}

	const gridLines = (() => {
		const v: number[] = [];
		for (let x = GRID; x < FLOOR_W; x += GRID) v.push(x);
		return v;
	})();
	const gridRows = $derived.by(() => {
		const v: number[] = [];
		for (let y = GRID; y < floorH; y += GRID) v.push(y);
		return v;
	});
</script>

<div class="floor-wrap">
	<div class="zoom-ctl">
		<button onclick={() => zoomBy(1.25)} title="Zoom in" aria-label="Zoom in">+</button>
		<button onclick={() => zoomBy(1 / 1.25)} title="Zoom out" aria-label="Zoom out">−</button>
		<button onclick={resetView} title="Reset view" aria-label="Reset view">⤢</button>
	</div>
	<svg
		bind:this={svgEl}
		{viewBox}
		class="floor"
		onpointermove={move}
		onpointerup={up}
		onpointerleave={up}
		onwheel={wheel}
		role="application"
		aria-label="Rack floor plan"
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<rect x="0" y="0" width={FLOOR_W} height={floorH} class="floor-bg" onpointerdown={bgDown} role="presentation" />
		{#each gridLines as x}<line x1={x} y1="0" x2={x} y2={floorH} class="grid" />{/each}
		{#each gridRows as y}<line x1="0" y1={y} x2={FLOOR_W} y2={y} class="grid" />{/each}

		{#if grouped}
			{#each layout.bands as b}
				<rect x={b.x} y={b.y} width={b.w} height={b.h} rx="8" class="band" pointer-events="none" />
				<text x={b.x + 12} y={b.y + 19} class="band-label" pointer-events="none">{b.label}</text>
				<text x={b.x + b.w - 12} y={b.y + 19} class="band-count" pointer-events="none">{b.count} rack{b.count === 1 ? '' : 's'}</text>
			{/each}
		{/if}

		{#each racks as r, i (r.id)}
			{@const p = posOf(r, i)}
			{@const u = used(r)}
			{@const pct = r.u_height ? Math.min(100, (u / r.u_height) * 100) : 0}
			<g
				transform={`translate(${p.x},${p.y}) rotate(${rotOf(r)} ${RW / 2} ${RH / 2})`}
				class="rack"
				class:selected={selectedId === r.id}
				class:draggable={canManage && !grouped}
				class:clickable={grouped || !canManage}
				onpointerdown={(e) => down(e, r, i)}
				onkeydown={(e) => rackKeydown(e, r.id)}
				role="button"
				tabindex="0"
				aria-label="Open {r.name}"
			>
				<rect x="0" y="0" width={RW} height={RH} rx="4" class="rack-body" />
				<!-- front indicator strip -->
				<rect x="0" y="0" width={RW} height="6" class="rack-front" />
				<!-- occupancy fill from bottom -->
				<rect
					x="3"
					y={RH - 10 - ((RH - 16) * pct) / 100}
					width={RW - 6}
					height={((RH - 16) * pct) / 100}
					class="rack-fill"
				/>
				<text x={RW / 2} y={RH / 2 - 4} class="rack-label">{r.name}</text>
				<text x={RW / 2} y={RH / 2 + 12} class="rack-sub">{u}/{r.u_height}U</text>
			</g>
		{/each}
	</svg>

	{#if selectedId && !grouped}
		{@const sel = racks.find((x) => x.id === selectedId)}
		{@const idx = racks.findIndex((x) => x.id === selectedId)}
		{#if sel}
			<div class="floor-toolbar">
				<strong>{sel.name}</strong>
				<span class="muted">{used(sel)}/{sel.u_height}U</span>
				<button class="btn small" onclick={() => onopen?.(sel.id)}>Open</button>
				{#if canManage}
					<button class="btn secondary small" onclick={() => rotate(sel, idx)}>Rotate</button>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<style>
	.floor-wrap {
		position: relative;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: auto;
		background: var(--floor-wrap-bg);
	}
	.floor {
		display: block;
		width: 100%;
		height: auto;
		min-width: 800px;
		touch-action: none;
	}
	.floor-bg {
		fill: var(--floor-canvas);
		cursor: grab;
	}
	.floor-bg:active {
		cursor: grabbing;
	}
	.grid {
		stroke: var(--floor-grid);
		stroke-width: 1;
		pointer-events: none;
	}
	.zoom-ctl {
		position: absolute;
		top: 10px;
		right: 10px;
		z-index: 3;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.zoom-ctl button {
		width: 30px;
		height: 30px;
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--text);
		border-radius: 6px;
		cursor: pointer;
		font-size: 16px;
		line-height: 1;
		display: grid;
		place-items: center;
	}
	.zoom-ctl button:hover {
		background: var(--surface-2);
	}
	.rack {
		cursor: default;
	}
	.rack.clickable {
		cursor: pointer;
	}
	.rack.draggable {
		cursor: pointer;
	}
	.rack.draggable:active {
		cursor: grabbing;
	}
	.band {
		fill: var(--floor-band-fill);
		stroke: var(--floor-band-stroke);
		stroke-width: 1;
		stroke-dasharray: 4 4;
	}
	.band-label {
		fill: var(--floor-band-label);
		font-size: 13px;
		font-weight: 600;
	}
	.band-count {
		fill: var(--floor-band-count);
		font-size: 11px;
		text-anchor: end;
	}
	.rack-body {
		fill: var(--floor-rack-fill);
		stroke: var(--floor-rack-stroke);
		stroke-width: 1.5;
	}
	.rack.selected .rack-body {
		stroke: #6aa6ff;
		stroke-width: 2.5;
	}
	.rack-front {
		fill: #6aa6ff;
		opacity: 0.7;
	}
	.rack-fill {
		fill: #2e7d5b;
		opacity: 0.55;
	}
	.rack-label {
		fill: var(--floor-rack-label);
		font-size: 11px;
		font-weight: 600;
		text-anchor: middle;
		pointer-events: none;
	}
	.rack-sub {
		fill: var(--floor-rack-sub);
		font-size: 9px;
		text-anchor: middle;
		pointer-events: none;
	}
	.floor-toolbar {
		position: absolute;
		left: 12px;
		bottom: 12px;
		display: flex;
		gap: 10px;
		align-items: center;
		background: var(--floor-toolbar-bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px 12px;
	}
</style>
