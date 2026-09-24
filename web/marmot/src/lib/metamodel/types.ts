export interface MetamodelPresentation {
	labelKey?: string;
	helpTextKey?: string;
	descriptionKey?: string;
	section?: string;
	order?: number;
	/** Alternate editor for a string field's value; the stored type is unchanged. Only "user" exists. */
	control?: string;
	/** Offer this field as a segmented Discover filter. Only enum and boolean fields qualify. */
	facet?: boolean;
}

export interface MetamodelConstraints {
	minimum?: number;
	maximum?: number;
	minLength?: number;
	maxLength?: number;
	minItems?: number;
	maxItems?: number;
}

export interface MetamodelAppliesTo {
	kinds?: string[];
	assetTypes?: string[];
}

export interface MetamodelField {
	id: string;
	type: string;
	itemType?: string;
	core: boolean;
	required: boolean;
	nullable?: boolean;
	storage: string;
	appliesTo: MetamodelAppliesTo;
	values?: string[];
	validation?: MetamodelConstraints;
	presentation?: MetamodelPresentation;
}

export interface MetamodelSchema {
	formatVersion: number;
	id: string;
	version: number;
	defaultLocale: string;
	fields: MetamodelField[];
	hash: string;
	enabled: boolean;
	/** Profile-supplied catalogues by locale, resolving presentation keys to text. */
	messages?: Record<string, Record<string, string>>;
}

export interface MetamodelViolation {
	field: string;
	code: string;
}

export interface MetamodelValidationError {
	fields: MetamodelViolation[];
}
