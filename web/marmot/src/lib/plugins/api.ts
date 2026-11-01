import { fetchApi } from '$lib/api';
import { parseApiError } from '$lib/stores/toast';
import type { PluginUpdatesStatus } from './types';

// Thrown when the server has no update manager wired up (503 on every route below), so the page
// can show a calm "not available here" state instead of an error.
export class PluginUpdatesUnavailableError extends Error {}

async function failure(res: Response): Promise<Error> {
	return new Error((await parseApiError(res)).message);
}

/** Every core plugin's running version, available updates and advisories. Needs ingestion:view. */
export async function getPluginUpdatesStatus(): Promise<PluginUpdatesStatus> {
	const res = await fetchApi('/plugins/versions');
	if (res.status === 503)
		throw new PluginUpdatesUnavailableError((await parseApiError(res)).message);
	if (!res.ok) throw await failure(res);
	return res.json();
}

/** Checks the plugin index for new versions and advisories now. Resolves to whether it found a newer index. */
export async function checkPluginUpdates(): Promise<{ updated: boolean }> {
	const res = await fetchApi('/plugins/versions/check', { method: 'POST' });
	if (!res.ok) throw await failure(res);
	return res.json();
}

/** Moves a core plugin to a version from the plugin index. */
export async function setPluginVersion(id: string, version: string): Promise<void> {
	const res = await fetchApi(`/plugins/versions/${encodeURIComponent(id)}`, {
		method: 'PUT',
		body: JSON.stringify({ version })
	});
	if (!res.ok) throw await failure(res);
}

/** Returns a core plugin to the version this Marmot release pins. */
export async function resetPluginVersion(id: string): Promise<void> {
	const res = await fetchApi(`/plugins/versions/${encodeURIComponent(id)}`, { method: 'DELETE' });
	if (!res.ok) throw await failure(res);
}

/** Pins or unpins a core plugin at the version it runs. */
export async function setPluginPinned(id: string, pinned: boolean): Promise<void> {
	const res = await fetchApi(`/plugins/versions/${encodeURIComponent(id)}/pin`, {
		method: 'PUT',
		body: JSON.stringify({ pinned })
	});
	if (!res.ok) throw await failure(res);
}

/** Rolls a core plugin back to the version it ran before its last change, and pins it there. */
export async function rollbackPluginVersion(id: string): Promise<void> {
	const res = await fetchApi(`/plugins/versions/${encodeURIComponent(id)}/rollback`, {
		method: 'POST'
	});
	if (!res.ok) throw await failure(res);
}
