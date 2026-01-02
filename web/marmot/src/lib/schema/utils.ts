import type { Field, SchemaSection, SchemaProcessingResult, SchemaType } from './types';
import { processJsonSchema, isJsonSchema, validateJsonSchema } from './json';
import { processAvroSchema, isAvroSchema, validateAvroSchema } from './avro';
import { processProtobufSchema, isProtobufSchema, validateProtobufSchema } from './protobuf';
import { processDbtSchema, isDbtSchema, validateDbtSchema } from './dbt';

/**
 * Format example (ensures JSON is properly parsed if it's a string)
 */
export function formatExample(example: any): any {
	if (Array.isArray(example)) {
		return example.length === 1 ? example[0] : example;
	}

	if (typeof example === 'string') {
		try {
			return JSON.parse(example);
		} catch (e) {
			return example;
		}
	}
	return example;
}

/**
 * Parse a response into schema sections
 */
export function parseSchemaResponse(response: any): SchemaSection[] {
	if (!response) return [];

	const sections: SchemaSection[] = [];

	if (typeof response === 'object') {
		Object.entries(response).forEach(([name, schemaContent]) => {
			// Parse JSON strings that come from API responses
			let parsedSchema = schemaContent;
			if (typeof schemaContent === 'string') {
				try {
					parsedSchema = JSON.parse(schemaContent);
				} catch (e) {
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

export function isSchemaAvailable(schemaSection: any): boolean {
	if (!schemaSection) return false;

	if (typeof schemaSection === 'string') {
		try {
			const parsed = JSON.parse(schemaSection);
			return isSchemaAvailable(parsed);
		} catch (e) {
			// For non-JSON strings (YAML, Avro, Protobuf)
			return isStringSchema(schemaSection);
		}
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

export function detectSchemaType(schemaSection: any): SchemaType {
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

	// Object-based detection logic - check dbt first since it's a simple array format
	if (isDbtSchema(schemaSection)) return 'dbt';
	if (isJsonSchema(schemaSection)) return 'json';
	if (isAvroSchema(schemaSection)) return 'avro';
	if (isProtobufSchema(schemaSection)) return 'protobuf';
	return 'json';
}

export function processSchema(schemaSection: any): SchemaProcessingResult {
	if (!schemaSection) {
		return { fields: [], example: null };
	}

	let processableSchema = schemaSection;
	let example = null;

	if (typeof schemaSection === 'string') {
		try {
			processableSchema = JSON.parse(schemaSection);

			if (processableSchema.examples) {
				example = formatExample(processableSchema.examples);
			} else if (processableSchema.example) {
				example = formatExample(processableSchema.example);
			}
		} catch (e) {}
	} else if (typeof schemaSection === 'object' && schemaSection !== null) {
		if (schemaSection.examples) {
			example = formatExample(schemaSection.examples);
		} else if (schemaSection.example) {
			example = formatExample(schemaSection.example);
		}
	}

	const schemaType = detectSchemaType(processableSchema);
	let fields: Field[] = [];

	try {
		switch (schemaType) {
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
export function validateSchema(schema: any): any[] {
	if (!schema) return [];

	let cleanSchema = schema;

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

			// Remove examples for validation
			if (cleanSchema.example) delete cleanSchema.example;
			if (cleanSchema.examples) delete cleanSchema.examples;

			if (cleanSchema.properties) {
				Object.keys(cleanSchema.properties).forEach((key) => {
					if (cleanSchema.properties[key].example) {
						delete cleanSchema.properties[key].example;
					}
					if (cleanSchema.properties[key].examples) {
						delete cleanSchema.properties[key].examples;
					}
				});
			}

			if (cleanSchema.patternProperties) {
				Object.keys(cleanSchema.patternProperties).forEach((pattern) => {
					if (cleanSchema.patternProperties[pattern].example) {
						delete cleanSchema.patternProperties[pattern].example;
					}
					if (cleanSchema.patternProperties[pattern].examples) {
						delete cleanSchema.patternProperties[pattern].examples;
					}
				});
			}
		}

		const schemaType = detectSchemaType(cleanSchema);

		switch (schemaType) {
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
	} catch (e) {
		return schema;
	}
}
