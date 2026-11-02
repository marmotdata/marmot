// Checks an Avro schema against the Avro 1.11 specification without compiling it. avsc, used before, builds its types with new Function, which the console's Content-Security-Policy forbids; this keeps its rules (names, namespaces, references, unions and defaults) so the same schemas are accepted and refused.

const PRIMITIVES = new Set([
	'null',
	'boolean',
	'int',
	'long',
	'float',
	'double',
	'bytes',
	'string'
]);
const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const INT_MIN = -2147483648;
const INT_MAX = 2147483647;

type Json = unknown;

/** A parsed type, enough to check defaults against and to name union branches. */
type AvroType =
	| { kind: 'primitive'; name: string }
	| {
			kind: 'record';
			name?: string;
			fields: { name: string; type: AvroType; hasDefault: boolean }[];
	  }
	| { kind: 'enum'; name?: string; symbols: string[] }
	| { kind: 'fixed'; name?: string; size: number }
	| { kind: 'array'; items: AvroType }
	| { kind: 'map'; values: AvroType }
	| { kind: 'union'; branches: AvroType[] };

class AvroSchemaError extends Error {}

function fail(message: string): never {
	throw new AvroSchemaError(message);
}

function isPlainObject(value: unknown): value is Record<string, Json> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function validFullName(name: string): boolean {
	return name.split('.').every((part) => NAME.test(part));
}

/** A type's full name, from its own dotted name or its namespace, as the specification resolves them. */
function fullName(name: string, namespace: string | undefined): string {
	if (name.includes('.')) return name;
	return namespace ? `${namespace}.${name}` : name;
}

class Parser {
	private named = new Map<string, AvroType>();

	parse(schema: Json, namespace: string | undefined): AvroType {
		if (typeof schema === 'string') return this.reference(schema, namespace);
		if (Array.isArray(schema)) return this.union(schema, namespace);
		if (!isPlainObject(schema)) fail('invalid schema: expected a type name, an object or a union');
		const type = schema.type;
		// avsc takes only a type name here; a nested object or union must be written without the wrapper.
		if (typeof type !== 'string') fail(`unknown type: ${JSON.stringify(type)}`);
		switch (type) {
			case 'record':
			case 'error':
				return this.record(schema, namespace);
			case 'enum':
				return this.enumType(schema, namespace);
			case 'fixed':
				return this.fixed(schema, namespace);
			case 'array':
				if (!('items' in schema)) fail('array is missing its items');
				return { kind: 'array', items: this.parse(schema.items, namespace) };
			case 'map':
				if (!('values' in schema)) fail('map is missing its values');
				return { kind: 'map', values: this.parse(schema.values, namespace) };
			default:
				// A primitive written as an object, with or without a logicalType, or a reference to a named type.
				return this.reference(type, namespace);
		}
	}

	private reference(name: string, namespace: string | undefined): AvroType {
		if (PRIMITIVES.has(name)) return { kind: 'primitive', name };
		const found = this.named.get(fullName(name, namespace)) ?? this.named.get(name);
		if (!found) fail(`undefined type name: ${name}`);
		return found;
	}

	private declare(
		schema: Record<string, Json>,
		namespace: string | undefined
	): { name: string | undefined; namespace: string | undefined } {
		const raw = schema.name;
		// Named types may be anonymous, as avsc allows; they just cannot be referred to.
		if (raw === undefined) return { name: undefined, namespace };
		if (typeof raw !== 'string' || !raw) fail(`invalid name: ${JSON.stringify(raw)}`);
		if (!validFullName(raw)) fail(`invalid name: ${raw}`);
		const ns =
			typeof schema.namespace === 'string' && !raw.includes('.') ? schema.namespace : namespace;
		if (ns && !validFullName(ns)) fail(`invalid namespace: ${ns}`);
		const name = fullName(raw, ns);
		if (PRIMITIVES.has(name)) fail(`cannot rename primitive type: ${name}`);
		if (this.named.has(name)) fail(`duplicate type name: ${name}`);
		const aliases = schema.aliases;
		if (aliases !== undefined) {
			if (!Array.isArray(aliases)) fail(`invalid aliases for ${name}`);
			for (const alias of aliases) {
				if (typeof alias !== 'string' || !validFullName(alias)) fail(`invalid alias: ${alias}`);
			}
		}
		const enclosing = name.includes('.') ? name.slice(0, name.lastIndexOf('.')) : undefined;
		return { name, namespace: enclosing };
	}

