import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { theme } from '$lib/stores/theme';

export const isAnonymousMode = writable<boolean>(false);

function createAuthStore() {
	const initialToken = browser ? localStorage.getItem('jwt') : null;
	const { subscribe, set } = writable<string | null>(initialToken);

	return {
		subscribe,
		setToken: (token: string) => {
			if (browser) {
				localStorage.setItem('jwt', token);
				set(token);

				const payload = parseJwt(token);
				if (payload && payload.preferences && payload.preferences.theme) {
					theme.set(payload.preferences.theme);
				}

				isAnonymousMode.set(false);
			}
		},
		clearToken: () => {
			if (browser) {
				localStorage.removeItem('jwt');
				set(null);
			}
		},
		getToken: (): string | null => {
			if (browser) {
				return localStorage.getItem('jwt');
			}
			return null;
		},
		getPayload: (): JwtPayload | null => {
			if (browser) {
				const token = localStorage.getItem('jwt');
				if (token) {
					return parseJwt(token);
				}
			}
			return null;
		},
		hasRole: (role: string): boolean => {
			if (browser) {
				const payload = auth.getPayload();
				if (payload && payload.roles) {
					return payload.roles.includes(role);
				}
			}
			return false;
		},
		hasPermission: (resourceType: string, action: string): boolean => {
			if (browser) {
				const payload = auth.getPayload();
				if (payload && payload.permissions) {
					const permKey = `${resourceType}:${action}`;
					return payload.permissions.includes(permKey);
				}
			}
			return false;
		},
		isAuthenticated: (): boolean => {
			if (browser) {
				return localStorage.getItem('jwt') !== null;
			}
			return false;
		},
		checkAnonymousMode: async (): Promise<boolean> => {
			try {
				if (auth.getToken()) {
					isAnonymousMode.set(false);
					return false;
				}

				// Try to access a protected endpoint without authentication
				// TODO: this is very crude, we should consider adding a status endpoint
				const response = await fetch('/api/v1/assets/list');

				if (response.ok) {
					isAnonymousMode.set(true);
					return true;
				}

				isAnonymousMode.set(false);
				return false;
			} catch (error) {
				isAnonymousMode.set(false);
				return false;
			}
		},
		getCurrentUserId: (): string | null => {
			if (browser) {
				const payload = auth.getPayload();
				if (payload && payload.sub) {
					return payload.sub;
				}
			}
			return null;
		}
	};
}

interface JwtPayload {
	roles?: string[];
	permissions?: string[];
	preferences?: {
		theme?: string;
	};
	[key: string]: any;
}

function parseJwt(token: string): JwtPayload | null {
	try {
		const base64Url = token.split('.')[1];
		const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
		const jsonPayload = decodeURIComponent(
			atob(base64)
				.split('')
				.map(function (c) {
					return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
				})
				.join('')
		);
		return JSON.parse(jsonPayload);
	} catch (e) {
		console.error('Error parsing JWT:', e);
		return null;
	}
}

export const auth = createAuthStore();
