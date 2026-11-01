import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Service accounts moved into Settings
export function load() {
	redirect(308, resolve('/settings/service-accounts'));
}
