import { m } from '#lib/paraglide/messages.js';
import { MATCH_LIMIT, type KnowledgeRule } from './api.ts';

/** A rule in words, such as "Tagged pii". */
export function ruleLabel(r: Pick<KnowledgeRule, 'selector_type' | 'selector_value'>): string {
	switch (r.selector_type) {
		case 'tag':
			return m.knowledge_rule_tagged({ value: r.selector_value });
		case 'type':
			return m.knowledge_rule_of_type({ value: r.selector_value });
		default:
			return m.knowledge_rule_from_provider({ value: r.selector_value });
	}
}

/** How many assets a rule covers today, capped where the server stops counting. */
export function ruleCoverage(r: KnowledgeRule): string {
	const n = r.matches ?? 0;
	return n >= MATCH_LIMIT
		? m.knowledge_rule_covers_many({ count: MATCH_LIMIT.toLocaleString() })
		: m.knowledge_rule_covers({ count: n });
}

/** Discover, filtered to what a rule covers, so anyone can check its reach. */
export function ruleDiscoverHref(
	r: Pick<KnowledgeRule, 'selector_type' | 'selector_value'>
): string {
	const key =
		r.selector_type === 'tag' ? 'tags' : r.selector_type === 'type' ? 'types' : 'providers';
	return `/discover?${new URLSearchParams({ [key]: r.selector_value })}`;
}
