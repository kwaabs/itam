import { get } from 'svelte/store';
import { goto } from '$app/navigation';
import { API_URL } from './config';
import { session } from './session';

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
	const s = get(session);
	const headers = new Headers(opts.headers);
	if (s) headers.set('Authorization', `Bearer ${s.access_token}`);
	const isFormData = typeof FormData !== 'undefined' && opts.body instanceof FormData;
	if (opts.body && !isFormData && !headers.has('Content-Type'))
		headers.set('Content-Type', 'application/json');

	const res = await fetch(`${API_URL}${path}`, { ...opts, headers });

	if (res.status === 401) {
		session.set(null);
		goto('/login');
		throw new Error('Session expired, please sign in again');
	}
	if (!res.ok) {
		let message = `Request failed (${res.status})`;
		try {
			const body = await res.json();
			message = body.error || JSON.stringify(body);
		} catch {
			/* ignore */
		}
		throw new Error(message);
	}
	if (res.status === 204) return undefined as T;
	return (await res.json()) as T;
}

export const apiGet = <T>(path: string) => request<T>(path);
export const apiPost = <T>(path: string, body?: unknown) =>
	request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined });
export const apiPut = <T>(path: string, body?: unknown) =>
	request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined });
export const apiDelete = <T>(path: string) => request<T>(path, { method: 'DELETE' });

// apiUpload posts a file as multipart/form-data (the browser sets the boundary,
// so we must not set Content-Type ourselves).
export const apiUpload = <T>(path: string, file: File): Promise<T> => {
	const fd = new FormData();
	fd.append('file', file);
	return request<T>(path, { method: 'POST', body: fd });
};

// apiDownload fetches a file with auth and triggers a browser download.
export async function apiDownload(path: string, filename: string): Promise<void> {
	const s = get(session);
	const headers = new Headers();
	if (s) headers.set('Authorization', `Bearer ${s.access_token}`);
	const res = await fetch(`${API_URL}${path}`, { headers });
	if (!res.ok) throw new Error(`Download failed (${res.status})`);
	const blob = await res.blob();
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	a.remove();
	URL.revokeObjectURL(url);
}
