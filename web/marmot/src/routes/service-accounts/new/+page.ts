import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Service accounts moved into Settings; the old address opens the create blade there
export function load() {
	redirect(308, resolve('/settings/service-accounts') + '?create=1');
}
