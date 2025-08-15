package openapi

import "encoding/json"

const (
	JsonSchemaTypeArray 	= "array"
	JsonSchemaTypeBoolean	= "boolean"
	JsonSchemaTypeNull 	= "null"
	JsonSchemaTypeInteger	= "integer"
	JsonSchemaTypeNumber	= "number"
	JsonSchemaTypeObject	= "object"
	JsonSchemaTypeString	= "string"
)

type JsonSchema struct {
	Schema		*string	`json:"$schema,omitempty"`
	Id          	*string `json:"id,omitempty"`
	Title       	*string `json:"title,omitempty"`
	Description 	*string `json:"description,omitempty"`
	Ref		*string	`json:"$ref,omitempty"`
	Type 		[]string `json:"type,omitempty"`
	Definitions	map[string]JsonSchema `json:"definitions,omitempty"`
	Properties	map[string]JsonSchema `json:"properties,omitempty"`
	Items		any `json:"items,omitempty"`
	AllOf		[]*JsonSchema `json:"allOf,omitempty"`
	AnyOf		[]*JsonSchema `json:"anyOf,omitempty"`
	OneOf		[]*JsonSchema `json:"oneOf,omitempty"`
	Required	[]string `json:"required,omitempty"`
	Enum		[]any `json:"enum,omitempty"`
	Example		string `json:"example,omitempty"`
	Pattern 	string `json:"pattern,omitempty"`
	Maximum         *float64 `json:"maximum,renderZero,omitempty"`
	Minimum         *float64 `json:"minimum,renderZero,omitempty,"`
	Format          string `json:"format,omitempty"`
}

func NewJsonSchema() JsonSchema {
	return JsonSchema{
	}
}

func NewRootJsonSchema() JsonSchema {
	schema := "https://json-schema.org/draft/2020-12/schema"
	return JsonSchema{
		Schema: &schema,
	}
}

func (js JsonSchema) MarshalJSON() ([]byte, error) {
	type jsonSchemaAlias JsonSchema
	aux := &struct {
		Type any `json:"type,omitempty"`
		jsonSchemaAlias
	}{
		jsonSchemaAlias: jsonSchemaAlias(js),
	}

	if len(js.Type) == 1 {
		aux.Type = js.Type[0]
	} else if len(js.Type) > 1 {
		aux.Type = js.Type
	}

	return json.Marshal(aux)
}
