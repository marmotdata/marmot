import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Teams moved into Settings; the old address opens the create blade there
export function load() {
	redirect(308, resolve('/settings/teams') + '?create=1');
}
