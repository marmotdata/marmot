import { fetchApi } from '#lib/api.ts';
import { parseApiError } from '#lib/stores/toast.ts';
import type { KnowledgeState } from './kinds.ts';

export interface KnowledgeAssetRef {
	id: string;
	name: string;
	type: string;
	mrn: string;
	status?: string;
}

export interface KnowledgeOwner {
	id: string;
	name: string;
	type: 'user' | 'team';
}

/** An asset's knowledge record. Field values are strings, string lists or rows of strings, by field type. */
export interface KnowledgeEntry {
	asset_id: string;
	kind: string;
	status: string;
	state?: KnowledgeState;
	fields: Record<string, FieldValue>;
	effective_from?: string;
	review_by?: string;
	review_due: boolean;
	last_reviewed_at?: string;
	last_reviewed_by?: string;
	supersedes?: KnowledgeAssetRef;
	superseded_by: KnowledgeAssetRef[];
	applies_to: KnowledgeAssetRef[];
	updated_by?: string;
	updated_at?: string;
}

export type FieldRow = Record<string, string>;
export type FieldValue = string | string[] | FieldRow[];

export interface KnowledgeSummary {
	asset_id: string;
	name: string;
	summary?: string;
	mrn: string;
	kind: string;
	status: string;
	state?: KnowledgeState;
	effective_from?: string;
	review_by?: string;
	review_due: boolean;
	owners: KnowledgeOwner[];
	updated_at: string;
}

export interface KnowledgeCounts {
	total: number;
	by_kind: Record<string, number>;
	by_state: Partial<Record<KnowledgeState, number>>;
	review_due: number;
	unset: number;
}

export interface SaveEntryInput {
	status: string;
	fields: Record<string, FieldValue>;
	effective_from?: string;
	review_by?: string;
	supersedes?: string;
	applies_to?: string[];
}

export interface ListParams {
	kinds?: string[];
	statuses?: string[];
	q?: string;
	reviewDue?: boolean;
	mine?: boolean;
	sort?: 'updated' | 'name' | 'review';
	limit?: number;
	offset?: number;
}

async function json<T>(res: Response): Promise<T> {
	if (!res.ok) {
		const info = await parseApiError(res);
		throw Object.assign(new Error(info.message), { status: res.status });
	}
	return res.json();
}

export async function listEntries(
	p: ListParams,
	signal?: AbortSignal
): Promise<{ entries: KnowledgeSummary[]; total: number }> {
	const q = new URLSearchParams();
	if (p.kinds?.length) q.set('kind', p.kinds.join(','));
	if (p.statuses?.length) q.set('status', p.statuses.join(','));
	if (p.q) q.set('q', p.q);
	if (p.reviewDue) q.set('review_due', 'true');
	if (p.mine) q.set('mine', 'true');
	if (p.sort) q.set('sort', p.sort);
	q.set('limit', String(p.limit ?? 25));
	q.set('offset', String(p.offset ?? 0));
	return json(await fetchApi(`/knowledge/entries?${q}`, { signal }));
}

export async function getCounts(): Promise<KnowledgeCounts> {
	return json(await fetchApi('/knowledge/counts'));
}

export async function getEntry(assetId: string): Promise<KnowledgeEntry> {
	return json(await fetchApi(`/knowledge/entries/${encodeURIComponent(assetId)}`));
}

export async function saveEntry(assetId: string, input: SaveEntryInput): Promise<KnowledgeEntry> {
	return json(
		await fetchApi(`/knowledge/entries/${encodeURIComponent(assetId)}`, {
			method: 'PUT',
			body: JSON.stringify(input)
		})
	);
}

export async function markReviewed(assetId: string, reviewBy?: string): Promise<KnowledgeEntry> {
	return json(
		await fetchApi(`/knowledge/entries/${encodeURIComponent(assetId)}/reviewed`, {
			method: 'POST',
			body: JSON.stringify(reviewBy ? { review_by: reviewBy } : {})
		})
	);
}

export async function entriesApplyingTo(assetId: string): Promise<KnowledgeSummary[]> {
	const data = await json<{ entries?: KnowledgeSummary[] }>(
		await fetchApi(`/knowledge/applies-to/${encodeURIComponent(assetId)}`)
	);
	return data.entries ?? [];
}

/** A date as YYYY-MM-DD, a number of months from today, for review presets. */
export function monthsFromToday(months: number): string {
	const d = new Date();
	d.setMonth(d.getMonth() + months);
	return d.toISOString().slice(0, 10);
}
