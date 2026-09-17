package presto_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Importing the plugin package registers the Presto driver, so the
	// seeding below can run DDL against the cluster.
	_ "github.com/marmotdata/marmot/plugins/presto/presto"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Presto started with:
//
//	docker run -d --name marmot-test-presto --memory 3g -p 18089:8080 prestodb/presto:latest
//	MARMOT_TEST_PRESTO_HOST=localhost MARMOT_TEST_PRESTO_PORT=18089 go test ./...
//
// The image ships the tpch and memory catalogs; the memory.shop schema is
// seeded here.

func prestoTarget(t *testing.T) (string, int) {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_PRESTO_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_PRESTO_HOST not set, skipping Presto e2e tests")
	}

	port := 8080
	if raw := os.Getenv("MARMOT_TEST_PRESTO_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_PRESTO_PORT must be a number")
		port = parsed
	}

	return host, port
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// seedPresto creates the memory.shop schema the assertions rely on. Every
// statement is idempotent, so reruns against the same container pass.
// The driver rejects Exec, so DDL goes through Query.
func seedPresto(t *testing.T, host string, port int) {
	t.Helper()

	db, err := sql.Open("presto", fmt.Sprintf("http://marmot@%s:%d?source=marmot-e2e", host, port))
	require.NoError(t, err)
	defer db.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	stmts := []string{
		`CREATE SCHEMA IF NOT EXISTS memory.shop`,
		`CREATE TABLE IF NOT EXISTS memory.shop.orders AS SELECT * FROM tpch.tiny.orders LIMIT 100`,
		`CREATE TABLE IF NOT EXISTS memory.shop.customers AS SELECT * FROM tpch.tiny.customer LIMIT 50`,
		`CREATE TABLE IF NOT EXISTS memory.shop.nested AS
		 SELECT CAST(ROW(1, 'a') AS ROW(id integer, name varchar)) AS r, ARRAY[1, 2] AS arr, MAP(ARRAY['k'], ARRAY[1]) AS m,
		        CAST(1.5 AS decimal(10,2)) AS price, TIMESTAMP '2024-01-01 00:00:00' AS ts`,
		`CREATE OR REPLACE VIEW memory.shop.big_orders AS SELECT * FROM memory.shop.orders WHERE totalprice > 100000`,
	}
	for _, stmt := range stmts {
		rows, err := db.QueryContext(ctx, stmt)
		require.NoError(t, err, stmt)
		for rows.Next() {
		}
		rows.Close()
	}
}

func e2eConfig(host string, port int) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host":             host,
		"port":             port,
		"user":             "marmot",
		"exclude_catalogs": []any{"system", "jmx", "tpcds"},
		// tpch ships one schema per scale factor; tiny is enough.
		"exclude_schemas": []any{"sf1", "sf100", "sf1000", "sf10000", "sf100000", "sf300", "sf3000", "sf30000"},
		"include_stats":   true,
	}
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	host, port := prestoTarget(t)
	seedPresto(t, host, port)

	result, err := buildBinary(t).Discover(t.Context(), e2eConfig(host, port))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func findEdge(result *pluginsdk.DiscoveryResult, edgeType, source, target string) bool {
	for _, e := range result.Lineage {
		if e.Type == edgeType && e.Source == source && e.Target == target {
			return true
		}
	}
	return false
}

func findStatistic(result *pluginsdk.DiscoveryResult, mrn, metric string) (float64, bool) {
	for _, s := range result.Statistics {
		if s.AssetMRN == mrn && s.MetricName == metric {
			return s.Value, true
		}
	}
	return 0, false
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) []pluginsdk.Column {
	t.Helper()
	require.NotNil(t, a.Schema)
	var cols []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(a.Schema["columns"]), &cols))
	return cols
}

func TestE2E_Meta(t *testing.T) {
	prestoTarget(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "presto", meta.ID)
	assert.Equal(t, "Presto", meta.Name)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Source implements DataFetcher")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	prestoTarget(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"user": "marmot"})
	require.Error(t, err)
}

