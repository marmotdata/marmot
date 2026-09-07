package cockroachdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real CockroachDB started with
// `cockroach start-single-node --insecure`:
//
//	docker run -d --name marmot-test-cockroachdb -p 26257:26257 \
//	  cockroachdb/cockroach:latest-v25.2 start-single-node --insecure
//	MARMOT_TEST_COCKROACHDB_HOST=localhost MARMOT_TEST_COCKROACHDB_PORT=26257 go test ./...
//
// The fixture is created by the tests themselves and is idempotent.

const (
	ordersMRN         = "mrn://table/cockroachdb/shop.public.orders"
	customersMRN      = "mrn://table/cockroachdb/shop.public.customers"
	customerTotalsMRN = "mrn://view/cockroachdb/shop.public.customer_totals"
	orderSummaryMRN   = "mrn://view/cockroachdb/shop.public.order_summary"
	shopMRN           = "mrn://database/cockroachdb/shop"
)

var seedStatements = []string{
	`CREATE DATABASE IF NOT EXISTS shop`,
	`CREATE DATABASE IF NOT EXISTS analytics`,
	`CREATE SCHEMA IF NOT EXISTS shop.sales`,

	`CREATE TABLE IF NOT EXISTS shop.public.customers (
		id INT PRIMARY KEY,
		name STRING NOT NULL,
		email STRING UNIQUE,
		created_at TIMESTAMPTZ DEFAULT now()
	)`,
	`COMMENT ON TABLE shop.public.customers IS 'People who buy things'`,
	`COMMENT ON COLUMN shop.public.customers.email IS 'Primary contact address'`,

	`CREATE TABLE IF NOT EXISTS shop.public.orders (
		id INT PRIMARY KEY,
		customer_id INT NOT NULL REFERENCES shop.public.customers(id),
		total DECIMAL(10,2) NOT NULL,
		status STRING DEFAULT 'new',
		total_with_tax DECIMAL AS (total * 1.21) STORED
	)`,
	`CREATE TABLE IF NOT EXISTS shop.sales.regions (
		code STRING PRIMARY KEY,
		name STRING
	)`,
	// No primary key: CockroachDB adds a hidden rowid column.
	`CREATE TABLE IF NOT EXISTS shop.public.audit_log (
		message STRING,
		logged_at TIMESTAMPTZ DEFAULT now()
	)`,
	// Hash-sharded primary key: CockroachDB adds a hidden shard column.
	`CREATE TABLE IF NOT EXISTS shop.public.events (
		id INT PRIMARY KEY USING HASH WITH (bucket_count = 8),
		payload JSONB
	)`,
	`CREATE TABLE IF NOT EXISTS shop.public.shipments (
		id INT,
		region STRING,
		PRIMARY KEY (region, id)
	) PARTITION BY LIST (region) (PARTITION eu VALUES IN ('eu'), PARTITION us VALUES IN ('us'))`,

	`CREATE VIEW IF NOT EXISTS shop.public.customer_totals AS
		SELECT c.id, c.name, sum(o.total) AS total
		FROM shop.public.orders AS o JOIN shop.public.customers AS c ON c.id = o.customer_id
		GROUP BY c.id, c.name`,
	`CREATE MATERIALIZED VIEW IF NOT EXISTS shop.public.order_summary AS
		SELECT status, count(*) AS n FROM shop.public.orders GROUP BY status`,

	`UPSERT INTO shop.public.customers (id, name, email) VALUES (1, 'alice', 'alice@example.com'), (2, 'bob', NULL), (3, 'carol', 'carol@example.com')`,
	`UPSERT INTO shop.public.orders (id, customer_id, total, status) VALUES (10, 1, 100.00, 'paid'), (11, 1, 20.50, 'new'), (12, 2, 7.25, 'paid')`,
	`UPSERT INTO shop.sales.regions (code, name) VALUES ('eu', 'Europe'), ('us', 'United States')`,
	`UPSERT INTO shop.public.events (id, payload) VALUES (1, '{"kind": "click"}')`,

	// Row counts come from the optimizer statistics, so collect them now
	// rather than waiting for the automatic collection.
	`ANALYZE shop.public.customers`,
	`ANALYZE shop.public.orders`,

	`CREATE TABLE IF NOT EXISTS analytics.public.page_views (
		id INT PRIMARY KEY,
		path STRING,
		viewed_at TIMESTAMPTZ
	)`,
	`UPSERT INTO analytics.public.page_views (id, path, viewed_at) VALUES (1, '/', now())`,
}

