import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export type Theme = 'light' | 'dark' | 'system';
const KEY = 'itam-theme';

function stored(): Theme {
	if (!browser) return 'system';
	const v = localStorage.getItem(KEY);
	return v === 'light' || v === 'dark' || v === 'system' ? v : 'system';
}

function systemDark(): boolean {
	return browser && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function apply(t: Theme) {
	if (!browser) return;
	const resolved = t === 'system' ? (systemDark() ? 'dark' : 'light') : t;
	document.documentElement.dataset.theme = resolved;
}

export const theme = writable<Theme>(stored());

if (browser) {
	let current = stored();
	theme.subscribe((t) => {
		current = t;
		localStorage.setItem(KEY, t);
		apply(t);
	});
	// React to OS theme changes while in "system" mode.
	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
		if (current === 'system') apply('system');
	});
}
