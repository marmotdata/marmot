export interface Role {
	id: string;
	name: string;
	description?: string;
}

export interface UserIdentity {
	id: string;
	provider: string;
	provider_user_id: string;
	provider_email?: string;
	created_at: string;
	updated_at: string;
}

export interface User {
	id: string;
	username: string;
	name: string;
	email?: string;
	profile_picture?: string;
	active: boolean;
	roles: Role[];
	identities?: UserIdentity[];
	created_at: string;
	updated_at: string;
}

export interface CreateUserInput {
	username: string;
	name: string;
	password: string;
	role_names: string[];
}

export interface UpdateUserInput {
	name?: string;
	email?: string;
	password?: string;
	active?: boolean;
	role_names?: string[];
}
