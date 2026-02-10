export interface AssetSource {
	name: string;
	last_sync_at: string;
	properties: Record<string, any>;
	priority: number;
}

export interface Environment {
	name: string;
	path: string;
	metadata: Record<string, any>;
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
	metadata: Record<string, any>;
	schema: Record<string, any>;
	parent_mrn?: string;
	last_sync_at?: string;
	has_run_history: boolean;
	environments?: Record<string, Environment>;
	sources: AssetSource[];
	query?: string;
	query_language?: string;
	external_links?: ExternalLink[];
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
	example: any;
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
