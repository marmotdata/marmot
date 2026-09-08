package couchbase

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func columnByName(columns []documentColumn, name string) (documentColumn, bool) {
	for _, c := range columns {
		if c.Name == name {
			return c, true
		}
	}
	return documentColumn{}, false
}

func TestColumnsFromDocuments_NoDocumentsGivesNoColumns(t *testing.T) {
	assert.Nil(t, columnsFromDocuments(nil))
}

func TestColumnsFromDocuments_FieldInEveryDocumentIsRequired(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"sku": "A1"},
		{"sku": "B2"},
	})

	sku, ok := columnByName(columns, "sku")
	require.True(t, ok)
	assert.Equal(t, "string", sku.DataType)
	assert.False(t, sku.Nullable)
	assert.Equal(t, 1.0, sku.Occurrence)
}

func TestColumnsFromDocuments_FieldMissingFromSomeDocumentsIsNullable(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"sku": "A1", "discontinued": true},
		{"sku": "B2"},
		{"sku": "C3"},
		{"sku": "D4"},
	})

	discontinued, ok := columnByName(columns, "discontinued")
	require.True(t, ok)
	assert.True(t, discontinued.Nullable)
	assert.Equal(t, 0.25, discontinued.Occurrence)
}

func TestColumnsFromDocuments_MixedTypesAreJoinedWithPipe(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"total": 42.5},
		{"total": "99.90"},
	})

	total, ok := columnByName(columns, "total")
	require.True(t, ok)
	assert.Equal(t, "number|string", total.DataType)
}

func TestColumnsFromDocuments_NestedFieldsOneLevelDeep(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"shipping": map[string]interface{}{"city": "Amsterdam", "geo": map[string]interface{}{"lat": 52.3}}},
	})

	shipping, ok := columnByName(columns, "shipping")
	require.True(t, ok)
	assert.Equal(t, "object", shipping.DataType)

	city, ok := columnByName(columns, "shipping.city")
	require.True(t, ok)
	assert.Equal(t, "string", city.DataType)

	geo, ok := columnByName(columns, "shipping.geo")
	require.True(t, ok)
	assert.Equal(t, "object", geo.DataType)

	_, ok = columnByName(columns, "shipping.geo.lat")
	assert.False(t, ok, "grandchildren are not expanded")
}

func TestColumnsFromDocuments_NestedFieldOccurrenceCountsDocuments(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"shipping": map[string]interface{}{"city": "Amsterdam"}},
		{"shipping": map[string]interface{}{"city": "Berlin", "postcode": "10115"}},
		{"note": "no shipping"},
	})

	postcode, ok := columnByName(columns, "shipping.postcode")
	require.True(t, ok)
	assert.True(t, postcode.Nullable)
	assert.InDelta(t, 0.33, postcode.Occurrence, 0.001)
}

func TestColumnsFromDocuments_ColumnsSortedWithChildrenAfterParent(t *testing.T) {
	columns := columnsFromDocuments([]map[string]interface{}{
		{"zip": "1", "address": map[string]interface{}{"city": "x"}, "age": 1.0},
	})

	names := make([]string, 0, len(columns))
	for _, c := range columns {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"address", "address.city", "age", "zip"}, names)
}

func TestJsonType_NamesEveryJSONType(t *testing.T) {
	assert.Equal(t, "null", jsonType(nil))
	assert.Equal(t, "string", jsonType("x"))
	assert.Equal(t, "boolean", jsonType(true))
	assert.Equal(t, "number", jsonType(1.5))
	assert.Equal(t, "number", jsonType(json.Number("7")))
	assert.Equal(t, "object", jsonType(map[string]interface{}{}))
	assert.Equal(t, "array", jsonType([]interface{}{}))
}

func TestJoinTypes_SortedAndPipeSeparated(t *testing.T) {
	assert.Equal(t, "boolean|number|string", joinTypes(map[string]struct{}{
		"string": {}, "boolean": {}, "number": {},
	}))
}

