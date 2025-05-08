import type { Field } from './types';
import protobufjs from 'protobufjs';

/**
 * Determines the field type from a Protobuf field
 */
export function getFieldType(
  field: protobufjs.Field | protobufjs.MapField | protobufjs.OneOf
): string {
  if (!field) return 'unknown';

  // Handle OneOf fields
  if (field instanceof protobufjs.OneOf) {
    return 'oneof';
  }

  // Handle map fields
  if (field instanceof protobufjs.MapField) {
    const keyType = field.keyType;
    const valueType = field.type;
    return `map<${keyType},${valueType}>`;
  }

  // Handle basic field types
  if (field.resolvedType) {
    if (field.resolvedType instanceof protobufjs.Enum) {
      return `enum(${field.resolvedType.name})`;
    }
    if (field.resolvedType instanceof protobufjs.Type) {
      return field.resolvedType.name;
    }
  }

  // Handle repeated fields
  if (field.repeated) {
    return `array<${field.type}>`;
  }

  return field.type;
}

/**
 * Maps Protobuf types to field constraints
 */
export function getFieldConstraints(field: protobufjs.Field): any {
  const constraints: any = {};

  // Add type-specific constraints based on protobuf rules
  switch (field.type) {
    case 'string':
      if (field.options) {
        if (field.options.min_length !== undefined)
          constraints.minLength = field.options.min_length;
        if (field.options.max_length !== undefined)
          constraints.maxLength = field.options.max_length;
        if (field.options.pattern !== undefined) constraints.pattern = field.options.pattern;
      }
      break;
    case 'int32':
    case 'int64':
    case 'uint32':
    case 'uint64':
    case 'sint32':
    case 'sint64':
    case 'fixed32':
    case 'fixed64':
    case 'sfixed32':
    case 'sfixed64':
    case 'float':
    case 'double':
      if (field.options) {
        if (field.options.min !== undefined) constraints.minimum = field.options.min;
        if (field.options.max !== undefined) constraints.maximum = field.options.max;
      }
      break;
  }

  return constraints;
}

/**
 * Processes a message field recursively
 */
export function processMessageField(
  field: protobufjs.Field | protobufjs.MapField | protobufjs.OneOf,
  parentName: string = '',
  depth: number = 0
): Field[] {
  const fields: Field[] = [];
  const fieldPath = parentName ? `${parentName}.${field.name}` : field.name;

  // Convert protobuf field to our Field type
  const fieldData: Field = {
    name: fieldPath,
    type: getFieldType(field),
    description: field.comment || undefined,
    required: field instanceof protobufjs.Field ? field.required : false,
    indentLevel: depth
  };

  // Add field constraints for regular fields
  if (field instanceof protobufjs.Field) {
    const constraints = getFieldConstraints(field);
    Object.assign(fieldData, constraints);
  }

  fields.push(fieldData);

  // Process nested message fields
  if (field instanceof protobufjs.Field && field.resolvedType instanceof protobufjs.Type) {
    const nestedMessage = field.resolvedType;

    // Process all fields in the nested message
    nestedMessage.fieldsArray.forEach((nestedField) => {
      fields.push(
        ...processMessageField(
          nestedField,
          field.repeated ? `${fieldPath}[]` : fieldPath,
          depth + 1
        )
      );
    });

    // Process oneofs in the nested message
    Object.values(nestedMessage.oneofs || {}).forEach((oneOf) => {
      fields.push(
        ...processMessageField(oneOf, field.repeated ? `${fieldPath}[]` : fieldPath, depth + 1)
      );
    });
  }

  // Process oneof fields
  if (field instanceof protobufjs.OneOf) {
    field.fieldsArray.forEach((oneOfField) => {
      fields.push(...processMessageField(oneOfField, `${fieldPath}`, depth + 1));
    });
  }

  return fields;
}

/**
 * Process the complete Protobuf schema
 */
export function processProtobufSchema(schema: any): Field[] {
  if (!schema) return [];

  try {
    if (typeof schema === 'string') {
      const messageMatches = schema.match(/message\s+(\w+)\s*\{[^}]*\}/g) || [];

      if (messageMatches.length > 0) {
        const fields: Field[] = [];

        messageMatches.forEach((messageBlock) => {
          const messageName = messageBlock.match(/message\s+(\w+)/)?.[1] || 'Unknown';
          fields.push({
            name: messageName,
            type: 'message',
            indentLevel: 0
          });

          const fieldMatches = messageBlock.match(/\s+(\w+)\s+(\w+)\s*=\s*(\d+)/g) || [];
          fieldMatches.forEach((fieldMatch) => {
            const parts = fieldMatch.trim().match(/(\w+)\s+(\w+)\s*=\s*(\d+)/);
            if (parts) {
              const [_, fieldType, fieldName] = parts;
              fields.push({
                name: fieldName,
                type: fieldType,
                indentLevel: 1
              });
            }
          });
        });

        return fields;
      }

      return [
        {
          name: 'root',
          type: 'message'
        }
      ];
    }

    return [
      {
        name: 'root',
        type: 'message'
      }
    ];
  } catch (error) {
    return [
      {
        name: 'Error',
        type: 'error',
        description: `Failed to process Protobuf schema: ${error.message}`
      }
    ];
  }
}

/**
 * Get all message types from the root
 */
function getAllMessageTypes(root: protobufjs.Root): protobufjs.Type[] {
  const messageTypes: protobufjs.Type[] = [];

  function traverseNamespace(ns: protobufjs.NamespaceBase) {
    // Add all message types in this namespace
    Object.values(ns.nested || {}).forEach((obj) => {
      if (obj instanceof protobufjs.Type) {
        messageTypes.push(obj);
      }

      // Recursively traverse nested namespaces
      if (obj instanceof protobufjs.Namespace) {
        traverseNamespace(obj);
      }
    });
  }

  traverseNamespace(root);
  return messageTypes;
}

/**
 * Validate a Protobuf schema using protobufjs
 * Returns array of validation errors
 */
export function validateProtobufSchema(schema: any): any[] {
  if (!schema) return [];

  try {
    // Convert schema to string if it's an object
    const schemaStr = typeof schema === 'string' ? schema : JSON.stringify(schema);

    // Try to parse the schema
    protobufjs.parse(schemaStr);
    return [];
  } catch (error) {
    return [{ message: error.message }];
  }
}

/**
 * Determines if the given schema is a Protobuf schema
 */
export function isProtobufSchema(schema: any): boolean {
  if (!schema) return false;

  // Check for common Protobuf schema properties
  if (schema.syntax && (schema.syntax === 'proto2' || schema.syntax === 'proto3')) {
    return true;
  }

  if (
    schema.nested &&
    Object.values(schema.nested).some(
      (obj) =>
        typeof obj === 'object' &&
        obj !== null &&
        ('fields' in obj || 'values' in obj || 'methods' in obj)
    )
  ) {
    return true;
  }

  // Try to parse with protobufjs
  try {
    if (typeof schema === 'string') {
      protobufjs.parse(schema);
    } else {
      const root = new protobufjs.Root();
      root.addJSON(schema);
    }
    return true;
  } catch (error) {
    return false;
  }
}
