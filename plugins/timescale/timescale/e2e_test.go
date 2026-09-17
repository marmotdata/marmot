package timescale_test

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
// protocol the Marmot host uses: plugintest.Build compiles the main package
// and every call spawns the process, runs one RPC and kills it again.
//
// They need a real TimescaleDB. Bring one up with:
//
//	docker run -d --name marmot-test-timescale -p 15436:5432 \
//	  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=shop \
//	  timescale/timescaledb:latest-pg16
//
// then seed it with the fixture in testdata/seed.sql and run:
//
//	MARMOT_TEST_TIMESCALE_HOST=localhost MARMOT_TEST_TIMESCALE_PORT=15436 go test ./...

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_TIMESCALE_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_TIMESCALE_HOST is not set, skipping the TimescaleDB end to end tests")
	}

	port := 5432
	if raw := os.Getenv("MARMOT_TEST_TIMESCALE_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_TIMESCALE_PORT must be a number")
		port = parsed
	}

	return pluginsdk.RawConfig{
		"host":           host,
		"port":           port,
		"user":           "postgres",
		"password":       "postgres",
		"database":       "shop",
		"include_chunks": true,
	}
}

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}
	t.Fatalf("no asset named %q in %d assets", name, len(result.Assets))
	return pluginsdk.Asset{}
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "timescale", meta.ID)
	assert.Equal(t, "TimescaleDB", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "the plugin implements DataFetcher")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"user": "postgres"})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheTestConfig(t *testing.T) {
	config := e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.NoError(t, err)
}

func TestE2E_DiscoversTheDatabase(t *testing.T) {
	result := discover(t)

	database := assetNamed(t, result, "shop")
	assert.Equal(t, "Database", database.Type)
	assert.Equal(t, "mrn://database/postgresql/shop", *database.MRN)
	assert.Equal(t, []string{"PostgreSQL"}, database.Providers)
}

func TestE2E_DiscoversThePlainTable(t *testing.T) {
	result := discover(t)

	customers := assetNamed(t, result, "customers")
	assert.Equal(t, "Table", customers.Type)
	assert.Equal(t, "mrn://table/postgresql/customers", *customers.MRN)
	assert.Equal(t, "Shop customers", customers.Metadata["comment"])
	assert.Equal(t, "public", customers.Metadata["schema"])
}

func TestE2E_PlainTableIsNotMarkedAsAHypertable(t *testing.T) {
	result := discover(t)

	customers := assetNamed(t, result, "customers")
	_, present := customers.Metadata["hypertable"]
	assert.False(t, present)
}

func TestE2E_DiscoversColumnsWithTypesAndKeys(t *testing.T) {
	result := discover(t)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(assetNamed(t, result, "customers").Schema["columns"]), &columns))

	byName := make(map[string]map[string]interface{})
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}

	require.Contains(t, byName, "id")
	assert.Equal(t, "integer", byName["id"]["data_type"])
	assert.Equal(t, true, byName["id"]["is_primary_key"])

	require.Contains(t, byName, "email")
	assert.Equal(t, "text", byName["email"]["data_type"])
	assert.Equal(t, true, byName["email"]["is_nullable"])
	assert.Equal(t, "Contact email address", byName["email"]["description"])
}

func TestE2E_HypertableIsMarkedAndCarriesItsTimeDimension(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	assert.Equal(t, "Table", conditions.Type)
	assert.Equal(t, "mrn://table/postgresql/conditions", *conditions.MRN)
	assert.Equal(t, true, conditions.Metadata["hypertable"])
	assert.Equal(t, "time", conditions.Metadata["time_column"])
	assert.Equal(t, "1 day", conditions.Metadata["time_interval"])
	assert.NotEmpty(t, conditions.Metadata["timescaledb_version"])
}

