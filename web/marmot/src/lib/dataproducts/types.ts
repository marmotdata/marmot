export interface Owner {
	id: string;
	username?: string;
	name: string;
	type: 'user' | 'team';
	email?: string;
	profile_picture?: string;
}

export interface OwnerInput {
	id: string;
	type: 'user' | 'team';
}

export type RuleType = 'query';

export interface Rule {
	id: string;
	data_product_id: string;
	name: string;
	description?: string;
	rule_type: RuleType;
	query_expression?: string;
	is_enabled: boolean;
	created_at: string;
	updated_at: string;
	matched_asset_count?: number;
}

export interface RuleInput {
	id?: string;
	name: string;
	description?: string;
	rule_type: RuleType;
	query_expression?: string;
	is_enabled: boolean;
}

export interface DataProduct {
	id: string;
	name: string;
	description?: string;
	documentation?: string;
	metadata: Record<string, any>;
	tags: string[];
	owners: Owner[];
	rules: Rule[];
	created_by?: string;
	created_at: string;
	updated_at: string;
	asset_count?: number;
	manual_asset_count?: number;
	rule_asset_count?: number;
}

export interface CreateDataProductInput {
	name: string;
	description?: string;
	documentation?: string;
	metadata?: Record<string, any>;
	tags?: string[];
	owners: OwnerInput[];
	rules?: RuleInput[];
}

export interface UpdateDataProductInput {
	name?: string;
	description?: string;
	documentation?: string;
	metadata?: Record<string, any>;
	tags?: string[];
	owners?: OwnerInput[];
}

export interface DataProductsListResponse {
	data_products: DataProduct[];
	total: number;
}

export interface ResolvedAssetsResponse {
	manual_assets: string[];
	dynamic_assets: string[];
	all_assets: string[];
	total: number;
}

export interface RulePreviewResponse {
	asset_ids: string[];
	asset_count: number;
	errors?: string[];
}

export interface AssetsResponse {
	asset_ids: string[];
	total: number;
}
