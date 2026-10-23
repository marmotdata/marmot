export type AccessPrincipalType = 'user' | 'service_account';

export type QueryDecision = 'allowed' | 'denied';

export type QueryRecordStatus = 'running' | 'completed' | 'failed' | 'denied';

export interface QuerySource {
	id: string;
	name: string;
	plugin_id: string;
	ingest_on_query: boolean;
	table_count: number;
	queries_last_week: number;
	last_queried_at?: string;
}

export interface QuerySourceTable {
	mrn: string;
	asset_id: string;
	name: string;
	type: string;
	schema?: string;
	description?: string;
	tags?: string[];
	queries: number;
	sample_statement?: string;
}

export interface QuerySourceList {
	sources: QuerySource[];
	total: number;
}

export interface QueryInstanceStatus {
	plugin_id: string;
	started_at: string;
	last_used_at: string;
	active_queries: number;
	restarts: number;
}

export interface QueryAssetContext {
	mrn: string;
	asset_id?: string;
	name?: string;
	type?: string;
	providers?: string[];
	description?: string;
	tags?: string[];
	last_sync_at?: string;
}

export interface QueryDecisionDetail {
	reason?: string;
}

export interface QueryRecord {
	id: string;
	principal_type: AccessPrincipalType;
	principal_id: string;
	audit_subject: string;
	client: 'api' | 'mcp' | 'cli';
	source_id?: string;
	source_name: string;
	statement: string;
	referenced_mrns: string[] | null;
	decision: QueryDecision;
	decision_detail?: QueryDecisionDetail;
	status: QueryRecordStatus;
	rows_returned?: number;
	truncated: boolean;
	error?: string;
	context?: QueryAssetContext[];
	started_at: string;
	completed_at?: string;
}

export interface QueryHistory {
	queries: QueryRecord[];
	total: number;
	limit: number;
	offset: number;
}

export interface QueryHistoryFilter {
	source?: string;
	decision?: QueryDecision | '';
	status?: QueryRecordStatus | '';
	mrn?: string;
	principal_type?: AccessPrincipalType | '';
	principal_id?: string;
	mine?: boolean;
	// Comma-separated clients to include, such as mcp,cli
	client?: string;
	// RFC 3339 bounds on when the query started
	from?: string;
	to?: string;
	// Matches the caller, source, statement or a referenced MRN
	q?: string;
	limit?: number;
	offset?: number;
}

export interface QueryColumn {
	name: string;
	type: string;
}

export interface QueryRunInput {
	source: string;
	statement: string;
	max_rows?: number;
}

export interface QueryResult {
	query_id: string;
	columns: QueryColumn[];
	rows: unknown[][];
	row_count: number;
	truncated: boolean;
	referenced_mrns: string[];
	context: QueryAssetContext[];
}

export type QueryStatsInterval = 'hour' | 'day';

export interface QueryStatsTotals {
	queries: number;
	allowed: number;
	denied: number;
	failed: number;
	callers: number;
	tables: number;
}

export interface QueryStatsPoint {
	bucket: string;
	succeeded: number;
	failed: number;
	denied: number;
}

export interface QueryStatsGroup {
	key: string;
	label: string;
	principal_type?: string;
	plugin_id?: string;
	tags?: string[];
	allowed: number;
	denied: number;
	total: number;
}

export interface QueryStats {
	from: string;
	to: string;
	interval: QueryStatsInterval;
	totals: QueryStatsTotals;
	series: QueryStatsPoint[];
	top_callers: QueryStatsGroup[];
	top_sources: QueryStatsGroup[];
	top_tables: QueryStatsGroup[];
}
