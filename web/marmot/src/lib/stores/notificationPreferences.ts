import { writable, get } from 'svelte/store';
import { browser } from '$app/environment';
import { fetchApi } from '$lib/api';

export type NotificationType =
	| 'system'
	| 'schema_change'
	| 'asset_change'
	| 'mention'
	| 'job_complete';

export interface NotificationPreferences {
	system: boolean;
	schema_change: boolean;
	asset_change: boolean;
	mention: boolean;
	job_complete: boolean;
}

const defaultPreferences: NotificationPreferences = {
	system: true,
	schema_change: true,
	asset_change: true,
	mention: true,
	job_complete: true
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
