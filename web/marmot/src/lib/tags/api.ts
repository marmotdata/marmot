import { fetchApi } from '../api';
import type { Tag } from './types';

export async function listTags(): Promise<Tag[]> {
	const res = await fetchApi('/tags');
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to fetch tags');
	}
	return res.json();
}

export async function createTag(input: { name: string; description?: string }): Promise<Tag> {
	const res = await fetchApi('/tags', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to create tag');
	}
	return res.json();
}

export async function updateTag(
	id: string,
	input: { name?: string; description?: string }
): Promise<Tag> {
	const res = await fetchApi(`/tags/${id}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to update tag');
	}
	return res.json();
}

export async function deleteTag(id: string): Promise<void> {
	const res = await fetchApi(`/tags/${id}`, { method: 'DELETE' });
	if (!res.ok) {
		const e = await res.json();
		throw new Error(e.error || 'Failed to delete tag');
	}
}
