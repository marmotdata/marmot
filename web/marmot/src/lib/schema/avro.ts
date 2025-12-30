import type { Field } from './types';
import avsc from 'avsc';

/**
 * Resolves an Avro schema reference/namespace
 */
export function resolveNamespace(namespace: string, name: string): string {
	return namespace ? `${namespace}.${name}` : name;
}

/**
 * Determines the field type from an Avro schema
 */
export function getFieldType(fieldSchema: any): string {
	if (!fieldSchema) return 'unknown';

	// Handle union types (array of types)
	if (Array.isArray(fieldSchema.type)) {
		// Filter out null type for optional fields
		const types = fieldSchema.type.filter((t: any) => t !== 'null');
		if (types.length === 1) {
			return types[0];
		}
		return types.join(' | ');
	}

	// Handle primitive types
	if (typeof fieldSchema.type === 'string') {
		if (fieldSchema.type === 'array' && fieldSchema.items) {
			if (typeof fieldSchema.items === 'string') {
				return `array<${fieldSchema.items}>`;
			} else if (fieldSchema.items.type) {
				if (fieldSchema.items.type === 'record') {
					return `array<${fieldSchema.items.name}>`;
				}
				return `array<${fieldSchema.items.type}>`;
			}
		}

		if (fieldSchema.type === 'enum' && fieldSchema.symbols) {
			return 'enum';
		}

		return fieldSchema.type;
	}

	// Handle complex types (record, enum, fixed, etc.)
	if (typeof fieldSchema.type === 'object') {
		if (fieldSchema.type.type === 'record') {
			return fieldSchema.type.name;
		}
		if (fieldSchema.type.type === 'enum') {
			return 'enum';
		}
		if (fieldSchema.type.type === 'fixed') {
			return 'fixed';
		}
		if (fieldSchema.type.type === 'array') {
			if (typeof fieldSchema.type.items === 'string') {
				return `array<${fieldSchema.type.items}>`;
			} else if (fieldSchema.type.items.type) {
				return `array<${fieldSchema.type.items.type}>`;
			}
		}
		if (fieldSchema.type.type === 'map') {
			if (typeof fieldSchema.type.values === 'string') {
				return `map<${fieldSchema.type.values}>`;
			} else if (fieldSchema.type.values.type) {
				return `map<${fieldSchema.type.values.type}>`;
			}
		}
	}

	return 'complex';
}

/**
 * Processes an Avro schema field and its children recursively
 */
export function processField(
	fieldName: string,
	fieldSchema: any,
	namespace: string = '',
	depth: number = 0
): Field[] {
	const fields: Field[] = [];

	// Default field properties
	const field: Field = {
		name: fieldName,
		type: getFieldType(fieldSchema),
		description: fieldSchema.doc,
		required: !Array.isArray(fieldSchema.type) || !fieldSchema.type.includes('null'),
		default: fieldSchema.default,
		indentLevel: depth
	};

	fields.push(field);

	// Process nested record fields
	if (fieldSchema.type === 'record' && fieldSchema.fields) {
		const recordNamespace = resolveNamespace(namespace, fieldSchema.name);
		fieldSchema.fields.forEach((nestedField: any) => {
			fields.push(...processField(nestedField.name, nestedField, recordNamespace, depth + 1));
		});
	}
	// Process array items if they are records
	else if (fieldSchema.type === 'array' && typeof fieldSchema.items === 'object') {
		if (fieldSchema.items.type === 'record' && fieldSchema.items.fields) {
			fieldSchema.items.fields.forEach((nestedField: any) => {
				fields.push(
					...processField(`${fieldName}[].${nestedField.name}`, nestedField, namespace, depth + 1)
				);
			});
		}
	}
	// Process union types (arrays in Avro)
	else if (Array.isArray(fieldSchema.type)) {
		// Look for record types in union
		fieldSchema.type.forEach((unionType: any) => {
			if (typeof unionType === 'object' && unionType.type === 'record' && unionType.fields) {
				unionType.fields.forEach((nestedField: any) => {
					fields.push(
						...processField(`${fieldName}.${nestedField.name}`, nestedField, namespace, depth + 1)
					);
				});
			}
		});
	}
	// Process nested record type objects
	else if (typeof fieldSchema.type === 'object') {
		if (fieldSchema.type.type === 'record' && fieldSchema.type.fields) {
			fieldSchema.type.fields.forEach((nestedField: any) => {
				fields.push(
					...processField(`${fieldName}.${nestedField.name}`, nestedField, namespace, depth + 1)
				);
			});
		} else if (fieldSchema.type.type === 'enum' && fieldSchema.type.symbols) {
			// Add enum values as a property
			field.enum = fieldSchema.type.symbols;
		}
	}

	return fields;
}

