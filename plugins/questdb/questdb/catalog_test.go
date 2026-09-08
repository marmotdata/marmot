package questdb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tradesRow is a tables() row as pgx decodes it from QuestDB 10.0.1: INT
// columns as int32, LONG as int64, CHAR as string.
func tradesRow() row {
	return row{
		"id":                  int32(9),
		"table_name":          "trades",
		"designatedTimestamp": "ts",
		"partitionBy":         "DAY",
		"walEnabled":          true,
		"dedup":               true,
		"ttlValue":            int32(0),
		"ttlUnit":             "HOUR",
		"matView":             false,
		"directoryName":       "trades~9",
		"maxUncommittedRows":  int32(500000),
		"o3MaxLag":            int64(600000000),
		"table_suspended":     false,
		"table_type":          "T",
	}
}

func TestParseTableRow_ReadsColumnsByName(t *testing.T) {
	info, ok := parseTableRow(tradesRow())
	require.True(t, ok)

	assert.Equal(t, "trades", info.Name)
	assert.Equal(t, kindTable, info.Kind)
	assert.Equal(t, "ts", info.DesignatedTimestamp)
	assert.Equal(t, "DAY", info.PartitionBy)
	assert.True(t, info.WALEnabled)
	assert.True(t, info.Dedup)
	assert.Equal(t, int64(500000), info.MaxUncommittedRows)
	assert.Equal(t, int64(600000000), info.O3MaxLag)
}

func TestParseTableRow_SkipsRowWithoutName(t *testing.T) {
	r := tradesRow()
	delete(r, "table_name")

	_, ok := parseTableRow(r)
	assert.False(t, ok)
}

func TestParseTableRow_NullDesignatedTimestampIsEmpty(t *testing.T) {
	r := tradesRow()
	r["designatedTimestamp"] = nil
	r["partitionBy"] = "NONE"

	info, ok := parseTableRow(r)
	require.True(t, ok)
	assert.Empty(t, info.DesignatedTimestamp)
	assert.Equal(t, "NONE", info.PartitionBy)
}

func TestParseTableRow_TableTypeDecidesTheKind(t *testing.T) {
	r := tradesRow()

	r["table_type"] = "V"
	info, _ := parseTableRow(r)
	assert.Equal(t, kindView, info.Kind)

	r["table_type"] = "M"
	info, _ = parseTableRow(r)
	assert.Equal(t, kindMaterializedView, info.Kind)

	r["table_type"] = "T"
	r["matView"] = true
	info, _ = parseTableRow(r)
	assert.Equal(t, kindTable, info.Kind, "table_type wins over the older matView flag")
}

func TestParseTableRow_FallsBackToMatViewFlagWithoutTableType(t *testing.T) {
	r := tradesRow()
	delete(r, "table_type")
	r["matView"] = true

	info, _ := parseTableRow(r)
	assert.Equal(t, kindMaterializedView, info.Kind)
}

func TestParseTableRow_DefaultsToTableWithoutAnyKindHint(t *testing.T) {
	r := tradesRow()
	delete(r, "table_type")
	delete(r, "matView")

	info, _ := parseTableRow(r)
	assert.Equal(t, kindTable, info.Kind)
}

func TestParseTableRow_ToleratesAnOldBuildWithFewColumns(t *testing.T) {
	// QuestDB 6 only had id, table_name, designatedTimestamp, partitionBy,
	// maxUncommittedRows and o3MaxLag.
	info, ok := parseTableRow(row{
		"id":                  int32(1),
		"table_name":          "old",
		"designatedTimestamp": nil,
		"partitionBy":         "NONE",
		"maxUncommittedRows":  int32(1000),
		"o3MaxLag":            int64(0),
	})
	require.True(t, ok)

	assert.Equal(t, kindTable, info.Kind)
	assert.False(t, info.WALEnabled)
	assert.False(t, info.Dedup)
	assert.Empty(t, info.ttl())
}

func TestParseTableRow_O3MaxLagIsUnknownWhenTheColumnIsMissing(t *testing.T) {
	r := tradesRow()
	delete(r, "o3MaxLag")

	info, _ := parseTableRow(r)
	assert.Equal(t, int64(-1), info.O3MaxLag)
}

