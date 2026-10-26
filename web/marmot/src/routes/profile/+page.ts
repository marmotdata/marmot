import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Profile became Account, with one page per former tab.
const sections: Record<string, string> = {
	profile: '/account',
	subscriptions: '/account/subscriptions',
	'api-keys': '/account/api-keys',
	password: '/account/password',
	totp: '/account/totp'
};

export function load({ url }: { url: URL }) {
	redirect(308, resolve((sections[url.searchParams.get('tab') ?? ''] ?? '/account') as '/account'));
}
