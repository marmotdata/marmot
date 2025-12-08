import type { Field } from './types';
import Ajv from 'ajv';
import addFormats from 'ajv-formats';

const ajv = new Ajv({
	allErrors: true,
	verbose: true,
	$data: true,
	strict: false
});
addFormats(ajv);

export function resolveRef(ref: string, rootSchema: any): any {
	if (!ref?.startsWith('#/')) return null;

	const parts = ref.substring(2).split('/');
	let current = rootSchema;

	for (const part of parts) {
		if (!current[part]) return null;
		current = current[part];
	}

	return current;
}

export function getSchemaNameFromRef(ref: string): string {
	if (!ref) return 'unknown';
	return ref.split('/').pop() || 'unknown';
}

export function getFieldType(fieldSchema: any): string {
	if (!fieldSchema) return 'unknown';

	if (fieldSchema.$ref) {
		return `ref(${getSchemaNameFromRef(fieldSchema.$ref)})`;
	}

	if (fieldSchema.type === 'array') {
		if (fieldSchema.items?.$ref) {
			return `array<${getSchemaNameFromRef(fieldSchema.items.$ref)}>`;
		}
		const itemType = fieldSchema.items?.type || 'any';
		return `array<${itemType}>`;
	}

	if (fieldSchema.enum) {
		return `enum`;
	}

	if (fieldSchema.anyOf) {
		return 'anyOf';
	}

	if (fieldSchema.oneOf) {
		return 'oneOf';
	}

	if (fieldSchema.allOf) {
		return 'allOf';
	}

	if (fieldSchema.not) {
		return 'not';
	}

	if (fieldSchema.type instanceof Array) {
		return fieldSchema.type.join(' | ');
	}

	return fieldSchema.type || 'any';
}

export function processPatternProperties(
	patternProperties: any,
	rootSchema: any = {},
	depth = 0,
	parentPath = ''
): Field[] {
	const fields: Field[] = [];

	if (!patternProperties || typeof patternProperties !== 'object') {
		return fields;
	}

	Object.entries(patternProperties).forEach(([pattern, schema]) => {
		const fullPath = parentPath ? `${parentPath}.{pattern: ${pattern}}` : `{pattern: ${pattern}}`;
		fields.push({
			name: fullPath,
			type: getFieldType(schema),
			description: schema.description || `Fields matching pattern: ${pattern}`,
			pattern: pattern,
			required: false,
			indentLevel: depth,
			format: schema.format,
			minimum: schema.minimum,
			maximum: schema.maximum,
			minLength: schema.minLength,
			maxLength: schema.maxLength,
			examples: schema.examples || (schema.example ? [schema.example] : undefined),
			enum: schema.enum,
			default: schema.default
		});

		const nestedFields = processSchemaRecursively(
			`{${pattern}}`,
			schema,
			rootSchema,
			depth,
			parentPath
		);
		if (nestedFields.length > 0) {
			fields.push(...nestedFields.slice(1));
		}
	});

	return fields;
}

