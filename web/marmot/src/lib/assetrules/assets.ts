import { resolve } from '$app/paths';
import { providerLabel } from './plain';

/** The fields an asset link needs, so a full asset and a list row both fit. */
type AssetRef = { name: string; mrn?: string; type?: string; providers?: string[] };

/** The asset's page, from its MRN (`mrn://type/service/name`). */
export function assetHref(asset: AssetRef): string {
	const parts = (asset.mrn ?? '').replace('mrn://', '').split('/');
	if (parts.length >= 3)
		return resolve('/discover/[type]/[service]/[name]', {
			type: parts[0],
			service: parts[1],
			name: parts.slice(2).join('/')
		});
	return resolve('/discover/[type]/[service]/[name]', {
		type: asset.type ?? '',
		service: asset.providers?.[0] ?? '',
		name: asset.name
	});
}

/** "Table · PostgreSQL", the quiet line under an asset's name. */
export function assetCaption(asset: AssetRef): string {
	const type = (asset.type ?? '').replace(/_/g, ' ');
	return [
		type.charAt(0).toUpperCase() + type.slice(1),
		...(asset.providers ?? []).map(providerLabel)
	]
		.filter(Boolean)
		.join(' · ');
}
