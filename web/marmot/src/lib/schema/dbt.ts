import type { Field } from './types';

/**
 * dbt column schema format:
 * [
 *   { "name": "column_name", "type": "INTEGER", "description": "Column description" },
 *   ...
 * ]
 */
export interface DbtColumn {
	name: string;
	type: string;
	description?: string;
}

/**
 * Check if the schema is a dbt column array format
 */
export function isDbtSchema(schemaSection: any): boolean {
	if (!schemaSection) return false;

	// Must be an array
	if (!Array.isArray(schemaSection)) return false;

	// Empty array is valid dbt schema (table with no columns yet)
	if (schemaSection.length === 0) return true;

	// Check if first element looks like a dbt column
	const first = schemaSection[0];
	if (typeof first !== 'object' || first === null) return false;

	// dbt columns have 'name' and optionally 'type' and 'description'
	return typeof first.name === 'string';
}

/**
 * Process dbt column array into Field[] for display
 */
export function processDbtSchema(schemaSection: any): Field[] {
	if (!schemaSection || !Array.isArray(schemaSection)) return [];

	const fields: Field[] = [];

	for (const col of schemaSection as DbtColumn[]) {
		if (!col || typeof col.name !== 'string') continue;

		fields.push({
			name: col.name,
			type: col.type || 'unknown',
			description: col.description,
			// dbt doesn't have required/optional concept - all columns exist in the table
			// Leave required undefined so the UI doesn't show the badge
			indentLevel: 0
		});
	}

	return fields;
}

/**
 * Validate dbt schema - basic structure check
 */
export function validateDbtSchema(schema: any): any[] {
	if (!schema) return [];

	if (!Array.isArray(schema)) {
		return [{ message: 'dbt schema must be an array of columns' }];
	}

	const errors: any[] = [];

	schema.forEach((col, index) => {
		if (typeof col !== 'object' || col === null) {
			errors.push({ message: `Column at index ${index} is not an object` });
			return;
		}
		if (typeof col.name !== 'string' || col.name.trim() === '') {
			errors.push({ message: `Column at index ${index} is missing a valid 'name' field` });
		}
	});

	return errors;
}
