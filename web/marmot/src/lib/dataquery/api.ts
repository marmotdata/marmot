import { fetchApi } from '$lib/api';
import { throwApiError } from '$lib/stores/toast';
import { parseSchemaResponse, processSchema } from '$lib/schema/utils';
import type {
	QueryInstanceStatus,
	QueryAccessRequest,
	QueryRecord,
	QueryRefusal,
	QueryHistoryFilter,
	QueryRunInput,
	QueryHistory,
	QueryResult,
	QuerySourceList,
	QuerySourceTable,
	QueryStats,
	QueryAssetStats
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

// A refused query, with what refused it and what would allow it
export class QueryRefusedError extends Error {
	refusal: QueryRefusal;

	constructor(message: string, refusal: QueryRefusal) {
		super(message);
		this.name = 'QueryRefusedError';
		this.refusal = refusal;
	}
}

// The editor names itself so the audit tells its queries from other API calls; aborting the signal hangs up, which cancels the query on the server
export async function runQuery(input: QueryRunInput, signal?: AbortSignal): Promise<QueryResult> {
	const res = await fetchApi('/query/run', {
		method: 'POST',
		body: JSON.stringify(input),
		headers: { 'X-Marmot-Client': 'editor' },
		signal
	});
	if (res.status === 403) {
		const body = await res
			.clone()
			.json()
			.catch(() => null);
		if (body?.code === 'query_denied' && body.refusal)
			throw new QueryRefusedError(body.error ?? '', body.refusal as QueryRefusal);
	}
	if (!res.ok) await throwApiError(res);
	return res.json();
}

export async function cancelQuery(id: string): Promise<void> {
	const res = await fetchApi(`/query/history/${encodeURIComponent(id)}/cancel`, { method: 'POST' });
	// 409 means it already finished or runs elsewhere; hanging up the run request stops it either way
	if (!res.ok && res.status !== 409) await throwApiError(res);
}

export async function requestQueryAccess(input: {
	query_id?: string;
	source?: string;
	message?: string;
}): Promise<QueryAccessRequest> {
	const res = await fetchApi('/query/access-requests', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) await throwApiError(res);
	return res.json();
}

function historyParams(filter: QueryHistoryFilter): URLSearchParams {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(filter)) {
		if (value !== undefined && value !== '') params.set(key, String(value));
	}
	return params;
}

// The server streams the export with the log's own filters, so it is fetched with the session's token and saved from a blob
export async function exportQueryHistory(filter: QueryHistoryFilter = {}): Promise<Blob> {
	const params = historyParams({ ...filter, limit: undefined, offset: undefined });
	params.set('format', 'csv');
	const res = await fetchApi(`/query/history/export?${params}`);
	if (!res.ok) await throwApiError(res);
	return res.blob();
}

export async function listQueryHistory(filter: QueryHistoryFilter = {}): Promise<QueryHistory> {
	const params = historyParams(filter);
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

// Columns come from the table's catalog schema, so the source explorer and SQL completion know them without a round trip to the source
export async function getTableColumns(assetId: string): Promise<{ name: string; type: string }[]> {
	const res = await fetchApi(`/assets/${encodeURIComponent(assetId)}`);
	if (!res.ok) await throwApiError(res);
	const asset: { schema?: unknown } = await res.json();
	if (!asset.schema) return [];
	for (const section of parseSchemaResponse(asset.schema)) {
		const { fields } = processSchema(section.schema);
		const columns = fields.filter((f) => f.name && f.type !== 'error');
		if (columns.length > 0) return columns.map((f) => ({ name: f.name, type: f.type ?? '' }));
	}
	return [];
}

export async function getAssetQueryStats(mrn: string): Promise<QueryAssetStats> {
	const res = await fetchApi(`/query/asset-stats?${new URLSearchParams({ mrn })}`);
	if (!res.ok) await throwApiError(res);
	const data: QueryAssetStats = await res.json();
	return { ...data, per_day: data.per_day ?? [] };
}
