import type { Field } from './types';

/**
 * Native SQL column schema format (Trino, ClickHouse, etc.):
 * [
 *   { "column_name": "id", "data_type": "integer", "is_nullable": "YES", ... },
 *   ...
 * ]
 */

/**
 * Check if the schema is a native SQL column array format.
 * Differentiates from dbt by checking for `column_name` instead of `name`.
 */
export function isSqlColumnSchema(schemaSection: any): boolean {
	if (!schemaSection) return false;
	if (!Array.isArray(schemaSection)) return false;
	if (schemaSection.length === 0) return false;

	const first = schemaSection[0];
	if (typeof first !== 'object' || first === null) return false;

	return typeof first.column_name === 'string' && typeof first.data_type === 'string';
}

/**
 * Process native SQL column array into Field[] for display.
 * Handles both Trino and ClickHouse column formats.
 */
export function processSqlColumnSchema(schemaSection: any): Field[] {
	if (!schemaSection || !Array.isArray(schemaSection)) return [];

	const fields: Field[] = [];

	for (const col of schemaSection) {
		if (!col || typeof col.column_name !== 'string') continue;

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
			type: col.data_type || 'unknown',
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
export function validateSqlColumnSchema(schema: any): any[] {
	if (!schema) return [];

	if (!Array.isArray(schema)) {
		return [{ message: 'SQL column schema must be an array of columns' }];
	}

	const errors: any[] = [];

	schema.forEach((col: any, index: number) => {
		if (typeof col !== 'object' || col === null) {
			errors.push({ message: `Column at index ${index} is not an object` });
			return;
		}
		if (typeof col.column_name !== 'string' || col.column_name.trim() === '') {
			errors.push({ message: `Column at index ${index} is missing a valid 'column_name' field` });
		}
	});

	return errors;
}