export function processComposition(
	fieldName: string,
	fieldSchema: any,
	rootSchema: any = {},
	depth = 0,
	parentPath = ''
): Field[] {
	const fields: Field[] = [];
	const fullPath = parentPath ? `${parentPath}.${fieldName}` : fieldName;

	if (fieldSchema.anyOf) {
		fields.push({
			name: fullPath,
			type: 'anyOf',
			description: fieldSchema.description || 'One or more of the following schemas',
			required: false,
			indentLevel: depth
		});

		fieldSchema.anyOf.forEach((schema: any, index: number) => {
			const optionName = schema.title || schema.description || `Option ${index + 1}`;

			if (!schema.properties && !schema.type && fieldSchema.properties) {
				const mergedSchema = {
					type: 'object',
					properties: fieldSchema.properties,
					required: schema.required || []
				};
				fields.push(
					...processSchemaRecursively(optionName, mergedSchema, rootSchema, depth + 1, fullPath)
				);
			} else {
				fields.push(
					...processSchemaRecursively(optionName, schema, rootSchema, depth + 1, fullPath)
				);
			}
		});
	}

	if (fieldSchema.oneOf) {
		fields.push({
			name: fullPath,
			type: 'oneOf',
			description: fieldSchema.description || 'Exactly one of the following schemas',
			required: false,
			indentLevel: depth
		});

		fieldSchema.oneOf.forEach((schema: any, index: number) => {
			const optionName = schema.title || schema.description || `Option ${index + 1}`;

			if (!schema.properties && !schema.type && fieldSchema.properties) {
				const mergedSchema = {
					type: 'object',
					properties: fieldSchema.properties,
					required: schema.required || []
				};
				fields.push(
					...processSchemaRecursively(optionName, mergedSchema, rootSchema, depth + 1, fullPath)
				);
			} else {
				fields.push(
					...processSchemaRecursively(optionName, schema, rootSchema, depth + 1, fullPath)
				);
			}
		});
	}

	if (fieldSchema.allOf) {
		const mergedSchema = { type: 'object', properties: {}, required: [] };

		fieldSchema.allOf.forEach((schema: any) => {
			if (schema.$ref) {
				const resolved = resolveRef(schema.$ref, rootSchema);
				if (resolved) schema = resolved;
			}

			if (schema.properties) {
				Object.assign(mergedSchema.properties, schema.properties);
			}
			if (schema.required) {
				mergedSchema.required.push(...schema.required);
			}
		});

		if (mergedSchema.properties && Object.keys(mergedSchema.properties).length > 0) {
			Object.entries(mergedSchema.properties).forEach(([name, schema]) => {
				const isRequired = mergedSchema.required.includes(name);
				const nestedFields = processSchemaRecursively(name, schema, rootSchema, depth, fullPath);

				if (nestedFields.length > 0) {
					nestedFields[0].required = isRequired;
				}

				fields.push(...nestedFields);
			});
		} else {
			fields.push({
				name: fullPath,
				type: 'allOf',
				description: fieldSchema.description || 'All of the following schemas',
				required: false,
				indentLevel: depth
			});
		}
	}

	if (fieldSchema.not) {
		fields.push({
			name: fullPath,
			type: 'not',
			description: fieldSchema.description || 'Must not match the following schema',
			required: false,
			indentLevel: depth
		});

		fields.push(
			...processSchemaRecursively(
				`${fieldName} (not)`,
				fieldSchema.not,
				rootSchema,
				depth + 1,
				fullPath
			)
		);
	}

	return fields;
}

