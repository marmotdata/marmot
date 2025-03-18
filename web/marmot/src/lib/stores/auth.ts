import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { theme } from '$lib/stores/theme';

function createAuthStore() {
	const initialToken = browser ? localStorage.getItem('jwt') : null;
	const { subscribe, set } = writable<string | null>(initialToken);

	return {
		subscribe,
		setToken: (token: string) => {
			if (browser) {
				localStorage.setItem('jwt', token);
				set(token);

				// Extract and set theme from JWT
				const payload = parseJwt(token);
				if (payload && payload.preferences && payload.preferences.theme) {
					theme.set(payload.preferences.theme);
				}
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
		}
	};
}

// Typescript interface for JWT payload
interface JwtPayload {
	roles?: string[];
	preferences?: {
		theme?: string;
	};
	[key: string]: any;
}

// Helper function to decode JWT payload (without verification)
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
