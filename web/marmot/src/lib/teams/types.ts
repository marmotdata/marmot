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
