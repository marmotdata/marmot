import type { DomainNode, WritableDomains } from './types';
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
		exclude,
		writable
	}: {
		includeUnassigned?: boolean;
		exclude?: (d: DomainNode) => boolean;
		/** Under write enforcement, only the domains the caller may write in. */
		writable?: WritableDomains;
	} = {}
): DomainOption[] {
	const allowed = writable && !writable.all ? new Set(writable.domain_ids) : undefined;
	const entries = flatten(forest).filter(
		(e) =>
			(includeUnassigned || !isUnassigned(e.domain)) &&
			!exclude?.(e.domain) &&
			(!allowed || allowed.has(e.domain.id))
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
