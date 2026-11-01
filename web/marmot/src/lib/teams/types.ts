export interface TeamMember {
	user_id: string;
	name: string;
	username: string;
	email?: string;
	profile_picture?: string;
	role: 'owner' | 'member';
	joined_at: string;
}

export interface Team {
	id: string;
	name: string;
	description?: string;
	tags: string[];
	metadata: Record<string, unknown>;
	created_at: string;
	updated_at: string;
	created_by?: string;
	member_count?: number;
}

export interface TeamWithMembers extends Team {
	members: TeamMember[];
}

/** A team as the list and detail endpoints return it, with the single sign-on provenance the base type leaves out. */
export type TeamRow = Team & {
	created_via_sso?: boolean;
	sso_provider?: string;
	asset_count?: number;
	webhook_count?: number;
	/** The single sign-on groups whose members join this team at sign-in. */
	sso_groups?: SSOGroupRef[];
};

/** An identity provider group that feeds a team, and the role the same mapping gives. */
export interface SSOGroupRef {
	provider: string;
	sso_group_name: string;
	role_name?: string;
}

/** A member as the team members endpoint returns it, including how they joined. */
export type TeamMemberRow = TeamMember & {
	source?: 'manual' | 'sso';
	provider_username?: string;
	provider_email?: string;
};

/** A person returned by the owner search, as offered when adding team members. */
export interface OwnerResult {
	id: string;
	name: string;
	type: 'user' | 'team';
	username?: string;
	email?: string;
	profile_picture?: string;
}
