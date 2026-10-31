export interface SSOProvider {
	type: string;
	name: string;
	issuer_url?: string;
	enabled?: boolean;
}

/** A configured provider as the Authentication settings show it. */
export interface SSOProviderStatus {
	type: string;
	name: string;
	protocol: string;
	issuer_url?: string;
	/** False when an administrator has switched it off here; its configuration is kept. */
	enabled: boolean;
	/** Members who have signed in with it at least once. */
	member_count: number;
}

/** A single sign-on group and the team, role or both its members get. */
export interface GroupMapping {
	id: string;
	provider: string;
	sso_group_name: string;
	/** Empty when the mapping only gives a role. */
	team_id: string;
	team_name?: string;
	role_id?: string;
	role_name?: string;
	member_role: 'owner' | 'member';
	created_at: string;
	updated_at: string;
}

/** Who may sign in and what a new member starts with. */
export interface SignInSettings {
	create_on_first_sso: boolean;
	default_role: string;
	default_team_id: string;
	allow_password_sign_in: boolean;
}
