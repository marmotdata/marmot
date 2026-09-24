import { fetchApi } from '../api';
import { m } from '$lib/paraglide/messages';
import type { Memory, MemoryEntityType, MemoryList, MemorySearchResult, MemorySort } from './types';

function base(entityType: MemoryEntityType, entityId: string): string {
	return `/memory/${entityType}/${encodeURIComponent(entityId)}`;
}

async function errorMessage(response: Response, fallback: string): Promise<string> {
	try {
		const body = await response.json();
		return body.error || fallback;
	} catch {
		return fallback;
	}
}

export async function listMemory(
	entityType: MemoryEntityType,
	entityId: string,
	sort: MemorySort,
	limit = 50,
	offset = 0
): Promise<MemoryList> {
	const params = new URLSearchParams({ sort, limit: String(limit), offset: String(offset) });
	const response = await fetchApi(`${base(entityType, entityId)}?${params}`);
	if (!response.ok) throw new Error(await errorMessage(response, m.memory_error_load()));
	return response.json();
}

export async function searchMemory(
	entityType: MemoryEntityType,
	entityId: string,
	query: string
): Promise<MemorySearchResult> {
	const params = new URLSearchParams({ q: query, limit: '50' });
	const response = await fetchApi(`${base(entityType, entityId)}/search?${params}`);
	if (!response.ok) throw new Error(await errorMessage(response, m.memory_error_search()));
	return response.json();
}

export async function rememberMemory(
	entityType: MemoryEntityType,
	entityId: string,
	content: string
): Promise<Memory> {
	const response = await fetchApi(base(entityType, entityId), {
		method: 'POST',
		body: JSON.stringify({ content })
	});
	if (!response.ok) throw new Error(await errorMessage(response, m.memory_error_save()));
	return response.json();
}

export async function updateMemory(
	entityType: MemoryEntityType,
	entityId: string,
	memoryId: string,
	content: string
): Promise<Memory> {
	const response = await fetchApi(`${base(entityType, entityId)}/${memoryId}`, {
		method: 'PUT',
		body: JSON.stringify({ content })
	});
	if (!response.ok) throw new Error(await errorMessage(response, m.memory_error_save()));
	return response.json();
}

export async function forgetMemory(
	entityType: MemoryEntityType,
	entityId: string,
	memoryId: string
): Promise<void> {
	const response = await fetchApi(`${base(entityType, entityId)}/${memoryId}`, {
		method: 'DELETE'
	});
	if (!response.ok) throw new Error(await errorMessage(response, m.memory_error_delete()));
}