func TestTableInfo_TTLIsEmptyWhenUnset(t *testing.T) {
	// QuestDB reports ttlValue 0 with ttlUnit HOUR for a table without TTL.
	info, _ := parseTableRow(tradesRow())

	assert.Empty(t, info.ttl())
}

func TestTableInfo_TTLRendersValueAndUnit(t *testing.T) {
	r := tradesRow()
	r["ttlValue"] = int32(1)
	r["ttlUnit"] = "WEEK"

	info, _ := parseTableRow(r)
	assert.Equal(t, "1 WEEK", info.ttl())
}

func TestTableInfo_OnlyPlainViewsAreNotStored(t *testing.T) {
	assert.True(t, tableInfo{Kind: kindTable}.isStored())
	assert.True(t, tableInfo{Kind: kindMaterializedView}.isStored())
	assert.False(t, tableInfo{Kind: kindView}.isStored())
}

func symbolColumnRow() row {
	return row{
		"column":             "symbol",
		"type":               "SYMBOL",
		"indexed":            true,
		"indexBlockCapacity": int32(256),
		"symbolCached":       true,
		"symbolCapacity":     int32(256),
		"symbolTableSize":    int32(2),
		"designated":         false,
		"upsertKey":          true,
		"indexType":          "BITMAP",
		"indexInclude":       "",
	}
}

func TestParseColumnRow_ReadsSymbolExtras(t *testing.T) {
	c, ok := parseColumnRow(symbolColumnRow())
	require.True(t, ok)

	assert.Equal(t, "symbol", c.Name)
	assert.Equal(t, "SYMBOL", c.DataType)
	assert.True(t, c.Indexed)
	assert.True(t, c.UpsertKey)
	assert.Equal(t, int64(256), c.SymbolCapacity)
	require.NotNil(t, c.SymbolCached)
	assert.True(t, *c.SymbolCached)
}

func TestParseColumnRow_OmitsSymbolExtrasForOtherTypes(t *testing.T) {
	r := symbolColumnRow()
	r["column"], r["type"] = "price", "DOUBLE"
	r["indexed"], r["upsertKey"] = false, false

	c, _ := parseColumnRow(r)
	assert.Equal(t, int64(0), c.SymbolCapacity)
	assert.Nil(t, c.SymbolCached)
}

func TestParseColumnRow_MarksTheDesignatedTimestamp(t *testing.T) {
	c, _ := parseColumnRow(row{"column": "ts", "type": "TIMESTAMP", "designated": true, "upsertKey": true})

	assert.True(t, c.DesignatedTimestamp)
	assert.True(t, c.UpsertKey)
}

func TestParseColumnRow_KeepsTheDataTypeVerbatim(t *testing.T) {
	c, _ := parseColumnRow(row{"column": "geo", "type": "GEOHASH(8c)"})

	assert.Equal(t, "GEOHASH(8c)", c.DataType)
}

func TestParseColumnRow_IsAlwaysNullableAndNeverAKey(t *testing.T) {
	c, _ := parseColumnRow(row{"column": "id", "type": "INT"})

	assert.True(t, c.Nullable)
	assert.False(t, c.PrimaryKey)
}

func TestParseColumnRow_SkipsRowWithoutName(t *testing.T) {
	_, ok := parseColumnRow(row{"type": "INT"})
	assert.False(t, ok)
}

func TestColumnInfo_SerialisesTheSchemaKeysMarmotReads(t *testing.T) {
	c, _ := parseColumnRow(symbolColumnRow())

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "symbol", got["column_name"])
	assert.Equal(t, "SYMBOL", got["data_type"])
	assert.Equal(t, true, got["is_nullable"])
	assert.Equal(t, true, got["indexed"])
	assert.Equal(t, float64(256), got["symbol_capacity"])
	assert.Equal(t, true, got["symbol_cached"])
	assert.Equal(t, true, got["upsert_key"])
	assert.NotContains(t, got, "designated_timestamp")
	assert.NotContains(t, got, "is_primary_key")
}

func TestColumnInfo_OmitsSymbolKeysForOtherTypes(t *testing.T) {
	c, _ := parseColumnRow(row{"column": "price", "type": "DOUBLE"})

	data, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "symbol_capacity")
	assert.NotContains(t, string(data), "symbol_cached")
	assert.NotContains(t, string(data), "indexed")
}

