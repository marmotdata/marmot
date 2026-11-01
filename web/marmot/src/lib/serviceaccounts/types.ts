export interface ServiceAccountRole {
	id: string;
	name: string;
	description?: string;
}

export interface ServiceAccount {
	id: string;
	name: string;
	description?: string;
	active: boolean;
	roles: ServiceAccountRole[];
	created_at: string;
	updated_at: string;
	created_by_name?: string;
	/** The latest use of any of the account's keys */
	last_used_at?: string;
	api_key_count?: number;
	calls_last_7_days?: number;
	/** Agents that have reported runs with one of the account's keys */
	used_by?: ServiceAccountAgent[];
}

export interface ServiceAccountAgent {
	id: string;
	name: string;
	mrn: string;
}

export interface ServiceAccountAPIKey {
	id: string;
	service_account_id: string;
	name: string;
	key?: string;
	/** The start of the key kept in clear; absent for keys made before it was stored */
	prefix?: string;
	last_used_at?: string;
	expires_at?: string;
	created_at: string;
}

export interface CreateServiceAccountInput {
	name: string;
	description?: string;
	role_ids?: string[];
}

export interface UpdateServiceAccountInput {
	name?: string;
	description?: string;
	active?: boolean;
	role_ids?: string[];
}

export interface CreateAPIKeyInput {
	name: string;
	expires_in_days?: number;
}
