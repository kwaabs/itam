import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export interface Session {
	access_token: string;
	refresh_token?: string;
	email: string;
}

const KEY = 'itam_session';

function load(): Session | null {
	if (!browser) return null;
	const raw = localStorage.getItem(KEY);
	return raw ? (JSON.parse(raw) as Session) : null;
}

export const session = writable<Session | null>(load());

if (browser) {
	session.subscribe((v) => {
		if (v) localStorage.setItem(KEY, JSON.stringify(v));
		else localStorage.removeItem(KEY);
	});
}
