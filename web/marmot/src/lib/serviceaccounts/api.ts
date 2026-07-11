import { fetchApi } from '$lib/api';
import type {
	ServiceAccount,
	ServiceAccountAPIKey,
	CreateServiceAccountInput,
	UpdateServiceAccountInput,
	CreateAPIKeyInput
} from './types';

export async function listServiceAccounts(): Promise<ServiceAccount[]> {
	const res = await fetchApi('/service-accounts');
	if (!res.ok) throw new Error('Failed to list service accounts');
	return res.json();
}

export async function getServiceAccount(id: string): Promise<ServiceAccount> {
	const res = await fetchApi(`/service-accounts/${id}`);
	if (!res.ok) throw new Error('Failed to get service account');
	return res.json();
}

export async function createServiceAccount(
	input: CreateServiceAccountInput
): Promise<ServiceAccount> {
	const res = await fetchApi('/service-accounts', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to create service account');
	}
	return res.json();
}

export async function updateServiceAccount(
	id: string,
	input: UpdateServiceAccountInput
): Promise<ServiceAccount> {
	const res = await fetchApi(`/service-accounts/${id}`, {
		method: 'PATCH',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to update service account');
	}
	return res.json();
}

export async function deleteServiceAccount(id: string): Promise<void> {
	const res = await fetchApi(`/service-accounts/${id}`, { method: 'DELETE' });
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to delete service account');
	}
}

export async function listAPIKeys(saID: string): Promise<ServiceAccountAPIKey[]> {
	const res = await fetchApi(`/service-accounts/${saID}/api-keys`);
	if (!res.ok) throw new Error('Failed to list API keys');
	return res.json();
}

export async function createAPIKey(
	saID: string,
	input: CreateAPIKeyInput
): Promise<ServiceAccountAPIKey> {
	const res = await fetchApi(`/service-accounts/${saID}/api-keys`, {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to create API key');
	}
	return res.json();
}

export async function deleteAPIKey(saID: string, keyID: string): Promise<void> {
	const res = await fetchApi(`/service-accounts/${saID}/api-keys/${keyID}`, { method: 'DELETE' });
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to delete API key');
	}
}
