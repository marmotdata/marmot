// A role as an editable document, {name, description, permission_keys}, and the conversions between it and the visual form, which works in permission ids.

import type { Permission, Role } from './types';

export interface RoleDocument {
	name: string;
	description?: string;
	permission_keys: string[];
}

/** The key GET /permissions spells a permission with on this instance. */
export function permissionKey(permission: Pick<Permission, 'resource_type' | 'action'>): string {
	return `${permission.resource_type}:${permission.action}`;
}

export function documentFromForm(
	name: string,
	description: string,
	permissionIds: string[],
	permissions: Permission[]
): RoleDocument {
	const byId = new Map(permissions.map((permission) => [permission.id, permission]));
	const keys = permissionIds
		.map((id) => byId.get(id))
		.filter((permission): permission is Permission => !!permission)
		.map(permissionKey)
		.sort();
	return {
		name,
		...(description ? { description } : {}),
		permission_keys: keys
	};
}

export function documentFromRole(role: Role): RoleDocument {
	const keys = role.permission_keys ?? (role.permissions ?? []).map(permissionKey);
	return {
		name: role.name,
		...(role.description ? { description: role.description } : {}),
		permission_keys: [...keys].sort()
	};
}

/** The ids of the given keys, and the keys no permission has. */
export function idsFromKeys(
	keys: string[],
	permissions: Permission[]
): { ids: string[]; unknown: string[] } {
	const byKey = new Map(
		permissions.map((permission) => [permissionKey(permission), permission.id])
	);
	const ids: string[] = [];
	const unknown: string[] = [];
	for (const key of keys) {
		const id = byKey.get(key);
		if (id) ids.push(id);
		else unknown.push(key);
	}
	return { ids, unknown };
}

/** Reads a parsed document that has passed the schema into a RoleDocument. */
export function readRoleDocument(value: unknown): RoleDocument {
	const record = (value ?? {}) as Record<string, unknown>;
	return {
		name: typeof record.name === 'string' ? record.name : '',
		...(typeof record.description === 'string' ? { description: record.description } : {}),
		permission_keys: Array.isArray(record.permission_keys)
			? record.permission_keys.filter((key): key is string => typeof key === 'string')
			: []
	};
}
