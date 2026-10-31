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
	/** Where the rule sits in the product's hand-set order, from 0. */
	position?: number;
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
	metadata: Record<string, unknown>;
	tags: string[];
	owners: Owner[];
	rules: Rule[];
	created_by?: string;
	created_at: string;
	updated_at: string;
	asset_count?: number;
	manual_asset_count?: number;
	rule_asset_count?: number;
	icon_url?: string;
	/** 0 to 4, from its assets' views over the last 30 days. */
	popularity?: number;
	/** Only on a single product: its assets counted by type. */
	asset_types?: { type: string; count: number }[];
}

export type ProductSort = 'popular' | 'updated' | 'name';

export interface ProductFacets {
	owners: { id: string; name: string; type: 'user' | 'team'; count: number }[];
	tags: { tag: string; count: number }[];
}

/** One asset in a data product, with how it got there. */
export interface Member {
	asset_id: string;
	name: string;
	mrn: string;
	type: string;
	providers: string[];
	description?: string;
	owner_names: string[];
	source: 'manual' | 'rule';
	rule_id?: string;
	rule_name?: string;
	popularity: number;
	added_at: string;
}

export interface MembersResponse {
	members: Member[];
	total: number;
}

export interface ProductImageMeta {
	id: string;
	data_product_id: string;
	purpose: 'icon' | 'header';
	filename: string;
	content_type: string;
	size_bytes: number;
	url: string;
	created_at: string;
}

export interface CreateDataProductInput {
	name: string;
	description?: string;
	metadata?: Record<string, unknown>;
	tags?: string[];
	owners: OwnerInput[];
	rules?: RuleInput[];
}

export interface UpdateDataProductInput {
	name?: string;
	description?: string;
	metadata?: Record<string, unknown>;
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
