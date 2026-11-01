export type AccessPrincipalType = 'user' | 'service_account';

export type QueryDecision = 'allowed' | 'denied';

export type QueryRecordStatus = 'running' | 'completed' | 'failed' | 'denied' | 'cancelled';

// Why a query ended the way it did, finer than its decision and status; an authorizer may add its own, such as grant in Marmot Cloud
export type QueryOutcomeReason =
	| 'role'
	| 'grant'
	| 'permission'
	| 'policy'
	| 'source_error'
	| 'timeout'
	| 'unreachable'
	| 'cancelled';

export type QueryClient = 'api' | 'mcp' | 'cli' | 'editor';

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
	// The table's size as its plugin last reported it in the catalog
	row_count?: number;
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
	owners?: string[];
	row_count?: number;
	// The table's catalogued columns among the result's columns, absent when the catalog has no columns for it
	columns_read?: number;
	personal_data?: { name: string; read: boolean }[];
	certified?: boolean;
}

export interface QueryDecisionDetail {
	reason?: string;
	rule?: string;
	permission?: string;
	policy?: string;
	needed?: string;
	resources?: string[];
	roles?: string[];
}

// What refused a query, what would allow it and who the caller was acting as, sent with a 403 from /query/run
export interface QueryRefusal {
	query_id?: string;
	reason: string;
	rule: string;
	permission?: string;
	policy?: string;
	needed?: string;
	resources?: string[];
	roles: string[];
	source: string;
	decided_at: string;
}

export interface QueryAccessRequest {
	id: string;
	source_name: string;
	needed?: string;
	created_at: string;
	notified: number;
	repeat: boolean;
}

export interface QueryRecord {
	id: string;
	principal_type: AccessPrincipalType;
	principal_id: string;
	audit_subject: string;
	client: QueryClient;
	source_id?: string;
	source_name: string;
	/** Plugin behind the source, such as snowflake; absent once the source is deleted. */
	source_provider?: string;
	statement: string;
	referenced_mrns: string[] | null;
	decision: QueryDecision;
	decision_detail?: QueryDecisionDetail;
	status: QueryRecordStatus;
	outcome_reason?: QueryOutcomeReason | string;
	// The service account a machine caller ran as, the agent that spoke for it and how it authenticated
	service_account?: string;
	agent?: string;
	credential?: string;
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

export type QueryOutcomeFilter = 'allowed' | 'denied' | 'failed' | 'cancelled' | 'running';

export interface QueryHistoryFilter {
	id?: string;
	// A caller as principal_type:principal_id or as its audit subject, such as user:alice
	caller?: string;
	outcome?: QueryOutcomeFilter | '';
	reason?: QueryOutcomeReason | '';
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
	// The ingestion schedule behind a source in top_sources, for linking to its pipeline
	schedule_id?: string;
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

/** Queries through Marmot that read one asset, over the last 14 days by default. */
export interface QueryAssetStats {
	mrn: string;
	from: string;
	to: string;
	queries: number;
	people: number;
	agents: number;
	denied: number;
	median_duration_ms?: number;
	/** Queries per UTC day, oldest first. */
	per_day: number[];
}
