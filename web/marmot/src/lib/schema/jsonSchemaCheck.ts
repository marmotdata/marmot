// Checks that a JSON Schema is well formed without compiling it. ajv.compile, used before, builds code with new Function, which the console's Content-Security-Policy forbids; this checks the same things it refused a schema for: the draft-07 meta-schema (with ajv's $data extension, via a validator generated ahead of time), regular expressions that do not compile and references that resolve nowhere.

import validateDraft7 from './generated/draft7Meta.js';

interface MetaError {
	instancePath: string;
	message?: string;
}

// Meta-schemas ajv knows without being told, so a reference to one resolves.
const KNOWN_IDS = new Set([
	'http://json-schema.org/draft-07/schema',
	'http://json-schema.org/draft-07/schema#',
	'https://raw.githubusercontent.com/ajv-validator/ajv/master/lib/refs/data.json',
	'https://raw.githubusercontent.com/ajv-validator/ajv/master/lib/refs/data.json#'
]);

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function pointerTarget(root: unknown, pointer: string): unknown {
	if (pointer === '') return root;
	if (!pointer.startsWith('/')) return undefined;
	let node: unknown = root;
	for (const raw of pointer.slice(1).split('/')) {
		const part = decodeURIComponent(raw).replace(/~1/g, '/').replace(/~0/g, '~');
		if (Array.isArray(node)) node = node[Number(part)];
		else if (isRecord(node)) node = node[part];
		else return undefined;
	}
	return node;
}

interface Walked {
	refs: string[];
	ids: Set<string>;
	patterns: string[];
	/** A $data pointer that climbs above the data root, which ajv refuses to compile. */
	badData?: string;
}

// Keywords whose value is a map of names to subschemas, so their keys are names, not keywords.
const SCHEMA_MAPS = new Set(['properties', 'patternProperties', 'definitions', 'dependencies']);
// Keywords whose subschemas describe data one level further down, which is what a relative $data pointer counts.
const DEEPER = new Set([
	'properties',
	'patternProperties',
	'additionalProperties',
	'items',
	'additionalItems',
	'contains'
]);
// Values here are data, not schema, so nothing inside them is a keyword.
const DATA_VALUES = new Set(['enum', 'const', 'default', 'examples']);

function walk(node: unknown, found: Walked, depth: number): void {
	if (Array.isArray(node)) {
		node.forEach((item) => walk(item, found, depth));
		return;
	}
	if (!isRecord(node)) return;
	if (typeof node.$data === 'string') {
		const up = node.$data.match(/^(\d+)/);
		if (up && Number(up[1]) > depth && !found.badData) found.badData = node.$data;
	}
	for (const [key, value] of Object.entries(node)) {
		if (key === '$ref' && typeof value === 'string') found.refs.push(value);
		else if (key === '$id' && typeof value === 'string') found.ids.add(value);
		else if (key === 'pattern' && typeof value === 'string') found.patterns.push(value);
		if (DATA_VALUES.has(key)) {
			// const and enum may themselves be a $data reference, which is still checked.
			if (isRecord(value) && typeof value.$data === 'string')
				walk({ $data: value.$data }, found, depth);
			continue;
		}
		const next = DEEPER.has(key) ? depth + 1 : depth;
		if (SCHEMA_MAPS.has(key) && isRecord(value)) {
			Object.values(value).forEach((child) => walk(child, found, next));
		} else {
			walk(value, found, next);
		}
	}
}

function refResolves(root: unknown, ref: string, ids: Set<string>): boolean {
	if (ref === '#' || ref === '') return true;
	if (KNOWN_IDS.has(ref) || ids.has(ref)) return true;
	const hash = ref.indexOf('#');
	const base = hash >= 0 ? ref.slice(0, hash) : ref;
	const fragment = hash >= 0 ? ref.slice(hash + 1) : '';
	// A reference within this schema: a JSON Pointer, or a plain-name fragment matching an $id.
	if (base === '' || ids.has(base) || ids.has(`${base}#`)) {
		if (fragment === '') return true;
		if (fragment.startsWith('/')) return pointerTarget(root, fragment) !== undefined;
		return ids.has(`#${fragment}`);
	}
	return false;
}

/** Why a JSON Schema would not compile, or null when it is well formed. */
export function jsonSchemaProblem(schema: unknown): string | null {
	if (!validateDraft7(schema)) {
		const errors = ((validateDraft7 as unknown as { errors?: MetaError[] }).errors ?? [])
			.map((e) => `data${e.instancePath} ${e.message ?? 'is invalid'}`)
			.join(', ');
		return `schema is invalid: ${errors}`;
	}
	const found: Walked = { refs: [], ids: new Set(), patterns: [] };
	walk(schema, found, 0);
	if (found.badData) return `Cannot access data above the root: ${found.badData}`;
	for (const pattern of found.patterns) {
		try {
			new RegExp(pattern, 'u');
		} catch (error) {
			return error instanceof Error ? error.message : `Invalid regular expression: ${pattern}`;
		}
	}
	for (const ref of found.refs) {
		if (!refResolves(schema, ref, found.ids)) return `can't resolve reference ${ref} from id #`;
	}
	return null;
}
