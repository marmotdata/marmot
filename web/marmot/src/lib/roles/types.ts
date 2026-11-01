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
	/** The role's permissions as resource_type:action keys, sorted. */
	permission_keys?: string[];
	deleted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateRoleInput {
	name: string;
	description?: string;
	permission_ids?: string[];
	/** resource_type:action keys, as GET /permissions spells them on this instance. */
	permission_keys?: string[];
}

export interface UpdateRoleInput {
	name?: string;
	description?: string;
}

export interface ReplacePermissionsInput {
	permission_ids?: string[];
	permission_keys?: string[];
}

/** The answer to POST /roles/validate. */
export interface RoleValidation {
	valid: boolean;
	problems: { path: string; message: string }[];
}
