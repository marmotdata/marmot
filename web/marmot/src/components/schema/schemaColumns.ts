import type { SchemaColumn } from '@marmotdata/elements/classes/schema-table.js';
import type { Field } from '$lib/schema/types';
import type { FieldAnnotation } from '$lib/assets/types';
import { processSchema, validateSchema } from '$lib/schema/utils';

export interface ParsedSchema {
	columns: SchemaColumn[];
	/** The document as the source wrote it, for the outline's JSON and YAML views; undefined when it is not JSON (protobuf text, for example). */
	source?: unknown;
	/** The original text when it is not JSON, shown as-is under the outline. */
	rawText?: string;
	example: unknown;
	warnings: string[];
}

// JSON Schema puts dates and IDs in `format`, so lift the ones the outline can name in everyday words into the type.
const FORMAT_TYPES: Record<string, string> = {
	'date-time': 'timestamp',
	date: 'date',
	time: 'time',
	uuid: 'uuid'
};

function toColumn(f: Field): SchemaColumn {
	const symbols = Array.isArray(f.enum) ? f.enum.map((v) => String(v)) : undefined;
	const col: SchemaColumn = {
		name: (f.name ?? '').split('.').pop() || f.name,
		type: (f.format && FORMAT_TYPES[f.format]) || (symbols?.length ? 'enum' : f.type || 'unknown')
	};
	if (f.description) col.description = f.description;
	if (f.required === true) col.nullable = false;
	else if (f.required === false) col.nullable = true;
	if (symbols?.length) col.symbols = symbols;
	if (f.default !== undefined && (f.default === null || typeof f.default !== 'object')) {
		col.default = f.default as string | number | boolean | null;
	}
	return col;
}

/** Turns the flat, indent-levelled field list the schema parsers produce into the nested outline `mm-schema-table` reads. */
export function fieldsToColumns(fields: Field[]): SchemaColumn[] {
	const roots: SchemaColumn[] = [];
	const stack: { level: number; col: SchemaColumn }[] = [];
	for (const f of fields) {
		const level = f.indentLevel ?? 0;
		const col = toColumn(f);
		while (stack.length && stack[stack.length - 1].level >= level) stack.pop();
		const parent = stack[stack.length - 1]?.col;
		if (parent) (parent.fields ??= []).push(col);
		else roots.push(col);
		stack.push({ level, col });
	}
	return roots;
}

/** Parses one stored schema (an object, a JSON string or other text) into outline columns plus what the views around it need. */
export function parseSchema(raw: unknown): ParsedSchema {
	let source: unknown = raw;
	let rawText: string | undefined;
	if (typeof raw === 'string') {
		try {
			source = JSON.parse(raw);
		} catch {
			source = undefined;
			rawText = raw;
		}
	}
	const { fields, example } = processSchema(source ?? raw);
	const warnings = (validateSchema(source ?? raw) as { message?: string }[]).map(
		(e) => e?.message || JSON.stringify(e)
	);
	return { columns: fieldsToColumns(fields), source, rawText, example, warnings };
}

/** Lays the markers people set (personal data, certified, an edited description, glossary terms) onto the outline, matching each annotation by the field's dotted path. */
export function applyAnnotations(
	columns: SchemaColumn[],
	annotations: FieldAnnotation[] | undefined
): SchemaColumn[] {
	if (!annotations?.length) return columns;
	const byPath = new Map(annotations.map((a) => [a.path, a]));
	const walk = (cols: SchemaColumn[], prefix: string): SchemaColumn[] =>
		cols.map((c) => {
			const path = prefix ? `${prefix}.${c.name}` : c.name;
			const a = byPath.get(path);
			const next: SchemaColumn = { ...c };
			if (c.fields) next.fields = walk(c.fields, path);
			if (!a) return next;
			if (a.pii) next.pii = true;
			if (a.certified) next.certified = true;
			if (a.glossary_terms.length) next.glossaryTerms = a.glossary_terms.map((t) => t.name);
			if (a.description) {
				next.description = a.description;
				next.descriptionSource = 'user';
			}
			return next;
		});
	return walk(columns, '');
}
