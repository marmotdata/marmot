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
	/** Set on list responses: an invited member has an account waiting but has not used their link yet. */
	status?: MemberStatus;
	/** Accurate to a few minutes, since the server writes it at most every five. */
	last_seen_at?: string;
	invite_expires_at?: string;
}

export type MemberStatus = 'active' | 'invited' | 'deactivated';

/** The one-time link for an invited member; the token appears only in this response. */
export interface Invitation {
	user: User;
	token: string;
	invite_url: string;
	expires_at: string;
	failed_team_ids?: string[];
}

/** What the holder of an invitation link may learn before accepting it. */
export interface InviteInfo {
	email: string;
	name: string;
	expires_at: string;
	password_sign_in: boolean;
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