export function processSchemaRecursively(
	fieldName: string,
	fieldSchema: any,
	rootSchema: any = {},
	depth = 0,
	parentPath = ''
): Field[] {
	if (!fieldSchema) return [];

	const fields: Field[] = [];
	const fullPath = parentPath ? `${parentPath}.${fieldName}` : fieldName;

	if (fieldSchema.$ref) {
		const resolvedSchema = resolveRef(fieldSchema.$ref, rootSchema);
		if (resolvedSchema) {
			return processSchemaRecursively(fieldName, resolvedSchema, rootSchema, depth, parentPath);
		} else {
			fields.push({
				name: fullPath,
				type: `ref(${getSchemaNameFromRef(fieldSchema.$ref)})`,
				description: fieldSchema.description,
				required: false,
				indentLevel: depth
			});
			return fields;
		}
	}

	if (fieldSchema.anyOf || fieldSchema.oneOf || fieldSchema.allOf || fieldSchema.not) {
		return processComposition(fieldName, fieldSchema, rootSchema, depth, parentPath);
	}

	if (fieldSchema.const !== undefined) {
		fields.push({
			name: fullPath,
			type: 'const',
			description: fieldSchema.description,
			format: fieldSchema.format,
			required: false,
			const: fieldSchema.const,
			examples: fieldSchema.examples || (fieldSchema.example ? [fieldSchema.example] : undefined),
			indentLevel: depth
		});
		return fields;
	}

	const field: Field = {
		name: fullPath,
		type: getFieldType(fieldSchema),
		description: fieldSchema.description,
		format: fieldSchema.format,
		required: false,
		enum: fieldSchema.enum,
		default: fieldSchema.default,
		pattern: fieldSchema.pattern,
		minimum: fieldSchema.minimum,
		maximum: fieldSchema.maximum,
		minLength: fieldSchema.minLength,
		maxLength: fieldSchema.maxLength,
		examples: fieldSchema.examples || (fieldSchema.example ? [fieldSchema.example] : undefined),
		const: fieldSchema.const,
		indentLevel: depth
	};

	fields.push(field);

	if (fieldSchema.type === 'object') {
		if (fieldSchema.properties) {
			Object.entries(fieldSchema.properties).forEach(([name, schema]) => {
				const isRequired = (fieldSchema.required || []).includes(name);
				const nestedFields = processSchemaRecursively(
					name,
					schema,
					rootSchema,
					depth + 1,
					fullPath
				);

				if (nestedFields.length > 0) {
					nestedFields[0].required = isRequired;
				}

				fields.push(...nestedFields);
			});
		}

		if (fieldSchema.patternProperties) {
			fields.push(
				...processPatternProperties(fieldSchema.patternProperties, rootSchema, depth + 1, fullPath)
			);
		}
	}

	if (fieldSchema.type === 'array' && fieldSchema.items) {
		if (fieldSchema.items.type === 'object' && fieldSchema.items.properties) {
			Object.entries(fieldSchema.items.properties).forEach(([name, schema]) => {
				const isRequired = (fieldSchema.items.required || []).includes(name);
				const nestedFields = processSchemaRecursively(
					name,
					schema,
					rootSchema,
					depth + 1,
					`${fullPath}[]`
				);

				if (nestedFields.length > 0) {
					nestedFields[0].required = isRequired;
				}

				fields.push(...nestedFields);
			});
		} else {
			const nestedFields = processSchemaRecursively(
				`${fieldName}[]`,
				fieldSchema.items,
				rootSchema,
				depth + 1,
				parentPath
			);
			fields.push(...nestedFields);
		}
	}

	return fields;
}

export function processField(
	fieldName: string,
	fieldSchema: any,
	required: string[] = [],
	rootSchema: any = {},
	depth = 0
): Field[] {
	if (!fieldSchema) return [];

	try {
		const fields = processSchemaRecursively(fieldName, fieldSchema, rootSchema, depth);

		if (fields.length > 0) {
			fields[0].required = required.includes(fieldName);
		}

		return fields;
	} catch (err) {
		console.error(`Error processing field ${fieldName}:`, err);
		return [
			{
				name: fieldName,
				type: 'error',
				description: `Error processing field: ${err.message}`,
				required: false,
				indentLevel: depth
			}
		];
	}
}

export function extractExamples(schema: any): any {
	if (schema.examples && Array.isArray(schema.examples)) {
		return schema.examples.length === 1 ? schema.examples[0] : schema.examples;
	}

	if (schema.example !== undefined) {
		return schema.example;
	}

	return null;
}

