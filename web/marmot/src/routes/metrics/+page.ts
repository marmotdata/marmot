import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Catalog metrics moved into Insights, so old links and bookmarks still land there
export function load() {
	redirect(308, resolve('/insights'));
}
