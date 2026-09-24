import { domainsEnabled, loadTree } from './api';
import type { DomainNode } from './types';

/**
 * Every domain as the name path an `@domain` query accepts ("Finanzas/Pagos"),
 * depth first. Empty when domains are off or cannot be loaded.
 */
export async function domainQueryValues(): Promise<string[]> {
	try {
		if (!(await domainsEnabled())) return [];
		const out: string[] = [];
		const walk = (list: DomainNode[], prefix: string) => {
			for (const node of list) {
				const path = prefix ? `${prefix}/${node.name}` : node.name;
				out.push(path);
				walk(node.children, path);
			}
		};
		walk(await loadTree(), '');
		return out;
	} catch {
		return [];
	}
}
