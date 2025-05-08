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

/**
 * Resolves a JSON Schema reference
 */
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

/**
 * Extracts a schema name from a reference
 */
export function getSchemaNameFromRef(ref: string): string {
  if (!ref) return 'unknown';
  return ref.split('/').pop() || 'unknown';
}

/**
 * Determines the field type from a schema
 */
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

  if (fieldSchema.anyOf || fieldSchema.oneOf) {
    return 'union';
  }

  if (fieldSchema.allOf) {
    return 'intersection';
  }

  if (fieldSchema.type instanceof Array) {
    return fieldSchema.type.join(' | ');
  }

  return fieldSchema.type || 'any';
}

/**
 * Processes a schema field and its children
 */
export function processField(
  fieldName: string,
  fieldSchema: any,
  required: string[] = [],
  rootSchema: any = {},
  depth = 0
): Field[] {
  if (!fieldSchema) return [];

  const fields: Field[] = [];

  try {
    const field: Field = {
      name: fieldName,
      type: getFieldType(fieldSchema),
      description: fieldSchema.description,
      format: fieldSchema.format,
      required: required.includes(fieldName),
      enum: fieldSchema.enum,
      default: fieldSchema.default,
      pattern: fieldSchema.pattern,
      minimum: fieldSchema.minimum,
      maximum: fieldSchema.maximum,
      minLength: fieldSchema.minLength,
      maxLength: fieldSchema.maxLength,
      indentLevel: depth
    };

    fields.push(field);

    // Handle nested object
    if (fieldSchema.type === 'object' && fieldSchema.properties) {
      Object.entries(fieldSchema.properties || {}).forEach(([name, schema]) => {
        fields.push(
          ...processField(
            `${fieldName}.${name}`,
            schema,
            fieldSchema.required || [],
            rootSchema,
            depth + 1
          )
        );
      });
    }

    // Handle array with object items
    if (fieldSchema.type === 'array' && fieldSchema.items) {
      if (fieldSchema.items.type === 'object' && fieldSchema.items.properties) {
        Object.entries(fieldSchema.items.properties).forEach(([name, schema]) => {
          fields.push(
            ...processField(
              `${fieldName}[].${name}`,
              schema,
              fieldSchema.items.required || [],
              rootSchema,
              depth + 1
            )
          );
        });
      }
    }
  } catch (err) {
    console.error(`Error processing field ${fieldName}:`, err);
  }

  return fields;
}

/**
 * Processes a JSON schema using Ajv for validation and metadata
 */
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
          let currentProperty = null;
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

    if (schema.type === 'object' && schema.properties) {
      fields.push({
        name: schema.title || 'root',
        type: 'object',
        description: schema.description,
        required: true,
        indentLevel: 0
      });

      Object.entries(schema.properties).forEach(([fieldName, fieldSchema]) => {
        fields.push({
          name: fieldName,
          type: getFieldType(fieldSchema),
          description: fieldSchema.description,
          required: (schema.required || []).includes(fieldName),
          indentLevel: 1
        });
      });
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

/**
 * Validate a JSON schema using Ajv
 * Returns array of validation errors
 */
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

/**
 * Detects if an object is likely a JSON schema
 */
export function isJsonSchema(schemaSection: any): boolean {
  if (!schemaSection) return false;

  if (schemaSection.$schema?.includes('json-schema')) {
    return true;
  }

  // Check for common JSON Schema properties
  if (
    typeof schemaSection === 'object' &&
    schemaSection !== null &&
    (schemaSection.properties ||
      schemaSection.type === 'object' ||
      schemaSection.allOf ||
      schemaSection.oneOf ||
      schemaSection.anyOf)
  ) {
    return true;
  }

  try {
    const schemaCopy = JSON.parse(JSON.stringify(schemaSection));
    // Remove example fields
    if (schemaCopy.example) delete schemaCopy.example;
    ajv.compile(schemaCopy);
    return true;
  } catch (error) {
    return false;
  }
}
