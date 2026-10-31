import { fetchApi } from '../api';
import type { Asset } from '../assets/types';
import type { DataProduct } from './types';

/** Route params for an asset page, from its MRN (mrn://type/service/full.name). */
export function assetRouteParams(asset: {
	mrn?: string;
}): { type: string; service: string; name: string } | null {
	if (!asset.mrn) return null;
	const parts = asset.mrn.replace('mrn://', '').split('/');
	if (parts.length < 3) return null;
	return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
}

/** The provider key for an asset's logo; assets from several providers fall back to their type. */
export function assetProvider(asset: { providers?: string[]; type?: string }): string {
	if (asset.providers && asset.providers.length === 1) return asset.providers[0].toLowerCase();
	return (asset.type || 'unknown').toLowerCase();
}

/** The bars mm-popularity draws, or null for an asset or product nobody has viewed lately. */
export function popularityLevel(level?: number): 1 | 2 | 3 | 4 | null {
	if (!level || level < 1) return null;
	return Math.min(Math.round(level), 4) as 1 | 2 | 3 | 4;
}

/** "Table · postgresql" style caption for an asset row. */
export function assetCaption(asset: { providers?: string[]; type?: string }): string {
	const type = (asset.type || '').replace(/_/g, ' ');
	const kind = type ? type.charAt(0).toUpperCase() + type.slice(1) : '';
	return [kind, ...(asset.providers ?? [])].filter(Boolean).join(' · ');
}

// The batch endpoint reads at most this many ids per request.
const BATCH_MAX = 200;

/** Fetches the assets not yet in the cache in one batch request; batch reads do not count as views, so previews leave popularity alone. */
export async function loadAssets(ids: string[], cache: Map<string, Asset>): Promise<void> {
	const missing = [...new Set(ids.filter((id) => !cache.has(id)))];
	for (let i = 0; i < missing.length; i += BATCH_MAX) {
		for (const asset of await fetchAssetBatch(missing.slice(i, i + BATCH_MAX))) {
			cache.set(asset.id, asset);
		}
	}
}

/** Reads several assets in one request, in the order asked for; ids that match nothing are left out. */
export async function fetchAssetBatch(ids: string[]): Promise<Asset[]> {
	if (ids.length === 0) return [];
	try {
		const res = await fetchApi('/assets/batch', {
			method: 'POST',
			body: JSON.stringify({ ids })
		});
		if (!res.ok) return [];
		const data = await res.json();
		return (data.assets as Asset[]) ?? [];
	} catch {
		return [];
	}
}

/** Free-text asset search for the pickers. */
export async function searchAssets(query: string, limit = 10): Promise<Asset[]> {
	const res = await fetchApi(`/assets/search?q=${encodeURIComponent(query)}&limit=${limit}`);
	if (!res.ok) return [];
	const data = await res.json();
	return (data.assets as Asset[]) || [];
}

/** The API omits empty collections, so fill them in before anything binds to them. */
export function normaliseProduct(product: DataProduct): DataProduct {
	return {
		...product,
		tags: product.tags || [],
		metadata: product.metadata || {},
		owners: product.owners || [],
		rules: product.rules || []
	};
}