type e2eEnv struct {
	host string
	port int
}

func e2e(t *testing.T) e2eEnv {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_COCKROACHDB_HOST")
	if host == "" {
		t.Skip("set MARMOT_TEST_COCKROACHDB_HOST (and MARMOT_TEST_COCKROACHDB_PORT) to run the CockroachDB e2e tests")
	}

	port := 26257
	if raw := os.Getenv("MARMOT_TEST_COCKROACHDB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_COCKROACHDB_PORT must be a number")
		port = parsed
	}

	env := e2eEnv{host: host, port: port}
	seedOnce(t, env)
	return env
}

func (e e2eEnv) config() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host": e.host,
		"port": e.port,
		"user": "root",
	}
}

var (
	seedMu   sync.Mutex
	seedDone bool
)

func seedOnce(t *testing.T, env e2eEnv) {
	t.Helper()

	seedMu.Lock()
	defer seedMu.Unlock()
	if seedDone {
		return
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, fmt.Sprintf("postgres://root@%s:%d/defaultdb?sslmode=disable", env.host, env.port))
	require.NoError(t, err, "connecting to CockroachDB to seed the fixture")
	defer conn.Close(ctx)

	for _, stmt := range seedStatements {
		_, err := conn.Exec(ctx, stmt)
		require.NoError(t, err, "seeding: %s", stmt)
	}
	seedDone = true
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

var (
	discoverMu     sync.Mutex
	discoverResult *pluginsdk.DiscoveryResult
)

// discoverAll runs one default discovery over the wire and shares the
// result, so each behaviour below can be checked in its own test without
// paying for a full run every time.
func discoverAll(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	env := e2e(t)

	discoverMu.Lock()
	defer discoverMu.Unlock()
	if discoverResult != nil {
		return discoverResult
	}

	result, err := buildBinary(t).Discover(t.Context(), env.config())
	require.NoError(t, err)
	require.NotNil(t, result)
	discoverResult = result
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, mrn string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].MRN != nil && *result.Assets[i].MRN == mrn {
			return &result.Assets[i]
		}
	}
	return nil
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected columns on %s", *a.MRN)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func statisticsOf(result *pluginsdk.DiscoveryResult, mrn string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == mrn {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func TestE2E_Meta(t *testing.T) {
	e2e(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "cockroachdb", meta.ID)
	assert.Equal(t, "CockroachDB", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Source implements DataFetcher")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	e2e(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"user": "root"})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheRealConfig(t *testing.T) {
	env := e2e(t)

	_, err := buildBinary(t).Validate(t.Context(), env.config())
	require.NoError(t, err)
}

func TestE2E_DiscoversEveryDatabaseExceptSystem(t *testing.T) {
	result := discoverAll(t)

	shop := findAsset(result, shopMRN)
	require.NotNil(t, shop)
	assert.Equal(t, "Database", shop.Type)
	assert.Equal(t, []string{"CockroachDB"}, shop.Providers)
	assert.Equal(t, "shop", *shop.Name)
	assert.Equal(t, "root", shop.Metadata["owner"])
	assert.Contains(t, shop.Metadata["server_version"], "CockroachDB")

	assert.NotNil(t, findAsset(result, "mrn://database/cockroachdb/analytics"))
	assert.Nil(t, findAsset(result, "mrn://database/cockroachdb/system"), "system is excluded by default")
}

func TestE2E_TableNamesAreFullyQualified(t *testing.T) {
	result := discoverAll(t)

	orders := findAsset(result, ordersMRN)
	require.NotNil(t, orders)
	assert.Equal(t, "shop.public.orders", *orders.Name)
	assert.Equal(t, "Table", orders.Type)
	assert.Equal(t, "shop", orders.Metadata["database"])
	assert.Equal(t, "public", orders.Metadata["schema"])
	assert.Equal(t, "orders", orders.Metadata["table_name"])

	regions := findAsset(result, "mrn://table/cockroachdb/shop.sales.regions")
	require.NotNil(t, regions, "a table in a second schema")
	assert.Equal(t, "sales", regions.Metadata["schema"])

	assert.NotNil(t, findAsset(result, "mrn://table/cockroachdb/analytics.public.page_views"), "a table in a second database")
}

