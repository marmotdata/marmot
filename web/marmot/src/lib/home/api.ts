import { fetchApi } from '$lib/api';

export type RecentKind = 'asset' | 'glossary_term' | 'data_product' | 'saved_query';

export interface RecentItem {
	kind: RecentKind;
	id: string;
	name: string;
	type?: string;
	provider?: string;
	mrn?: string;
	viewed_at: string;
}

export interface AgentSummary {
	principal_type: string;
	principal_id: string;
	name: string;
	audit_subject: string;
	queries_24h: number;
	queries_7d: number;
	refused_24h: number;
	refused_7d: number;
	tables_read_7d: number;
	last_active_at?: string;
}

/** One asset from GET /assets/stale, owned by the search area. */
export interface StaleAsset {
	id: string;
	name: string;
	mrn: string;
	type: string;
	providers?: string[];
	last_refreshed_at?: string;
	pipeline_name?: string;
}

export async function listRecent(limit = 10): Promise<RecentItem[]> {
	const res = await fetchApi(`/users/me/recent?limit=${limit}`);
	if (!res.ok) throw new Error(res.statusText);
	const data = await res.json();
	return data.items ?? [];
}

export async function listAgentSummaries(limit = 4): Promise<AgentSummary[]> {
	const res = await fetchApi(`/query/agents?limit=${limit}`);
	if (!res.ok) throw new Error(res.statusText);
	const data = await res.json();
	return data.agents ?? [];
}

export async function listStaleAssets(limit = 5): Promise<{ assets: StaleAsset[]; total: number }> {
	const res = await fetchApi(`/assets/stale?limit=${limit}`);
	if (!res.ok) throw new Error(res.statusText);
	const data = await res.json();
	return { assets: data.assets ?? [], total: data.total ?? 0 };
}

type ViewBody =
	| { kind: 'asset'; type: string; service: string; name: string }
	| { kind: Exclude<RecentKind, 'asset'>; id: string };

/** Which item, if any, a page shows, worked out from its route so the pages themselves need not know about recents. */
export function viewedItem(
	routeId: string | null,
	params: Record<string, string | undefined>,
	search: URLSearchParams
): ViewBody | null {
	switch (routeId) {
		case '/discover/[type]/[service]/[name]':
			if (!params.type || !params.service || !params.name) return null;
			return { kind: 'asset', type: params.type, service: params.service, name: params.name };
		case '/glossary/[[id]]':
			return params.id && params.id !== 'new' ? { kind: 'glossary_term', id: params.id } : null;
		case '/products/[id]':
			return params.id ? { kind: 'data_product', id: params.id } : null;
		case '/query': {
			const saved = search.get('saved');
			return saved ? { kind: 'saved_query', id: saved } : null;
		}
	}
	return null;
}

let lastRecorded = '';

/** Notes a page view for the viewer's Jump back in list. Best effort: a failure never reaches the page. */
export function recordView(body: ViewBody | null): void {
	if (!body) return;
	const key = JSON.stringify(body);
	// Navigating within one page, such as switching an asset's tabs, is not a new view
	if (key === lastRecorded) return;
	lastRecorded = key;
	fetchApi('/users/me/recent', { method: 'POST', body: JSON.stringify(body) }).catch(() => {});
}
