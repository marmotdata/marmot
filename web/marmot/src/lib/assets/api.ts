import { fetchApi } from '../api';
import type { Tag } from '$lib/tags/types';

export async function listAssetTags(assetId: string): Promise<Tag[]> {
	const res = await fetchApi(`/assets/tags/${assetId}`);
	if (!res.ok) {
		throw new Error('Failed to fetch asset tags');
	}
	return res.json();
}

export async function replaceAssetTags(assetId: string, tagIds: string[]): Promise<void> {
	const res = await fetchApi(`/assets/tags/${assetId}`, {
		method: 'PUT',
		body: JSON.stringify({ tag_ids: tagIds })
	});
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to update asset tags');
	}
}
