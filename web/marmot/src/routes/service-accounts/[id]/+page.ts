import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Service account pages moved into Settings
export function load({ params, url }: { params: { id: string }; url: URL }) {
	redirect(308, resolve('/settings/service-accounts/[id]', { id: params.id }) + url.search);
}
