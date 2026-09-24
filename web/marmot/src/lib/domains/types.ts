export const UNASSIGNED_DOMAIN_ID = '00000000-0000-4000-8000-000000000001';

export type DomainKind = 'asset' | 'data_product' | 'glossary_term' | 'ingestion_schedule';

export interface Domain {
	id: string;
	parent_id?: string;
	path: string;
	depth: number;
	name: string;
	description?: string;
	metadata: Record<string, unknown>;
	tags: string[];
	restricted: boolean;
	created_by?: string;
	created_at: string;
	updated_at: string;
}

export interface DomainNode extends Domain {
	children: DomainNode[];
}

export type DomainErrorCode =
	| 'not_found'
	| 'entity_not_found'
	| 'invalid_input'
	| 'cycle'
	| 'too_deep'
	| 'name_conflict'
	| 'has_children'
	| 'not_empty'
	| 'protected'
	| 'restricted_unsupported'
	| 'forbidden'
	| 'duplicate'
	| 'plan_changed';

export type DomainRole = 'domain_admin' | 'steward' | 'reader';
export type SubjectType = 'user' | 'team' | 'service_account';

export interface RoleAssignment {
	id: string;
	domain_id: string;
	domain_name: string;
	subject_type: SubjectType;
	subject_id: string;
	subject_name?: string;
	subject_missing: boolean;
	role: DomainRole;
	inherited: boolean;
	created_by?: string;
	created_at: string;
}

export interface DomainCapabilities {
	domain_id: string;
	/** Whether writes are scoped by domain at all; while false, write is always true. */
	enforced: boolean;
	write: boolean;
	admin: boolean;
}

export interface WritableDomains {
	enforced: boolean;
	all: boolean;
	domain_ids: string[];
}

export interface EnforcementState {
	write: boolean;
	updated_by?: string;
	updated_at?: string;
}

export interface DomainRef {
	id: string;
	path: string;
}

export interface EnforcementPlan {
	state: EnforcementState;
	principals: {
		subject_type: SubjectType;
		subject_id: string;
		name: string;
		permissions: string[];
		keeps: DomainRef[];
		loses: DomainRef[];
	}[];
	pipelines: {
		schedule_id: string;
		name: string;
		domain: DomainRef;
		assets_outside: number;
	}[];
	hash: string;
	generated_at: string;
}
