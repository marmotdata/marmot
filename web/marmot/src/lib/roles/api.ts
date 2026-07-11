import { fetchApi } from '$lib/api';
import type {
	Role,
	Permission,
	CreateRoleInput,
	UpdateRoleInput,
	ReplacePermissionsInput
} from './types';

export async function listRoles(): Promise<Role[]> {
	const res = await fetchApi('/roles');
	if (!res.ok) throw new Error('Failed to list roles');
	return res.json();
}

export async function getRole(id: string): Promise<Role> {
	const res = await fetchApi(`/roles/${id}`);
	if (!res.ok) throw new Error('Failed to get role');
	return res.json();
}

export async function createRole(input: CreateRoleInput): Promise<Role> {
	const res = await fetchApi('/roles', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to create role');
	}
	return res.json();
}

export async function updateRole(id: string, input: UpdateRoleInput): Promise<Role> {
	const res = await fetchApi(`/roles/${id}`, {
		method: 'PATCH',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to update role');
	}
	return res.json();
}

export async function deleteRole(id: string): Promise<void> {
	const res = await fetchApi(`/roles/${id}`, { method: 'DELETE' });
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to delete role');
	}
}

export async function replacePermissions(
	id: string,
	input: ReplacePermissionsInput
): Promise<Role> {
	const res = await fetchApi(`/roles/${id}/permissions`, {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to update permissions');
	}
	return res.json();
}

export async function listPermissions(): Promise<Permission[]> {
	const res = await fetchApi('/permissions');
	if (!res.ok) throw new Error('Failed to list permissions');
	return res.json();
}