/**
 * Process the complete Avro schema
 */
export function processAvroSchema(schema: any): Field[] {
	if (!schema) return [];

	try {
		let avroSchema: any;

		if (typeof schema === 'string') {
			if (schema.includes('type: record') && schema.includes('fields:')) {
				try {
					const fields: Field[] = [];
					const lines = schema.split('\n');

					const nameMatch = schema.match(/name:\s*([\w.]+)/);
					const namespaceMatch = schema.match(/namespace:\s*([\w.]+)/);
					const name = nameMatch ? nameMatch[1] : 'root';
					const namespace = namespaceMatch ? namespaceMatch[1] : '';

					fields.push({
						name,
						type: 'record',
						description: schema.match(/doc:\s*(.+)/) ? schema.match(/doc:\s*(.+)/)[1] : undefined,
						required: true,
						indentLevel: 0
					});

					const currentIndent = 0;
					const currentField = null;

					for (let i = 0; i < lines.length; i++) {
						const line = lines[i];

						if (line.trim().startsWith('- name:')) {
							const fieldName = line.match(/name:\s*(\w+)/)[1];
							const typeMatch = lines[i + 1]?.match(/type:\s*(\w+|\[.*\])/);
							const docMatch = lines
								.slice(i, i + 3)
								.join(' ')
								.match(/doc:\s*(.+?)(\s+\w+:|$)/);

							fields.push({
								name: fieldName,
								type: typeMatch ? typeMatch[1] : 'unknown',
								description: docMatch ? docMatch[1].trim() : undefined,
								required: true,
								indentLevel: 1
							});
						}
					}

					return fields;
				} catch (e) {
					return [
						{
							name: 'root',
							type: 'record',
							description: 'Avro schema in YAML format'
						}
					];
				}
			}

			try {
				avroSchema = JSON.parse(schema);
			} catch (e) {
				return [
					{
						name: 'root',
						type: 'record',
						description: 'Avro schema (parsing needed)'
					}
				];
			}
		} else {
			avroSchema = schema;
		}

		const fields: Field[] = [];

		if (avroSchema.type === 'record' && avroSchema.fields) {
			fields.push({
				name: avroSchema.name || 'root',
				type: 'record',
				description: avroSchema.doc,
				required: true,
				indentLevel: 0
			});

			avroSchema.fields.forEach((field: any) => {
				fields.push(...processField(field.name, field, avroSchema.namespace, 1));
			});
		}

		return fields;
	} catch (error) {
		return [
			{
				name: 'Error',
				type: 'error',
				description: `Failed to process Avro schema: ${error.message}`
			}
		];
	}
}

// Add this to utils.ts for highlighting
export function prettyPrintSchema(schema: string): string {
	// Check for YAML format
	if (
		schema.includes('type: record') ||
		schema.includes('type: object') ||
		schema.trim().startsWith('fields:') ||
		schema.includes('\nfields:')
	) {
		// It's YAML, return as is
		return schema;
	}

	// Try parsing as JSON
	try {
		const parsed = JSON.parse(schema);
		return JSON.stringify(parsed, null, 2);
	} catch (e) {
		// Not valid JSON, return as is
		return schema;
	}
}

/**
 * Validate an Avro schema using avsc
 * Returns array of validation errors
 */
export function validateAvroSchema(schema: any): any[] {
	if (!schema) return [];

	try {
		// Use avsc to parse and validate the schema
		avsc.Type.forSchema(schema);
		return [];
	} catch (error) {
		return [{ message: error.message }];
	}
}

/**
 * Determines if the given schema is an Avro schema
 */
export function isAvroSchema(schema: any): boolean {
	if (!schema) return false;

	// Check for common Avro schema properties
	if (schema.type === 'record' && schema.fields && Array.isArray(schema.fields)) {
		return true;
	}

	// Try to parse with avsc
	try {
		avsc.Type.forSchema(schema);
		return true;
	} catch (error) {
		return false;
	}
}
