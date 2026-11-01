package starrocks_test

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real StarRocks cluster.
//
// Bring one up, apply testdata/seed.sql to it, then:
//
//	MARMOT_TEST_STARROCKS_HOST=localhost \
//	MARMOT_TEST_STARROCKS_PORT=19031 \
//	MARMOT_TEST_STARROCKS_USER=root \
//	go test ./starrocks -run TestE2E -v

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_STARROCKS_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_STARROCKS_HOST is not set, skipping the StarRocks end to end tests")
	}

	port := 9030
	if raw := os.Getenv("MARMOT_TEST_STARROCKS_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_STARROCKS_PORT must be a number")
		port = parsed
	}

	user := os.Getenv("MARMOT_TEST_STARROCKS_USER")
	if user == "" {
		user = "root"
	}

	return pluginsdk.RawConfig{
		"host":      host,
		"port":      port,
		"user":      user,
		"password":  os.Getenv("MARMOT_TEST_STARROCKS_PASSWORD"),
		"databases": []string{"shop"},
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// discoverOnce runs discovery for the whole test binary once, since a
// full run takes a few seconds and every assertion reads the same result.
func discoverOnce(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// Metadata crosses the gRPC wire as JSON, so numbers arrive as
// float64 no matter what the plugin put in the map. Numeric
// assertions below use EqualValues for that reason.

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, mrnValue string) pluginsdk.Asset {
	t.Helper()

	for _, a := range result.Assets {
		if a.MRN != nil && *a.MRN == mrnValue {
			return a
		}
	}
	t.Fatalf("no asset with MRN %s in the %d discovered", mrnValue, len(result.Assets))
	return pluginsdk.Asset{}
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func columnsOf(t *testing.T, a pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "asset %s has no columns", *a.MRN)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &list))

	out := make(map[string]map[string]any, len(list))
	for _, c := range list {
		name, _ := c["column_name"].(string)
		out[name] = c
	}
	return out
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "starrocks", meta.ID)
	assert.Equal(t, "StarRocks", meta.Name)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"user": "root"})

	require.Error(t, err)
}

func TestE2E_ValidateMissingUserFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": "localhost"})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAFullConfig(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), config)

	require.NoError(t, err)
}

