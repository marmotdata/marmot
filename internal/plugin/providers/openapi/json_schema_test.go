package openapi

import (
	"context"
	"encoding/json"
	"testing"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/low"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestJsonSchemaFromOpenAPISchema(t *testing.T) {
	given := `type: [object]
description: "I'm a Labubu"
properties:
  name:
    type: string
    enum: [labubu]`
	expectedJsonSchema := `
{
  "type": "object",
  "description": "I'm a Labubu",
  "properties": {
    "name": {
      "type": "string",
      "description": "",
      "enum": [
        "labubu"
      ],
      "example": "labubu"
    }
  },
  "example": "{\"name\":\"labubu\"}"
}`

	proxy := getSchemaProxy([]byte(given))
	compiled, err := NewJsonSchemaFromOpenAPISchema(proxy)
	assert.NoError(t, err)
	receivedJsonSchema, err := json.Marshal(compiled)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedJsonSchema, string(receivedJsonSchema))
}

func TestJsonSchemaFromNullableOpenAPISchema(t *testing.T) {
	given := `type: [object]
description: ""
properties:
  name:
    type: string
    nullable: true
    example: BigIntoEnergy`
	expectedJsonSchema := `
{
  "type": "object",
  "description": "",
  "properties": {
    "name": {
      "type": ["string", "null"],
      "description": "",
      "example": "BigIntoEnergy"
    }
  },
  "example": "{\"name\":\"BigIntoEnergy\"}"
}`

	proxy := getSchemaProxy([]byte(given))
	compiled, err := NewJsonSchemaFromOpenAPISchema(proxy)
	assert.NoError(t, err)
	receivedJsonSchema, err := json.Marshal(compiled)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedJsonSchema, string(receivedJsonSchema))
}

func getSchemaProxy(schema []byte) *highbase.SchemaProxy {
	var compNode yaml.Node
	e := yaml.Unmarshal(schema, &compNode)
	if e != nil {
		panic(e)
	}
	sp := new(lowbase.SchemaProxy)
	_ = sp.Build(context.Background(), nil, compNode.Content[0], nil)
	lp := low.NodeReference[*lowbase.SchemaProxy]{
		Value:     sp,
		ValueNode: compNode.Content[0],
	}
	return highbase.NewSchemaProxy(&lp)
}
