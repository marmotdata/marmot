import { m } from '$lib/paraglide/messages';
import { formatList } from '$lib/utils/format';
import type { Notification } from '$lib/stores/notifications';
import { accessExpiryCopy } from './accessExpiry';

// Copy is rebuilt from the stored type and data so it translates, falling back to the server's English for old rows and unknown types
function text(data: Record<string, unknown> | undefined, key: string): string | null {
	const value = data?.[key];
	return typeof value === 'string' && value.length > 0 ? value : null;
}

function count(data: Record<string, unknown> | undefined, key: string): number | null {
	const value = data?.[key];
	return typeof value === 'number' ? value : null;
}

function isBatched(notification: Notification): boolean {
	return count(notification.data, 'asset_count') !== null;
}

export function renderNotificationTitle(notification: Notification): string {
	const batched = isBatched(notification);
	switch (notification.type) {
		case 'schema_change':
			return m.notif_schema_change_title();
		case 'upstream_schema_change':
			return m.notif_upstream_schema_change_title();
		case 'downstream_schema_change':
			return m.notif_downstream_schema_change_title();
		case 'lineage_change':
			return batched ? m.notif_batch_lineage_change_title() : m.notif_lineage_change_title();
		case 'asset_deleted':
			return batched ? m.notif_batch_asset_deleted_title() : m.notif_asset_deleted_title();
		case 'asset_change':
			return batched ? m.notif_batch_asset_change_title() : m.notif_asset_change_title();
		case 'job_complete':
			switch (text(notification.data, 'status')) {
				case 'failed':
					return m.notif_job_failed_title();
				case 'cancelled':
					return m.notif_job_cancelled_title();
				case 'completed':
					return m.notif_job_complete_title();
				default:
					return notification.title;
			}
		case 'access_expiry':
			return accessExpiryCopy(notification).title;
		case 'access_request':
			return m.notif_access_request_title();
		case 'team_invite':
			return m.notif_team_invite_title();
		case 'mention':
			return text(notification.data, 'mention_target') === 'team'
				? m.notif_mention_team_title()
				: m.notif_mention_user_title();
		default:
			return notification.title;
	}
}

export function renderNotificationMessage(notification: Notification): string {
	const data = notification.data;
	const assetName = text(data, 'asset_name');
	const assetCount = count(data, 'asset_count');

	switch (notification.type) {
		case 'schema_change':
			return assetName ? m.notif_schema_change_message({ name: assetName }) : notification.message;
		case 'upstream_schema_change':
			return assetName
				? m.notif_upstream_schema_change_message({ name: assetName })
				: notification.message;
		case 'downstream_schema_change':
			return assetName
				? m.notif_downstream_schema_change_message({ name: assetName })
				: notification.message;
		case 'lineage_change':
			if (assetCount !== null) return m.notif_batch_lineage_change_message({ count: assetCount });
			return assetName ? m.notif_lineage_change_message({ name: assetName }) : notification.message;
		case 'asset_deleted':
			if (assetCount !== null) return m.notif_batch_asset_deleted_message({ count: assetCount });
			return assetName ? m.notif_asset_deleted_message({ name: assetName }) : notification.message;
		case 'asset_change': {
			const eventCount = count(data, 'count');
			if (assetCount !== null && eventCount !== null) {
				return m.notif_batch_asset_change_message({ count: eventCount, assetCount });
			}
			const fields = data?.changed_fields;
			if (assetName && Array.isArray(fields) && fields.length > 0) {
				return m.notif_asset_change_message({
					fields: formatList(fields.map(String)),
					name: assetName
				});
			}
			return notification.message;
		}
		case 'job_complete': {
			const pipeline = text(data, 'pipeline_name');
			if (!pipeline) return notification.message;
			switch (text(data, 'status')) {
				case 'completed': {
					const entities = count(data, 'total_entities');
					return entities !== null
						? m.notif_job_complete_message_entities({ name: pipeline, count: entities })
						: m.notif_job_complete_message({ name: pipeline });
				}
				case 'failed': {
					const error = text(data, 'error_message');
					return error
						? m.notif_job_failed_message_reason({ name: pipeline, error })
						: m.notif_job_failed_message({ name: pipeline });
				}
				case 'cancelled':
					return m.notif_job_cancelled_message({ name: pipeline });
				default:
					return notification.message;
			}
		}
		case 'access_expiry':
			return accessExpiryCopy(notification).message;
		case 'access_request': {
			const requester = text(data, 'requester');
			const needed = text(data, 'needed');
			return requester && needed
				? m.notif_access_request_message({ requester, needed })
				: notification.message;
		}
		case 'team_invite': {
			const teamName = text(data, 'team_name');
			const role = text(data, 'role');
			return teamName && role
				? m.notif_team_invite_message({ name: teamName, role })
				: notification.message;
		}
		case 'mention': {
			const actor = text(data, 'mentioner');
			const page = text(data, 'page_title');
			if (!actor || !page) return notification.message;
			return text(data, 'mention_target') === 'team'
				? m.notif_mention_team_message({ actor, page })
				: m.notif_mention_user_message({ actor, page });
		}
		default:
			return notification.message;
	}
}

// Deleted assets no longer exist, so their notifications open nothing
export function notificationPath(notification: Notification): string | null {
	if (notification.type === 'asset_deleted') return null;
	const data = notification.data;
	if (typeof data?.link === 'string' && data.link.startsWith('/')) return data.link;
	if (typeof data?.asset_mrn === 'string') return `/discover/${encodeURIComponent(data.asset_mrn)}`;
	if (data?.team_id) return `/settings/teams/${String(data.team_id)}`;
	return null;
}
