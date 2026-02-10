export type RuleType = 'query' | 'metadata_match';

export interface ExternalLink {
	name: string;
	url: string;
	icon?: string;
}

export interface AssetRule {
	id: string;
	name: string;
	description?: string;
	links: ExternalLink[];
	term_ids: string[];
	rule_type: RuleType;
	query_expression?: string;
	metadata_field?: string;
	pattern_type?: string;
	pattern_value?: string;
	priority: number;
	is_enabled: boolean;
	created_by?: string;
	created_at: string;
	updated_at: string;
	membership_count: number;
	last_reconciled_at?: string;
}

export interface CreateAssetRuleInput {
	name: string;
	description?: string;
	links?: ExternalLink[];
	term_ids?: string[];
	rule_type: RuleType;
	query_expression?: string;
	metadata_field?: string;
	pattern_type?: string;
	pattern_value?: string;
	priority: number;
	is_enabled: boolean;
}

export interface UpdateAssetRuleInput {
	name?: string;
	description?: string;
	links?: ExternalLink[];
	term_ids?: string[];
	rule_type?: RuleType;
	query_expression?: string;
	metadata_field?: string;
	pattern_type?: string;
	pattern_value?: string;
	priority?: number;
	is_enabled?: boolean;
}

export interface AssetRulesListResponse {
	asset_rules: AssetRule[];
	total: number;
}

export interface RulePreviewResponse {
	asset_ids: string[];
	asset_count: number;
	errors?: string[];
}

export interface RuleAssetsResponse {
	asset_ids: string[];
	total: number;
}

export interface EnrichedExternalLink extends ExternalLink {
	source: string;
	rule_id?: string;
	rule_name?: string;
}