func TestE2E_HypertableRecordsItsSpaceDimension(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	partitions, ok := conditions.Metadata["space_partitions"].([]interface{})
	require.True(t, ok, "space_partitions should be a list, got %T", conditions.Metadata["space_partitions"])
	require.Len(t, partitions, 1)

	partition := partitions[0].(map[string]interface{})
	assert.Equal(t, "device_id", partition["column"])
	assert.EqualValues(t, 4, partition["partitions"])
}

func TestE2E_HypertableHasMoreThanOneChunk(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	chunks, ok := conditions.Metadata["num_chunks"].(float64)
	require.True(t, ok, "num_chunks should be a number, got %T", conditions.Metadata["num_chunks"])
	assert.Greater(t, chunks, float64(1), "the fixture spans several days at a one day chunk interval")
}

func TestE2E_HypertableRecordsItsChunkRange(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	chunkRange, ok := conditions.Metadata["chunk_range"].(map[string]interface{})
	require.True(t, ok, "chunk_range should be a map, got %T", conditions.Metadata["chunk_range"])
	assert.NotEmpty(t, chunkRange["oldest"])
	assert.NotEmpty(t, chunkRange["newest"])
}

func TestE2E_HypertableRecordsItsCompressionSettings(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	assert.Equal(t, true, conditions.Metadata["compression_enabled"])
	assert.Equal(t, []interface{}{"device_id"}, conditions.Metadata["compression_segment_by"])
	assert.Equal(t, []interface{}{"time DESC"}, conditions.Metadata["compression_order_by"])
}

func TestE2E_HypertableRecordsItsPolicies(t *testing.T) {
	result := discover(t)

	conditions := assetNamed(t, result, "conditions")
	policies, ok := conditions.Metadata["policies"].([]interface{})
	require.True(t, ok, "policies should be a list, got %T", conditions.Metadata["policies"])

	procs := make(map[string]bool)
	for _, raw := range policies {
		procs[raw.(map[string]interface{})["proc"].(string)] = true
	}
	assert.True(t, procs["policy_compression"], "expected the compression policy")
	assert.True(t, procs["policy_retention"], "expected the retention policy")
}

func TestE2E_ContinuousAggregateIsAViewWithItsOriginalQuery(t *testing.T) {
	result := discover(t)

	daily := assetNamed(t, result, "conditions_daily")
	assert.Equal(t, "View", daily.Type)
	assert.Equal(t, "mrn://view/postgresql/conditions_daily", *daily.MRN)
	assert.Equal(t, true, daily.Metadata["continuous_aggregate"])
	assert.Equal(t, "public.conditions", daily.Metadata["source_hypertable"])

	require.NotNil(t, daily.Query)
	// PostgreSQL rewrites the view to read from the hidden materialization
	// hypertable; the TimescaleDB catalog keeps the query as it was written.
	assert.Contains(t, *daily.Query, "time_bucket")
	assert.Contains(t, *daily.Query, "FROM conditions")
	assert.NotContains(t, *daily.Query, "_materialized_hypertable")
}

func TestE2E_ContinuousAggregateRecordsItsRefreshPolicy(t *testing.T) {
	result := discover(t)

	daily := assetNamed(t, result, "conditions_daily")
	policy, ok := daily.Metadata["refresh_policy"].(map[string]interface{})
	require.True(t, ok, "refresh_policy should be a map, got %T", daily.Metadata["refresh_policy"])
	assert.Equal(t, "policy_refresh_continuous_aggregate", policy["proc"])
	assert.Equal(t, "1 hour", policy["schedule"])
}

func TestE2E_ContinuousAggregateHasAViewOfEdgeFromItsHypertable(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/conditions",
		Target: "mrn://view/postgresql/conditions_daily",
		Type:   "VIEW_OF",
	})
}

func TestE2E_DiscoversThePlainViewAndItsSources(t *testing.T) {
	result := discover(t)

	readings := assetNamed(t, result, "customer_readings")
	assert.Equal(t, "View", readings.Type)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/customers",
		Target: "mrn://view/postgresql/customer_readings",
		Type:   "VIEW_OF",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/conditions",
		Target: "mrn://view/postgresql/customer_readings",
		Type:   "VIEW_OF",
	})
}

