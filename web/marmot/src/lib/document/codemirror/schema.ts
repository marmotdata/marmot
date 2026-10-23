// What a JSON Schema says about one place in a document: the keys an object there may have, the values a field may take and the problems with a parsed document, for linting and completion.

import Ajv2020, { type ErrorObject, type ValidateFunction } from 'ajv/dist/2020';
import addFormats from 'ajv-formats';
import { m } from '$lib/paraglide/messages';
import { pointerToPath, type PathSegment } from '../paths';

export type Schema = {
	$ref?: string;
	type?: string | string[];
	description?: string;
	properties?: Record<string, Schema>;
	items?: Schema | boolean;
	prefixItems?: Schema[];
	enum?: unknown[];
	const?: unknown;
	anyOf?: Schema[];
	oneOf?: Schema[];
	allOf?: Schema[];
	$defs?: Record<string, Schema>;
	definitions?: Record<string, Schema>;
	[key: string]: unknown;
};

function resolveRef(root: Schema, ref: string): Schema | null {
	if (ref === '#') return root;
	if (!ref.startsWith('#/')) return null;
	let node: unknown = root;
	for (const raw of ref.slice(2).split('/')) {
		const part = decodeURIComponent(raw).replace(/~1/g, '/').replace(/~0/g, '~');
		if (!node || typeof node !== 'object') return null;
		node = (node as Record<string, unknown>)[part];
	}
	return node && typeof node === 'object' ? (node as Schema) : null;
}

/** A schema with its references followed and its anyOf, oneOf and allOf branches spread out. */
function expand(root: Schema, schema: Schema, depth = 0): Schema[] {
	if (depth > 16) return [];
	const out: Schema[] = [];
	const own = schema.$ref ? resolveRef(root, schema.$ref) : null;
	if (own) out.push(...expand(root, own, depth + 1));
	out.push(schema);
	for (const branch of [
		...(schema.anyOf ?? []),
		...(schema.oneOf ?? []),
		...(schema.allOf ?? [])
	]) {
		out.push(...expand(root, branch, depth + 1));
	}
	return out;
}

/** Every schema that applies at path. */
export function schemasAt(root: Schema, path: PathSegment[]): Schema[] {
	let current = expand(root, root);
	for (const segment of path) {
		const next: Schema[] = [];
		for (const schema of current) {
			if (typeof segment === 'string' && schema.properties?.[segment]) {
				next.push(...expand(root, schema.properties[segment]));
			} else if (typeof segment === 'number') {
				const positional = schema.prefixItems?.[segment];
				if (positional) next.push(...expand(root, positional));
				else if (schema.items && typeof schema.items === 'object') {
					next.push(...expand(root, schema.items));
				}
			}
		}
		current = next;
		if (current.length === 0) break;
	}
	return current;
}

function types(schema: Schema): string[] {
	return Array.isArray(schema.type) ? schema.type : schema.type ? [schema.type] : [];
}

/** Whether the place holds an object, so a bare word typed there is a key. */
export function expectsObject(schemas: Schema[]): boolean {
	return schemas.some((schema) => types(schema).includes('object') || !!schema.properties);
}

export interface KeyOption {
	name: string;
	description?: string;
	required: boolean;
}

export function keyOptions(root: Schema, path: PathSegment[]): KeyOption[] {
	const options = new Map<string, KeyOption>();
	for (const schema of schemasAt(root, path)) {
		const required = new Set(Array.isArray(schema.required) ? (schema.required as string[]) : []);
		for (const [name, property] of Object.entries(schema.properties ?? {})) {
			if (options.has(name)) continue;
			const description =
				property.description ??
				expand(root, property).find((candidate) => candidate.description)?.description;
			options.set(name, { name, description, required: required.has(name) });
		}
	}
	return [...options.values()];
}

export interface ValueOption {
	value: string;
	description?: string;
}

