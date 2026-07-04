export interface Permission {
	id: string;
	name: string;
	description: string;
	resource_type: string;
	action: string;
}

export interface Role {
	id: string;
	name: string;
	description: string;
	is_system: boolean;
	user_count?: number;
	permissions?: Permission[];
	deleted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateRoleInput {
	name: string;
	description?: string;
	permission_ids?: string[];
}

export interface UpdateRoleInput {
	name?: string;
	description?: string;
}

export interface ReplacePermissionsInput {
	permission_ids: string[];
}
