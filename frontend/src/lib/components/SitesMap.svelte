<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { get } from 'svelte/store';
	import 'leaflet/dist/leaflet.css';
	import type { Location } from '$lib/types';
	import { buildBasemapLayers } from '$lib/basemaps';
	import { basemapId, loadMapConfig } from '$lib/mapconfig';

	let { sites, onopen }: { sites: Location[]; onopen?: (id: string) => void } = $props();

	let el: HTMLDivElement;
	// Leaflet is browser-only, so it is dynamically imported in onMount.
	let L: any = null;
	let map: any = null;
	let markers: any[] = [];

	function drColor(role?: string): string {
		switch ((role || '').toLowerCase()) {
			case 'primary':
				return '#2e9e6b';
			case 'dr':
				return '#e0a93b';
			case 'edge':
				return '#9b6dff';
			case 'colo':
				return '#e06a9b';
			default:
				return '#6aa6ff';
		}
	}

	function render() {
		if (!map || !L) return;
		markers.forEach((m) => m.remove());
		markers = [];
		const pts = sites.filter((s) => s.latitude != null && s.longitude != null);
		for (const s of pts) {
			const color = drColor(s.dr_role);
			const m = L.circleMarker([s.latitude, s.longitude], {
				radius: 8,
				color,
				fillColor: color,
				fillOpacity: 0.85,
				weight: 2
			}).addTo(map);
			const bits = [s.kind, s.dr_role, s.tier].filter(Boolean).join(' · ');
			m.bindPopup(
				`<b>${s.name}</b><br>${bits}${s.address ? `<br>${s.address}` : ''}` +
					`<br><span style="opacity:.7">${s.latitude!.toFixed(4)}, ${s.longitude!.toFixed(4)}</span>`
			);
			if (onopen) m.on('dblclick', () => onopen(s.id));
			markers.push(m);
		}
		if (pts.length) {
			const g = L.featureGroup(markers);
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
		// Containers that mount inside a tab can report zero size; recompute once
		// painted so tiles actually load.
		setTimeout(() => map && map.invalidateSize(), 50);
		render();
	});

	// Re-plot whenever the site list changes.
	$effect(() => {
		sites;
		render();
	});

	onDestroy(() => {
		if (map) map.remove();
	});
</script>

<div bind:this={el} class="sitesmap"></div>

<style>
	.sitesmap {
		height: 520px;
		border-radius: 8px;
		overflow: hidden;
		border: 1px solid var(--border, #2a2a3a);
		background: #1a1d28;
	}
	/* Keep Leaflet popups readable on the dark theme. */
	:global(.leaflet-popup-content) {
		font:
			13px/1.4 system-ui,
			sans-serif;
	}
</style>
