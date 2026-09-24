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