func TestE2E_DiscoverFailsAgainstAnUnknownCatalog(t *testing.T) {
	config := e2eConfig(t)
	config["catalog"] = "no_such_catalog"
	bin := buildBinary(t)

	_, err := bin.Discover(t.Context(), config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_such_catalog")
}

func TestE2E_DiscoversTheDatabaseAsset(t *testing.T) {
	result := discoverOnce(t)

	db := findAsset(t, result, "mrn://database/starrocks/shop")

	assert.Equal(t, "Database", db.Type)
	assert.Equal(t, []string{"StarRocks"}, db.Providers)
	assert.Equal(t, "shop", *db.Name)
	assert.Equal(t, "shop", db.Metadata["database"])
	assert.Equal(t, "default_catalog", db.Metadata["catalog"])
	assert.Equal(t, "Internal", db.Metadata["catalog_type"])
	assert.NotEmpty(t, db.Metadata["starrocks_version"])
	assert.EqualValues(t, 4, db.Metadata["table_count"])
	assert.EqualValues(t, 2, db.Metadata["view_count"])
}

func TestE2E_DiscoversADuplicateKeyTable(t *testing.T) {
	result := discoverOnce(t)

	events := findAsset(t, result, "mrn://table/starrocks/shop.events")

	assert.Equal(t, "Table", events.Type)
	assert.Equal(t, "shop.events", *events.Name)
	assert.Equal(t, "table", events.Metadata["object_type"])
	assert.Equal(t, "StarRocks", events.Metadata["engine"])
	assert.Equal(t, "DUPLICATE", events.Metadata["key_model"])
	assert.Equal(t, "event_date, event_id", events.Metadata["key_columns"])
	require.NotNil(t, events.Description)
	assert.Equal(t, "Raw clickstream events", *events.Description)
}

func TestE2E_DiscoversAPrimaryKeyTable(t *testing.T) {
	result := discoverOnce(t)

	customers := findAsset(t, result, "mrn://table/starrocks/shop.customers")

	assert.Equal(t, "PRIMARY", customers.Metadata["key_model"])
	assert.Equal(t, "customer_id", customers.Metadata["key_columns"])
	assert.Equal(t, "Customer master record", customers.Metadata["comment"])
}

func TestE2E_DiscoversAnAggregateKeyTable(t *testing.T) {
	result := discoverOnce(t)

	stats := findAsset(t, result, "mrn://table/starrocks/shop.daily_stats")

	assert.Equal(t, "AGGREGATE", stats.Metadata["key_model"])
	assert.Equal(t, "stat_date, country", stats.Metadata["key_columns"])
}

func TestE2E_DiscoversAUniqueKeyTable(t *testing.T) {
	result := discoverOnce(t)

	orders := findAsset(t, result, "mrn://table/starrocks/shop.orders")

	assert.Equal(t, "UNIQUE", orders.Metadata["key_model"])
	assert.Equal(t, "order_id, order_date", orders.Metadata["key_columns"])
	assert.Equal(t, "order_date, order_id", orders.Metadata["order_by"])
}

func TestE2E_ReportsRangePartitioning(t *testing.T) {
	result := discoverOnce(t)

	events := findAsset(t, result, "mrn://table/starrocks/shop.events")

	assert.Equal(t, "RANGE", events.Metadata["partition_type"])
	assert.Equal(t, "event_date", events.Metadata["partition_columns"])
	assert.EqualValues(t, 2, events.Metadata["partition_count"], "the fixture declares two range partitions")
}

func TestE2E_ReportsHashDistribution(t *testing.T) {
	result := discoverOnce(t)

	events := findAsset(t, result, "mrn://table/starrocks/shop.events")

	assert.Equal(t, "HASH", events.Metadata["distribution"])
	assert.Equal(t, "event_id", events.Metadata["distribution_columns"])
	assert.EqualValues(t, 4, events.Metadata["buckets"])
	assert.EqualValues(t, 1, events.Metadata["replication_num"])
}

func TestE2E_KeepsTheTableDDL(t *testing.T) {
	result := discoverOnce(t)

	events := findAsset(t, result, "mrn://table/starrocks/shop.events")

	ddl, ok := events.Metadata["ddl"].(string)
	require.True(t, ok)
	assert.Contains(t, ddl, "CREATE TABLE `events`")
	assert.Contains(t, ddl, "DUPLICATE KEY")
}

func TestE2E_ReportsPrimaryKeyColumns(t *testing.T) {
	result := discoverOnce(t)

	columns := columnsOf(t, findAsset(t, result, "mrn://table/starrocks/shop.customers"))

	require.Contains(t, columns, "customer_id")
	assert.Equal(t, true, columns["customer_id"]["is_primary_key"])
	assert.Equal(t, "Customer identifier", columns["customer_id"]["description"])
	assert.Equal(t, false, columns["customer_id"]["is_nullable"])
	assert.Nil(t, columns["email"]["is_primary_key"], "email is not part of the primary key")
}

func TestE2E_ReportsSortingKeyColumnsForADuplicateKeyTable(t *testing.T) {
	result := discoverOnce(t)

	columns := columnsOf(t, findAsset(t, result, "mrn://table/starrocks/shop.events"))

	require.Contains(t, columns, "event_date")
	assert.Equal(t, true, columns["event_date"]["is_sorting_key"])
	assert.Nil(t, columns["event_date"]["is_primary_key"], "a DUPLICATE key does not identify a row")
}

func TestE2E_ReportsAggregationTypes(t *testing.T) {
	result := discoverOnce(t)

	columns := columnsOf(t, findAsset(t, result, "mrn://table/starrocks/shop.daily_stats"))

	assert.Equal(t, "SUM", columns["order_total"]["aggregation_type"])
	assert.Equal(t, "MAX", columns["max_order"]["aggregation_type"])
	assert.Equal(t, "REPLACE", columns["last_channel"]["aggregation_type"])
	assert.Equal(t, "web", columns["last_channel"]["default_expression"])
}

func TestE2E_KeepsComplexColumnTypesVerbatim(t *testing.T) {
	result := discoverOnce(t)

	columns := columnsOf(t, findAsset(t, result, "mrn://table/starrocks/shop.events"))

	assert.Equal(t, "array<int>", columns["tag_ids"]["data_type"])
	assert.Equal(t, "json", columns["payload"]["data_type"])
	assert.Equal(t, "Tag identifiers", columns["tag_ids"]["description"])
}

func TestE2E_DiscoversAViewWithItsQuery(t *testing.T) {
	result := discoverOnce(t)

	view := findAsset(t, result, "mrn://view/starrocks/shop.daily_sales")

	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "view", view.Metadata["object_type"])
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM orders o JOIN customers c")
	assert.NotContains(t, *view.Query, "CREATE VIEW")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
	assert.NotContains(t, view.Metadata, "materialized")
}

func TestE2E_DiscoversAMaterializedView(t *testing.T) {
	result := discoverOnce(t)

	mv := findAsset(t, result, "mrn://view/starrocks/shop.mv_sales")

	assert.Equal(t, "View", mv.Type)
	assert.Equal(t, "materialized_view", mv.Metadata["object_type"])
	assert.Equal(t, true, mv.Metadata["materialized"])
	assert.Equal(t, "ASYNC", mv.Metadata["refresh_type"])
	assert.Equal(t, true, mv.Metadata["is_active"])
	assert.Equal(t, "SUCCESS", mv.Metadata["last_refresh_state"])
	assert.NotEmpty(t, mv.Metadata["task_name"])
	assert.NotEmpty(t, mv.Metadata["last_refresh_start_time"])
	require.NotNil(t, mv.Query)
	assert.Contains(t, *mv.Query, "FROM orders")
}

