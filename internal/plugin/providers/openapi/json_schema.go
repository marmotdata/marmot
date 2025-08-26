package openapi

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/renderer"
	"github.com/rs/zerolog/log"
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
	Format          string `json:"format,omitempty"`
	Maximum         *float64 `json:"maximum,renderZero,omitempty"`
	Minimum         *float64 `json:"minimum,renderZero,omitempty,"`
	MaxLength       *int64 `json:"maxLength,omitempty"`
	MinLength       *int64 `json:"minLength,omitempty"`
	MaxItems        *int64`json:"maxItems,omitempty"`
	MinItems        *int64 `json:"minItems,omitempty"`
	UniqueItems     *bool `json:"uniqueItems,omitempty"`
	MultipleOf      *float64 `json:"multipleOf,omitempty"`
	Not		*JsonSchema `json:"not,omitempty"`
	AdditionalProperties any `json:"additionalProperties,omitempty"`
	MaxProperties   *int64 `json:"maxProperties,omitempty"`
	MinProperties   *int64 `json:"minProperties,omitempty"`
	Deprecated      *bool `json:"deprecated,omitempty"`
}

func NewJsonSchema() *JsonSchema {
	return &JsonSchema{
	}
}

func NewJsonSchemaFromOpenAPISchema(schemaProxy *base.SchemaProxy) (*JsonSchema, error) {
	mg := mockGenerator()

	var dfs func(p *base.SchemaProxy, depth int) (*JsonSchema, error)
	dfs = func(p *base.SchemaProxy, depth int) (*JsonSchema, error) {
		jSchema := NewJsonSchema()

		if p == nil {
			return jSchema, nil
		}

		if depth > 5 {
			description := "Maximum depth reached. Stop rendering schema"
			jSchema.Description = &description
			return jSchema, nil
		}

		schema, err := p.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to build schema at line %d: %w", p.GetValueNode().Line, err)
		}
		if len(schema.Type) > 0 {
			jSchema.Type = schema.Type
		}
		jSchema.Description = &schema.Description

		if schema.Nullable != nil && *schema.Nullable {
			if jSchema.Type != nil {
				jSchema.Type = append(jSchema.Type, "null")
			} else {
				jSchema.Type = []string{"null"}
			}
		}

		if schema.Properties != nil {
			jSchema.Properties = make(map[string]JsonSchema)
			for name, prop := range schema.Properties.FromOldest() {
				jSubSchema, err := dfs(prop, depth + 1)
				if err != nil {
					return nil, fmt.Errorf("failed to render properties schema at line %d: %w", prop.GetValueNode().Line, err)
				}
				jSchema.Properties[name] = *jSubSchema
			}
		}

		if schema.Items != nil && slices.Contains(schema.Type, "array") {
			isSchemaProxy := schema.Items.IsA()
			if isSchemaProxy {
				jsonItemsSchema, err := dfs(schema.Items.A, depth + 1)
				if err != nil {
					return nil, fmt.Errorf("failed to render items schema at line %d: %w", schema.Items.A.GetValueNode().Line, err)
				}
				jSchema.Items = jsonItemsSchema
			} else {
				jSchema.Items = schema.Items.B
			}
		}

		if schema.AllOf != nil {
			jSchema.AllOf = []*JsonSchema{}
			for _, allOfSchema := range schema.AllOf {
				jsonAllOfSchema, err := dfs(allOfSchema, depth + 1)
				if err != nil {
					return nil, fmt.Errorf("failed to render AllOf schema at line %d: %w", allOfSchema.GetValueNode().Line, err)
				}
				jSchema.AllOf = append(jSchema.AllOf, jsonAllOfSchema)
			}
		}

		if schema.AnyOf != nil {
			jSchema.AnyOf = []*JsonSchema{}
			for _, anyOfSchema := range schema.AnyOf {
				jsonAnyOfSchema, err := dfs(anyOfSchema, depth + 1)
				if err != nil {
					return nil, fmt.Errorf("failed to render AnyOf schema at line %d: %w", anyOfSchema.GetValueNode().Line, err)
				}
				jSchema.AnyOf = append(jSchema.AnyOf, jsonAnyOfSchema)
			}
		}

		if schema.OneOf != nil {
			jSchema.OneOf = []*JsonSchema{}
			for _, oneOfSchema := range schema.OneOf {
				jsonOneOfSchema, err := dfs(oneOfSchema, depth + 1)
				if err != nil {
					return nil, fmt.Errorf("failed to render OneOf schema at line %d: %w", oneOfSchema.GetValueNode().Line, err)
				}
				jSchema.OneOf = append(jSchema.OneOf, jsonOneOfSchema)
			}
		}

		if schema.Required != nil {
			jSchema.Required = schema.Required
		}

		if schema.Enum != nil {
			enum := []any{}
			for _, e := range schema.Enum {
				enum = append(enum, e.Value)
			}
			jSchema.Enum = enum
		}

		if schema.AdditionalProperties != nil {
			if schema.AdditionalProperties.IsA() {
				jSchema.AdditionalProperties = schema.AdditionalProperties.A
			} else {
				jSchema.AdditionalProperties = schema.AdditionalProperties.B
			}
		}

		if schema.Not != nil {
			jsonNotSchema, err := dfs(schema.Not, depth + 1)
			if err != nil {
				return nil, fmt.Errorf("failed to render Not schema at line %d: %w", schema.Not.GetValueNode().Line, err)
			}
			jSchema.Not = jsonNotSchema
		}

		jSchema.Pattern = schema.Pattern
		jSchema.MultipleOf = schema.MultipleOf
		jSchema.Maximum = schema.Maximum
		jSchema.Minimum = schema.Minimum
		jSchema.MaxLength = schema.MaxLength
		jSchema.MinLength = schema.MinLength
		jSchema.MaxItems = schema.MaxItems
		jSchema.MinItems = schema.MinItems
		jSchema.UniqueItems = schema.UniqueItems
		jSchema.MaxProperties = schema.MaxProperties
		jSchema.MinProperties = schema.MinProperties
		jSchema.MinProperties = schema.MinProperties
		jSchema.Deprecated = schema.Deprecated

		mock, err := mg.GenerateMock(schema, "")
		if err != nil {
			if err.Error() != "unable to render schema for mock, it's empty" {
				log.Warn().Err(err).Msg(fmt.Sprintf("Failed to generate mock for schema at line %d", p.GetValueNode().Line))
			} 
		}
		jSchema.Example = string(mock)

		return jSchema, nil
	}

	jsonSchema, err := dfs(schemaProxy, 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to dive into schema at line %d: %w", schemaProxy.GetValueNode().Line, err)
	}

	return jsonSchema, nil 
}

func mockGenerator() *renderer.MockGenerator {
	mg := renderer.NewMockGenerator(renderer.JSON)
	return mg
}

func (js JsonSchema) MarshalJSON() ([]byte, error) {
	type jsonSchemaAlias JsonSchema
	aux := &struct {
		Type any `json:"type,omitempty"`
		jsonSchemaAlias
	}{
		jsonSchemaAlias: jsonSchemaAlias(js),
	}

	// If js.Type contains a single element, unwrap it. Otherwise, use the slice as is.
	if len(js.Type) == 1 {
		aux.Type = js.Type[0]
	} else if len(js.Type) > 1 {
		aux.Type = js.Type
	}

	return json.Marshal(aux)
}

