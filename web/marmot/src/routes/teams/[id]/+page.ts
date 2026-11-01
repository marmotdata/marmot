import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Team pages moved into Settings, so links in notifications and docs still land
export function load({ params, url }: { params: { id: string }; url: URL }) {
	redirect(308, resolve('/settings/teams/[id]', { id: params.id }) + url.search);
}
