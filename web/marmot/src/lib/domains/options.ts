import type { DomainNode } from './types';
import { flatten } from './api';
import { domainName, isUnassigned } from './labels';

export interface DomainOption {
	id: string;
	/** Name shown in the list, indented by depth. */
	name: string;
	/** Full path, shown on the closed picker so the choice is unambiguous. */
	path: string;
	depth: number;
	unassigned: boolean;
}

/** Tree-ordered picker options; Unassigned goes last so the tree reads first. */
export function domainOptions(
	forest: DomainNode[],
	{
		includeUnassigned = false,
		exclude
	}: { includeUnassigned?: boolean; exclude?: (d: DomainNode) => boolean } = {}
): DomainOption[] {
	const entries = flatten(forest).filter(
		(e) => (includeUnassigned || !isUnassigned(e.domain)) && !exclude?.(e.domain)
	);
	const toOption = (e: (typeof entries)[number]): DomainOption => ({
		id: e.domain.id,
		name: domainName(e.domain),
		path: isUnassigned(e.domain) ? domainName(e.domain) : e.label,
		depth: e.domain.depth - 1,
		unassigned: isUnassigned(e.domain)
	});
	return [
		...entries.filter((e) => !isUnassigned(e.domain)).map(toOption),
		...entries.filter((e) => isUnassigned(e.domain)).map(toOption)
	];
}
