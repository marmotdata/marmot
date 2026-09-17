package hive_test

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real HiveServer2 seeded with the
// sales database from the plugin's test recipe. They are skipped unless
// MARMOT_TEST_HIVE_HOST is set.

func hiveConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_HIVE_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_HIVE_HOST not set, skipping Hive e2e test")
	}
	port := 10000
	if raw := os.Getenv("MARMOT_TEST_HIVE_PORT"); raw != "" {
		var err error
		port, err = strconv.Atoi(raw)
		require.NoError(t, err)
	}
	return pluginsdk.RawConfig{
		"host":      host,
		"port":      port,
		"auth":      "NONE",
		"username":  "hive",
		"databases": []any{"sales"},
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverSales(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	config := hiveConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
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
	require.True(t, ok, "expected columns on %s", *a.Name)
	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &cols))
	byName := make(map[string]map[string]any, len(cols))
	for _, c := range cols {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func statsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, s := range result.Statistics {
		if s.AssetMRN == assetMRN {
			stats[s.MetricName] = s.Value
		}
	}
	return stats
}

func TestE2E_Meta(t *testing.T) {
	hiveConfig(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "hive", meta.ID)
	assert.Equal(t, "Apache Hive", meta.Name)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Source implements DataFetcher")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	config := hiveConfig(t)
	bin := buildBinary(t)

	delete(config, "host")
	_, err := bin.Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheSeededServer(t *testing.T) {
	config := hiveConfig(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), config)
	require.NoError(t, err)
}

func TestE2E_DiscoverDatabaseAsset(t *testing.T) {
	result := discoverSales(t)

	db := findAsset(result, "sales")
	require.NotNil(t, db)
	assert.Equal(t, "Database", db.Type)
	assert.Equal(t, []string{"Hive"}, db.Providers)
	assert.Equal(t, "mrn://database/hive/sales", *db.MRN)
	require.NotNil(t, db.Description)
	assert.Equal(t, "Sales data", *db.Description)
	assert.Equal(t, "hive", db.Metadata["owner"])
	assert.Equal(t, map[string]any{"team": "data"}, db.Metadata["parameters"])
	assert.NotEmpty(t, db.Metadata["hive_version"])
	assert.EqualValues(t, 4, db.Metadata["table_count"])
	assert.EqualValues(t, 2, db.Metadata["view_count"])
}

func TestE2E_DiscoverOnlySelectedDatabases(t *testing.T) {
	result := discoverSales(t)

	assert.Nil(t, findAsset(result, "default"), "databases: [sales] must leave default out")
}

func TestE2E_DiscoverTableColumnsAndComments(t *testing.T) {
	result := discoverSales(t)

	customers := findAsset(result, "sales.customers")
	require.NotNil(t, customers)
	assert.Equal(t, "Table", customers.Type)
	assert.Equal(t, "mrn://table/hive/sales.customers", *customers.MRN)
	assert.Equal(t, "external", customers.Metadata["object_type"], "Hive 4 translates plain managed tables to external")
	require.NotNil(t, customers.Description)
	assert.Equal(t, "Customer master data", *customers.Description)

	cols := columnsOf(t, customers)
	assert.Equal(t, "int", cols["id"]["data_type"])
	assert.Equal(t, "Customer id", cols["id"]["description"])
	assert.Equal(t, true, cols["id"]["is_primary_key"])
	assert.Equal(t, "decimal(10,2)", cols["balance"]["data_type"])
	assert.Equal(t, "map<string,string>", cols["attributes"]["data_type"])
	assert.Equal(t, "array<struct<sku:string,qty:int>>", cols["recent_items"]["data_type"])
	assert.Equal(t, true, cols["email"]["is_nullable"])
}

func TestE2E_DiscoverPartitionedExternalTable(t *testing.T) {
	result := discoverSales(t)

	events := findAsset(result, "sales.events")
	require.NotNil(t, events)
	assert.Equal(t, "external", events.Metadata["object_type"])
	assert.Equal(t, "file:/tmp/marmot-hive-events", events.Metadata["location"])
	assert.Equal(t, []any{"dt"}, events.Metadata["partition_columns"])
	assert.EqualValues(t, 2, events.Metadata["partition_count"])
	assert.Equal(t, []any{"dt=2026-01-01", "dt=2026-01-02"}, events.Metadata["partition_values"])
	assert.Contains(t, events.Metadata["input_format"], "Parquet")

	cols := columnsOf(t, events)
	assert.Equal(t, true, cols["dt"]["is_partition_column"])
	assert.Equal(t, "Event date", cols["dt"]["description"])
	assert.Nil(t, cols["event_id"]["is_partition_column"])
}

func TestE2E_DiscoverBucketedTableAndForeignKey(t *testing.T) {
	result := discoverSales(t)

	orders := findAsset(result, "sales.orders")
	require.NotNil(t, orders)
	assert.EqualValues(t, 4, orders.Metadata["num_buckets"])
	assert.Equal(t, []any{"customer_id"}, orders.Metadata["bucket_columns"])
	assert.Contains(t, orders.Metadata["serde"], "OrcSerde")

	assert.True(t, hasEdge(result, "mrn://table/hive/sales.orders", "mrn://table/hive/sales.customers", "FOREIGN_KEY"),
		"orders.customer_id references customers.id")
}