func TestE2E_DiscoverCatalogs(t *testing.T) {
	result := discoverE2E(t)

	memory := findAsset(result, "Catalog", "memory")
	require.NotNil(t, memory)
	assert.Equal(t, "mrn://catalog/presto/memory", *memory.MRN)
	assert.Equal(t, []string{"Presto"}, memory.Providers)
	assert.Equal(t, "memory", memory.Metadata["connector_name"])
	assert.NotEmpty(t, memory.Metadata["presto_version"])
	// default (empty, shipped with the image) and shop (seeded).
	assert.EqualValues(t, 2, memory.Metadata["schema_count"])
	assert.EqualValues(t, 3, memory.Metadata["table_count"])
	assert.EqualValues(t, 1, memory.Metadata["view_count"])

	tpch := findAsset(result, "Catalog", "tpch")
	require.NotNil(t, tpch)
	assert.Equal(t, "tpch", tpch.Metadata["connector_name"])
	assert.EqualValues(t, 1, tpch.Metadata["schema_count"], "only tiny survives exclude_schemas")
	assert.EqualValues(t, 8, tpch.Metadata["table_count"])

	assert.Nil(t, findAsset(result, "Catalog", "system"))
	assert.Nil(t, findAsset(result, "Catalog", "jmx"))
	assert.Nil(t, findAsset(result, "Catalog", "tpcds"))
}

func TestE2E_DiscoverTablesWithColumns(t *testing.T) {
	result := discoverE2E(t)

	orders := findAsset(result, "Table", "memory.shop.orders")
	require.NotNil(t, orders)
	assert.Equal(t, "mrn://table/presto/memory.shop.orders", *orders.MRN)
	assert.Equal(t, []string{"Presto"}, orders.Providers)
	assert.Equal(t, "memory", orders.Metadata["catalog"])
	assert.Equal(t, "shop", orders.Metadata["schema"])
	assert.Equal(t, "orders", orders.Metadata["table_name"])
	assert.Equal(t, "BASE TABLE", orders.Metadata["table_type"])

	cols := columnsOf(t, orders)
	require.Len(t, cols, 9)
	assert.Equal(t, "orderkey", cols[0].Name)
	assert.Equal(t, "bigint", cols[0].DataType)
	assert.True(t, cols[0].Nullable)
	assert.Equal(t, "orderstatus", cols[2].Name)
	assert.Equal(t, "varchar(1)", cols[2].DataType)

	nation := findAsset(result, "Table", "tpch.tiny.nation")
	require.NotNil(t, nation)
	nationCols := columnsOf(t, nation)
	require.Len(t, nationCols, 4)
	assert.Equal(t, "nationkey", nationCols[0].Name)
	assert.False(t, nationCols[0].Nullable, "tpch columns are NOT NULL")

	assert.Nil(t, findAsset(result, "Table", "tpch.sf1.nation"), "sf1 is excluded")
}

func TestE2E_NestedColumnTypesStayVerbatim(t *testing.T) {
	result := discoverE2E(t)

	nested := findAsset(result, "Table", "memory.shop.nested")
	require.NotNil(t, nested)

	cols := columnsOf(t, nested)
	require.Len(t, cols, 5)
	assert.Equal(t, `row("id" integer, "name" varchar)`, cols[0].DataType)
	assert.Equal(t, "array(integer)", cols[1].DataType)
	assert.Equal(t, "map(varchar(1), integer)", cols[2].DataType)
	assert.Equal(t, "decimal(10,2)", cols[3].DataType)
	assert.Equal(t, "timestamp", cols[4].DataType)
}

func TestE2E_ViewCarriesItsDefinitionAndLineage(t *testing.T) {
	result := discoverE2E(t)

	view := findAsset(result, "View", "memory.shop.big_orders")
	require.NotNil(t, view)
	assert.Equal(t, "mrn://view/presto/memory.shop.big_orders", *view.MRN)
	assert.Equal(t, "VIEW", view.Metadata["table_type"])
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "memory.shop.orders")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)

	assert.True(t, findEdge(result, "VIEW_OF", "mrn://table/presto/memory.shop.orders", "mrn://view/presto/memory.shop.big_orders"),
		"expected orders -> big_orders VIEW_OF edge")
}

