import { fetchApi } from '../api';

/** An asset as the glossary shows it: in a term's linked list or the link drawer's results. */
export interface LinkedAsset {
	id: string;
	name: string;
	type: string;
	mrn?: string;
	providers?: string[];
	description?: string;
}

/** How a term's linked assets got there: linked manually, or brought in by an asset rule. */
export interface LinkGroup {
	source: 'manual' | 'rule';
	rule_id?: string;
	rule_name?: string;
	count: number;
}

/** The `link` filter that pages through one group. */
export function groupLink(g: LinkGroup): string {
	return g.source === 'rule' && g.rule_id ? `rule:${g.rule_id}` : 'manual';
}

/** A page of the term's linked assets, optionally only one group, with every group's count. */
export async function getLinkedAssets(
	termId: string,
	limit = 20,
	link = ''
): Promise<{ assets: LinkedAsset[]; total: number; groups: LinkGroup[] }> {
	const params = new URLSearchParams({ limit: String(limit) });
	if (link) params.set('link', link);
	const res = await fetchApi(
		`/assets/by-glossary-term/${encodeURIComponent(termId)}?${params.toString()}`
	);
	if (!res.ok) throw new Error('Failed to load linked assets');
	const data = await res.json();
	const assets: LinkedAsset[] = data.assets ?? [];
	return { assets, total: data.total ?? assets.length, groups: data.groups ?? [] };
}

/** Assets that mention the term but are not linked to it, best match first. */
export async function getSuggestedAssets(termId: string, limit = 8): Promise<LinkedAsset[]> {
	const res = await fetchApi(
		`/glossary/suggested-assets/${encodeURIComponent(termId)}?limit=${limit}`
	);
	if (!res.ok) return [];
	const data = await res.json();
	return data.assets ?? [];
}

export async function linkAsset(assetId: string, termId: string): Promise<void> {
	const res = await fetchApi(`/assets/terms/${encodeURIComponent(assetId)}`, {
		method: 'POST',
		body: JSON.stringify({ term_ids: [termId] })
	});
	if (!res.ok) throw new Error('Failed to link asset');
}

export async function unlinkAsset(assetId: string, termId: string): Promise<void> {
	const res = await fetchApi(`/assets/terms/${encodeURIComponent(assetId)}`, {
		method: 'DELETE',
		body: JSON.stringify({ term_id: termId })
	});
	if (!res.ok) throw new Error('Failed to unlink asset');
}

/** Route params for an asset's page, taken from its MRN when it has one. */
export function assetRouteParams(a: LinkedAsset): { type: string; service: string; name: string } {
	const parts = (a.mrn ?? '').replace('mrn://', '').split('/');
	if (parts.length >= 3) {
		return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
	}
	return { type: a.type, service: a.providers?.[0] ?? '', name: a.name };
}
