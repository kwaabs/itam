import { writable } from 'svelte/store';
import { apiGet } from './api';
import type { Currency } from './types';

export const currencies = writable<Currency[]>([]);

let loaded = false;
let inflight: Promise<Currency[]> | null = null;

/** Load the enabled currency list once and cache it in the store. */
export async function loadCurrencies(force = false): Promise<Currency[]> {
	if (loaded && !force) {
		let cur: Currency[] = [];
		currencies.subscribe((v) => (cur = v))();
		return cur;
	}
	if (inflight) return inflight;
	inflight = apiGet<Currency[]>('/api/metadata/currencies')
		.then((list) => {
			const arr = list ?? [];
			currencies.set(arr);
			loaded = true;
			return arr;
		})
		.catch(() => {
			const fallback: Currency[] = [
				{ code: 'USD', name: 'US Dollar', symbol: '$', is_default: true, enabled: true, sort: 1 }
			];
			currencies.set(fallback);
			return fallback;
		})
		.finally(() => {
			inflight = null;
		});
	return inflight;
}

/** The default (reporting) currency code, falling back to USD. */
export function defaultCurrency(list: Currency[]): string {
	return list.find((c) => c.is_default)?.code ?? list[0]?.code ?? 'USD';
}
