import { m } from '#lib/paraglide/messages.js';
import { isRequired, type KnowledgeField, type KnowledgeKind } from './kinds.ts';
import type {
	FieldRow,
	FieldValue,
	KnowledgeAssetRef,
	KnowledgeEntry,
	KnowledgeLink,
	KnowledgeRule,
	SaveEntryInput
} from './api.ts';

/** An entry being written or edited: what the details form holds before it is saved. */
export interface EntryDraft {
	status: string;
	fields: Record<string, FieldValue>;
	effective_from: string;
	review_by: string;
	supersedes: KnowledgeAssetRef[];
	/** Null until every link is loaded, and while there are too many to edit one by one; then links are left as they are. */
	links: KnowledgeLink[] | null;
	rules: KnowledgeRule[];
}

export function newDraft(kind: KnowledgeKind): EntryDraft {
	return {
		status: kind.default_status,
		fields: {},
		effective_from: '',
		review_by: '',
		supersedes: [],
		links: [],
		rules: []
	};
}

export function draftFrom(kind: KnowledgeKind, e: KnowledgeEntry): EntryDraft {
	return {
		status: e.status || kind.default_status,
		fields: structuredClone(e.fields ?? {}),
		effective_from: e.effective_from ?? '',
		review_by: e.review_by ?? '',
		supersedes: e.supersedes ? [e.supersedes] : [],
		links: e.link_count <= e.links.length ? structuredClone(e.links) : null,
		rules: structuredClone(e.rules ?? [])
	};
}

export function toInput(d: EntryDraft): SaveEntryInput {
	return {
		status: d.status,
		fields: d.fields,
		effective_from: d.effective_from || undefined,
		review_by: d.review_by || undefined,
		supersedes: d.supersedes[0]?.id,
		links: d.links?.map((l) => ({ relation: l.relation, target_id: l.target.id })),
		rules: d.rules
			.filter((r) => r.selector_value.trim())
			.map(({ relation, selector_type, selector_value }) => ({
				relation,
				selector_type,
				selector_value: selector_value.trim()
			}))
	};
}

/** Whether a field holds anything worth saving, the way the server decides: blank text, empty lines and empty rows do not count. */
export function hasValue(v: FieldValue | undefined): boolean {
	if (typeof v === 'string') return v.trim() !== '';
	if (!Array.isArray(v)) return false;
	return (v as unknown[]).some((item) =>
		typeof item === 'string'
			? item.trim() !== ''
			: Object.values(item as FieldRow).some((c) => typeof c === 'string' && c.trim() !== '')
	);
}

/** Errors keyed by field, for the fields the chosen status makes required. */
export function draftErrors(kind: KnowledgeKind, d: EntryDraft): Record<string, string> {
	const errors: Record<string, string> = {};
	for (const f of kind.fields) {
		if (isRequired(f, d.status) && !hasValue(d.fields[f.key])) {
			errors[f.key] = m.knowledge_error_field_required({ field: f.label });
		}
	}
	return errors;
}

/** Fields shown before anyone asks for more: the required ones and any already filled in. */
export function visibleFields(
	kind: KnowledgeKind,
	d: EntryDraft,
	opened: Set<string>
): KnowledgeField[] {
	return kind.fields.filter(
		(f) => isRequired(f, d.status) || hasValue(d.fields[f.key]) || opened.has(f.key)
	);
}