	private record(schema: Record<string, Json>, namespace: string | undefined): AvroType {
		const { name, namespace: inner } = this.declare(schema, namespace);
		if (!Array.isArray(schema.fields)) fail(`non-array ${schema.type} fields`);
		const type: AvroType = { kind: 'record', name, fields: [] };
		// Registered before its fields so a record can refer to itself.
		if (name) this.named.set(name, type);
		const seen = new Set<string>();
		for (const field of schema.fields) {
			if (!isPlainObject(field)) fail(`invalid field in ${name}`);
			const fieldName = field.name;
			if (typeof fieldName !== 'string' || !NAME.test(fieldName))
				fail(`invalid field name: ${String(fieldName)}`);
			if (seen.has(fieldName)) fail(`duplicate field name: ${fieldName}`);
			seen.add(fieldName);
			if (!('type' in field)) fail(`missing type for field ${fieldName}`);
			const fieldType = this.parse(field.type, inner);
			const hasDefault = field.default !== undefined;
			if (hasDefault && !validDefault(fieldType, field.default))
				fail(`invalid default for field ${fieldName}: ${JSON.stringify(field.default)}`);
			type.fields.push({ name: fieldName, type: fieldType, hasDefault });
		}
		return type;
	}

	private enumType(schema: Record<string, Json>, namespace: string | undefined): AvroType {
		const { name } = this.declare(schema, namespace);
		const symbols = schema.symbols;
		if (!Array.isArray(symbols)) fail(`invalid ${name} symbols`);
		const seen = new Set<string>();
		for (const symbol of symbols) {
			if (typeof symbol !== 'string' || !NAME.test(symbol))
				fail(`invalid ${name} symbol: ${String(symbol)}`);
			if (seen.has(symbol)) fail(`duplicate enum symbol: ${symbol}`);
			seen.add(symbol);
		}
		if (
			schema.default !== undefined &&
			(typeof schema.default !== 'string' || !seen.has(schema.default))
		)
			fail(`invalid ${name} default: ${JSON.stringify(schema.default)}`);
		const type: AvroType = { kind: 'enum', name, symbols: symbols as string[] };
		if (name) this.named.set(name, type);
		return type;
	}

	private fixed(schema: Record<string, Json>, namespace: string | undefined): AvroType {
		const { name } = this.declare(schema, namespace);
		const size = schema.size;
		if (typeof size !== 'number' || !Number.isInteger(size) || size < 0)
			fail(`invalid ${name} size`);
		const type: AvroType = { kind: 'fixed', name, size };
		if (name) this.named.set(name, type);
		return type;
	}

	private union(schema: Json[], namespace: string | undefined): AvroType {
		if (schema.length === 0) fail('empty union');
		const branches: AvroType[] = [];
		const names = new Set<string>();
		for (const branch of schema) {
			const type = this.parse(branch, namespace);
			if (type.kind === 'union') fail('unions cannot be directly nested');
			const key = branchName(type);
			if (names.has(key)) fail(`duplicate union branch name: ${key}`);
			names.add(key);
			branches.push(type);
		}
		return { kind: 'union', branches };
	}
}

function branchName(type: AvroType): string {
	switch (type.kind) {
		case 'primitive':
			return type.name;
		case 'array':
			return 'array';
		case 'map':
			return 'map';
		case 'union':
			return 'union';
		default:
			// Anonymous named types are told apart by kind alone.
			return type.name ?? type.kind;
	}
}

/** Whether a JSON default fits a type; a union's default must fit its first branch. */
function validDefault(type: AvroType, value: Json): boolean {
	switch (type.kind) {
		case 'primitive':
			switch (type.name) {
				case 'null':
					return value === null;
				case 'boolean':
					return typeof value === 'boolean';
				case 'int':
					return (
						typeof value === 'number' &&
						Number.isInteger(value) &&
						value >= INT_MIN &&
						value <= INT_MAX
					);
				case 'long':
					return typeof value === 'number' && Number.isInteger(value);
				case 'float':
				case 'double':
					return typeof value === 'number';
				case 'bytes':
				case 'string':
					return typeof value === 'string';
				default:
					return false;
			}
		case 'enum':
			return typeof value === 'string' && type.symbols.includes(value);
		case 'fixed':
			return typeof value === 'string' && value.length === type.size;
		case 'array':
			return Array.isArray(value) && value.every((item) => validDefault(type.items, item));
		case 'map':
			return (
				isPlainObject(value) &&
				Object.values(value).every((item) => validDefault(type.values, item))
			);
		case 'record':
			return (
				isPlainObject(value) &&
				type.fields.every((field) =>
					field.name in value ? validDefault(field.type, value[field.name]) : field.hasDefault
				)
			);
		case 'union':
			// Avro encodes a union default as its first branch, wherever the union sits.
			return type.branches.length > 0 && validDefault(type.branches[0], value);
	}
}

/** Why a schema is not valid Avro, or null when it is. */
export function avroSchemaError(schema: unknown): string | null {
	try {
		new Parser().parse(schema, undefined);
		return null;
	} catch (error) {
		if (error instanceof AvroSchemaError) return error.message;
		return error instanceof Error ? error.message : String(error);
	}
}
