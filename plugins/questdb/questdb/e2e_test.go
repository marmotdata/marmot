package questdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real QuestDB. They are gated on
// MARMOT_TEST_QUESTDB_HOST (and MARMOT_TEST_QUESTDB_PORT, default 8812) and
// sign in as admin/quest, QuestDB's default credentials. Run one with
//
//	docker run -d -p 8812:8812 questdb/questdb:latest
//	MARMOT_TEST_QUESTDB_HOST=localhost go test ./...

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_QUESTDB_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_QUESTDB_HOST is not set, skipping e2e test against a live QuestDB")
	}
	port := 8812
	if raw := os.Getenv("MARMOT_TEST_QUESTDB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_QUESTDB_PORT must be a port number")
		port = parsed
	}

	return pluginsdk.RawConfig{
		"host":     host,
		"port":     port,
		"user":     "admin",
		"password": "quest",
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// seedStatements builds the fixtures every assertion below relies on. They
// are idempotent: IF NOT EXISTS on the DDL, DEDUP on trades and TRUNCATE on
// venues keep the counts stable across repeated runs.
var seedStatements = []string{
	`CREATE TABLE IF NOT EXISTS trades (
		ts TIMESTAMP, symbol SYMBOL CAPACITY 256 CACHE INDEX, side SYMBOL,
		price DOUBLE, qty LONG, venue VARCHAR
	) TIMESTAMP(ts) PARTITION BY DAY WAL DEDUP UPSERT KEYS(ts, symbol)`,
	`CREATE TABLE IF NOT EXISTS venues (id INT, name STRING, geo GEOHASH(8c), ip IPv4, uid UUID, big LONG256)`,
	`CREATE TABLE IF NOT EXISTS sensor_readings (ts TIMESTAMP, sensor SYMBOL, value DOUBLE)
		TIMESTAMP(ts) PARTITION BY HOUR TTL 7 DAYS`,
	`INSERT INTO trades VALUES
		('2026-09-01T10:00:00.000000Z', 'BTC-USD', 'buy', 65000.5, 2, 'coinbase'),
		('2026-09-01T11:00:00.000000Z', 'BTC-USD', 'sell', 65100.0, 1, 'kraken'),
		('2026-09-02T10:00:00.000000Z', 'ETH-USD', 'buy', 3400.25, 10, 'coinbase')`,
	`TRUNCATE TABLE venues`,
	`INSERT INTO venues VALUES
		(1, 'coinbase', #u4pruydq, '10.0.0.1', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 0x01),
		(2, 'kraken', #u4pruyd7, '10.0.0.2', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12', 0x02)`,
	`CREATE MATERIALIZED VIEW IF NOT EXISTS trades_1h AS (
		SELECT ts, symbol, avg(price) AS avg_price FROM trades SAMPLE BY 1h
	) PARTITION BY DAY`,
	`CREATE VIEW IF NOT EXISTS recent_trades AS
		SELECT t.ts, t.symbol, t.price, v.name FROM trades t JOIN venues v ON t.venue = v.name`,
}

var (
	seedOnce sync.Once
	seedErr  error
)

// seed loads the fixtures over the PostgreSQL wire once per test binary and
// waits for the WAL to apply the trades rows, since WAL writes are visible
// only after the apply job has run.
func seed(t *testing.T, config pluginsdk.RawConfig) {
	t.Helper()

	seedOnce.Do(func() {
		seedErr = runSeed(config)
	})
	require.NoError(t, seedErr, "seeding QuestDB fixtures")
}

func runSeed(config pluginsdk.RawConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	dsn := fmt.Sprintf("postgres://admin:quest@%s:%d/qdb?sslmode=disable", config["host"], config["port"])
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer conn.Close(ctx)

	for _, stmt := range seedStatements {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("running %q: %w", stmt, err)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		var count int64
		if err := conn.QueryRow(ctx, `SELECT count() FROM "trades"`).Scan(&count); err != nil {
			return fmt.Errorf("counting trades: %w", err)
		}
		if count == 3 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("trades has %d rows after WAL apply, want 3", count)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func discoverE2E(t *testing.T, extra pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	seed(t, config)
	for k, v := range extra {
		config[k] = v
	}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].Name != nil && *result.Assets[i].Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column schema on %s", *a.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]any, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func statsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "questdb", meta.ID)
	assert.Equal(t, "QuestDB", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Source implements DataFetcher")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"port": 8812})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsLiveConfig(t *testing.T) {
	config := e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.NoError(t, err)
}

func TestE2E_DiscoverFindsTablesAndViews(t *testing.T) {
	result := discoverE2E(t, nil)

	names := make(map[string]string) // name -> type
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		assert.Equal(t, []string{"QuestDB"}, a.Providers)
		names[*a.Name] = a.Type
	}
	assert.Equal(t, "Table", names["trades"])
	assert.Equal(t, "Table", names["venues"])
	assert.Equal(t, "Table", names["sensor_readings"])
	assert.Equal(t, "View", names["trades_1h"])
	assert.Equal(t, "View", names["recent_trades"])
}

func TestE2E_TableCarriesItsStorageLayout(t *testing.T) {
	result := discoverE2E(t, nil)

	trades := findAsset(result, "trades")
	require.NotNil(t, trades)
	assert.Equal(t, "mrn://table/questdb/trades", *trades.MRN)
	assert.Equal(t, "trades", trades.Metadata["table_name"])
	assert.Equal(t, "table", trades.Metadata["object_type"])
	assert.Equal(t, "ts", trades.Metadata["designated_timestamp"])
	assert.Equal(t, "DAY", trades.Metadata["partition_by"])
	assert.Equal(t, true, trades.Metadata["wal_enabled"])
	assert.Equal(t, true, trades.Metadata["dedup"])
	// Two days of trades were inserted, so two daily partitions exist.
	assert.EqualValues(t, 2, trades.Metadata["partition_count"])
	assert.NotContains(t, trades.Metadata, "ttl", "trades keeps data forever")
	assert.Nil(t, trades.Description, "QuestDB has no table comments to describe with")
}

func TestE2E_TableCarriesTTL(t *testing.T) {
	result := discoverE2E(t, nil)

	readings := findAsset(result, "sensor_readings")
	require.NotNil(t, readings)
	// QuestDB normalises TTL 7 DAYS to one week.
	assert.Equal(t, "1 WEEK", readings.Metadata["ttl"])
	assert.Equal(t, "HOUR", readings.Metadata["partition_by"])
}

func TestE2E_NonPartitionedTableHasOnePartitionAndNoTimestamp(t *testing.T) {
	result := discoverE2E(t, nil)

	venues := findAsset(result, "venues")
	require.NotNil(t, venues)
	assert.Equal(t, "NONE", venues.Metadata["partition_by"])
	assert.NotContains(t, venues.Metadata, "designated_timestamp")
	assert.EqualValues(t, 1, venues.Metadata["partition_count"])
	assert.Equal(t, false, venues.Metadata["wal_enabled"])
}

func TestE2E_ColumnsCarrySymbolAndTimestampExtras(t *testing.T) {
	result := discoverE2E(t, nil)

	trades := findAsset(result, "trades")
	require.NotNil(t, trades)
	columns := columnsOf(t, trades)

	require.Contains(t, columns, "ts")
	assert.Equal(t, "TIMESTAMP", columns["ts"]["data_type"])
	assert.Equal(t, true, columns["ts"]["designated_timestamp"])
	assert.Equal(t, true, columns["ts"]["upsert_key"])
	assert.Equal(t, true, columns["ts"]["is_nullable"])

	require.Contains(t, columns, "symbol")
	assert.Equal(t, "SYMBOL", columns["symbol"]["data_type"])
	assert.Equal(t, true, columns["symbol"]["indexed"])
	assert.EqualValues(t, 256, columns["symbol"]["symbol_capacity"])
	assert.Equal(t, true, columns["symbol"]["symbol_cached"])
	assert.Equal(t, true, columns["symbol"]["upsert_key"])

	require.Contains(t, columns, "side")
	assert.NotContains(t, columns["side"], "indexed")
	assert.NotContains(t, columns["side"], "upsert_key")

	require.Contains(t, columns, "venue")
	assert.Equal(t, "VARCHAR", columns["venue"]["data_type"])
	assert.NotContains(t, columns["venue"], "symbol_capacity")
	assert.NotContains(t, columns["venue"], "symbol_cached")
}

func TestE2E_ColumnTypesAreKeptVerbatim(t *testing.T) {
	result := discoverE2E(t, nil)

	venues := findAsset(result, "venues")
	require.NotNil(t, venues)
	columns := columnsOf(t, venues)

	assert.Equal(t, "INT", columns["id"]["data_type"])
	assert.Equal(t, "STRING", columns["name"]["data_type"])
	assert.Equal(t, "GEOHASH(8c)", columns["geo"]["data_type"])
	assert.Equal(t, "IPv4", columns["ip"]["data_type"])
	assert.Equal(t, "UUID", columns["uid"]["data_type"])
	assert.Equal(t, "LONG256", columns["big"]["data_type"])
}

func TestE2E_EmitsRowColumnAndSizeStatistics(t *testing.T) {
	result := discoverE2E(t, nil)

	stats := statsOf(result, "mrn://table/questdb/trades")
	assert.Equal(t, float64(3), stats["asset.row_count"])
	assert.Equal(t, float64(6), stats["asset.column_count"])
	assert.Greater(t, stats["asset.size_bytes"], float64(0))
}

func TestE2E_MaterializedViewCarriesQueryAndBaseTable(t *testing.T) {
	result := discoverE2E(t, nil)

	view := findAsset(result, "trades_1h")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "mrn://view/questdb/trades_1h", *view.MRN)
	assert.Equal(t, "materialized_view", view.Metadata["object_type"])
	assert.Equal(t, true, view.Metadata["materialized"])
	assert.Equal(t, "trades", view.Metadata["base_table"])
	assert.Equal(t, "immediate", view.Metadata["refresh_type"])
	assert.Contains(t, view.Metadata, "last_refresh")
	// A materialized view is stored like a table, so it has partitions too.
	assert.Equal(t, "DAY", view.Metadata["partition_by"])

	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "SAMPLE BY 1h")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)

	assert.True(t, hasEdge(result, "mrn://table/questdb/trades", "mrn://view/questdb/trades_1h", "VIEW_OF"),
		"expected trades -> trades_1h VIEW_OF edge, got %v", result.Lineage)
}

