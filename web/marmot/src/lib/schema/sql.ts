import type { Field } from './types';

/**
 * Native SQL column schema format (Trino, ClickHouse, etc.):
 * [
 *   { "column_name": "id", "data_type": "integer", "is_nullable": "YES", ... },
 *   ...
 * ]
 */

interface SqlColumn {
	column_name?: unknown;
	data_type?: unknown;
	is_nullable?: unknown;
	is_primary_key?: unknown;
	is_sorting_key?: unknown;
	comment?: unknown;
	default_expression?: unknown;
}

interface SchemaValidationError {
	message: string;
}

function isObject(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

/**
 * Check if the schema is a native SQL column array format.
 * Differentiates from dbt by checking for `column_name` instead of `name`.
 */
export function isSqlColumnSchema(schemaSection: unknown): boolean {
	if (!schemaSection) return false;
	if (!Array.isArray(schemaSection)) return false;
	if (schemaSection.length === 0) return false;

	const first = schemaSection[0];
	if (!isObject(first)) return false;

	return typeof first.column_name === 'string' && typeof first.data_type === 'string';
}

/**
 * Process native SQL column array into Field[] for display.
 * Handles both Trino and ClickHouse column formats.
 */
export function processSqlColumnSchema(schemaSection: unknown): Field[] {
	if (!schemaSection || !Array.isArray(schemaSection)) return [];

	const fields: Field[] = [];

	for (const item of schemaSection) {
		if (!isObject(item)) continue;
		const col = item as SqlColumn;
		if (typeof col.column_name !== 'string') continue;

		// Build description from comment + key annotations
		const descParts: string[] = [];

		if (col.is_primary_key === true) {
			descParts.push('Primary Key');
		}
		if (col.is_sorting_key === true) {
			descParts.push('Sorting Key');
		}
		if (typeof col.comment === 'string' && col.comment !== '') {
			descParts.push(col.comment);
		}

		// Determine required from is_nullable (Trino format)
		let required: boolean | undefined;
		if (typeof col.is_nullable === 'string') {
			required = col.is_nullable === 'NO';
		}

		fields.push({
			name: col.column_name,
			type: typeof col.data_type === 'string' ? col.data_type : 'unknown',
			description: descParts.length > 0 ? descParts.join(' · ') : undefined,
			required,
			default: col.default_expression,
			indentLevel: 0
		});
	}

	return fields;
}

/**
 * Validate SQL column schema - basic structure check
 */
export function validateSqlColumnSchema(schema: unknown): SchemaValidationError[] {
	if (!schema) return [];

	if (!Array.isArray(schema)) {
		return [{ message: 'SQL column schema must be an array of columns' }];
	}

	const errors: SchemaValidationError[] = [];

	schema.forEach((col: unknown, index: number) => {
		if (!isObject(col)) {
			errors.push({ message: `Column at index ${index} is not an object` });
			return;
		}
		const name = (col as SqlColumn).column_name;
		if (typeof name !== 'string' || name.trim() === '') {
			errors.push({ message: `Column at index ${index} is missing a valid 'column_name' field` });
		}
	});

	return errors;
}
