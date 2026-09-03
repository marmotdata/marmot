import { fetchApi } from '$lib/api';
import type {
	GatewayAuditEntry,
	GatewayGrant,
	GatewaySession,
	GatewayTarget,
	PluginInstanceStatus
} from './types';

async function json<T>(response: Response): Promise<T> {
	if (!response.ok) {
		const body = await response.json().catch(() => ({}));
		throw new Error(body.error || `Request failed (${response.status})`);
	}
	if (response.status === 204) return undefined as T;
	return response.json();
}

export async function listTargets(): Promise<GatewayTarget[]> {
	return json(await fetchApi('/gateway/targets')).then((t) => (t as GatewayTarget[]) ?? []);
}



export async function instanceStatus(): Promise<PluginInstanceStatus[]> {
	const data = await json<{ instances: PluginInstanceStatus[] }>(
		await fetchApi('/gateway/targets/status')
	);
	return data.instances ?? [];
}

export async function listGrants(
	principalType?: string,
	principalId?: string
): Promise<GatewayGrant[]> {
	const params = new URLSearchParams();
	if (principalType) params.set('principal_type', principalType);
	if (principalId) params.set('principal_id', principalId);
	const query = params.size > 0 ? `?${params}` : '';
	return json(await fetchApi(`/gateway/grants${query}`)).then(
		(g) => (g as GatewayGrant[]) ?? []
	);
}

export async function createGrant(input: {
	principal_type: string;
	principal_id: string;
	resource_selector: string;
	expires_at?: string;
	reason?: string;
}): Promise<GatewayGrant> {
	return json(await fetchApi('/gateway/grants', { method: 'POST', body: JSON.stringify(input) }));
}

export async function revokeGrant(id: string, reason?: string): Promise<void> {
	return json(
		await fetchApi(`/gateway/grants/${id}`, {
			method: 'DELETE',
			body: JSON.stringify({ reason: reason ?? '' })
		})
	);
}

export async function listSessions(limit = 50, offset = 0): Promise<GatewaySession[]> {
	return json(await fetchApi(`/gateway/sessions?limit=${limit}&offset=${offset}`)).then(
		(s) => (s as GatewaySession[]) ?? []
	);
}

export async function revokeSession(id: string): Promise<void> {
	return json(await fetchApi(`/gateway/sessions/${id}`, { method: 'DELETE' }));
}

export async function listAudit(filter: {
	principal_id?: string;
	session_id?: string;
	target?: string;
	decision?: string;
	limit?: number;
	offset?: number;
}): Promise<GatewayAuditEntry[]> {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(filter)) {
		if (value !== undefined && value !== '') params.set(key, String(value));
	}
	return json(await fetchApi(`/gateway/audit?${params}`)).then(
		(e) => (e as GatewayAuditEntry[]) ?? []
	);
}