func TestE2E_MaterializedViewHasRowCount(t *testing.T) {
	result := discoverE2E(t, nil)

	stats := statsOf(result, "mrn://view/questdb/trades_1h")
	assert.Equal(t, float64(3), stats["asset.column_count"])
	assert.Contains(t, stats, "asset.row_count")
	assert.Greater(t, stats["asset.size_bytes"], float64(0))
}

func TestE2E_PlainViewLinksToEveryTableItReads(t *testing.T) {
	result := discoverE2E(t, nil)

	view := findAsset(result, "recent_trades")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "view", view.Metadata["object_type"])
	assert.Equal(t, false, view.Metadata["materialized"])
	assert.NotContains(t, view.Metadata, "partition_by")
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "JOIN venues v")

	assert.True(t, hasEdge(result, "mrn://table/questdb/trades", "mrn://view/questdb/recent_trades", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/questdb/venues", "mrn://view/questdb/recent_trades", "VIEW_OF"))

	stats := statsOf(result, "mrn://view/questdb/recent_trades")
	assert.Equal(t, float64(4), stats["asset.column_count"])
	assert.NotContains(t, stats, "asset.row_count", "counting a plain view would run its query")
}

func TestE2E_ExcludesViewsWhenConfigured(t *testing.T) {
	result := discoverE2E(t, pluginsdk.RawConfig{"include_views": false})

	assert.Nil(t, findAsset(result, "recent_trades"))
	assert.NotNil(t, findAsset(result, "trades_1h"), "materialized views are a separate switch")
	assert.NotNil(t, findAsset(result, "trades"))
}

func TestE2E_ExcludesMaterializedViewsWhenConfigured(t *testing.T) {
	result := discoverE2E(t, pluginsdk.RawConfig{"include_materialized_views": false})

	assert.Nil(t, findAsset(result, "trades_1h"))
	assert.NotNil(t, findAsset(result, "recent_trades"))
}

func TestE2E_SkipsStatisticsWhenConfigured(t *testing.T) {
	result := discoverE2E(t, pluginsdk.RawConfig{"include_statistics": false})

	assert.Empty(t, result.Statistics)
	trades := findAsset(result, "trades")
	require.NotNil(t, trades)
	assert.Contains(t, trades.Schema, "columns", "columns are independent of statistics")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	config := e2eConfig(t)
	seed(t, config)

	name := "trades"
	asset := &pluginsdk.Asset{
		Name:     &name,
		Type:     "Table",
		Metadata: map[string]any{"table_name": "trades"},
	}

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), config, asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"ts", "symbol", "side", "price", "qty", "venue"}, columns)
	require.Len(t, rows, 3)
	assert.Equal(t, "2026-09-01T10:00:00Z", rows[0][0])
	assert.Equal(t, "BTC-USD", rows[0][1])
	assert.EqualValues(t, 65000.5, rows[0][3])
}

func TestE2E_FetchSampleDataRendersQuestDBTypes(t *testing.T) {
	config := e2eConfig(t)
	seed(t, config)

	name := "venues"
	asset := &pluginsdk.Asset{Name: &name, Type: "Table"}

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), config, asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "name", "geo", "ip", "uid", "big"}, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, "u4pruydq", rows[0][2])
	assert.Equal(t, "10.0.0.1", rows[0][3])
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", rows[0][4])
	assert.Equal(t, "0x01", rows[0][5])
}
