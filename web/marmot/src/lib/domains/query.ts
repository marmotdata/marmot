import { domainsEnabled, loadTree } from './api';
import { domainName, isUnassigned } from './labels';
import type { DomainNode } from './types';

export interface DomainQueryValue {
	/** What the query gets: the name path an `@domain` token accepts ("Finanzas/Pagos"). */
	value: string;
	/** What the user sees: the same path, with Unassigned in the UI language. */
	label: string;
}

/**
 * Every domain as an `@domain` query value, depth first. Unassigned is inserted
 * by its stored name, which every locale resolves. Empty when domains are off
 * or cannot be loaded.
 */
export async function domainQueryValues(): Promise<DomainQueryValue[]> {
	try {
		if (!(await domainsEnabled())) return [];
		const out: DomainQueryValue[] = [];
		const walk = (list: DomainNode[], prefix: string) => {
			for (const node of list) {
				const path = prefix ? `${prefix}/${node.name}` : node.name;
				out.push({ value: path, label: isUnassigned(node) ? domainName(node) : path });
				walk(node.children, path);
			}
		};
		walk(await loadTree(), '');
		return out;
	} catch {
		return [];
	}
}
