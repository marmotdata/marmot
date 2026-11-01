import { m } from '$lib/paraglide/messages';
import { formatDateTime } from '$lib/utils/format';

// Copy for access_expiry notifications, rebuilt from data so it reads in the viewer's language and time zone. Each kind of credential or grant has its own case; anything unrecognised keeps the server's English.

export interface AccessExpiryCopy {
	title: string;
	message: string;
	expired: boolean;
}

interface Stored {
	title: string;
	message: string;
	data?: Record<string, unknown>;
}

function text(data: Record<string, unknown> | undefined, key: string): string | null {
	const value = data?.[key];
	return typeof value === 'string' && value.length > 0 ? value : null;
}

export function accessExpiryCopy(notification: Stored): AccessExpiryCopy {
	const data = notification.data;
	const expired = text(data, 'state') === 'expired';
	const fallback = { title: notification.title, message: notification.message, expired };
	const key = text(data, 'key_name');
	const expiresAt = text(data, 'expires_at');
	const date = expiresAt ? formatDateTime(expiresAt) : null;

	switch (text(data, 'kind')) {
		case 'api_key':
			if (!key) return fallback;
			return expired
				? {
						title: m.notif_access_expiry_api_key_expired_title(),
						message: m.notif_access_expiry_api_key_expired_message({ key }),
						expired
					}
				: date
					? {
							title: m.notif_access_expiry_api_key_expiring_title(),
							message: m.notif_access_expiry_api_key_expiring_message({ key, date }),
							expired
						}
					: fallback;
		case 'service_account_key': {
			const account = text(data, 'account_name');
			if (!key || !account) return fallback;
			return expired
				? {
						title: m.notif_access_expiry_sa_key_expired_title(),
						message: m.notif_access_expiry_sa_key_expired_message({ key, account }),
						expired
					}
				: date
					? {
							title: m.notif_access_expiry_sa_key_expiring_title(),
							message: m.notif_access_expiry_sa_key_expiring_message({ key, account, date }),
							expired
						}
					: fallback;
		}
		default:
			return fallback;
	}
}