func TestE2E_DiscoverManagedTransactionalTable(t *testing.T) {
	result := discoverSales(t)

	stock := findAsset(result, "sales.stock")
	require.NotNil(t, stock)
	assert.Equal(t, "managed", stock.Metadata["object_type"])
	assert.Equal(t, true, stock.Metadata["transactional"])

	cols := columnsOf(t, stock)
	assert.Equal(t, false, cols["sku"]["is_nullable"], "NOT NULL constraint")
	assert.Equal(t, true, cols["sku"]["is_primary_key"])
	assert.Equal(t, "0", cols["qty"]["default_expression"])
}

func TestE2E_DiscoverViewWithQueryAndLineage(t *testing.T) {
	result := discoverSales(t)

	view := findAsset(result, "sales.daily_totals")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "mrn://view/hive/sales.daily_totals", *view.MRN)
	assert.Equal(t, "view", view.Metadata["object_type"])
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM sales.orders o")
	assert.Contains(t, *view.Query, "JOIN sales.customers c")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "HiveQL", *view.QueryLanguage)
	require.NotNil(t, view.Description)
	assert.Equal(t, "Order totals per customer", *view.Description)

	assert.True(t, hasEdge(result, "mrn://table/hive/sales.orders", "mrn://view/hive/sales.daily_totals", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/hive/sales.customers", "mrn://view/hive/sales.daily_totals", "VIEW_OF"))
}

func TestE2E_DiscoverMaterializedView(t *testing.T) {
	result := discoverSales(t)

	mv := findAsset(result, "sales.stock_totals")
	require.NotNil(t, mv)
	assert.Equal(t, "View", mv.Type)
	assert.Equal(t, "materialized_view", mv.Metadata["object_type"])
	require.NotNil(t, mv.Query)
	assert.Contains(t, *mv.Query, "FROM sales.stock")

	assert.True(t, hasEdge(result, "mrn://table/hive/sales.stock", "mrn://view/hive/sales.stock_totals", "VIEW_OF"))
}

func TestE2E_DiscoverContainsEdges(t *testing.T) {
	result := discoverSales(t)

	for _, target := range []string{
		"mrn://table/hive/sales.customers",
		"mrn://table/hive/sales.events",
		"mrn://table/hive/sales.orders",
		"mrn://table/hive/sales.stock",
		"mrn://view/hive/sales.daily_totals",
		"mrn://view/hive/sales.stock_totals",
	} {
		assert.True(t, hasEdge(result, "mrn://database/hive/sales", target, "CONTAINS"), target)
	}
}

func TestE2E_DiscoverStatistics(t *testing.T) {
	result := discoverSales(t)

	customers := statsOf(result, "mrn://table/hive/sales.customers")
	assert.Equal(t, float64(2), customers["asset.row_count"], "two rows inserted then ANALYZE TABLE")
	assert.Equal(t, float64(6), customers["asset.column_count"])
	assert.Greater(t, customers["asset.size_bytes"], float64(0))

	events := statsOf(result, "mrn://table/hive/sales.events")
	assert.Equal(t, float64(4), events["asset.column_count"], "three columns plus the partition column")
}

func TestE2E_DiscoverExcludesViewsWhenConfigured(t *testing.T) {
	config := hiveConfig(t)
	bin := buildBinary(t)
	config["include_views"] = false

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Nil(t, findAsset(result, "sales.daily_totals"))
	assert.Nil(t, findAsset(result, "sales.stock_totals"))
	assert.NotNil(t, findAsset(result, "sales.customers"))
}

func TestE2E_DiscoverIncludesDDLWhenConfigured(t *testing.T) {
	config := hiveConfig(t)
	bin := buildBinary(t)
	config["include_ddl"] = true

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	orders := findAsset(result, "sales.orders")
	require.NotNil(t, orders)
	assert.Contains(t, orders.Metadata["ddl"], "CREATE EXTERNAL TABLE `sales`.`orders`")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	config := hiveConfig(t)
	bin := buildBinary(t)
	name := "sales.customers"

	columns, rows, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{
		Name: &name,
		Type: "Table",
		Metadata: map[string]any{
			"database":   "sales",
			"table_name": "customers",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "name", "email", "balance", "attributes", "recent_items"}, columns)
	require.Len(t, rows, 2)
	assert.EqualValues(t, 1, rows[0][0])
	assert.Equal(t, "Alice Smith", rows[0][1])
	assert.Equal(t, "10.50", rows[0][3])
	assert.Equal(t, `{"tier":"gold"}`, rows[0][4])
}

func TestE2E_KerberosWithoutTheBuildTagFailsCleanly(t *testing.T) {
	// The released binary is built without the driver's kerberos tag, and
	// the driver panics rather than erroring for GSSAPI there. The plugin
	// must turn that into an error the host can show, not a dead process.
	config := hiveConfig(t)
	bin := buildBinary(t)
	config["auth"] = "KERBEROS"

	_, err := bin.Discover(t.Context(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KERBEROS authentication is not available in this build")
}
