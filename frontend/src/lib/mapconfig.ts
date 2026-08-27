import { writable } from 'svelte/store';
import { apiGet } from './api';
import { DEFAULT_BASEMAP } from './basemaps';

// Currently-selected default basemap id. Backed by the `map.basemap` app
// setting; falls back to the default if the setting is missing or unreadable.
export const basemapId = writable<string>(DEFAULT_BASEMAP);

let loaded = false;

export async function loadMapConfig(): Promise<void> {
	if (loaded) return;
	loaded = true;
	try {
		const rows = await apiGet<{ key: string; value: unknown }[]>('/api/settings');
		const row = rows?.find((r) => r.key === 'map.basemap');
		if (row && row.value) basemapId.set(String(row.value));
	} catch {
		// settings may be unreadable for this role — keep the default.
	}
}
