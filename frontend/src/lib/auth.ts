import { get } from 'svelte/store';
import { API_URL } from './config';
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

// Microsoft Entra ID SSO, handled natively by the API (see backend/internal/auth/azure.go).
export function azureLogin(): void {
	const returnTo = encodeURIComponent(window.location.origin);
	window.location.href = `${API_URL}/auth/azure/start?redirect_to=${returnTo}`;
}

// Whether Azure AD SSO is configured and enabled.
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

// The API returns tokens in the URL hash after an Azure OAuth redirect.
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
