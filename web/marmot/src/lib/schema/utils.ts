import type { Field, SchemaSection, SchemaProcessingResult, SchemaType } from './types';
import { processJsonSchema, isJsonSchema, validateJsonSchema } from './json';
import { processAvroSchema, isAvroSchema, validateAvroSchema } from './avro';
import { processProtobufSchema, isProtobufSchema, validateProtobufSchema } from './protobuf';
import { processDbtSchema, isDbtSchema, validateDbtSchema } from './dbt';
import { processSqlColumnSchema, isSqlColumnSchema, validateSqlColumnSchema } from './sql';

/**
 * Format example (ensures JSON is properly parsed if it's a string)
 */
export function formatExample(example: unknown): unknown {
	if (Array.isArray(example)) {
		return example.length === 1 ? example[0] : example;
	}

	if (typeof example === 'string') {
		try {
			return JSON.parse(example);
		} catch {
			return example;
		}
	}
	return example;
}

/**
 * Parse a response into schema sections
 */
export function parseSchemaResponse(response: unknown): SchemaSection[] {
	if (!response) return [];

	const sections: SchemaSection[] = [];

	if (typeof response === 'object') {
		Object.entries(response as Record<string, unknown>).forEach(([name, schemaContent]) => {
			// Parse JSON strings that come from API responses
			let parsedSchema: unknown = schemaContent;
			if (typeof schemaContent === 'string') {
				try {
					parsedSchema = JSON.parse(schemaContent);
				} catch {
					// Keep as string if it's not JSON (could be YAML, protobuf, etc.)
					parsedSchema = schemaContent;
				}
			}

			sections.push({
				name: name,
				schema: parsedSchema
			});
		});
	}

	return sections.length > 0 ? sections : [{ name: 'schema', schema: response }];
}

export function isSchemaAvailable(schemaSection: unknown): boolean {
	if (!schemaSection) return false;

	if (typeof schemaSection === 'string') {
		try {
			const parsed = JSON.parse(schemaSection);
			return isSchemaAvailable(parsed);
		} catch {
			// For non-JSON strings (YAML, Avro, Protobuf)
			return isStringSchema(schemaSection);
		}
	}

	if (isSqlColumnSchema(schemaSection)) {
		return true;
	}

	if (isDbtSchema(schemaSection)) {
		return true;
	}

	if (isJsonSchema(schemaSection)) {
		return true;
	}

	if (isAvroSchema(schemaSection)) {
		return true;
	}

	if (isProtobufSchema(schemaSection)) {
		return true;
	}

	return false;
}

function isStringSchema(str: string): boolean {
	// YAML Avro detection
	if (str.includes('type: record') && str.includes('fields:')) return true;
	if (str.includes('type: array') || str.includes('type: enum')) return true;

	// JSON Schema indicators
	if (str.includes('"$schema"') || str.includes('$schema:')) return true;

	// Avro indicators
	if (str.includes('"type":"record"') || str.includes('type: record')) return true;

	// Protobuf indicators
	if (str.includes('syntax = "proto') || str.includes('message ') || str.includes('package '))
		return true;

	// JSON Schema in YAML
	if (str.includes('type: object') && str.includes('properties:')) return true;

	// Pattern properties indicator
	if (str.includes('patternProperties:') || str.includes('"patternProperties"')) return true;

	return false;
}

export function detectSchemaType(schemaSection: unknown): SchemaType {
	if (!schemaSection) return 'json';

	if (typeof schemaSection === 'string') {
		const str = schemaSection.toLowerCase();

		// YAML schema detection
		if (str.includes('$schema:') && str.includes('type: object')) {
			return 'json';
		}

		// YAML Avro detection
		if (str.includes('type: record') && str.includes('fields:')) {
			return 'avro';
		}

		// JSON schema detection
		if (str.includes('"$schema"') || str.includes('"type":"object"')) {
			return 'json';
		}

		// Avro schema detection
		if (str.includes('"type":"record"') || str.includes('"fields":')) {
			return 'avro';
		}

		// Protobuf detection
		if (str.includes('syntax = "proto') || str.includes('message ')) {
			return 'protobuf';
		}

		// Default to JSON
		return 'json';
	}

	// Object-based detection logic - check sql before dbt since both are arrays
	// but sql columns use `column_name` while dbt uses `name`
	if (isSqlColumnSchema(schemaSection)) return 'sql';
	if (isDbtSchema(schemaSection)) return 'dbt';
	if (isJsonSchema(schemaSection)) return 'json';
	if (isAvroSchema(schemaSection)) return 'avro';
	if (isProtobufSchema(schemaSection)) return 'protobuf';
	return 'json';
}

