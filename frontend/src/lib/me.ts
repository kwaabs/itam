import { writable } from 'svelte/store';
import type { Me } from './types';

export const me = writable<Me | null>(null);

export function can(m: Me | null, perm: string): boolean {
	if (!m) return false;
	return m.is_superuser || m.permissions.includes(perm);
}
