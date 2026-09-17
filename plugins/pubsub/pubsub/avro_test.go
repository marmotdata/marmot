package pubsub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvroColumns_ReadsTheTopLevelRecordFields(t *testing.T) {
	columns, err := avroColumns(ordersAvroSchema)
	require.NoError(t, err)

	require.Len(t, columns, 3)
	assert.Equal(t, "order_id", columns[0].Name)
	assert.Equal(t, "amount", columns[1].Name)
	assert.Equal(t, "items", columns[2].Name)
}

func TestAvroColumns_KeepsTheFieldDocAsTheDescription(t *testing.T) {
	columns, err := avroColumns(ordersAvroSchema)
	require.NoError(t, err)

	assert.Equal(t, "Unique order identifier", columns[0].Description)
}

func TestAvroColumns_APlainTypeIsNotNullable(t *testing.T) {
	columns, err := avroColumns(ordersAvroSchema)
	require.NoError(t, err)

	assert.Equal(t, "string", columns[0].DataType)
	assert.False(t, columns[0].Nullable)
}

// Avro spells an optional field as a union with null.
func TestAvroColumns_AUnionWithNullIsNullable(t *testing.T) {
	columns, err := avroColumns(ordersAvroSchema)
	require.NoError(t, err)

	assert.Equal(t, "double", columns[1].DataType)
	assert.True(t, columns[1].Nullable)
}

func TestAvroColumns_AUnionWithoutNullIsNotNullable(t *testing.T) {
	columns, err := avroColumns(`{"type":"record","name":"E","fields":[{"name":"v","type":["string","long"]}]}`)
	require.NoError(t, err)

	require.Len(t, columns, 1)
	assert.Equal(t, "string|long", columns[0].DataType)
	assert.False(t, columns[0].Nullable)
}

func TestAvroColumns_AnArrayRendersItsItemType(t *testing.T) {
	columns, err := avroColumns(ordersAvroSchema)
	require.NoError(t, err)

	assert.Equal(t, "array<string>", columns[2].DataType)
}

func TestAvroColumns_ANestedRecordRendersItsName(t *testing.T) {
	columns, err := avroColumns(`{"type":"record","name":"Order","fields":[
		{"name":"address","type":{"type":"record","name":"Address","fields":[{"name":"city","type":"string"}]}}
	]}`)
	require.NoError(t, err)

	require.Len(t, columns, 1)
	assert.Equal(t, "record:Address", columns[0].DataType)
}

func TestAvroColumns_AMapRendersItsTypeName(t *testing.T) {
	columns, err := avroColumns(`{"type":"record","name":"Order","fields":[
		{"name":"labels","type":{"type":"map","values":"string"}}
	]}`)
	require.NoError(t, err)

	require.Len(t, columns, 1)
	assert.Equal(t, "map", columns[0].DataType)
}

func TestAvroColumns_IgnoresASchemaThatIsNotARecord(t *testing.T) {
	columns, err := avroColumns(`{"type":"enum","name":"Status","symbols":["NEW","DONE"]}`)
	require.NoError(t, err)

	assert.Empty(t, columns)
}

func TestAvroColumns_FailsOnADefinitionThatIsNotJSON(t *testing.T) {
	_, err := avroColumns(`syntax = "proto3";`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing Avro schema")
}
