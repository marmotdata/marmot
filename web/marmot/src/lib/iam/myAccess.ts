import { writable, derived, get } from 'svelte/store';
import { browser } from '$app/environment';
import { auth } from '$lib/stores/auth';

export interface MyAccess {
	/** Whether the token has been read; until then nothing is held. */
	checked: boolean;
	/** The caller is an administrator, so every check passes. */
	admin: boolean;
	/** Permissions held over the whole catalog, read from the JWT's role permissions. */
	root: Set<string>;
	/** Permissions held anywhere, which without scoped grants is the same set as root. */
	anywhere: Set<string>;
}

const empty = (): MyAccess => ({
	checked: false,
	admin: false,
	root: new Set(),
	anywhere: new Set()
});

// Permissions come only from role assignments, so the JWT is the whole answer and there is nothing to fetch.
export const myAccess = writable<MyAccess>(empty());

let loadedFor: string | null = null;

/** Reads the caller's permissions from the token; pass force to re-read after the token changes. */
export function loadMyAccess(force = false): Promise<MyAccess> {
	if (!browser) return Promise.resolve(empty());
	const token = auth.getToken();
	if (!token) {
		loadedFor = null;
		myAccess.set({ ...empty(), checked: true });
		return Promise.resolve(get(myAccess));
	}
	if (!force && loadedFor === token) return Promise.resolve(get(myAccess));
	loadedFor = token;
	const perms = new Set<string>(auth.getPayload()?.permissions ?? []);
	const result: MyAccess = {
		checked: true,
		admin: auth.hasRole('admin'),
		root: perms,
		anywhere: new Set(perms)
	};
	myAccess.set(result);
	return Promise.resolve(result);
}

/** Whether the caller holds a permission over the whole catalog. */
export function holdsAtRoot(access: MyAccess, permission: string): boolean {
	return access.admin || access.root.has(permission);
}

/** Whether the caller holds a permission anywhere, which without scoped grants equals holdsAtRoot. */
export function holdsAnywhere(access: MyAccess, permission: string): boolean {
	return holdsAtRoot(access, permission);
}

/** Reactive form of holdsAtRoot, for templates. */
export const canAtRoot = derived(
	myAccess,
	($a) => (permission: string) => holdsAtRoot($a, permission)
);
