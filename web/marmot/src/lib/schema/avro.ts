import type { Field } from './types';
import avsc from 'avsc';

type AvroSchemaValue = string | AvroSchemaObject | AvroSchemaValue[] | null | undefined;

interface AvroSchemaObject {
	type?: AvroSchemaValue;
	name?: string;
	namespace?: string;
	doc?: string;
	fields?: AvroSchemaObject[];
	items?: AvroSchemaValue;
	values?: AvroSchemaValue;
	symbols?: string[];
	default?: unknown;
	[key: string]: unknown;
}

function isObject(value: unknown): value is AvroSchemaObject {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Resolves an Avro schema reference/namespace
 */
export function resolveNamespace(namespace: string, name: string): string {
	return namespace ? `${namespace}.${name}` : name;
}

/**
 * Determines the field type from an Avro schema
 */
export function getFieldType(fieldSchema: AvroSchemaObject | null | undefined): string {
	if (!fieldSchema) return 'unknown';

	// Handle union types (array of types)
	if (Array.isArray(fieldSchema.type)) {
		// Filter out null type for optional fields
		const types = fieldSchema.type.filter((t) => t !== 'null');
		if (types.length === 1) {
			const t = types[0];
			return typeof t === 'string' ? t : 'complex';
		}
		return types.map((t) => (typeof t === 'string' ? t : 'complex')).join(' | ');
	}

	// Handle primitive types
	if (typeof fieldSchema.type === 'string') {
		if (fieldSchema.type === 'array' && fieldSchema.items) {
			if (typeof fieldSchema.items === 'string') {
				return `array<${fieldSchema.items}>`;
			} else if (isObject(fieldSchema.items) && fieldSchema.items.type) {
				if (fieldSchema.items.type === 'record') {
					return `array<${fieldSchema.items.name ?? ''}>`;
				}
				return `array<${String(fieldSchema.items.type)}>`;
			}
		}

		if (fieldSchema.type === 'enum' && fieldSchema.symbols) {
			return 'enum';
		}

		return fieldSchema.type;
	}

	// Handle complex types (record, enum, fixed, etc.)
	if (isObject(fieldSchema.type)) {
		const inner = fieldSchema.type;
		if (inner.type === 'record') {
			return inner.name ?? 'record';
		}
		if (inner.type === 'enum') {
			return 'enum';
		}
		if (inner.type === 'fixed') {
			return 'fixed';
		}
		if (inner.type === 'array') {
			if (typeof inner.items === 'string') {
				return `array<${inner.items}>`;
			} else if (isObject(inner.items) && inner.items.type) {
				return `array<${String(inner.items.type)}>`;
			}
		}
		if (inner.type === 'map') {
			if (typeof inner.values === 'string') {
				return `map<${inner.values}>`;
			} else if (isObject(inner.values) && inner.values.type) {
				return `map<${String(inner.values.type)}>`;
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
	fieldSchema: AvroSchemaObject,
	namespace: string = '',
	depth: number = 0
): Field[] {
	const fields: Field[] = [];

	// Default field properties
	const field: Field = {
		name: fieldName,
		type: getFieldType(fieldSchema),
		description: fieldSchema.doc,
		required:
			!Array.isArray(fieldSchema.type) || !(fieldSchema.type as AvroSchemaValue[]).includes('null'),
		default: fieldSchema.default,
		indentLevel: depth
	};

	fields.push(field);

	// Process nested record fields
	if (fieldSchema.type === 'record' && fieldSchema.fields) {
		const recordNamespace = resolveNamespace(namespace, fieldSchema.name ?? '');
		fieldSchema.fields.forEach((nestedField: AvroSchemaObject) => {
			fields.push(...processField(nestedField.name ?? '', nestedField, recordNamespace, depth + 1));
		});
	}
	// Process array items if they are records
	else if (fieldSchema.type === 'array' && isObject(fieldSchema.items)) {
		if (fieldSchema.items.type === 'record' && fieldSchema.items.fields) {
			fieldSchema.items.fields.forEach((nestedField: AvroSchemaObject) => {
				fields.push(
					...processField(
						`${fieldName}[].${nestedField.name ?? ''}`,
						nestedField,
						namespace,
						depth + 1
					)
				);
			});
		}
	}
	// Process union types (arrays in Avro)
	else if (Array.isArray(fieldSchema.type)) {
		// Look for record types in union
		fieldSchema.type.forEach((unionType) => {
			if (isObject(unionType) && unionType.type === 'record' && unionType.fields) {
				unionType.fields.forEach((nestedField: AvroSchemaObject) => {
					fields.push(
						...processField(
							`${fieldName}.${nestedField.name ?? ''}`,
							nestedField,
							namespace,
							depth + 1
						)
					);
				});
			}
		});
	}
	// Process nested record type objects
	else if (isObject(fieldSchema.type)) {
		const inner = fieldSchema.type;
		if (inner.type === 'record' && inner.fields) {
			inner.fields.forEach((nestedField: AvroSchemaObject) => {
				fields.push(
					...processField(
						`${fieldName}.${nestedField.name ?? ''}`,
						nestedField,
						namespace,
						depth + 1
					)
				);
			});
		} else if (inner.type === 'enum' && inner.symbols) {
			// Add enum values as a property
			field.enum = inner.symbols;
		}
	}

	return fields;
}

/**
 * Process the complete Avro schema
 */
export function processAvroSchema(schema: unknown): Field[] {
	if (!schema) return [];

	try {
		let avroSchema: AvroSchemaObject;

		if (typeof schema === 'string') {
			if (schema.includes('type: record') && schema.includes('fields:')) {
				try {
					const fields: Field[] = [];
					const lines = schema.split('\n');

					const nameMatch = schema.match(/name:\s*([\w.]+)/);
					const name = nameMatch ? nameMatch[1] : 'root';

					const docMatch = schema.match(/doc:\s*(.+)/);
					fields.push({
						name,
						type: 'record',
						description: docMatch ? docMatch[1] : undefined,
						required: true,
						indentLevel: 0
					});

					for (let i = 0; i < lines.length; i++) {
						const line = lines[i];

						if (line.trim().startsWith('- name:')) {
							const fieldNameMatch = line.match(/name:\s*(\w+)/);
							const fieldName = fieldNameMatch ? fieldNameMatch[1] : 'unknown';
							const typeMatch = lines[i + 1]?.match(/type:\s*(\w+|\[.*\])/);
							const fieldDocMatch = lines
								.slice(i, i + 3)
								.join(' ')
								.match(/doc:\s*(.+?)(\s+\w+:|$)/);

							fields.push({
								name: fieldName,
								type: typeMatch ? typeMatch[1] : 'unknown',
								description: fieldDocMatch ? fieldDocMatch[1].trim() : undefined,
								required: true,
								indentLevel: 1
							});
						}
					}

					return fields;
				} catch {
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
			} catch {
				return [
					{
						name: 'root',
						type: 'record',
						description: 'Avro schema (parsing needed)'
					}
				];
			}
		} else {
			avroSchema = schema as AvroSchemaObject;
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

			avroSchema.fields.forEach((field: AvroSchemaObject) => {
				fields.push(...processField(field.name ?? '', field, avroSchema.namespace ?? '', 1));
			});
		}

		return fields;
	} catch (error) {
		return [
			{
				name: 'Error',
				type: 'error',
				description: `Failed to process Avro schema: ${error instanceof Error ? error.message : String(error)}`
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
	} catch {
		// Not valid JSON, return as is
		return schema;
	}
}

/**
 * Validate an Avro schema using avsc
 * Returns array of validation errors
 */
export function validateAvroSchema(schema: unknown): { message: string }[] {
	if (!schema) return [];

	try {
		// Use avsc to parse and validate the schema
		avsc.Type.forSchema(schema as avsc.Schema);
		return [];
	} catch (error) {
		return [{ message: error instanceof Error ? error.message : String(error) }];
	}
}

/**
 * Determines if the given schema is an Avro schema
 */
export function isAvroSchema(schema: unknown): boolean {
	if (!schema) return false;

	// Check for common Avro schema properties
	if (
		isObject(schema) &&
		schema.type === 'record' &&
		schema.fields &&
		Array.isArray(schema.fields)
	) {
		return true;
	}

	// Try to parse with avsc
	try {
		avsc.Type.forSchema(schema as avsc.Schema);
		return true;
	} catch {
		return false;
	}
}
