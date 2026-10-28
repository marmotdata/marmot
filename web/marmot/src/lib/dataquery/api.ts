import { fetchApi } from '$lib/api';
import { throwApiError } from '$lib/stores/toast';
import type {
	QueryInstanceStatus,
	QueryRecord,
	QueryHistoryFilter,
	QueryRunInput,
	QueryHistory,
	QueryResult,
	QuerySourceList,
	QuerySourceTable,
	QueryStats
} from './types';

export async function listQuerySources(
	filter: { q?: string; limit?: number; offset?: number } = {}
): Promise<QuerySourceList> {
	const params = new URLSearchParams();
	if (filter.q) params.set('q', filter.q);
	if (filter.limit) params.set('limit', String(filter.limit));
	if (filter.offset) params.set('offset', String(filter.offset));
	const query = params.size > 0 ? `?${params}` : '';
	const res = await fetchApi(`/query/sources${query}`);
	if (!res.ok) await throwApiError(res);
	const data: QuerySourceList = await res.json();
	return { sources: data.sources ?? [], total: data.total ?? 0 };
}

export async function listQuerySourceTables(
	source: string,
	filter: { q?: string; limit?: number } = {}
): Promise<QuerySourceTable[]> {
	const params = new URLSearchParams();
	if (filter.q) params.set('q', filter.q);
	if (filter.limit) params.set('limit', String(filter.limit));
	const query = params.size > 0 ? `?${params}` : '';
	const res = await fetchApi(`/query/sources/${encodeURIComponent(source)}/tables${query}`);
	if (!res.ok) await throwApiError(res);
	return (await res.json()) ?? [];
}

export async function getQueryStatus(): Promise<QueryInstanceStatus[]> {
	const res = await fetchApi('/query/status');
	if (!res.ok) await throwApiError(res);
	const data = await res.json();
	return data?.instances ?? [];
}

export async function runQuery(input: QueryRunInput): Promise<QueryResult> {
	const res = await fetchApi('/query/run', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) await throwApiError(res);
	return res.json();
}

export async function listQueryHistory(filter: QueryHistoryFilter = {}): Promise<QueryHistory> {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(filter)) {
		if (value !== undefined && value !== '') params.set(key, String(value));
	}
	const query = params.size > 0 ? `?${params}` : '';
	const res = await fetchApi(`/query/history${query}`);
	if (!res.ok) await throwApiError(res);
	const data: QueryHistory = await res.json();
	return { ...data, queries: data.queries ?? [] };
}

export async function getQueryRecord(id: string): Promise<QueryRecord> {
	const res = await fetchApi(`/query/history/${encodeURIComponent(id)}`);
	if (!res.ok) await throwApiError(res);
	return res.json();
}

export async function getQueryStats(from?: Date, to?: Date): Promise<QueryStats> {
	const params = new URLSearchParams();
	if (from) params.set('from', from.toISOString().replace(/\.\d{3}Z$/, 'Z'));
	if (to) params.set('to', to.toISOString().replace(/\.\d{3}Z$/, 'Z'));
	const query = params.size > 0 ? `?${params}` : '';
	const res = await fetchApi(`/query/stats${query}`);
	if (!res.ok) await throwApiError(res);
	return res.json();
}
