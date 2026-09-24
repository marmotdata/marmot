import { m } from '$lib/paraglide/messages';
import { UNASSIGNED_DOMAIN_ID, type Domain } from './types';

/** The stored name of Unassigned is English; show it in the user's language. */
export function domainName(domain: Pick<Domain, 'id' | 'name'>): string {
	return domain.id === UNASSIGNED_DOMAIN_ID ? m.domains_unassigned() : domain.name;
}

export function isUnassigned(domain: Pick<Domain, 'id'> | null | undefined): boolean {
	return domain?.id === UNASSIGNED_DOMAIN_ID;
}

/** A Discover query listing everything in a domain's subtree. */
export function discoverQuery(domainId: string): string {
	return `@domain:${domainId}`;
}

const domainTokenRegex = /@domain\s*[:=]\s*"?([0-9a-fA-F-]{36})"?/g;

/** The first domain an `@domain:` token in a search query selects. */
export function domainInQuery(query: string): string | null {
	const match = new RegExp(domainTokenRegex.source).exec(query);
	return match ? match[1].toLowerCase() : null;
}

/** The query with its `@domain:` tokens replaced by one for domainId, or removed when null. */
export function withDomain(query: string, domainId: string | null): string {
	const rest = query.replace(domainTokenRegex, '').replace(/\s+/g, ' ').trim();
	if (!domainId) return rest;
	return rest ? `${discoverQuery(domainId)} ${rest}` : discoverQuery(domainId);
}
