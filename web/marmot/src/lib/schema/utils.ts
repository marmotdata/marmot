import type { Field, SchemaSection, SchemaProcessingResult, SchemaType } from './types';
import { processJsonSchema, isJsonSchema, validateJsonSchema } from './json';
import { processAvroSchema, isAvroSchema, validateAvroSchema } from './avro';
import { processProtobufSchema, isProtobufSchema, validateProtobufSchema } from './protobuf';

/**
 * Format example (ensures JSON is properly parsed if it's a string)
 */
export function formatExample(example: any): any {
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
      sections.push({
        name: name,
        schema: schemaContent
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

  // Object-based detection logic
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
      if (processableSchema.example) {
        example = formatExample(processableSchema.example);
      }
    } catch (e) { }
  } else if (typeof schemaSection === 'object' && schemaSection !== null) {
    if (schemaSection.example) {
      example = formatExample(schemaSection.example);
    }
  }

  const schemaType = detectSchemaType(processableSchema);
  let fields: Field[] = [];

  try {
    switch (schemaType) {
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
        description: `Failed to process schema: ${error.message}`
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
        return [];
      }
    }

    if (typeof cleanSchema === 'object' && cleanSchema !== null) {
      cleanSchema = JSON.parse(JSON.stringify(cleanSchema));
      if (cleanSchema.example) delete cleanSchema.example;

      if (cleanSchema.properties) {
        Object.keys(cleanSchema.properties).forEach((key) => {
          if (cleanSchema.properties[key].example) {
            delete cleanSchema.properties[key].example;
          }
        });
      }
    }

    const schemaType = detectSchemaType(cleanSchema);

    switch (schemaType) {
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
    return [{ message: `Schema validation error: ${error.message}` }];
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
