import type { Asset, AssetPlacement, AssetType } from './types';

/** Asset types that commonly mount in racks or participate in DCIM. */
const INFRA_PATH_PREFIXES = [
	'hardware.computer.server',
	'hardware.network',
	'hardware.storage',
	'hardware.power'
];

export function isInfrastructureType(type: AssetType | null | undefined): boolean {
	if (!type?.path) return false;
	const p = type.path;
	return INFRA_PATH_PREFIXES.some((prefix) => p === prefix || p.startsWith(prefix + '.'));
}

export function isInfrastructureAsset(asset: Asset | null | undefined): boolean {
	return isInfrastructureType(asset?.asset_type);
}

/** Show DCIM UI when the type is infrastructure or we already have placement data. */
export function showInfrastructure(asset: Asset | null | undefined, placement: AssetPlacement | null): boolean {
	return isInfrastructureAsset(asset) || !!placement?.mount || !!placement?.rack_record;
}

export function mountLabel(m: { position: number; u_height: number; face: string }): string {
	const top = m.position + m.u_height - 1;
	const range = m.u_height > 1 ? `U${m.position}–U${top}` : `U${m.position}`;
	return `${range} · ${m.face}`;
}

export function powerLabel(watts: number | null | undefined): string {
	if (watts == null) return '—';
	return `${watts.toLocaleString()} W`;
}
