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