// inferFixture is the trimmed result of
// INFER `shop`.`_default`.`_default` WITH {"sample_size": 100}
// on Couchbase Server 7.6.2: one row holding one flavour.
const inferFixture = `[
  {
    "#docs": 2,
    "$schema": "http://json-schema.org/draft-06/schema",
    "Flavor": "",
    "properties": {
      "discontinued": {"#docs": 1, "%docs": 50, "samples": [true], "type": "boolean"},
      "name": {"#docs": 2, "%docs": 100, "samples": ["Gadget", "Widget"], "type": "string"},
      "price": {"#docs": 2, "%docs": 100, "samples": [12.5, 99.9], "type": "number"},
      "sku": {"#docs": 2, "%docs": 100, "samples": ["A1", "B2"], "type": "string"},
      "tags": {"#docs": 1, "%docs": 50, "items": {"type": "string"}, "maxItems": 1, "minItems": 1, "samples": [["new"]], "type": "array"}
    },
    "type": "object"
  }
]`

func TestColumnsFromInfer_ParsesRealServerOutput(t *testing.T) {
	columns, err := columnsFromInfer([]json.RawMessage{json.RawMessage(inferFixture)})
	require.NoError(t, err)
	require.Len(t, columns, 5)

	sku, ok := columnByName(columns, "sku")
	require.True(t, ok)
	assert.Equal(t, "string", sku.DataType)
	assert.False(t, sku.Nullable)
	assert.Equal(t, 1.0, sku.Occurrence)

	tags, ok := columnByName(columns, "tags")
	require.True(t, ok)
	assert.Equal(t, "array", tags.DataType)
	assert.True(t, tags.Nullable)
	assert.Equal(t, 0.5, tags.Occurrence)
}

func TestColumnsFromInfer_MixedTypeFieldUsesListsForTypeAndCount(t *testing.T) {
	// INFER writes "type" and "#docs" as lists when a field was seen with
	// more than one type.
	row := `[{"#docs": 3, "properties": {
		"total": {"#docs": [2, 1], "type": ["number", "string"]}
	}}]`

	columns, err := columnsFromInfer([]json.RawMessage{json.RawMessage(row)})
	require.NoError(t, err)

	total, ok := columnByName(columns, "total")
	require.True(t, ok)
	assert.Equal(t, "number|string", total.DataType)
	assert.False(t, total.Nullable)
}

func TestColumnsFromInfer_MergesFlavours(t *testing.T) {
	// Two document shapes in one collection come back as two flavours;
	// a field's count is summed over them and the total is all documents.
	row := `[
		{"#docs": 3, "properties": {"kind": {"#docs": 3, "type": "string"}, "sku": {"#docs": 3, "type": "string"}}},
		{"#docs": 1, "properties": {"kind": {"#docs": 1, "type": "string"}, "email": {"#docs": 1, "type": "string"}}}
	]`

	columns, err := columnsFromInfer([]json.RawMessage{json.RawMessage(row)})
	require.NoError(t, err)

	kind, ok := columnByName(columns, "kind")
	require.True(t, ok)
	assert.False(t, kind.Nullable)
	assert.Equal(t, 1.0, kind.Occurrence)

	sku, ok := columnByName(columns, "sku")
	require.True(t, ok)
	assert.True(t, sku.Nullable)
	assert.Equal(t, 0.75, sku.Occurrence)
}

func TestColumnsFromInfer_NestedObjectOneLevelDeep(t *testing.T) {
	row := `[{"#docs": 1, "properties": {
		"shipping": {"#docs": 1, "type": "object", "properties": {
			"city": {"#docs": 1, "type": "string"},
			"geo": {"#docs": 1, "type": "object", "properties": {"lat": {"#docs": 1, "type": "number"}}}
		}}
	}}]`

	columns, err := columnsFromInfer([]json.RawMessage{json.RawMessage(row)})
	require.NoError(t, err)

	_, ok := columnByName(columns, "shipping.city")
	assert.True(t, ok)
	_, ok = columnByName(columns, "shipping.geo")
	assert.True(t, ok)
	_, ok = columnByName(columns, "shipping.geo.lat")
	assert.False(t, ok, "grandchildren are not expanded")
}

func TestColumnsFromInfer_NoRowsGivesNoColumns(t *testing.T) {
	columns, err := columnsFromInfer(nil)
	require.NoError(t, err)
	assert.Nil(t, columns)
}

func TestColumnsFromInfer_EmptyCollectionGivesNoColumns(t *testing.T) {
	columns, err := columnsFromInfer([]json.RawMessage{json.RawMessage(`[]`)})
	require.NoError(t, err)
	assert.Nil(t, columns)
}

func TestColumnsFromInfer_MalformedRowIsAnError(t *testing.T) {
	_, err := columnsFromInfer([]json.RawMessage{json.RawMessage(`{"not": "a list"}`)})
	require.Error(t, err)
}