export function valueOptions(root: Schema, path: PathSegment[]): ValueOption[] {
	const options = new Map<string, ValueOption>();
	for (const schema of schemasAt(root, path)) {
		const values = [...(schema.enum ?? []), ...('const' in schema ? [schema.const] : [])];
		for (const value of values) {
			if (typeof value !== 'string' && typeof value !== 'number' && typeof value !== 'boolean')
				continue;
			const text = String(value);
			if (!options.has(text)) options.set(text, { value: text, description: schema.description });
		}
	}
	return [...options.values()];
}

export interface SchemaProblem {
	path: PathSegment[];
	message: string;
	/** The problem is with a key itself, such as one the schema does not allow. */
	key?: boolean;
}

const validators = new WeakMap<object, ValidateFunction | null>();

function compile(schema: Schema): ValidateFunction | null {
	if (validators.has(schema)) return validators.get(schema) ?? null;
	let validate: ValidateFunction | null = null;
	try {
		// One instance per schema: ajv refuses to register the same $id twice.
		const ajv = new Ajv2020({ allErrors: true, strict: false, verbose: true });
		addFormats(ajv);
		validate = ajv.compile(schema);
	} catch {
		validate = null;
	}
	validators.set(schema, validate);
	return validate;
}

function describe(root: Schema, error: ErrorObject): string {
	const params = error.params as Record<string, unknown>;
	switch (error.keyword) {
		case 'required':
			return m.ui_document_missing_field({ field: String(params.missingProperty) });
		case 'additionalProperties':
			return m.ui_document_unknown_field({ field: String(params.additionalProperty) });
		case 'format':
			return params.format === 'date-time'
				? m.ui_document_bad_datetime()
				: m.ui_document_bad_format({ format: String(params.format) });
		case 'uniqueItems':
			return m.ui_document_duplicate_item();
		case 'minItems':
			return Number(params.limit) === 1
				? m.ui_document_empty_list()
				: m.ui_document_too_few_items({ min: Number(params.limit) });
		case 'anyOf':
		case 'oneOf': {
			const parent = error.parentSchema as Schema | undefined;
			const description =
				parent?.description ??
				(parent
					? expand(root, parent).find((candidate) => candidate.description)?.description
					: '');
			return description
				? m.ui_document_expected({ description })
				: (error.message ?? m.ui_document_invalid_value());
		}
		default:
			return error.message ?? m.ui_document_invalid_value();
	}
}

/** The problems the schema finds in a parsed document, one per place. */
export function validateAgainst(root: Schema, value: unknown): SchemaProblem[] {
	const validate = compile(root);
	if (!validate || validate(value)) return [];
	const errors = validate.errors ?? [];
	// The failing branches of an anyOf repeat what the anyOf error says once.
	const branches = errors
		.filter((error) => error.keyword === 'anyOf' || error.keyword === 'oneOf')
		.map((error) => `${error.schemaPath}/`);
	const seen = new Set<string>();
	const problems: SchemaProblem[] = [];
	for (const error of errors) {
		if (branches.some((prefix) => error.schemaPath.startsWith(prefix))) continue;
		const path = pointerToPath(error.instancePath);
		const extra = (error.params as Record<string, unknown>).additionalProperty;
		const params = error.params as Record<string, unknown>;
		let problem: SchemaProblem = { path, message: describe(root, error) };
		if (error.keyword === 'additionalProperties' && typeof extra === 'string') {
			problem = { path: [...path, extra], key: true, message: problem.message };
		} else if (error.keyword === 'uniqueItems' && typeof params.i === 'number') {
			// ajv names the two equal items; the later one is the repeat.
			problem = { path: [...path, Math.max(params.i, Number(params.j))], message: problem.message };
		}
		const id = `${problem.path.join('.')}|${problem.message}`;
		if (seen.has(id)) continue;
		seen.add(id);
		problems.push(problem);
	}
	return problems;
}
