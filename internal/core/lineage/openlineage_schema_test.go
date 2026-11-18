package lineage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaFromFacet_WritesTheColumnsSchema(t *testing.T) {
	schema := schemaFromFacet(map[string]interface{}{
		"fields": []interface{}{
			map[string]interface{}{"name": "id", "type": "BIGINT"},
			map[string]interface{}{"name": "email", "type": "VARCHAR", "description": "Login address"},
			map[string]interface{}{"name": "payload"},
		},
	})

	require.Len(t, schema, 1)
	assert.JSONEq(t, `[
		{"column_name": "id", "data_type": "BIGINT"},
		{"column_name": "email", "data_type": "VARCHAR", "description": "Login address"},
		{"column_name": "payload", "data_type": "unknown"}
	]`, schema["columns"])
}

func TestSchemaFromFacet_KeepsTheProducersColumnOrder(t *testing.T) {
	schema := schemaFromFacet(map[string]interface{}{
		"fields": []interface{}{
			map[string]interface{}{"name": "zeta", "type": "INTEGER"},
			map[string]interface{}{"name": "alpha", "type": "INTEGER"},
		},
	})

	assert.Equal(t, `[{"column_name":"zeta","data_type":"INTEGER"},{"column_name":"alpha","data_type":"INTEGER"}]`, schema["columns"])
}

func TestSchemaFromFacet_IsNilWithoutFields(t *testing.T) {
	assert.Nil(t, schemaFromFacet(map[string]interface{}{}))
	assert.Nil(t, schemaFromFacet(map[string]interface{}{"fields": []interface{}{}}))
	assert.Nil(t, schemaFromFacet(map[string]interface{}{"fields": []interface{}{map[string]interface{}{"type": "INTEGER"}}}))
}

func TestSchemaFromFacet_FollowsOrdinalPositionAndLeavesNestedFieldsOut(t *testing.T) {
	schema := schemaFromFacet(map[string]interface{}{
		"fields": []interface{}{
			map[string]interface{}{"name": "b", "type": "INT", "ordinal_position": float64(2)},
			map[string]interface{}{"name": "address", "type": "STRUCT<city STRING>", "ordinal_position": float64(3), "fields": []interface{}{
				map[string]interface{}{"name": "city", "type": "STRING"},
			}},
			map[string]interface{}{"name": "a", "type": "INT", "ordinal_position": float64(1)},
		},
	})
	assert.Equal(t, `[{"column_name":"a","data_type":"INT"},{"column_name":"b","data_type":"INT"},{"column_name":"address","data_type":"STRUCT<city STRING>"}]`, schema["columns"])
}

func TestDatasetFieldsReadsDuckLineageSizeInBytes(t *testing.T) {
	ds := &Dataset{Namespace: "file", Name: "out.parquet", OutputFacets: map[string]interface{}{
		"outputStatistics": map[string]interface{}{"rowCount": float64(4), "sizeInBytes": float64(1024)},
	}}
	fields := datasetFields(ds, identifyDataset(ds, "https://github.com/ilum-cloud/duck_lineage"))
	assert.EqualValues(t, 4, fields.Metadata["output_row_count"])
	assert.EqualValues(t, 1024, fields.Metadata["output_size_bytes"])
}
