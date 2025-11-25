import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url }) => {
	// Preserve query parameters when redirecting
	const searchParams = url.searchParams.toString();
	const redirectUrl = searchParams ? `/discover?${searchParams}` : '/discover';
	throw redirect(307, redirectUrl);
};
