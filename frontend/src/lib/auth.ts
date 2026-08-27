import { get } from 'svelte/store';
import { API_URL, GOTRUE_URL } from './config';
import { session } from './session';

// Email/password login against the API's local auth (see backend/internal/auth).
export async function login(email: string, password: string): Promise<void> {
	const res = await fetch(`${API_URL}/auth/login`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, password })
	});
	if (!res.ok) {
		throw new Error('Invalid email or password');
	}
	const data = await res.json();
	session.set({
		access_token: data.access_token,
		refresh_token: data.refresh_token,
		email: data.user?.email ?? email
	});
}

// Microsoft Entra ID SSO via GoTrue (same OAuth flow as Supabase Auth).
export function azureLogin(): void {
	const returnTo = encodeURIComponent(window.location.origin);
	window.location.href = `${GOTRUE_URL}/authorize?provider=azure&redirect_to=${returnTo}`;
}

// Whether GoTrue has Azure AD enabled (proxied via API to avoid browser CORS).
export async function azureEnabled(): Promise<boolean> {
	try {
		const res = await fetch(`${API_URL}/auth/sso/status`);
		if (!res.ok) return false;
		const data = await res.json();
		return data?.enabled === true;
	} catch {
		return false;
	}
}

export function logout(): void {
	const s = get(session);
	if (s?.refresh_token) {
		// Best-effort server-side revocation; don't block clearing the local
		// session on it.
		fetch(`${API_URL}/auth/logout`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: s.refresh_token })
		}).catch(() => {});
	}
	session.set(null);
}

// GoTrue returns tokens in the URL hash after an OAuth redirect.
export function captureOAuthRedirect(): boolean {
	if (typeof window === 'undefined' || !window.location.hash) return false;
	const params = new URLSearchParams(window.location.hash.slice(1));
	const token = params.get('access_token');
	if (!token) return false;
	session.set({
		access_token: token,
		refresh_token: params.get('refresh_token') ?? undefined,
		email: ''
	});
	history.replaceState(null, '', window.location.pathname);
	return true;
}
