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
	/** The first links; link_count says how many there are in all. */
	links: KnowledgeLink[];
	link_count: number;
	rules: KnowledgeRule[];
	updated_by?: string;
	updated_at?: string;
}

export interface KnowledgeLink {
	relation: string;
	target: KnowledgeAssetRef;
}

export type SelectorType = 'tag' | 'type' | 'provider';

/** Links an entry to every asset with a tag, of a type or from a provider, matched when the asset is read. Matches counts the assets covered today, up to MATCH_LIMIT. */
export interface KnowledgeRule {
	id?: string;
	relation: string;
	selector_type: SelectorType;
	selector_value: string;
	matches?: number;
}

export const MATCH_LIMIT = 10000;

/** Where an entry about an asset came from. */
export type KnowledgeSource = 'direct' | 'rule' | 'inherited';

export interface KnowledgeAboutItem extends KnowledgeSummary {
	source: KnowledgeSource;
	relation: string;
	/** For an inherited entry, the containing asset it is linked to. */
	via?: KnowledgeAssetRef;
	/** For a matched entry, the rule that matched. */
	rule?: KnowledgeRule;
}

export interface KnowledgeAboutPage {
	entries: KnowledgeAboutItem[];
	next?: string;
	total: number;
	by_kind: Record<string, number>;
	by_source: Partial<Record<KnowledgeSource, number>>;
}

export interface KnowledgeLinkPage {
	links: KnowledgeLink[];
	next?: string;
	total: number;
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
	by_team: { id: string; name: string; count: number }[];
	review_due: number;
	unset: number;
}

export interface SaveEntryInput {
	status: string;
	fields: Record<string, FieldValue>;
	effective_from?: string;
	review_by?: string;
	supersedes?: string;
	/** Replaces the entry's links; leave out to keep them. */
	links?: { relation: string; target_id: string }[];
	/** Replaces the entry's rules; leave out to keep them. */
	rules?: KnowledgeRule[];
}

export interface ListParams {
	kinds?: string[];
	statuses?: string[];
	q?: string;
	reviewDue?: boolean;
	mine?: boolean;
	teams?: string[];
	sort?: 'updated' | 'name' | 'review' | 'relevance';
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
	if (p.teams?.length) q.set('team', p.teams.join(','));
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

export interface AboutParams {
	kinds?: string[];
	states?: KnowledgeState[];
	sources?: KnowledgeSource[];
	relation?: string;
	cursor?: string;
	limit?: number;
}

/** Entries about any asset: linked to it, matched by a rule, or linked to something that contains it. */
export async function knowledgeAbout(
	assetId: string,
	p: AboutParams = {},
	signal?: AbortSignal
): Promise<KnowledgeAboutPage> {
	const q = new URLSearchParams();
	if (p.kinds?.length) q.set('kind', p.kinds.join(','));
	if (p.states?.length) q.set('state', p.states.join(','));
	if (p.sources?.length) q.set('source', p.sources.join(','));
	if (p.relation) q.set('relation', p.relation);
	if (p.cursor) q.set('cursor', p.cursor);
	q.set('limit', String(p.limit ?? 25));
	return json(await fetchApi(`/knowledge/about/${encodeURIComponent(assetId)}?${q}`, { signal }));
}

export async function entryLinks(
	assetId: string,
	cursor = '',
	limit = 100
): Promise<KnowledgeLinkPage> {
	const q = new URLSearchParams({ limit: String(limit) });
	if (cursor) q.set('cursor', cursor);
	return json(await fetchApi(`/knowledge/entries/${encodeURIComponent(assetId)}/links?${q}`));
}

/** Every link of an entry, up to max, for editing. Returns null when there are more than max, since those are edited through rules instead. */
export async function allEntryLinks(assetId: string, max = 1000): Promise<KnowledgeLink[] | null> {
	const out: KnowledgeLink[] = [];
	let cursor = '';
	do {
		const page = await entryLinks(assetId, cursor, 100);
		if (page.total > max) return null;
		out.push(...page.links);
		cursor = page.next ?? '';
	} while (cursor);
	return out;
}

/** A date as YYYY-MM-DD, a number of months from today, for review presets. */
export function monthsFromToday(months: number): string {
	const d = new Date();
	d.setMonth(d.getMonth() + months);
	return d.toISOString().slice(0, 10);
}
