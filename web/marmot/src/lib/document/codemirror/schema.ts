// What a JSON Schema says about one place in a document: the keys an object there may have, the values a field may take and the problems with a parsed document, for linting and completion.

import { Validator, type OutputUnit } from '@cfworker/json-schema';
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

// Validation interprets the schema rather than compiling it to code, because the console's Content-Security-Policy forbids eval and new Function. Its findings are put in the shape ajv used (keyword, instance and schema paths, params, message) so the problems people see read exactly as before.

interface SchemaError {
	keyword: string;
	instancePath: string;
	schemaPath: string;
	params: Record<string, unknown>;
	message?: string;
	parentSchema?: Schema;
}

const validators = new WeakMap<object, Validator | null>();

function compile(schema: Schema): Validator | null {
	if (validators.has(schema)) return validators.get(schema) ?? null;
	let validator: Validator | null = null;
	try {
		validator = new Validator(schema as never, '2020-12', false);
	} catch {
		validator = null;
	}
	validators.set(schema, validator);
	return validator;
}

/** The value a JSON Pointer (with or without a leading #) points at, or undefined. */
function at(root: unknown, pointer: string): unknown {
	const path = pointer.replace(/^#/, '');
	if (path === '') return root;
	let node: unknown = root;
	for (const raw of path.slice(1).split('/')) {
		const part = decodeURIComponent(raw).replace(/~1/g, '/').replace(/~0/g, '~');
		if (!node || typeof node !== 'object') return undefined;
		node = (node as Record<string, unknown>)[part];
	}
	return node;
}

/** The schema value at a keyword location, following $ref segments the way the validator reports them. */
function schemaAtPointer(root: Schema, pointer: string): unknown {
	const path = pointer.replace(/^#/, '');
	if (path === '') return root;
	let node: unknown = root;
	for (const raw of path.slice(1).split('/')) {
		const part = decodeURIComponent(raw).replace(/~1/g, '/').replace(/~0/g, '~');
		if (!node || typeof node !== 'object') return undefined;
		const record = node as Record<string, unknown>;
		node =
			part === '$ref' && typeof record.$ref === 'string'
				? resolveRef(root, record.$ref)
				: record[part];
	}
	return node;
}

function parentPointer(pointer: string): string {
	return pointer.slice(0, pointer.lastIndexOf('/'));
}

/** A stable text form for comparing values, independent of key order. */
function canonical(value: unknown): string {
	if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
	if (value && typeof value === 'object') {
		const entries = Object.keys(value as object)
			.sort()
			.map((k) => `${JSON.stringify(k)}:${canonical((value as Record<string, unknown>)[k])}`);
		return `{${entries.join(',')}}`;
	}
	return JSON.stringify(value);
}

// The wrappers the validator reports above the real failure ("property x does not match") carry nothing ajv showed.
const WRAPPERS = new Set([
	'properties',
	'patternProperties',
	'items',
	'prefixItems',
	'additionalItems',
	'allOf',
	'$ref',
	'$dynamicRef',
	'dependentSchemas',
	'if',
	'then',
	'else',
	'unevaluatedItems'
]);

/** The validator's findings in ajv's shape, one per failing place. */
function schemaErrors(root: Schema, value: unknown, units: OutputUnit[]): SchemaError[] {
	const errors: SchemaError[] = [];
	const extraKeys = new Set(
		units.filter((u) => u.keyword === 'additionalProperties').map((u) => u.instanceLocation)
	);
	for (const unit of units) {
		const instancePath = unit.instanceLocation.replace(/^#/, '');
		const schemaPath = unit.keywordLocation;
		const keywordValue = schemaAtPointer(root, schemaPath);
		const owner = schemaAtPointer(root, parentPointer(schemaPath)) as Schema | undefined;
		const instance = at(value, unit.instanceLocation);
		const base = { keyword: unit.keyword, instancePath, schemaPath };
		switch (unit.keyword) {
			case 'required': {
				const required = Array.isArray(keywordValue) ? (keywordValue as string[]) : [];
				const present = instance && typeof instance === 'object' ? (instance as object) : {};
				for (const name of required) {
					if (!(name in present))
						errors.push({
							...base,
							params: { missingProperty: name },
							message: `must have required property '${name}'`
						});
				}
				break;
			}
			case 'additionalProperties': {
				const known = new Set(Object.keys(owner?.properties ?? {}));
				const patterns = Object.keys(
					(owner?.patternProperties as Record<string, unknown>) ?? {}
				).map((p) => new RegExp(p, 'u'));
				const keys =
					instance && typeof instance === 'object' ? Object.keys(instance as object) : [];
				for (const key of keys) {
					if (known.has(key) || patterns.some((re) => re.test(key))) continue;
					errors.push({
						...base,
						params: { additionalProperty: key },
						message: 'must NOT have additional properties'
					});
				}
				break;
			}
			case 'false':
				// additionalProperties: false reports each extra key once more as a false schema; ajv only had the first.
				if (!extraKeys.has(parentPointer(unit.instanceLocation) || '#'))
					errors.push({ ...base, params: {}, message: 'boolean schema is false' });
				break;
			case 'uniqueItems': {
				// Scanned from the end, as ajv does, so the reported repeat is the same item it named.
				const items = Array.isArray(instance) ? instance : [];
				const seen = new Map<string, number>();
				for (let i = items.length - 1; i >= 0; i--) {
					const key = canonical(items[i]);
					const j = seen.get(key);
					if (j !== undefined) {
						errors.push({
							...base,
							params: { i, j },
							message: `must NOT have duplicate items (items ## ${j} and ${i} are identical)`
						});
						break;
					}
					seen.set(key, i);
				}
				break;
			}
			case 'anyOf':
			case 'oneOf':
				errors.push({
					...base,
					params: {},
					parentSchema: owner,
					message:
						unit.keyword === 'anyOf'
							? 'must match a schema in anyOf'
							: 'must match exactly one schema in oneOf'
				});
				break;
			default: {
				if (WRAPPERS.has(unit.keyword)) break;
				errors.push({
					...base,
					params: paramsFor(unit.keyword, keywordValue),
					message: messageFor(unit.keyword, keywordValue) ?? unit.error
				});
			}
		}
	}
	return errors;
}

function paramsFor(keyword: string, value: unknown): Record<string, unknown> {
	switch (keyword) {
		case 'format':
			return { format: value };
		case 'minItems':
		case 'maxItems':
		case 'minLength':
		case 'maxLength':
		case 'minProperties':
		case 'maxProperties':
			return { limit: value };
		default:
			return {};
	}
}

// ajv's own wording, so messages read as they always have.
function messageFor(keyword: string, value: unknown): string | undefined {
	switch (keyword) {
		case 'type':
			return `must be ${Array.isArray(value) ? value.join(',') : value}`;
		case 'minLength':
			return `must NOT have fewer than ${value} characters`;
		case 'maxLength':
			return `must NOT have more than ${value} characters`;
		case 'minItems':
			return `must NOT have fewer than ${value} items`;
		case 'maxItems':
			return `must NOT have more than ${value} items`;
		case 'minProperties':
			return `must NOT have fewer than ${value} properties`;
		case 'maxProperties':
			return `must NOT have more than ${value} properties`;
		case 'pattern':
			return `must match pattern "${value}"`;
		case 'format':
			return `must match format "${value}"`;
		case 'enum':
			return 'must be equal to one of the allowed values';
		case 'const':
			return 'must be equal to constant';
		case 'minimum':
			return `must be >= ${value}`;
		case 'maximum':
			return `must be <= ${value}`;
		case 'exclusiveMinimum':
			return `must be > ${value}`;
		case 'exclusiveMaximum':
			return `must be < ${value}`;
		case 'multipleOf':
			return `must be multiple of ${value}`;
		case 'not':
			return 'must NOT be valid';
		default:
			return undefined;
	}
}

function describe(root: Schema, error: SchemaError): string {
	const params = error.params;
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
			const parent = error.parentSchema;
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
	const validator = compile(root);
	if (!validator) return [];
	const result = validator.validate(value);
	if (result.valid) return [];
	const errors = schemaErrors(root, value, result.errors);
	// The failing branches of an anyOf repeat what the anyOf error says once.
	const branches = errors
		.filter((error) => error.keyword === 'anyOf' || error.keyword === 'oneOf')
		.map((error) => `${error.schemaPath}/`);
	const seen = new Set<string>();
	const problems: SchemaProblem[] = [];
	for (const error of errors) {
		if (branches.some((prefix) => error.schemaPath.startsWith(prefix))) continue;
		const path = pointerToPath(error.instancePath);
		const extra = error.params.additionalProperty;
		const params = error.params;
		let problem: SchemaProblem = { path, message: describe(root, error) };
		if (error.keyword === 'additionalProperties' && typeof extra === 'string') {
			problem = { path: [...path, extra], key: true, message: problem.message };
		} else if (error.keyword === 'uniqueItems' && typeof params.i === 'number') {
			// The later of the two equal items is the repeat.
			problem = { path: [...path, Math.max(params.i, Number(params.j))], message: problem.message };
		}
		const id = `${problem.path.join('.')}|${problem.message}`;
		if (seen.has(id)) continue;
		seen.add(id);
		problems.push(problem);
	}
	return problems;
}
