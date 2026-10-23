import { fetchApi } from '$lib/api';
import { documentErrorFrom } from '$lib/document/errors';
import type { Schema } from '$lib/document/codemirror/schema';
import type {
	Role,
	Permission,
	CreateRoleInput,
	UpdateRoleInput,
	ReplacePermissionsInput,
	RoleValidation
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
	if (!res.ok) throw await documentErrorFrom(res, 'Failed to create role');
	return res.json();
}

export async function updateRole(id: string, input: UpdateRoleInput): Promise<Role> {
	const res = await fetchApi(`/roles/${id}`, {
		method: 'PATCH',
		body: JSON.stringify(input)
	});
	if (!res.ok) throw await documentErrorFrom(res, 'Failed to update role');
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
	if (!res.ok) throw await documentErrorFrom(res, 'Failed to update permissions');
	return res.json();
}

export async function listPermissions(): Promise<Permission[]> {
	const res = await fetchApi('/permissions');
	if (!res.ok) throw new Error('Failed to list permissions');
	return res.json();
}

/**
 * Checks a role document without writing it. Pass the id of the role being edited so its own name does not count as taken. A document the API cannot read at all, such as one with an unknown field, comes back as a single problem with no path.
 */
export async function validateRole(input: unknown, id?: string): Promise<RoleValidation> {
	const query = id ? `?id=${encodeURIComponent(id)}` : '';
	const res = await fetchApi(`/roles/validate${query}`, {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (res.ok) return res.json();
	const error = await documentErrorFrom(res, 'Failed to check the role');
	if (res.status === 400) {
		return {
			valid: false,
			problems: error.fields.length ? error.fields : [{ path: '', message: error.message }]
		};
	}
	throw error;
}

let roleSchema: Promise<Schema | null> | null = null;

/** The JSON Schema of a role document, read once per page load. */
export function getRoleSchema(): Promise<Schema | null> {
	roleSchema ??= fetchApi('/roles/schema')
		.then((res) => (res.ok ? (res.json() as Promise<Schema>) : null))
		.catch(() => null)
		.then((schema) => {
			if (!schema) roleSchema = null;
			return schema;
		});
	return roleSchema;
}
