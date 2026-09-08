package mariadb

import (
	"database/sql"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixtures below are copied from a MariaDB 11.8 server: the EXTRA
// values, TABLE_TYPE values, VIEW_DEFINITION text and sequence row are
// what information_schema really returns.

func TestObjectKind_BaseTableIsATable(t *testing.T) {
	assetType, kind, versioned, ok := objectKind("BASE TABLE")

	require.True(t, ok)
	assert.Equal(t, "Table", assetType)
	assert.Equal(t, "table", kind)
	assert.False(t, versioned)
}

func TestObjectKind_SystemVersionedIsATableWithTheFlagSet(t *testing.T) {
	assetType, kind, versioned, ok := objectKind("SYSTEM VERSIONED")

	require.True(t, ok)
	assert.Equal(t, "Table", assetType)
	assert.Equal(t, "table", kind)
	assert.True(t, versioned)
}

func TestObjectKind_View(t *testing.T) {
	assetType, kind, _, ok := objectKind("VIEW")

	require.True(t, ok)
	assert.Equal(t, "View", assetType)
	assert.Equal(t, "view", kind)
}

func TestObjectKind_Sequence(t *testing.T) {
	assetType, kind, _, ok := objectKind("SEQUENCE")

	require.True(t, ok)
	assert.Equal(t, "Sequence", assetType)
	assert.Equal(t, "sequence", kind)
}

func TestObjectKind_UnknownTypeIsSkipped(t *testing.T) {
	_, _, _, ok := objectKind("TEMPORARY")

	assert.False(t, ok)
}

func TestTablesQuery_SelectsTemporaryOnNewerServers(t *testing.T) {
	query := tablesQuery(true)

	assert.Contains(t, query, "TEMPORARY AS temporary")
	assert.NotContains(t, query, "NULL AS temporary")
}

func TestTablesQuery_SubstitutesNullOnOlderServers(t *testing.T) {
	query := tablesQuery(false)

	assert.Contains(t, query, "NULL AS temporary")
	assert.NotContains(t, query, "TEMPORARY AS temporary")
}

func TestColumnFlags_AutoIncrement(t *testing.T) {
	autoIncrement, generated, invisible := columnFlags("auto_increment")

	assert.True(t, autoIncrement)
	assert.False(t, generated)
	assert.False(t, invisible)
}

func TestColumnFlags_VirtualGenerated(t *testing.T) {
	autoIncrement, generated, invisible := columnFlags("VIRTUAL GENERATED")

	assert.False(t, autoIncrement)
	assert.True(t, generated)
	assert.False(t, invisible)
}

func TestColumnFlags_StoredGeneratedAndInvisibleCombine(t *testing.T) {
	autoIncrement, generated, invisible := columnFlags("STORED GENERATED, INVISIBLE")

	assert.False(t, autoIncrement)
	assert.True(t, generated)
	assert.True(t, invisible)
}

func TestColumnFlags_Invisible(t *testing.T) {
	_, generated, invisible := columnFlags("INVISIBLE")

	assert.False(t, generated)
	assert.True(t, invisible)
}

func TestColumnFlags_OnUpdateIsNotAFlag(t *testing.T) {
	autoIncrement, generated, invisible := columnFlags("on update current_timestamp()")

	assert.False(t, autoIncrement)
	assert.False(t, generated)
	assert.False(t, invisible)
}

func TestColumnFlags_EmptyExtra(t *testing.T) {
	autoIncrement, generated, invisible := columnFlags("")

	assert.False(t, autoIncrement)
	assert.False(t, generated)
	assert.False(t, invisible)
}

const customerTotalsDefinition = "select `c`.`id` AS `customer_id`,`c`.`email` AS `email`,sum(`o`.`total`) AS `total_spent` " +
	"from (`shop`.`orders` `o` join `shop`.`customers` `c` on(`c`.`id` = `o`.`customer_id`)) group by `c`.`id`,`c`.`email`"

func TestViewReferences_FindsTheQualifiedTablesOfAJoin(t *testing.T) {
	assert.Equal(t, []string{"orders", "customers"}, viewReferences(customerTotalsDefinition, "shop"))
}

func TestViewReferences_SkipsTablesOfAnotherDatabase(t *testing.T) {
	definition := "select 1 from (`other`.`orders` `o` join `shop`.`customers` `c` on(`c`.`id` = `o`.`customer_id`))"

	assert.Equal(t, []string{"customers"}, viewReferences(definition, "shop"))
}

func TestViewReferences_ReadsAViewBuiltOnAView(t *testing.T) {
	definition := "select `customer_totals`.`customer_id` AS `customer_id` from `shop`.`customer_totals` where `customer_totals`.`total_spent` > 20"

	assert.Equal(t, []string{"customer_totals"}, viewReferences(definition, "shop"))
}

func TestViewReferences_AcceptsUnqualifiedNames(t *testing.T) {
	assert.Equal(t, []string{"orders"}, viewReferences("select id from orders", "shop"))
}

func TestViewReferences_DeduplicatesARepeatedTable(t *testing.T) {
	definition := "select 1 from (`shop`.`orders` `a` join `shop`.`orders` `b`)"

	assert.Equal(t, []string{"orders"}, viewReferences(definition, "shop"))
}

func TestViewReferences_LooksInsideADerivedTable(t *testing.T) {
	// The word after "from (" is "select", which the caller drops because
	// no object has that name; the inner table still has to be found.
	definition := "select `x`.`customer_id` AS `customer_id` from (select `shop`.`orders`.`customer_id` AS `customer_id` from `shop`.`orders`) `x`"

	assert.Contains(t, viewReferences(definition, "shop"), "orders")
}

func TestViewReferences_DoesNotMistakeAColumnQualifierForATable(t *testing.T) {
	definition := "select `shop`.`orders`.`id` from `shop`.`customers`"

	assert.Equal(t, []string{"customers"}, viewReferences(definition, "shop"))
}

func TestViewReferences_UnescapesADoubledBacktick(t *testing.T) {
	assert.Equal(t, []string{"we`ird"}, viewReferences("select 1 from `shop`.`we``ird`", "shop"))
}

func TestViewReferences_EmptyDefinition(t *testing.T) {
	assert.Empty(t, viewReferences("", "shop"))
}

func TestUnquoteIdentifier_LeavesBareNamesAlone(t *testing.T) {
	assert.Equal(t, "orders", unquoteIdentifier("orders"))
}

var sequenceColumns = []string{
	"next_not_cached_value", "minimum_value", "maximum_value", "start_value",
	"increment", "cache_size", "cycle_option", "cycle_count",
}

func TestSequenceMetadata_ReadsTheRealRowShape(t *testing.T) {
	// A non-prepared "SELECT * FROM seq" hands every value back as bytes.
	values := []interface{}{
		[]byte("1000"), []byte("1"), []byte("9223372036854775806"), []byte("1000"),
		[]byte("1"), []byte("1000"), []byte("0"), []byte("0"),
	}

	metadata := sequenceMetadata(sequenceColumns, values)

	assert.Equal(t, int64(1000), metadata["start_value"])
	assert.Equal(t, int64(1), metadata["minimum_value"])
	assert.Equal(t, int64(9223372036854775806), metadata["maximum_value"])
	assert.Equal(t, int64(1), metadata["increment"])
	assert.Equal(t, int64(1000), metadata["cache_size"])
	assert.Equal(t, false, metadata["cycle_option"])
}

func TestSequenceMetadata_CycleOptionBecomesABool(t *testing.T) {
	metadata := sequenceMetadata([]string{"cycle_option"}, []interface{}{int64(1)})

	assert.Equal(t, true, metadata["cycle_option"])
}

func TestSequenceMetadata_LeavesBookkeepingColumnsOut(t *testing.T) {
	metadata := sequenceMetadata(sequenceColumns, []interface{}{
		int64(1000), int64(1), int64(100), int64(1000), int64(1), int64(10), int64(0), int64(0),
	})

	assert.NotContains(t, metadata, "next_not_cached_value")
	assert.NotContains(t, metadata, "cycle_count")
}

func TestSequenceMetadata_SkipsValuesThatAreNotIntegers(t *testing.T) {
	metadata := sequenceMetadata([]string{"increment", "start_value"}, []interface{}{nil, "abc"})

	assert.Empty(t, metadata)
}

func TestToInt64_AcceptsTypedAndTextValues(t *testing.T) {
	typed, ok := toInt64(int64(7))
	require.True(t, ok)
	assert.Equal(t, int64(7), typed)

	unsigned, ok := toInt64(uint64(8))
	require.True(t, ok)
	assert.Equal(t, int64(8), unsigned)

	text, ok := toInt64([]byte("9"))
	require.True(t, ok)
	assert.Equal(t, int64(9), text)
}

func valid(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: true} }

