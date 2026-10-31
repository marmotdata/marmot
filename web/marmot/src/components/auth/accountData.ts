import { writable, get } from 'svelte/store';
import { fetchApi } from '$lib/api';
import { handleApiError } from '$lib/stores/toast';
import { m } from '$lib/paraglide/messages';

export interface ApiKey {
	id: string;
	name: string;
	key?: string;
	/** The start of the key kept in clear; absent for keys made before it was stored */
	prefix?: string;
	created_at: string;
	expires_at?: string | null;
	last_used_at?: string | null;
}

export interface SubscriptionWithAsset {
	id: string;
	asset_id: string;
	user_id: string;
	notification_types: string[];
	asset_name: string;
	asset_mrn: string;
	asset_type: string;
	created_at: string;
}

export interface ScopeDescription {
	scope: string;
	summary: string;
}

export interface ConnectedApp {
	client_id: string;
	client_name?: string;
	redirect_uri?: string;
	scopes: string[];
	scope_descriptions: ScopeDescription[];
	created_at: string;
	last_authorised_at: string;
	expires_at: string;
}

export interface Loaded<T> {
	items: T[] | null;
	error: string;
}

// The account layout shows each list's size on its tab, so the lists are loaded once here and shared with the pages that show them
export const apiKeys = writable<Loaded<ApiKey>>({ items: null, error: '' });
export const subscriptions = writable<Loaded<SubscriptionWithAsset>>({ items: null, error: '' });
export const connectedApps = writable<Loaded<ConnectedApp>>({ items: null, error: '' });

let keysRequest: Promise<void> | null = null;
let subsRequest: Promise<void> | null = null;

export function loadApiKeys(force = false): Promise<void> {
	if (!force && (keysRequest || get(apiKeys).items)) return keysRequest ?? Promise.resolve();
	apiKeys.update((s) => ({ ...s, error: '' }));
	keysRequest = (async () => {
		try {
			const response = await fetchApi('/users/apikeys');
			if (!response.ok) {
				apiKeys.set({ items: null, error: await handleApiError(response) });
				return;
			}
			const data = await response.json();
			apiKeys.set({ items: data ?? [], error: '' });
		} catch {
			apiKeys.set({ items: null, error: m.apikeys_fetch_error() });
		} finally {
			keysRequest = null;
		}
	})();
	return keysRequest;
}

export function loadSubscriptions(force = false): Promise<void> {
	if (!force && (subsRequest || get(subscriptions).items)) return subsRequest ?? Promise.resolve();
	subscriptions.update((s) => ({ ...s, error: '' }));
	subsRequest = (async () => {
		try {
			const response = await fetchApi('/subscriptions/list');
			if (!response.ok) {
				subscriptions.set({ items: null, error: await handleApiError(response) });
				return;
			}
			const data = await response.json();
			subscriptions.set({ items: data.subscriptions ?? [], error: '' });
		} catch {
			subscriptions.set({ items: null, error: m.subscriptions_load_error() });
		} finally {
			subsRequest = null;
		}
	})();
	return subsRequest;
}

let appsRequest: Promise<void> | null = null;

export function loadConnectedApps(force = false): Promise<void> {
	if (!force && (appsRequest || get(connectedApps).items)) return appsRequest ?? Promise.resolve();
	connectedApps.update((s) => ({ ...s, error: '' }));
	appsRequest = (async () => {
		try {
			const response = await fetchApi('/users/me/connected-apps');
			if (!response.ok) {
				connectedApps.set({ items: null, error: await handleApiError(response) });
				return;
			}
			const data = await response.json();
			connectedApps.set({ items: data.apps ?? [], error: '' });
		} catch {
			connectedApps.set({ items: null, error: m.connected_apps_load_error() });
		} finally {
			appsRequest = null;
		}
	})();
	return appsRequest;
}
