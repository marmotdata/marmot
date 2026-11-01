import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Members moved into Settings; the old address opens the create blade there
export function load() {
	redirect(308, resolve('/settings/members') + '?create=1');
}