func TestE2E_DatabaseContainsItsTablesAndViews(t *testing.T) {
	result := discoverAll(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: shopMRN, Target: ordersMRN, Type: "CONTAINS"})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: shopMRN, Target: customerTotalsMRN, Type: "CONTAINS"})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://database/cockroachdb/analytics",
		Target: "mrn://table/cockroachdb/analytics.public.page_views",
		Type:   "CONTAINS",
	})
}

func TestE2E_TableCommentBecomesTheDescription(t *testing.T) {
	result := discoverAll(t)

	customers := findAsset(result, customersMRN)
	require.NotNil(t, customers)
	require.NotNil(t, customers.Description)
	assert.Equal(t, "People who buy things", *customers.Description)
}

func TestE2E_HiddenRowIDIsExcluded(t *testing.T) {
	result := discoverAll(t)

	auditLog := findAsset(result, "mrn://table/cockroachdb/shop.public.audit_log")
	require.NotNil(t, auditLog)

	columns := columnsOf(t, auditLog)
	assert.Contains(t, columns, "message")
	assert.Contains(t, columns, "logged_at")
	assert.NotContains(t, columns, "rowid")
	assert.Len(t, columns, 2)
}

func TestE2E_HiddenShardColumnIsExcluded(t *testing.T) {
	result := discoverAll(t)

	events := findAsset(result, "mrn://table/cockroachdb/shop.public.events")
	require.NotNil(t, events)

	columns := columnsOf(t, events)
	assert.Len(t, columns, 2)
	assert.NotContains(t, columns, "crdb_internal_id_shard_8")
	require.Contains(t, columns, "id")
	assert.Equal(t, true, columns["id"]["is_primary_key"])
	assert.Equal(t, "JSONB", columns["payload"]["data_type"])
}

func TestE2E_ColumnsCarryTypesKeysDefaultsAndComputedExpressions(t *testing.T) {
	result := discoverAll(t)

	orders := findAsset(result, ordersMRN)
	require.NotNil(t, orders)
	columns := columnsOf(t, orders)
	require.Len(t, columns, 5)

	assert.Equal(t, true, columns["id"]["is_primary_key"])
	assert.Equal(t, false, columns["id"]["is_nullable"])
	assert.Equal(t, "INT8", columns["id"]["data_type"])
	assert.Equal(t, "DECIMAL(10,2)", columns["total"]["data_type"])
	assert.Equal(t, "'new'", columns["status"]["default_expression"])
	assert.Equal(t, true, columns["status"]["is_nullable"])
	assert.Equal(t, "total * 1.21", columns["total_with_tax"]["generation_expression"])
	assert.NotContains(t, columns["total"], "generation_expression")
}

func TestE2E_ColumnCommentBecomesTheColumnDescription(t *testing.T) {
	result := discoverAll(t)

	customers := findAsset(result, customersMRN)
	require.NotNil(t, customers)
	columns := columnsOf(t, customers)

	assert.Equal(t, "Primary contact address", columns["email"]["description"])
	assert.NotContains(t, columns["name"], "description")
}

func TestE2E_ForeignKeyBecomesALineageEdge(t *testing.T) {
	result := discoverAll(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: ordersMRN, Target: customersMRN, Type: "FOREIGN_KEY"})
}

func TestE2E_ViewIsLinkedToTheTablesItReads(t *testing.T) {
	result := discoverAll(t)

	// Base table -> view, the direction the data flows.
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: ordersMRN, Target: customerTotalsMRN, Type: "VIEW_OF"})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: customersMRN, Target: customerTotalsMRN, Type: "VIEW_OF"})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: ordersMRN, Target: orderSummaryMRN, Type: "VIEW_OF"})
}

func TestE2E_ViewCarriesItsDefinition(t *testing.T) {
	result := discoverAll(t)

	view := findAsset(result, customerTotalsMRN)
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "view", view.Metadata["object_type"])
	assert.NotContains(t, view.Metadata, "materialized")
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM shop.public.orders")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
}

