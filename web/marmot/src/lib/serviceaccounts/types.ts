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
}

export interface ServiceAccountAPIKey {
	id: string;
	service_account_id: string;
	name: string;
	key?: string;
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