func TestE2E_ForeignKeyBecomesAnEdge(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/conditions",
		Target: "mrn://table/postgresql/customers",
		Type:   "FOREIGN_KEY",
	})
}

func TestE2E_DatabaseContainsItsObjects(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://database/postgresql/shop",
		Target: "mrn://table/postgresql/conditions",
		Type:   "CONTAINS",
	})
}

// TimescaleDB creates a chunk table per time slice, a materialization
// hypertable per continuous aggregate and a schema full of catalog views.
// None of that belongs in a data catalog.
func TestE2E_NoInternalObjectLeaksIntoTheResults(t *testing.T) {
	result := discover(t)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		assert.NotContains(t, *asset.Name, "_hyper_", "chunk table leaked: %s", *asset.Name)
		assert.NotContains(t, *asset.Name, "_materialized_hypertable", "materialization hypertable leaked: %s", *asset.Name)

		schema, _ := asset.Metadata["schema"].(string)
		assert.NotContains(t, schema, "_timescaledb", "internal schema leaked: %s.%s", schema, *asset.Name)
		assert.NotEqual(t, "timescaledb_information", schema)
		assert.NotEqual(t, "timescaledb_experimental", schema)
	}
}

func TestE2E_NoEdgePointsAtAnAssetThatWasNotCreated(t *testing.T) {
	result := discover(t)

	created := make(map[string]bool, len(result.Assets))
	for _, asset := range result.Assets {
		created[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, created[edge.Source], "edge source %s was never created", edge.Source)
		assert.True(t, created[edge.Target], "edge target %s was never created", edge.Target)
	}
}

// A hypertable's parent table holds no rows and no data of its own, so
// PostgreSQL's own estimates report zero for it.
func TestE2E_HypertableStatisticsComeFromTimescale(t *testing.T) {
	result := discover(t)

	statistics := make(map[string]float64)
	for _, statistic := range result.Statistics {
		if statistic.AssetMRN == "mrn://table/postgresql/conditions" {
			statistics[statistic.MetricName] = statistic.Value
		}
	}

	assert.Equal(t, float64(145), statistics["asset.row_count"], "approximate_row_count, not reltuples")
	assert.Greater(t, statistics["asset.size_bytes"], float64(100000), "hypertable_size, not pg_total_relation_size of the empty parent")
	assert.Greater(t, statistics["asset.chunk_count"], float64(1))
	assert.Greater(t, statistics["asset.column_count"], float64(0))
}

func TestE2E_PlainTableStatisticsComeFromPostgreSQL(t *testing.T) {
	result := discover(t)

	statistics := make(map[string]float64)
	for _, statistic := range result.Statistics {
		if statistic.AssetMRN == "mrn://table/postgresql/customers" {
			statistics[statistic.MetricName] = statistic.Value
		}
	}

	assert.Equal(t, float64(3), statistics["asset.row_count"])
	assert.Equal(t, float64(4), statistics["asset.column_count"])
	_, present := statistics["asset.chunk_count"]
	assert.False(t, present, "a plain table has no chunks")
}

func TestE2E_FetchSampleDataFromAHypertable(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	conditions := assetNamed(t, result, "conditions")

	columns, rows, err := bin.FetchSampleData(t.Context(), config, &conditions)
	require.NoError(t, err)

	assert.Contains(t, columns, "time")
	assert.Contains(t, columns, "device_id")
	assert.Contains(t, columns, "temperature")
	assert.NotEmpty(t, rows)
	assert.Len(t, rows[0], len(columns))
}

func TestE2E_FetchSampleDataFromAContinuousAggregate(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	daily := assetNamed(t, result, "conditions_daily")

	columns, rows, err := bin.FetchSampleData(t.Context(), config, &daily)
	require.NoError(t, err)

	assert.Contains(t, columns, "bucket")
	assert.Contains(t, columns, "avg_temp")
	assert.NotEmpty(t, rows)
}