export function processSchema(schemaSection: unknown): SchemaProcessingResult {
	if (!schemaSection) {
		return { fields: [], example: null };
	}

	let processableSchema: unknown = schemaSection;
	let example: unknown = null;

	if (typeof schemaSection === 'string') {
		try {
			processableSchema = JSON.parse(schemaSection);
			const parsed = processableSchema as Record<string, unknown>;

			if (parsed.examples) {
				example = formatExample(parsed.examples);
			} else if (parsed.example) {
				example = formatExample(parsed.example);
			}
		} catch {
			/* ignore */
		}
	} else if (typeof schemaSection === 'object' && schemaSection !== null) {
		const obj = schemaSection as Record<string, unknown>;
		if (obj.examples) {
			example = formatExample(obj.examples);
		} else if (obj.example) {
			example = formatExample(obj.example);
		}
	}

	const schemaType = detectSchemaType(processableSchema);
	let fields: Field[] = [];

	try {
		switch (schemaType) {
			case 'sql':
				fields = processSqlColumnSchema(processableSchema);
				break;
			case 'dbt':
				fields = processDbtSchema(processableSchema);
				break;
			case 'avro':
				fields = processAvroSchema(processableSchema);
				break;
			case 'protobuf':
				fields = processProtobufSchema(processableSchema);
				break;
			case 'json':
			default:
				fields = processJsonSchema(processableSchema);
				break;
		}
	} catch (error) {
		console.error('Error processing schema:', error);
		fields = [
			{
				name: 'Error',
				type: 'error',
				description: `Failed to process schema: ${error instanceof Error ? error.message : String(error)}`
			}
		];
	}

	return { fields, example };
}

/**
 * Validate a schema based on its type
 */
export function validateSchema(schema: unknown): unknown[] {
	if (!schema) return [];

	let cleanSchema: unknown = schema;

	try {
		if (typeof schema === 'string') {
			try {
				cleanSchema = JSON.parse(schema);
			} catch (e) {
				// If it's not JSON, check if it's a valid string schema format
				if (isStringSchema(schema)) {
					// For string schemas that aren't JSON, we can't easily validate
					// Return empty array (no errors) for now
					return [];
				}
				return [
					{ message: `Invalid schema format: ${e instanceof Error ? e.message : String(e)}` }
				];
			}
		}

		if (typeof cleanSchema === 'object' && cleanSchema !== null) {
			cleanSchema = JSON.parse(JSON.stringify(cleanSchema));
			const obj = cleanSchema as Record<string, unknown>;

			// Remove examples for validation
			if (obj.example) delete obj.example;
			if (obj.examples) delete obj.examples;

			if (obj.properties && typeof obj.properties === 'object') {
				const properties = obj.properties as Record<string, Record<string, unknown>>;
				Object.keys(properties).forEach((key) => {
					if (properties[key].example) {
						delete properties[key].example;
					}
					if (properties[key].examples) {
						delete properties[key].examples;
					}
				});
			}

			if (obj.patternProperties && typeof obj.patternProperties === 'object') {
				const patternProperties = obj.patternProperties as Record<string, Record<string, unknown>>;
				Object.keys(patternProperties).forEach((pattern) => {
					if (patternProperties[pattern].example) {
						delete patternProperties[pattern].example;
					}
					if (patternProperties[pattern].examples) {
						delete patternProperties[pattern].examples;
					}
				});
			}
		}

		const schemaType = detectSchemaType(cleanSchema);

		switch (schemaType) {
			case 'sql':
				return validateSqlColumnSchema(cleanSchema);
			case 'dbt':
				return validateDbtSchema(cleanSchema);
			case 'json':
				return validateJsonSchema(cleanSchema);
			case 'avro':
				return validateAvroSchema(cleanSchema);
			case 'protobuf':
				return validateProtobufSchema(cleanSchema);
			default:
				return [];
		}
	} catch (error) {
		return [
			{
				message: `Schema validation error: ${error instanceof Error ? error.message : String(error)}`
			}
		];
	}
}

export function prettyPrintSchema(schema: string): string {
	try {
		const parsed = JSON.parse(schema);
		return JSON.stringify(parsed, null, 2);
	} catch {
		return schema;
	}
}
