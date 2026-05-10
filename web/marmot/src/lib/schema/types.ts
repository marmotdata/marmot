export type JsonValue =
	| string
	| number
	| boolean
	| null
	| JsonValue[]
	| { [key: string]: JsonValue };

export interface Field {
	name: string;
	type: string;
	description?: string;
	format?: string;
	required?: boolean;
	fields?: Field[];
	items?: {
		type: string;
		fields?: Field[];
	};
	$ref?: string;
	indentLevel?: number;
	enum?: unknown[];
	default?: unknown;
	pattern?: string;
	minimum?: number;
	maximum?: number;
	minLength?: number;
	maxLength?: number;
	examples?: unknown[];
	const?: unknown;
}

export interface SchemaSection {
	name: string;
	schema: unknown;
}

export interface SchemaProcessingResult {
	fields: Field[];
	example: unknown;
}

export type SchemaType = 'json' | 'avro' | 'protobuf' | 'dbt' | 'sql';