func TestE2E_CatalogContainsItsTables(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, findEdge(result, "CONTAINS", "mrn://catalog/presto/memory", "mrn://table/presto/memory.shop.orders"))
	assert.True(t, findEdge(result, "CONTAINS", "mrn://catalog/presto/memory", "mrn://view/presto/memory.shop.big_orders"))
	assert.True(t, findEdge(result, "CONTAINS", "mrn://catalog/presto/tpch", "mrn://table/presto/tpch.tiny.nation"))
}

func TestE2E_Statistics(t *testing.T) {
	result := discoverE2E(t)

	columnCount, ok := findStatistic(result, "mrn://table/presto/memory.shop.orders", "asset.column_count")
	require.True(t, ok)
	assert.Equal(t, 9.0, columnCount)

	viewColumnCount, ok := findStatistic(result, "mrn://view/presto/memory.shop.big_orders", "asset.column_count")
	require.True(t, ok)
	assert.Equal(t, 9.0, viewColumnCount)

	// SHOW STATS knows tpch row counts; the memory connector reports
	// none, so memory tables carry no row_count rather than zero.
	rowCount, ok := findStatistic(result, "mrn://table/presto/tpch.tiny.nation", "asset.row_count")
	require.True(t, ok, "expected a row count for tpch.tiny.nation")
	assert.Equal(t, 25.0, rowCount)

	_, ok = findStatistic(result, "mrn://table/presto/memory.shop.orders", "asset.row_count")
	assert.False(t, ok)
}

func TestE2E_ExcludingViewsDropsThem(t *testing.T) {
	host, port := prestoTarget(t)
	seedPresto(t, host, port)

	config := e2eConfig(host, port)
	config["include_views"] = false
	config["include_stats"] = false
	config["catalog"] = "memory"

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Nil(t, findAsset(result, "View", "memory.shop.big_orders"))
	assert.NotNil(t, findAsset(result, "Table", "memory.shop.orders"))
	assert.Nil(t, findAsset(result, "Catalog", "tpch"), "catalog narrows discovery to one catalog")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	host, port := prestoTarget(t)
	seedPresto(t, host, port)

	bin := buildBinary(t)
	name := "memory.shop.orders"
	asset := &pluginsdk.Asset{
		Name:     &name,
		Type:     "Table",
		Metadata: map[string]any{"catalog": "memory", "schema": "shop", "table_name": "orders"},
	}

	columns, rows, err := bin.FetchSampleData(t.Context(), e2eConfig(host, port), asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"orderkey", "custkey", "orderstatus", "totalprice", "orderdate", "orderpriority", "clerk", "shippriority", "comment"}, columns)
	assert.Len(t, rows, 20)
	assert.Len(t, rows[0], 9)
}

func TestE2E_FetchSampleDataHandlesNestedTypes(t *testing.T) {
	host, port := prestoTarget(t)
	seedPresto(t, host, port)

	bin := buildBinary(t)
	name := "memory.shop.nested"
	asset := &pluginsdk.Asset{
		Name:     &name,
		Type:     "Table",
		Metadata: map[string]any{"catalog": "memory", "schema": "shop", "table_name": "nested"},
	}

	columns, rows, err := bin.FetchSampleData(t.Context(), e2eConfig(host, port), asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"r", "arr", "m", "price", "ts"}, columns)
	require.Len(t, rows, 1)
	assert.JSONEq(t, `{"id":1,"name":"a"}`, rows[0][0].(string))
	assert.JSONEq(t, `[1,2]`, rows[0][1].(string))
	assert.JSONEq(t, `{"k":1}`, rows[0][2].(string))
	assert.Equal(t, "1.50", rows[0][3], "decimals arrive as text")
	// The driver reads a zone-less timestamp in the machine's local zone,
	// so only the wall clock is stable across machines.
	ts, err := time.Parse(time.RFC3339, rows[0][4].(string))
	require.NoError(t, err, "timestamps are RFC3339 strings")
	assert.Equal(t, "2024-01-01 00:00:00", ts.Format("2006-01-02 15:04:05"))
}