func TestE2E_DatabaseContainsItsTablesAndViews(t *testing.T) {
	result := discoverOnce(t)

	db := "mrn://database/starrocks/shop"

	assert.True(t, hasEdge(result, db, "mrn://table/starrocks/shop.events", "CONTAINS"))
	assert.True(t, hasEdge(result, db, "mrn://table/starrocks/shop.orders", "CONTAINS"))
	assert.True(t, hasEdge(result, db, "mrn://view/starrocks/shop.daily_sales", "CONTAINS"))
	assert.True(t, hasEdge(result, db, "mrn://view/starrocks/shop.mv_sales", "CONTAINS"))
}

func TestE2E_ViewOfPointsFromTheBaseTablesToTheView(t *testing.T) {
	result := discoverOnce(t)

	view := "mrn://view/starrocks/shop.daily_sales"

	assert.True(t, hasEdge(result, "mrn://table/starrocks/shop.orders", view, "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/starrocks/shop.customers", view, "VIEW_OF"))
}

func TestE2E_MaterializedViewPointsBackAtItsBaseTable(t *testing.T) {
	result := discoverOnce(t)

	assert.True(t, hasEdge(result,
		"mrn://table/starrocks/shop.orders", "mrn://view/starrocks/shop.mv_sales", "VIEW_OF"))
}

func TestE2E_EveryAssetMRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name); an
	// asset that disagrees is stored under a different MRN than the one
	// its own lineage edges point at.
	result := discoverOnce(t)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestE2E_EveryLineageEndpointIsADiscoveredAsset(t *testing.T) {
	// The server drops edges whose endpoint does not exist, so an edge
	// pointing outside the run is a silent loss of lineage.
	result := discoverOnce(t)

	known := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = struct{}{}
	}

	require.NotEmpty(t, result.Lineage)
	for _, e := range result.Lineage {
		assert.Contains(t, known, e.Source, "edge source is not a discovered asset")
		assert.Contains(t, known, e.Target, "edge target is not a discovered asset")
	}
}

func TestE2E_ReportsColumnCountStatistics(t *testing.T) {
	result := discoverOnce(t)

	var value float64
	var found bool
	for _, s := range result.Statistics {
		if s.AssetMRN == "mrn://table/starrocks/shop.events" && s.MetricName == "asset.column_count" {
			value, found = s.Value, true
		}
	}

	require.True(t, found, "no asset.column_count for shop.events")
	assert.Equal(t, float64(6), value, "the fixture declares six columns")
}

func TestE2E_ReportsRowCountAndSizeForTables(t *testing.T) {
	// The figures come from information_schema.tables, which the
	// all-in-one image reports as 0 until the backends have published
	// tablet statistics. The metrics must still be emitted.
	result := discoverOnce(t)

	metrics := map[string]bool{}
	for _, s := range result.Statistics {
		if s.AssetMRN == "mrn://table/starrocks/shop.orders" {
			metrics[s.MetricName] = true
		}
	}

	assert.True(t, metrics["asset.row_count"])
	assert.True(t, metrics["asset.size_bytes"])
}

func TestE2E_APlainViewHasNoRowCount(t *testing.T) {
	result := discoverOnce(t)

	for _, s := range result.Statistics {
		if s.AssetMRN == "mrn://view/starrocks/shop.daily_sales" {
			assert.Equal(t, "asset.column_count", s.MetricName)
		}
	}
}

func TestE2E_ExcludesSystemDatabases(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "databases")
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	for _, a := range result.Assets {
		assert.NotEqual(t, "information_schema", a.Metadata["database"])
		assert.NotEqual(t, "_statistics_", a.Metadata["database"])
		assert.NotEqual(t, "sys", a.Metadata["database"])
	}
}

func TestE2E_IncludeViewsFalseDropsViews(t *testing.T) {
	config := e2eConfig(t)
	config["include_views"] = false
	config["include_materialized_views"] = false
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	for _, a := range result.Assets {
		assert.NotEqual(t, "View", a.Type)
	}
}

func TestE2E_IncludeColumnsFalseDropsSchema(t *testing.T) {
	config := e2eConfig(t)
	config["include_columns"] = false
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	events := findAsset(t, result, "mrn://table/starrocks/shop.events")
	assert.NotContains(t, events.Schema, "columns")
}

func TestE2E_FetchSampleDataReturnsRows(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	asset := pluginsdk.Asset{
		Metadata: map[string]any{"database": "shop", "table_name": "events"},
	}

	columns, rows, err := bin.FetchSampleData(t.Context(), config, &asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"event_date", "event_id", "user_id", "event_type", "payload", "tag_ids"}, columns)
	assert.Len(t, rows, 3, "the fixture inserts three events")
}

func TestE2E_FetchSampleDataFailsWithoutATable(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	_, _, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{Metadata: map[string]any{}})

	require.Error(t, err)
}