func TestTableStatistics_EmitsRowCountAndSize(t *testing.T) {
	stats := tableStatistics("mrn://table/mariadb/orders", valid(3), valid(16384), valid(16384))

	assert.Equal(t, []pluginsdk.Statistic{
		{AssetMRN: "mrn://table/mariadb/orders", MetricName: "asset.row_count", Value: 3},
		{AssetMRN: "mrn://table/mariadb/orders", MetricName: "asset.size_bytes", Value: 32768},
	}, stats)
}

func TestTableStatistics_SkipsTheRowCountWhenUnknown(t *testing.T) {
	stats := tableStatistics("mrn://table/mariadb/orders", sql.NullInt64{}, valid(16384), valid(0))

	require.Len(t, stats, 1)
	assert.Equal(t, "asset.size_bytes", stats[0].MetricName)
	assert.Equal(t, float64(16384), stats[0].Value)
}

func TestTableStatistics_EmitsNothingWithoutFigures(t *testing.T) {
	assert.Empty(t, tableStatistics("mrn://table/mariadb/orders", sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{}))
}

func TestConvertValue_TextBytesBecomeAString(t *testing.T) {
	assert.Equal(t, "alice@example.com", convertValue([]byte("alice@example.com")))
}

func TestConvertValue_BinaryBytesBecomeHex(t *testing.T) {
	assert.Equal(t, "0xff00", convertValue([]byte{0xff, 0x00}))
}

func TestConvertValue_TimeBecomesRFC3339(t *testing.T) {
	at := time.Date(2026, 9, 7, 21, 12, 48, 0, time.UTC)

	assert.Equal(t, "2026-09-07T21:12:48Z", convertValue(at))
}

func TestConvertValue_NilStaysNil(t *testing.T) {
	assert.Nil(t, convertValue(nil))
}
