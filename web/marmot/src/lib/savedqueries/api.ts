import { fetchApi } from '$lib/api';
import { throwApiError } from '$lib/stores/toast';

export interface SavedQuery {
	id: string;
	name: string;
	sql: string;
	source: string;
	last_run_at?: string;
	created_at: string;
	updated_at: string;
}

export interface SavedQueryInput {
	name: string;
	sql: string;
	source: string;
}

export async function listSavedQueries(
	limit = 50
): Promise<{ queries: SavedQuery[]; total: number }> {
	const res = await fetchApi(`/query/saved?limit=${limit}`);
	if (!res.ok) await throwApiError(res);
	const data = await res.json();
	return { queries: data.queries ?? [], total: data.total ?? 0 };
}

export async function createSavedQuery(input: SavedQueryInput): Promise<SavedQuery> {
	const res = await fetchApi('/query/saved', { method: 'POST', body: JSON.stringify(input) });
	if (!res.ok) await throwApiError(res);
	return res.json();
}

export async function updateSavedQuery(id: string, input: SavedQueryInput): Promise<SavedQuery> {
	const res = await fetchApi(`/query/saved/${encodeURIComponent(id)}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!res.ok) await throwApiError(res);
	return res.json();
}

export async function deleteSavedQuery(id: string): Promise<void> {
	const res = await fetchApi(`/query/saved/${encodeURIComponent(id)}`, { method: 'DELETE' });
	if (!res.ok) await throwApiError(res);
}

// The statement itself runs through /query/run; this only moves the saved query up the list
export async function markSavedQueryRun(id: string): Promise<void> {
	const res = await fetchApi(`/query/saved/${encodeURIComponent(id)}/run`, { method: 'POST' });
	if (!res.ok) await throwApiError(res);
}

/** The Query workspace address that opens a saved query, which also records it as recently viewed. */
export function savedQueryHref(base: string, q: Pick<SavedQuery, 'id' | 'source' | 'sql'>): string {
	const params = new URLSearchParams({ source: q.source, q: q.sql, saved: q.id });
	return `${base}?${params}`;
}