func TestParseViewRow_ReadsNameAndSQL(t *testing.T) {
	v, ok := parseViewRow(row{
		"view_name":           "recent_trades",
		"view_sql":            "SELECT t.ts FROM trades t JOIN venues v ON t.venue = v.name;",
		"view_table_dir_name": "recent_trades~13",
		"invalidation_reason": nil,
		"view_status":         "valid",
	})
	require.True(t, ok)

	assert.Equal(t, "recent_trades", v.Name)
	assert.Equal(t, "SELECT t.ts FROM trades t JOIN venues v ON t.venue = v.name;", v.SQL)
	assert.False(t, v.Materialized)
	assert.Empty(t, v.BaseTable)
}

func TestParseViewRow_SkipsRowWithoutName(t *testing.T) {
	_, ok := parseViewRow(row{"view_sql": "SELECT 1"})
	assert.False(t, ok)
}

func materializedViewRow() row {
	return row{
		"view_name":                     "trades_1h",
		"refresh_type":                  "immediate",
		"base_table_name":               "trades",
		"last_refresh_start_timestamp":  time.Date(2026, 9, 8, 2, 20, 57, 423572000, time.UTC),
		"last_refresh_finish_timestamp": time.Date(2026, 9, 8, 2, 20, 57, 431865000, time.UTC),
		"view_sql":                      "SELECT ts, symbol, avg(price) AS avg_price FROM trades SAMPLE BY 1h",
		"view_status":                   "valid",
		"timer_interval":                int32(0),
		"timer_interval_unit":           nil,
		"period_length":                 int32(0),
		"period_length_unit":            nil,
	}
}

func TestParseMaterializedViewRow_ReadsBaseTableAndRefresh(t *testing.T) {
	v, ok := parseMaterializedViewRow(materializedViewRow())
	require.True(t, ok)

	assert.Equal(t, "trades_1h", v.Name)
	assert.True(t, v.Materialized)
	assert.Equal(t, "trades", v.BaseTable)
	assert.Equal(t, "immediate", v.RefreshType)
	assert.Empty(t, v.RefreshPeriod)
	assert.Equal(t, time.Date(2026, 9, 8, 2, 20, 57, 423572000, time.UTC), v.LastRefresh)
	assert.Contains(t, v.SQL, "SAMPLE BY 1h")
}

func TestParseMaterializedViewRow_NoRefreshYetLeavesLastRefreshZero(t *testing.T) {
	r := materializedViewRow()
	r["last_refresh_start_timestamp"] = nil

	v, _ := parseMaterializedViewRow(r)
	assert.True(t, v.LastRefresh.IsZero())
}

func TestRefreshPeriodOf_UsesTheTimerInterval(t *testing.T) {
	// REFRESH EVERY 1h is reported as timer_interval 1, unit HOUR.
	r := materializedViewRow()
	r["refresh_type"] = "timer"
	r["timer_interval"] = int32(1)
	r["timer_interval_unit"] = "HOUR"

	assert.Equal(t, "1 HOUR", refreshPeriodOf(r))
}

func TestRefreshPeriodOf_UsesThePeriodLength(t *testing.T) {
	r := materializedViewRow()
	r["refresh_type"] = "period"
	r["period_length"] = int32(30)
	r["period_length_unit"] = "MINUTE"

	assert.Equal(t, "30 MINUTE", refreshPeriodOf(r))
}

func TestRefreshPeriodOf_PrefersALiteralRefreshPeriodColumn(t *testing.T) {
	r := materializedViewRow()
	r["refresh_period"] = "2h"
	r["timer_interval"] = int32(1)
	r["timer_interval_unit"] = "HOUR"

	assert.Equal(t, "2h", refreshPeriodOf(r))
}

func TestIsSystemTable_MatchesQuestDBInternals(t *testing.T) {
	assert.True(t, isSystemTable("sys.text_import_log"))
	assert.True(t, isSystemTable("sys.telemetry_wal"))
	assert.True(t, isSystemTable("_query_trace"))
	assert.True(t, isSystemTable("telemetry"))
	assert.True(t, isSystemTable("telemetry_config"))
}

func TestIsSystemTable_LeavesUserTablesAlone(t *testing.T) {
	assert.False(t, isSystemTable("trades"))
	assert.False(t, isSystemTable("system_events"))
	assert.False(t, isSystemTable("telemetry_events"))
	assert.False(t, isSystemTable("my.dotted.table"))
}
