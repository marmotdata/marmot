import { writable, get } from 'svelte/store';
import { browser } from '$app/environment';
import { fetchApi } from '$lib/api';

export type NotificationType =
	| 'system'
	| 'schema_change'
	| 'asset_change'
	| 'mention'
	| 'job_complete'
	| 'upstream_schema_change'
	| 'downstream_schema_change'
	| 'lineage_change';

export interface NotificationPreferences {
	system: boolean;
	schema_change: boolean;
	asset_change: boolean;
	mention: boolean;
	job_complete: boolean;
	upstream_schema_change: boolean;
	downstream_schema_change: boolean;
	lineage_change: boolean;
}

const defaultPreferences: NotificationPreferences = {
	system: true,
	schema_change: true,
	asset_change: true,
	mention: true,
	job_complete: true,
	upstream_schema_change: true,
	downstream_schema_change: true,
	lineage_change: true
};

function createNotificationPreferencesStore() {
	const { subscribe, set, update } = writable<NotificationPreferences>(defaultPreferences);
	let initialized = false;

	async function initialize() {
		if (!browser || initialized) return;

		try {
			const response = await fetchApi('/users/me');
			if (response.ok) {
				const user = await response.json();
				if (user.preferences?.notifications) {
					set({ ...defaultPreferences, ...user.preferences.notifications });
				}
			}
		} catch (error) {
			console.error('Failed to fetch notification preferences:', error);
		}
		initialized = true;
	}

	async function setPreference(type: NotificationType, enabled: boolean) {
		const previousValue = get({ subscribe })[type];
		update((prefs) => ({ ...prefs, [type]: enabled }));

		try {
			const currentPrefs = get({ subscribe });
			const response = await fetchApi('/users/preferences', {
				method: 'PUT',
				body: JSON.stringify({
					preferences: { notifications: currentPrefs }
				})
			});

			if (!response.ok) {
				update((prefs) => ({ ...prefs, [type]: previousValue }));
				console.error('Failed to save notification preference');
			}
		} catch (error) {
			update((prefs) => ({ ...prefs, [type]: previousValue }));
			console.error('Failed to save notification preference:', error);
		}
	}

	return {
		subscribe,
		initialize,
		setPreference,
		reset: () => {
			set(defaultPreferences);
			initialized = false;
		}
	};
}

export const notificationPreferences = createNotificationPreferencesStore();
