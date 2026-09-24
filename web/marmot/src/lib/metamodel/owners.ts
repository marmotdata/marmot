import { fetchApi } from '$lib/api';

export interface OwnerResult {
	id: string;
	name: string;
	username?: string;
	profile_picture?: string;
	type: 'user' | 'team';
}

/** Resolves a `presentation.control: user` field's stored id to a name, or null if not found. */
export async function lookupOwnerById(id: string): Promise<OwnerResult | null> {
	try {
		const response = await fetchApi(`/owners/search?q=${encodeURIComponent(id)}&limit=1`);
		const data = response.ok ? await response.json() : null;
		const hit: OwnerResult | undefined = data?.owners?.find(
			(o: OwnerResult) => o.id === id && o.type === 'user'
		);
		return hit ?? null;
	} catch {
		return null;
	}
}

export async function searchUsers(query: string, limit = 20): Promise<OwnerResult[]> {
	const response = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=${limit}`);
	const data = response.ok ? await response.json() : null;
	return ((data?.owners ?? []) as OwnerResult[]).filter((o) => o.type === 'user');
}