export function processJsonSchema(schemaSection: any): Field[] {
	if (!schemaSection) return [];

	try {
		let schema: any;

		if (typeof schemaSection === 'string') {
			if (
				schemaSection.includes('type: object') &&
				(schemaSection.includes('properties:') || schemaSection.includes('\nproperties:'))
			) {
				try {
					const fields: Field[] = [];
					const lines = schemaSection.split('\n');

					const titleMatch = schemaSection.match(/title:\s*(.+?)(\n|$)/);
					const descMatch = schemaSection.match(/description:\s*(.+?)(\n|$)/);

					fields.push({
						name: titleMatch ? titleMatch[1].trim() : 'root',
						type: 'object',
						description: descMatch ? descMatch[1].trim() : undefined,
						required: true,
						indentLevel: 0
					});

					let inProperties = false;
					let propertyIndent = 0;

					for (let i = 0; i < lines.length; i++) {
						const line = lines[i];
						const trimmedLine = line.trim();

						if (trimmedLine === 'properties:') {
							inProperties = true;
							propertyIndent = line.search(/\S/);
							continue;
						}

						if (inProperties) {
							const currentIndent = line.search(/\S/);

							if (currentIndent === propertyIndent + 2 && !trimmedLine.startsWith('-')) {
								const propertyName = trimmedLine.split(':')[0].trim();

								let propertyType = 'unknown';
								let propertyDesc = undefined;

								for (let j = i + 1; j < i + 5 && j < lines.length; j++) {
									const propLine = lines[j].trim();
									if (propLine.startsWith('type:')) {
										propertyType = propLine.split(':')[1].trim();
									} else if (propLine.startsWith('description:')) {
										propertyDesc = propLine.substring(propLine.indexOf(':') + 1).trim();
									}
								}

								fields.push({
									name: propertyName,
									type: propertyType,
									description: propertyDesc,
									required: schemaSection.includes(`required:\n  - ${propertyName}`),
									indentLevel: 1
								});
							}
						}
					}

					return fields;
				} catch (e) {
					return [
						{
							name: 'root',
							type: 'object',
							description: 'JSON Schema in YAML format'
						}
					];
				}
			}

			try {
				schema = JSON.parse(schemaSection);
			} catch (e) {
				return [
					{
						name: 'root',
						type: 'object',
						description: 'JSON Schema (parsing needed)'
					}
				];
			}
		} else {
			schema = schemaSection;
		}

		let fields: Field[] = [];

		if (schema.anyOf || schema.oneOf || schema.allOf || schema.not) {
			fields.push(...processComposition('root', schema, schema, 0, ''));
		}

		if (
			schema.type === 'object' ||
			(Array.isArray(schema.type) && schema.type.includes('object'))
		) {
			if (schema.properties) {
				Object.entries(schema.properties).forEach(([fieldName, fieldSchema]) => {
					const isRequired = (schema.required || []).includes(fieldName);
					const fieldResults = processSchemaRecursively(fieldName, fieldSchema, schema, 0, '');

					if (fieldResults.length > 0) {
						fieldResults[0].required = isRequired;
					}

					fields.push(...fieldResults);
				});
			}

			if (schema.patternProperties) {
				fields.push(...processPatternProperties(schema.patternProperties, schema, 0, ''));
			}
		}

		if (fields.length === 0) {
			fields.push(...processSchemaRecursively('root', schema, schema, 0, ''));
		}

		return fields;
	} catch (error) {
		return [
			{
				name: 'Error',
				type: 'error',
				description: `Failed to process JSON schema: ${error.message}`
			}
		];
	}
}

export function validateJsonSchema(schema: any): any[] {
	if (!schema) return [];

	try {
		let schemaObj = schema;
		if (typeof schema === 'string') {
			try {
				schemaObj = JSON.parse(schema);
			} catch (e) {
				return [{ message: e.message }];
			}
		}

		if (schemaObj && typeof schemaObj === 'object') {
			const schemaCopy = JSON.parse(JSON.stringify(schemaObj));

			if (schemaCopy.example) delete schemaCopy.example;
			if (schemaCopy.examples) delete schemaCopy.examples;

			try {
				ajv.compile(schemaCopy);
				return [];
			} catch (error) {
				return [{ message: error.message }];
			}
		}

		return [];
	} catch (error) {
		return [{ message: error.message }];
	}
}

export function isJsonSchema(schemaSection: any): boolean {
	if (!schemaSection) return false;

	if (schemaSection.$schema?.includes('json-schema')) {
		return true;
	}

	if (
		typeof schemaSection === 'object' &&
		schemaSection !== null &&
		(schemaSection.properties ||
			schemaSection.patternProperties ||
			schemaSection.type === 'object' ||
			schemaSection.allOf ||
			schemaSection.oneOf ||
			schemaSection.anyOf ||
			schemaSection.not)
	) {
		return true;
	}

	try {
		const schemaCopy = JSON.parse(JSON.stringify(schemaSection));
		if (schemaCopy.example) delete schemaCopy.example;
		if (schemaCopy.examples) delete schemaCopy.examples;
		ajv.compile(schemaCopy);
		return true;
	} catch (error) {
		return false;
	}
}
