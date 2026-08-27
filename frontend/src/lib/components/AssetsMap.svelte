<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { get } from 'svelte/store';
	import 'leaflet/dist/leaflet.css';
	import type { AssetMapPoint } from '$lib/types';
	import { buildBasemapLayers } from '$lib/basemaps';
	import { basemapId, loadMapConfig } from '$lib/mapconfig';

	let { points }: { points: AssetMapPoint[] } = $props();

	let el: HTMLDivElement;
	let L: any = null;
	let map: any = null;
	let layer: any = null;

	// Group assets that resolve to the same coordinate (e.g. all assets at a site).
	function coordKey(p: AssetMapPoint): string {
		return `${p.lat.toFixed(4)},${p.lng.toFixed(4)}`;
	}

	function render() {
		if (!map || !L) return;
		if (layer) layer.remove();
		layer = L.layerGroup().addTo(map);

		const groups = new Map<string, AssetMapPoint[]>();
		for (const p of points) {
			const k = coordKey(p);
			if (!groups.has(k)) groups.set(k, []);
			groups.get(k)!.push(p);
		}

		const all: any[] = [];
		for (const [k, ps] of groups) {
			const [lat, lng] = k.split(',').map(Number);
			const n = ps.length;
			const m = L.circleMarker([lat, lng], {
				radius: Math.min(22, 8 + Math.log2(n) * 4),
				color: '#6366f1',
				fillColor: '#6366f1',
				fillOpacity: 0.65,
				weight: 2
			}).addTo(layer);
			const list = ps
				.slice(0, 12)
				.map(
					(p) =>
						`<a href="/assets/${p.id}">${p.asset_tag}</a> <span style="opacity:.6">${p.name}</span>`
				)
				.join('<br>');
			const more = n > 12 ? `<br><span style="opacity:.6">+${n - 12} more…</span>` : '';
			m.bindPopup(`<b>${ps[0].location_name || 'Location'}</b> · ${n} asset(s)<br>${list}${more}`, {
				maxHeight: 240
			});
			all.push(m);
		}
		if (all.length) {
			const g = L.featureGroup(all);
			map.fitBounds(g.getBounds().pad(0.4));
		}
	}

	onMount(async () => {
		await loadMapConfig();
		const leaflet = await import('leaflet');
		L = (leaflet as any).default ?? leaflet;
		map = L.map(el, { worldCopyJump: true }).setView([20, 0], 2);
		const { layers, labelById } = buildBasemapLayers(L);
		const activeLabel = labelById[get(basemapId)] ?? Object.keys(layers)[0];
		layers[activeLabel].addTo(map);
		L.control.layers(layers, {}, { position: 'topright', collapsed: true }).addTo(map);
		setTimeout(() => map && map.invalidateSize(), 50);
		render();
	});

	$effect(() => {
		points;
		render();
	});

	onDestroy(() => {
		if (map) map.remove();
	});
</script>

<div bind:this={el} class="amap"></div>

<style>
	.amap {
		height: 560px;
		border-radius: 8px;
		overflow: hidden;
		border: 1px solid var(--border, #2a2a3a);
		background: var(--surface-2, #1a1d28);
	}
	:global(.leaflet-popup-content) {
		font:
			13px/1.5 system-ui,
			sans-serif;
	}
</style>
