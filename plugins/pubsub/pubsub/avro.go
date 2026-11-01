package pubsub

import (
	"encoding/json"
	"fmt"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// avroSchema is the part of an Avro record declaration the plugin reads. Only
// the top level is walked: nested records become one column with their type
// name, which keeps the schema view readable for deeply nested messages.
type avroSchema struct {
	Type   string      `json:"type"`
	Name   string      `json:"name"`
	Fields []avroField `json:"fields"`
}

type avroField struct {
	Name string          `json:"name"`
	Type json.RawMessage `json:"type"`
	Doc  string          `json:"doc"`
}

// avroColumns turns an Avro record definition into Marmot columns. A field is
// nullable when its type is a union containing "null", which is how Avro
// spells an optional field.
func avroColumns(definition string) ([]pluginsdk.Column, error) {
	var schema avroSchema
	if err := json.Unmarshal([]byte(definition), &schema); err != nil {
		return nil, fmt.Errorf("parsing Avro schema: %w", err)
	}

	if schema.Type != "record" {
		return nil, nil
	}

	columns := make([]pluginsdk.Column, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		dataType, nullable := avroFieldType(field.Type)
		columns = append(columns, pluginsdk.Column{
			Name:        field.Name,
			DataType:    dataType,
			Nullable:    nullable,
			Description: field.Doc,
		})
	}

	return columns, nil
}

// avroFieldType renders an Avro type as a single string. Avro types come in
// three shapes: a bare name ("string"), a union (["null", "string"]) and a
// complex type ({"type": "array", "items": "string"}).
func avroFieldType(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}

	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return name, false
	}

	var union []json.RawMessage
	if err := json.Unmarshal(raw, &union); err == nil {
		var (
			names    []string
			nullable bool
		)
		for _, member := range union {
			memberName, _ := avroFieldType(member)
			if memberName == "null" {
				nullable = true
				continue
			}
			names = append(names, memberName)
		}
		if len(names) == 0 {
			return "null", true
		}
		return strings.Join(names, "|"), nullable
	}

	var complexType struct {
		Type  string          `json:"type"`
		Items json.RawMessage `json:"items"`
		Name  string          `json:"name"`
	}
	if err := json.Unmarshal(raw, &complexType); err != nil {
		return "", false
	}

	switch {
	case complexType.Type == "array" && len(complexType.Items) > 0:
		items, _ := avroFieldType(complexType.Items)
		return "array<" + items + ">", false
	case complexType.Type == "record" && complexType.Name != "":
		return "record:" + complexType.Name, false
	default:
		return complexType.Type, false
	}
}
