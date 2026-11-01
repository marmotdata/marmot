import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Roles moved into Settings; the old address opens the create blade there
export function load() {
	redirect(308, resolve('/settings/roles') + '?create=1');
}
