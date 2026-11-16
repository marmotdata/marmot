import { writable, get } from 'svelte/store';
import { fetchApi } from '#lib/api.ts';

/** How a field is entered and shown. Table cells may only be text, long text, select or URL. */
export type FieldType = 'text' | 'long_text' | 'date' | 'select' | 'url' | 'list' | 'table';

/** What a status means to someone relying on an entry: not yet binding, binding, or no longer binding. */
export type KnowledgeState = 'pending' | 'current' | 'historical';

export interface KnowledgeOption {
	value: string;
	label: string;
}

export interface KnowledgeField {
	key: string;
	label: string;
	help?: string;
	placeholder?: string;
	type: FieldType;
	required?: boolean;
	required_when?: string[];
	max_rows?: number;
	options?: KnowledgeOption[];
	columns?: KnowledgeField[];
}

export interface KnowledgeStatus {
	value: string;
	label: string;
	state: KnowledgeState;
}

/** A kind of knowledge entry, served by the API so the server, the UI and assistants share one definition. An entry is an asset of the kind's type. */
export interface KnowledgeKind {
	id: string;
	type: string;
	label: string;
	description: string;
	icon: string;
	statuses: KnowledgeStatus[];
	default_status: string;
	fields: KnowledgeField[];
	template?: string;
}

/** The provider every entry written in Marmot carries, since an asset needs one and the entry lives nowhere else. */
export const KNOWLEDGE_PROVIDER = 'Marmot';

/** The kinds, empty until loadKinds resolves. */
export const knowledgeKinds = writable<KnowledgeKind[]>([]);

let pending: Promise<KnowledgeKind[]> | null = null;

/** Loads the kinds once per session; later calls share the first request. */
export function loadKinds(): Promise<KnowledgeKind[]> {
	if (!pending) {
		pending = fetchApi('/knowledge/kinds')
			.then(async (res) => {
				if (!res.ok) throw new Error(`kinds: ${res.status}`);
				const data: { kinds?: KnowledgeKind[] } = await res.json();
				const kinds = data.kinds ?? [];
				knowledgeKinds.set(kinds);
				return kinds;
			})
			.catch((err) => {
				pending = null;
				throw err;
			});
	}
	return pending;
}

/** The kind an asset type belongs to, ignoring case, or undefined for a type that is not a knowledge entry or before the kinds have loaded. */
export function kindOfType(
	type: string | null | undefined,
	kinds: KnowledgeKind[] = get(knowledgeKinds)
): KnowledgeKind | undefined {
	const t = (type ?? '').toLowerCase();
	return t ? kinds.find((k) => k.type.toLowerCase() === t) : undefined;
}

/** The kind with the given key, as read from a URL. */
export function kindById(
	id: string | null | undefined,
	kinds: KnowledgeKind[] = get(knowledgeKinds)
): KnowledgeKind | undefined {
	return id ? kinds.find((k) => k.id === id) : undefined;
}

/** The status with the given value, for its label and state. */
export function statusOf(
	kind: KnowledgeKind | undefined,
	value: string
): KnowledgeStatus | undefined {
	return kind?.statuses.find((s) => s.value === value);
}

/** Whether a field must be filled in, given the status the entry will have. */
export function isRequired(field: KnowledgeField, status: string): boolean {
	return !!field.required || !!field.required_when?.includes(status);
}
