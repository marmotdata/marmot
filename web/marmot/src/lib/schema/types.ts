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
	enum?: any[];
	default?: any;
	pattern?: string;
	minimum?: number;
	maximum?: number;
	minLength?: number;
	maxLength?: number;
	examples?: any[];
	const?: any;
}

export interface SchemaSection {
	name: string;
	schema: any;
}

export interface SchemaProcessingResult {
	fields: Field[];
	example: any;
}

export type SchemaType = 'json' | 'avro' | 'protobuf';
