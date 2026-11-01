export interface Owner {
	id: string;
	username?: string;
	name: string;
	type: 'user' | 'team';
	email?: string;
	profile_picture?: string;
}

export interface GlossaryTerm {
	id: string;
	name: string;
	definition: string;
	description?: string;
	parent_term_id?: string;
	owners: Owner[];
	metadata: Record<string, unknown>;
	tags: string[];
	created_by?: string;
	created_by_name?: string;
	created_at: string;
	updated_at: string;
	deleted_at?: string;
	/** Every asset linked to the term, by hand or by an asset rule. */
	linked_asset_count?: number;
	/** Only present when a single term is read. */
	related_terms?: RelatedTerm[];
}

export interface RelatedTerm {
	id: string;
	name: string;
}

export interface OwnerInput {
	id: string;
	type: 'user' | 'team';
}

export interface CreateTermInput {
	name: string;
	definition: string;
	description?: string;
	parent_term_id?: string;
	owners?: OwnerInput[];
	metadata?: Record<string, unknown>;
	tags?: string[];
	related_term_ids?: string[];
}

export interface UpdateTermInput {
	name?: string;
	definition?: string;
	description?: string | null;
	parent_term_id?: string;
	owners?: OwnerInput[];
	metadata?: Record<string, unknown>;
	tags?: string[];
	related_term_ids?: string[];
}

export interface TermsListResponse {
	terms: GlossaryTerm[];
	total: number;
}

export interface TermChildrenResponse {
	children: GlossaryTerm[];
	total: number;
}

export interface TermAncestorsResponse {
	ancestors: GlossaryTerm[];
	total: number;
}
