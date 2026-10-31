export interface AssetSource {
	name: string;
	last_sync_at: string;
	properties: Record<string, unknown>;
	priority: number;
}

export interface Environment {
	name: string;
	path: string;
	metadata: Record<string, unknown>;
}

export interface ExternalLink {
	name: string;
	url: string;
	icon?: string;
}

export interface EnrichedExternalLink extends ExternalLink {
	source: string;
	rule_id?: string;
	rule_name?: string;
}

export interface Asset {
	id: string;
	name: string;
	mrn: string;
	type: string;
	providers: string[];
	description?: string; // Technical description (from plugins)
	user_description?: string; // User-provided notes
	tags: string[];
	created_at: string;
	updated_at: string;
	created_by: string;
	metadata: Record<string, unknown>;
	schema: Record<string, unknown>;
	parent_mrn?: string;
	is_stub?: boolean; // Placeholder created from lineage before the asset was catalogued
	last_sync_at?: string;
	has_run_history: boolean;
	environments?: Record<string, Environment>;
	sources: AssetSource[];
	query?: string;
	query_language?: string;
	external_links?: ExternalLink[];
	/** Size from what the asset's plugin recorded; absent when no plugin said. */
	row_count?: number;
	column_count?: number;
	/** True when the pipeline that refreshes the asset has missed more than two runs' worth of time. */
	stale?: boolean;
	/** 0 to 4 from views over the last 30 days; 0 means not viewed recently. */
	popularity?: number;
	/** Markers people set on schema fields, merged onto the outline by schema key and dotted path. */
	field_annotations?: FieldAnnotation[];
}

export interface FieldAnnotation {
	schema: string;
	path: string;
	pii: boolean;
	certified: boolean;
	description?: string;
	glossary_terms: { id: string; name: string }[];
	updated_by?: string;
	updated_at: string;
}

/** Numbers beside the asset page's tabs; queries is absent for callers who may not read the query log. */
export interface AssetTabCounts {
	lineage: number;
	runs: number;
	agent_runs: number;
	queries?: number;
}

export interface GlossaryTerm {
	id: string;
	name: string;
	definition: string;
	created_at: string;
	updated_at: string;
	created_by?: string;
	created_by_username?: string;
}

export interface AssetTerm {
	term_id: string;
	term_name: string;
	definition: string;
	source: string; // "user" or "plugin:name"
	created_at: string;
	created_by?: string;
	created_by_username?: string;
}

export interface AssetsResponse {
	assets: Asset[];
	total: number;
	limit: number;
	offset: number;
	filters: {
		types: { [key: string]: number };
		providers: { [key: string]: number };
		tags: { [key: string]: number };
	};
}

export interface AssetSummaryResponse {
	types: { [key: string]: number };
	providers: { [key: string]: number };
	tags: { [key: string]: number };
}

export interface MetadataFieldSuggestion {
	field: string;
	type: string;
	example: unknown;
	count: number;
}

export interface MetadataValueSuggestion {
	value: string;
	count: number;
	example?: Asset;
}

export interface QueryToken {
	text: string;
	type: 'field' | 'operator' | 'value' | 'boolean' | 'text';
	color: string;
}

export interface Filters {
	types: string[];
	providers: string[];
	tags: string[];
	updatedAfter?: string;
}
