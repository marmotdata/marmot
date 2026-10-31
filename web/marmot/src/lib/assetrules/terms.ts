import { getTerm } from '$lib/glossary/api';
import type { GlossaryTerm } from '$lib/glossary/types';

// Rules store glossary term IDs only, so names are fetched once per term and shared across every rule shown on the page.
const cache = new Map<string, Promise<GlossaryTerm | null>>();

function load(id: string): Promise<GlossaryTerm | null> {
	let p = cache.get(id);
	if (!p) {
		p = getTerm(id).catch(() => null);
		cache.set(id, p);
	}
	return p;
}

/** The glossary terms for these IDs, in order, leaving out any that have been deleted. */
export async function loadTerms(ids: string[] | undefined): Promise<GlossaryTerm[]> {
	const terms = await Promise.all((ids ?? []).map(load));
	return terms.filter((t): t is GlossaryTerm => t !== null);
}
