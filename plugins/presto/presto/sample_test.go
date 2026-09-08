package presto

import (
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
)

func TestTypeFamily(t *testing.T) {
	assert.Equal(t, "varchar", typeFamily("varchar(25)"))
	assert.Equal(t, "row", typeFamily(`row("id" integer, "name" varchar)`))
	assert.Equal(t, "timestamp with time zone", typeFamily("timestamp with time zone"))
	assert.Equal(t, "bigint", typeFamily("BIGINT"))
}

func TestSampleExpression_ScalarColumnsAreSelectedAsIs(t *testing.T) {
	assert.Equal(t, `"orderkey"`, sampleExpression("orderkey", "bigint"))
	assert.Equal(t, `"name"`, sampleExpression("name", "varchar(25)"))
	assert.Equal(t, `"price"`, sampleExpression("price", "decimal(10,2)"))
}

func TestSampleExpression_NestedTypesAreCastToJSON(t *testing.T) {
	// Presto sends these in a text form the driver cannot convert.
	assert.Equal(t, `CAST("r" AS JSON) AS "r"`, sampleExpression("r", `row("id" integer, "name" varchar)`))
	assert.Equal(t, `CAST("arr" AS JSON) AS "arr"`, sampleExpression("arr", "array(integer)"))
	assert.Equal(t, `CAST("m" AS JSON) AS "m"`, sampleExpression("m", "map(varchar(1), integer)"))
}

func TestSampleExpression_UnknownTypesAreCastToVarchar(t *testing.T) {
	assert.Equal(t, `CAST("u" AS varchar) AS "u"`, sampleExpression("u", "uuid"))
}

func TestSampleQuery_WithColumns(t *testing.T) {
	columns := []column{
		{Column: pluginsdk.Column{Name: "id", DataType: "bigint"}},
		{Column: pluginsdk.Column{Name: "tags", DataType: "array(varchar)"}},
	}

	query := sampleQuery(tableKey{"memory", "shop", "orders"}, columns)

	assert.Equal(t, `SELECT "id", CAST("tags" AS JSON) AS "tags" FROM "memory"."shop"."orders" LIMIT 20`, query)
}

func TestSampleQuery_WithoutColumnsSelectsEverything(t *testing.T) {
	query := sampleQuery(tableKey{"memory", "shop", "orders"}, nil)
	assert.Equal(t, `SELECT * FROM "memory"."shop"."orders" LIMIT 20`, query)
}

func TestSampleValue_FormatsTimesAsRFC3339(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.Equal(t, "2024-01-02T03:04:05Z", sampleValue(ts))
}

func TestSampleValue_PassesScalarsThrough(t *testing.T) {
	assert.Equal(t, int64(7), sampleValue(int64(7)))
	assert.Equal(t, "x", sampleValue([]byte("x")))
	assert.Nil(t, sampleValue(nil))
}