func TestE2E_MaterializedViewIsFlagged(t *testing.T) {
	result := discoverAll(t)

	view := findAsset(result, orderSummaryMRN)
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "materialized_view", view.Metadata["object_type"])
	assert.Equal(t, true, view.Metadata["materialized"])
	require.NotNil(t, view.Query)

	// The materialized view's hidden rowid is dropped like a table's.
	columns := columnsOf(t, view)
	assert.Len(t, columns, 2)
	assert.NotContains(t, columns, "rowid")
}

func TestE2E_PartitionedTableListsItsPartitionColumns(t *testing.T) {
	result := discoverAll(t)

	shipments := findAsset(result, "mrn://table/cockroachdb/shop.public.shipments")
	require.NotNil(t, shipments)
	assert.Equal(t, true, shipments.Metadata["partitioned"])
	assert.Equal(t, "region", shipments.Metadata["partition_columns"])

	orders := findAsset(result, ordersMRN)
	require.NotNil(t, orders)
	assert.NotContains(t, orders.Metadata, "partitioned")
}

func TestE2E_RowAndColumnStatistics(t *testing.T) {
	result := discoverAll(t)

	customers := statisticsOf(result, customersMRN)
	assert.Equal(t, float64(3), customers["asset.row_count"])
	assert.Equal(t, float64(4), customers["asset.column_count"])
	assert.Greater(t, customers["asset.size_bytes"], float64(0))

	orders := statisticsOf(result, ordersMRN)
	assert.Equal(t, float64(3), orders["asset.row_count"])
	assert.Equal(t, float64(5), orders["asset.column_count"], "hidden columns do not count")

	ordersAsset := findAsset(result, ordersMRN)
	require.NotNil(t, ordersAsset)
	assert.EqualValues(t, 3, ordersAsset.Metadata["estimated_row_count"])
}

func TestE2E_PlainViewsHaveNoRowCount(t *testing.T) {
	result := discoverAll(t)

	stats := statisticsOf(result, customerTotalsMRN)
	assert.NotContains(t, stats, "asset.row_count")
	assert.NotContains(t, stats, "asset.size_bytes")
	assert.Equal(t, float64(3), stats["asset.column_count"])
}

func TestE2E_SingleDatabaseConfigDiscoversOnlyThatDatabase(t *testing.T) {
	env := e2e(t)

	config := env.config()
	config["database"] = "analytics"

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "mrn://database/cockroachdb/analytics"))
	assert.NotNil(t, findAsset(result, "mrn://table/cockroachdb/analytics.public.page_views"))
	assert.Nil(t, findAsset(result, shopMRN))
	assert.Nil(t, findAsset(result, ordersMRN))
}

func TestE2E_UnknownDatabaseFails(t *testing.T) {
	env := e2e(t)

	config := env.config()
	config["database"] = "does_not_exist"

	_, err := buildBinary(t).Discover(t.Context(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does_not_exist")
}

func TestE2E_ViewsCanBeSkipped(t *testing.T) {
	env := e2e(t)

	config := env.config()
	config["include_views"] = false

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, ordersMRN))
	assert.Nil(t, findAsset(result, customerTotalsMRN))
	assert.Nil(t, findAsset(result, orderSummaryMRN))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "VIEW_OF", edge.Type)
	}
}

func TestE2E_WrongCredentialsFail(t *testing.T) {
	env := e2e(t)

	config := env.config()
	config["user"] = "nobody_here"
	config["password"] = "wrong"

	_, err := buildBinary(t).Discover(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	env := e2e(t)
	result := discoverAll(t)

	orders := findAsset(result, ordersMRN)
	require.NotNil(t, orders)

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), env.config(), orders)
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "customer_id", "total", "status", "total_with_tax"}, columns)
	require.Len(t, rows, 3)

	statuses := make(map[interface{}]int)
	for _, row := range rows {
		require.Len(t, row, 5)
		statuses[row[3]]++
	}
	assert.Equal(t, 2, statuses["paid"])
	assert.Equal(t, 1, statuses["new"])
}

func TestE2E_FetchSampleDataFromAView(t *testing.T) {
	env := e2e(t)
	result := discoverAll(t)

	view := findAsset(result, customerTotalsMRN)
	require.NotNil(t, view)

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), env.config(), view)
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "name", "total"}, columns)
	assert.Len(t, rows, 2, "alice and bob have orders, carol has none")
}
