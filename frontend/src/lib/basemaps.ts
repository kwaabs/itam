// Shared Leaflet basemap definitions + helpers. Used by every map component so
// the basemap set and the configured default stay consistent.

export type BasemapDef = {
	id: string;
	label: string;
	url: string;
	subdomains?: string;
	attribution: string;
	maxZoom?: number;
};

export const BASEMAPS: BasemapDef[] = [
	{
		id: 'carto_voyager',
		label: 'Carto Voyager',
		url: 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png',
		subdomains: 'abcd',
		attribution: '&copy; OpenStreetMap contributors &copy; CARTO',
		maxZoom: 20
	},
	{
		id: 'carto_positron',
		label: 'Carto Positron (light)',
		url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
		subdomains: 'abcd',
		attribution: '&copy; OpenStreetMap contributors &copy; CARTO',
		maxZoom: 20
	},
	{
		id: 'carto_dark',
		label: 'Carto Dark Matter',
		url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
		subdomains: 'abcd',
		attribution: '&copy; OpenStreetMap contributors &copy; CARTO',
		maxZoom: 20
	},
	{
		id: 'osm',
		label: 'OpenStreetMap',
		url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
		subdomains: 'abc',
		attribution: '&copy; OpenStreetMap contributors',
		maxZoom: 19
	},
	{
		id: 'esri_topo',
		label: 'Esri Topographic',
		url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Topo_Map/MapServer/tile/{z}/{y}/{x}',
		attribution: 'Tiles &copy; Esri',
		maxZoom: 19
	},
	{
		id: 'esri_satellite',
		label: 'Esri Satellite',
		url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
		attribution: 'Tiles &copy; Esri, Maxar, Earthstar Geographics',
		maxZoom: 19
	},
	{
		id: 'google_roadmap',
		label: 'Google Maps',
		url: 'https://mt{s}.google.com/vt/lyrs=m&x={x}&y={y}&z={z}',
		subdomains: '0123',
		attribution: '&copy; Google',
		maxZoom: 20
	},
	{
		id: 'google_satellite',
		label: 'Google Satellite',
		url: 'https://mt{s}.google.com/vt/lyrs=s&x={x}&y={y}&z={z}',
		subdomains: '0123',
		attribution: '&copy; Google',
		maxZoom: 20
	},
	{
		id: 'google_hybrid',
		label: 'Google Hybrid',
		url: 'https://mt{s}.google.com/vt/lyrs=y&x={x}&y={y}&z={z}',
		subdomains: '0123',
		attribution: '&copy; Google',
		maxZoom: 20
	}
];

export const DEFAULT_BASEMAP = 'carto_voyager';

export function basemapById(id: string): BasemapDef {
	return BASEMAPS.find((b) => b.id === id) ?? BASEMAPS[0];
}

// buildBasemapLayers creates a Leaflet tileLayer per definition, keyed by label
// (for use with L.control.layers), plus a lookup from id → label.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function buildBasemapLayers(L: any): { layers: Record<string, any>; labelById: Record<string, string> } {
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const layers: Record<string, any> = {};
	const labelById: Record<string, string> = {};
	for (const b of BASEMAPS) {
		layers[b.label] = L.tileLayer(b.url, {
			subdomains: b.subdomains ?? 'abc',
			maxZoom: b.maxZoom ?? 19,
			attribution: b.attribution
		});
		labelById[b.id] = b.label;
	}
	return { layers, labelById };
}
