import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Admin became Settings. Each old tab maps onto its section, and ?create=1 is carried so old "new user" links still open the blade.
const sections: Record<string, string> = {
	users: '/settings/members',
	teams: '/settings/teams',
	roles: '/settings/roles',
	service_accounts: '/settings/service-accounts',
	federation: '/settings/authentication',
	system: '/settings/system'
};

export function load({ url }: { url: URL }) {
	const target = sections[url.searchParams.get('tab') ?? ''] ?? '/settings';
	const create = url.searchParams.get('create') === '1' ? '?create=1' : '';
	redirect(308, resolve(target as '/settings') + create);
}
