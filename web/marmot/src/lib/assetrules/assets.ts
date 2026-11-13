import { resolve } from '$app/paths';
import { assetRouteParamsFor, zoneOf, type MRNParts } from '$lib/mrn';
import { providerLabel } from './plain';

/** The fields an asset link needs, so a full asset and a list row both fit. */
type AssetRef = {
	name: string;
	mrn?: string;
	type?: string;
	providers?: string[];
	mrn_parts?: MRNParts;
};

/** The asset's page, from its MRN. */
export function assetHref(asset: AssetRef): string {
	return resolve('/discover/[type]/[service]/[name]', assetRouteParamsFor(asset));
}

/** "Table · PostgreSQL", the quiet line under an asset's name. */
export function assetCaption(asset: AssetRef): string {
	const type = (asset.type ?? '').replace(/_/g, ' ');
	return [
		type.charAt(0).toUpperCase() + type.slice(1),
		(asset.providers ?? []).map(providerLabel).join(', '),
		zoneOf(asset)
	]
		.filter(Boolean)
		.join(' · ');
}
