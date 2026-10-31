import type { TreeNode } from '@marmotdata/elements/classes/tree.js';
import type { GlossaryTerm } from './types';

/** The terms that may become a term's parent: everything except the term itself and the terms beneath it. */
export function possibleParents(terms: GlossaryTerm[], termId?: string): GlossaryTerm[] {
	if (!termId) return terms;
	const excluded = new Set([termId]);
	let grew = true;
	while (grew) {
		grew = false;
		for (const t of terms) {
			if (t.parent_term_id && excluded.has(t.parent_term_id) && !excluded.has(t.id)) {
				excluded.add(t.id);
				grew = true;
			}
		}
	}
	return terms.filter((t) => !excluded.has(t.id));
}

/** Nests terms under their parents; a term whose parent is not in the list sits at the top level. */
export function termTree(terms: GlossaryTerm[]): TreeNode[] {
	const ids = new Set(terms.map((t) => t.id));
	const byParent = new Map<string, GlossaryTerm[]>();
	const roots: GlossaryTerm[] = [];
	for (const t of terms) {
		if (t.parent_term_id && ids.has(t.parent_term_id)) {
			const list = byParent.get(t.parent_term_id) ?? [];
			list.push(t);
			byParent.set(t.parent_term_id, list);
		} else {
			roots.push(t);
		}
	}
	const byName = (a: GlossaryTerm, b: GlossaryTerm) => a.name.localeCompare(b.name);
	const node = (t: GlossaryTerm, seen: Set<string>): TreeNode => {
		const kids = seen.has(t.id) ? [] : (byParent.get(t.id) ?? []).sort(byName);
		const next = new Set(seen).add(t.id);
		return {
			id: t.id,
			label: t.name,
			secondary: linkedCount(t),
			children: kids.length ? kids.map((k) => node(k, next)) : undefined
		};
	};
	return roots.sort(byName).map((t) => node(t, new Set()));
}

/** Ids of a term's ancestors, nearest first, so the tree can open the path to it. */
export function ancestorIds(terms: GlossaryTerm[], termId: string): string[] {
	const byId = new Map(terms.map((t) => [t.id, t]));
	const out: string[] = [];
	let current = byId.get(termId)?.parent_term_id;
	while (current && !out.includes(current)) {
		out.push(current);
		current = byId.get(current)?.parent_term_id;
	}
	return out;
}

/** Flat nodes for search results, where nesting would hide matches under collapsed parents. */
export function flatNodes(terms: GlossaryTerm[]): TreeNode[] {
	return terms.map((t) => ({ id: t.id, label: t.name, secondary: linkedCount(t) }));
}

/** The number of linked assets a tree row shows after the term's name. */
function linkedCount(t: GlossaryTerm): string {
	return String(t.linked_asset_count ?? 0);
}
