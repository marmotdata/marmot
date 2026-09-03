export interface GatewayTarget {
	id: string;
	name: string;
	plugin_id: string;
	modes: string[];
	config?: Record<string, unknown>;
	enabled: boolean;
	created_by?: string;
	created_at: string;
	updated_at: string;
}

export interface GatewayGrant {
	id: string;
	principal_type: string;
	principal_id: string;
	resource_selector: string;
	actions: string[];
	expires_at?: string;
	created_by?: string;
	created_at: string;
	revoked_at?: string;
	revoked_by?: string;
	reason?: string;
}

export interface GatewaySession {
	id: string;
	principal_type: string;
	principal_id: string;
	purpose?: string;
	created_at: string;
	expires_at: string;
	last_activity_at?: string;
	revoked_at?: string;
}

export interface GatewayAuditEntry {
	id: string;
	session_id?: string;
	principal_type: string;
	principal_id: string;
	audit_subject: string;
	target_id?: string;
	target_name: string;
	query_text: string;
	referenced_mrns?: string[];
	decision: 'allowed' | 'denied';
	decision_detail?: Record<string, unknown>;
	status: string;
	rows_returned?: number;
	error?: string;
	source: string;
	started_at: string;
	completed_at?: string;
}

export interface PluginInstanceStatus {
	plugin_id: string;
	started_at: string;
	last_used_at: string;
	active_queries: number;
	restarts: number;
}

export interface PluginMeta {
	id: string;
	name: string;
	supports_query?: boolean;
}
