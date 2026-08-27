import type { OrgUnit, OrgUnitKind } from '$lib/types';

/** Ancestor chain from root → … → unit for an org assignment. */
export function orgBreadcrumb(units: OrgUnit[], owner: OrgUnit | null | undefined): OrgUnit[] {
	if (!owner?.path) return [];
	return units
		.filter((u) => owner.path === u.path || owner.path.startsWith(u.path + '.'))
		.sort((a, b) => a.path.split('.').length - b.path.split('.').length);
}

export function kindLabel(kinds: OrgUnitKind[], kind: string | null | undefined): string {
	if (!kind) return '—';
	return kinds.find((k) => k.key === kind)?.label ?? kind;
}

export function depth(path: string): number {
	return Math.max(0, path.split('.').length - 1);
}
