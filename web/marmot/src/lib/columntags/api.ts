import { fetchApi } from '../api';
import type { ColumnTagMap } from './types';

export async function listColumnTags(assetId: string): Promise<ColumnTagMap> {
	const res = await fetchApi(`/assets/column-tags/${assetId}`);
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to fetch column tags');
	}
	return res.json();
}

export async function replaceColumnTags(
	assetId: string,
	columnPath: string,
	tagIds: string[]
): Promise<void> {
	const res = await fetchApi(`/assets/column-tags/${assetId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ column_path: columnPath, tag_ids: tagIds })
	});
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to update column tags');
	}
}

export async function removeColumnTag(
	assetId: string,
	columnPath: string,
	tagId: string
): Promise<void> {
	const res = await fetchApi(`/assets/column-tags/${assetId}`, {
		method: 'DELETE',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ column_path: columnPath, tag_id: tagId })
	});
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to delete column tag');
	}
}
