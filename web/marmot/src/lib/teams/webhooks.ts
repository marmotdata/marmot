export interface TeamWebhook {
	id: string;
	team_id: string;
	name: string;
	provider: 'slack' | 'discord' | 'generic';
	webhook_url: string;
	notification_types: string[];
	enabled: boolean;
	last_triggered_at?: string;
	last_error?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateWebhookInput {
	name: string;
	provider: 'slack' | 'discord' | 'generic';
	webhook_url: string;
	notification_types: string[];
	enabled?: boolean;
}

export interface UpdateWebhookInput {
	name?: string;
	webhook_url?: string;
	notification_types?: string[];
	enabled?: boolean;
}

export const NOTIFICATION_TYPE_OPTIONS: { type: string; label: string; icon: string }[] = [
	{ type: 'system', label: 'System', icon: 'material-symbols:info-outline' },
	{ type: 'schema_change', label: 'Schema Change', icon: 'material-symbols:schema-outline' },
	{ type: 'asset_change', label: 'Asset Change', icon: 'material-symbols:database-outline' },
	{ type: 'team_invite', label: 'Team Invite', icon: 'material-symbols:group-add-outline' },
	{ type: 'mention', label: 'Mention', icon: 'material-symbols:alternate-email' },
	{ type: 'job_complete', label: 'Job Complete', icon: 'material-symbols:check-circle-outline' },
	{
		type: 'upstream_schema_change',
		label: 'Upstream Schema Change',
		icon: 'material-symbols:arrow-upward-alt'
	},
	{
		type: 'downstream_schema_change',
		label: 'Downstream Schema Change',
		icon: 'material-symbols:arrow-downward-alt'
	},
	{ type: 'lineage_change', label: 'Lineage Change', icon: 'material-symbols:timeline' }
];

export const NOTIFICATION_TYPE_LABELS: Record<string, string> = Object.fromEntries(
	NOTIFICATION_TYPE_OPTIONS.map((o) => [o.type, o.label])
);

export const NOTIFICATION_TYPES = NOTIFICATION_TYPE_OPTIONS.map((o) => o.type);

export const PROVIDER_OPTIONS = [
	{ value: 'slack', label: 'Slack', icon: 'mdi:slack' },
	{ value: 'discord', label: 'Discord', icon: 'mdi:discord' },
	{ value: 'generic', label: 'Generic Webhook', icon: 'mdi:webhook' }
] as const;

export const PROVIDER_LABELS: Record<string, string> = {
	slack: 'Slack',
	discord: 'Discord',
	generic: 'Generic Webhook'
};
