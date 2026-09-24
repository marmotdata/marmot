import { fetchApi } from '$lib/api';
import { m } from '$lib/paraglide/messages';
import type { Domain, DomainErrorCode, DomainKind, DomainNode } from './types';

export class DomainError extends Error {
	constructor(
		readonly code: DomainErrorCode | undefined,
		message: string
	) {
		super(message);
	}
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
	const response = await fetchApi(endpoint, options);
	if (!response.ok) {
		let code: DomainErrorCode | undefined;
		let message = response.statusText;
		try {
			const body = await response.json();
			code = body.code;
			message = body.error ?? message;
		} catch {
			// non-JSON error body
		}
		throw new DomainError(code, message);
	}
	return response.json();
}

function send<T>(endpoint: string, method: string, body: unknown): Promise<T> {
	return request<T>(endpoint, {
		method,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

let enabled: Promise<boolean> | undefined;

/** Whether the server has domains enabled and the user may see them; asked once per session. */
export function domainsEnabled(): Promise<boolean> {
	enabled ??= fetchApi('/domains')
		.then((response) => response.ok)
		.catch(() => false);
	return enabled;
}

export function listChildren(parentId?: string): Promise<Domain[]> {
	const query = parentId ? `?parent_id=${encodeURIComponent(parentId)}` : '';
	return request<Domain[]>(`/domains${query}`);
}

export function getDomain(id: string): Promise<Domain> {
	return request<Domain>(`/domains/${encodeURIComponent(id)}`);
}

export function domainOf(kind: DomainKind, id: string): Promise<Domain> {
	return request<Domain>(`/domains/of/${kind}/${encodeURIComponent(id)}`);
}

export function createDomain(input: {
	name: string;
	description?: string;
	parent_id?: string;
}): Promise<Domain> {
	return send<Domain>('/domains', 'POST', input);
}

export function updateDomain(
	id: string,
	input: { name?: string; description?: string }
): Promise<Domain> {
	return send<Domain>(`/domains/${encodeURIComponent(id)}`, 'PUT', input);
}

export function deleteDomain(id: string): Promise<unknown> {
	return request(`/domains/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function moveDomain(id: string, parentId: string | null): Promise<Domain> {
	return send<Domain>(`/domains/${encodeURIComponent(id)}/move`, 'POST', { parent_id: parentId });
}

export function assignToDomain(
	domainId: string,
	kind: DomainKind,
	ids: string[]
): Promise<unknown> {
	return send(`/domains/${encodeURIComponent(domainId)}/members`, 'PUT', { kind, ids });
}

/** Every domain, as a forest ordered by name. */
export async function loadTree(): Promise<DomainNode[]> {
	const roots = await listChildren();
	const subtrees = await Promise.all(
		roots.map((root) => request<Domain[]>(`/domains/${encodeURIComponent(root.id)}/tree`))
	);
	const nodes = new Map<string, DomainNode>();
	for (const domain of subtrees.flat()) nodes.set(domain.id, { ...domain, children: [] });
	const forest: DomainNode[] = [];
	for (const node of nodes.values()) {
		const parent = node.parent_id ? nodes.get(node.parent_id) : undefined;
		(parent ? parent.children : forest).push(node);
	}
	const byName = (a: DomainNode, b: DomainNode) => a.name.localeCompare(b.name);
	const sort = (list: DomainNode[]) => {
		list.sort(byName);
		list.forEach((n) => sort(n.children));
	};
	sort(forest);
	return forest;
}

/** Depth-first list with each domain's full name, for pickers. */
export function flatten(forest: DomainNode[]): { domain: DomainNode; label: string }[] {
	const out: { domain: DomainNode; label: string }[] = [];
	const walk = (list: DomainNode[], prefix: string) => {
		for (const node of list) {
			const label = prefix ? `${prefix} / ${node.name}` : node.name;
			out.push({ domain: node, label });
			walk(node.children, label);
		}
	};
	walk(forest, '');
	return out;
}

export function errorMessage(error: unknown): string {
	if (!(error instanceof DomainError)) return m.domains_error_generic();
	switch (error.code) {
		case 'name_conflict':
			return m.domains_error_name_conflict();
		case 'has_children':
			return m.domains_error_has_children();
		case 'not_empty':
			return m.domains_error_not_empty();
		case 'protected':
			return m.domains_error_protected();
		case 'cycle':
			return m.domains_error_cycle();
		case 'too_deep':
			return m.domains_error_too_deep();
		case 'not_found':
			return m.domains_error_not_found();
		case 'forbidden':
			return m.domains_error_forbidden();
		case 'invalid_input':
			return m.domains_error_invalid_input();
		default:
			return error.message || m.domains_error_generic();
	}
}
