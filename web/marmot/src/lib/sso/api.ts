import { fetchApi } from '$lib/api';
import { handleApiError, parseApiError } from '$lib/stores/toast';
import type { GroupMapping, SignInSettings, SSOProvider, SSOProviderStatus } from './types';

export async function listSSOProviders(): Promise<SSOProvider[]> {
	const res = await fetchApi('/sso-providers');
	if (!res.ok) throw new Error(await handleApiError(res));
	const data = await res.json();
	return data.providers ?? [];
}

async function failure(res: Response): Promise<Error> {
	return new Error((await parseApiError(res)).message);
}

/** Every configured provider with whether it is on and how many members use it. Needs sso:manage. */
export async function listProviderStatuses(): Promise<SSOProviderStatus[]> {
	const res = await fetchApi('/admin/auth/providers');
	if (!res.ok) throw await failure(res);
	return (await res.json()).providers ?? [];
}

export async function setProviderEnabled(
	type: string,
	enabled: boolean
): Promise<SSOProviderStatus> {
	const res = await fetchApi(`/admin/auth/providers/${encodeURIComponent(type)}`, {
		method: 'PUT',
		body: JSON.stringify({ enabled })
	});
	if (!res.ok) throw await failure(res);
	return res.json();
}

export async function getSignInSettings(): Promise<SignInSettings> {
	const res = await fetchApi('/admin/auth/settings');
	if (!res.ok) throw await failure(res);
	return res.json();
}

export async function saveSignInSettings(input: SignInSettings): Promise<SignInSettings> {
	const res = await fetchApi('/admin/auth/settings', {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!res.ok) throw await failure(res);
	return res.json();
}

export async function listGroupMappings(): Promise<GroupMapping[]> {
	const res = await fetchApi('/sso/team-mappings');
	if (!res.ok) throw await failure(res);
	return (await res.json()).mappings ?? [];
}

export async function createGroupMapping(input: {
	provider: string;
	sso_group_name: string;
	team_id?: string;
	role_id?: string;
}): Promise<GroupMapping> {
	const res = await fetchApi('/sso/team-mappings', { method: 'POST', body: JSON.stringify(input) });
	if (!res.ok) throw await failure(res);
	return res.json();
}

export async function deleteGroupMapping(id: string): Promise<void> {
	const res = await fetchApi(`/sso/team-mappings/${id}`, { method: 'DELETE' });
	if (!res.ok) throw await failure(res);
}
