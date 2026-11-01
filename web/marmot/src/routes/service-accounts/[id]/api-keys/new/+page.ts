import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Keys are created in a blade on the service account page
export function load({ params }: { params: { id: string } }) {
	redirect(308, resolve('/settings/service-accounts/[id]', { id: params.id }) + '?create=1');
}
