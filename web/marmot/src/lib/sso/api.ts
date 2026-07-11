import { fetchApi } from '$lib/api';
import { handleApiError } from '$lib/stores/toast';
import type { SSOProvider } from './types';

export async function listSSOProviders(): Promise<SSOProvider[]> {
	const res = await fetchApi('/sso-providers');
	if (!res.ok) throw new Error(await handleApiError(res));
	const data = await res.json();
	return data.providers ?? [];
}
