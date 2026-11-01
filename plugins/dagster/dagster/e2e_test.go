package dagster_test

import (
	"os"
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
// They need a Dagster webserver holding the fixture project described in the
// plugin's development notes: an op job etl_job with extract, transform and
// load, an asset job orders_job, and the assets raw_orders, clean_orders and
// order_metrics.

func dagsterURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("MARMOT_TEST_DAGSTER_URL")
	if url == "" {
		t.Skip("set MARMOT_TEST_DAGSTER_URL to a Dagster webserver to run the end to end tests")
	}
	return url
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"host": dagsterURL(t)})
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}
	t.Fatalf("no asset named %q among %d discovered assets", name, len(result.Assets))
	return pluginsdk.Asset{}
}

func containsEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "dagster", meta.ID)
	assert.Equal(t, "Dagster", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAHost(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": dagsterURL(t)})
	require.NoError(t, err)
}

func TestE2E_DiscoversBothJobsAsPipelines(t *testing.T) {
	result := discoverE2E(t)

	etl := findAsset(t, result, "etl_job")
	assert.Equal(t, "Pipeline", etl.Type)
	assert.Equal(t, []string{"Dagster"}, etl.Providers)
	assert.Equal(t, "mrn://pipeline/dagster/etl_job", *etl.MRN)

	orders := findAsset(t, result, "orders_job")
	assert.Equal(t, "Pipeline", orders.Type)
	assert.Equal(t, true, orders.Metadata["is_asset_job"])
}

func TestE2E_DoesNotDiscoverDagstersImplicitAssetJob(t *testing.T) {
	result := discoverE2E(t)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "__ASSET_JOB", *asset.Name)
	}
}

func TestE2E_PipelineCarriesItsCronScheduleAndSensor(t *testing.T) {
	result := discoverE2E(t)

	etl := findAsset(t, result, "etl_job")

	schedules, ok := etl.Metadata["schedules"].([]any)
	require.True(t, ok, "expected schedules on etl_job")
	require.Len(t, schedules, 1)
	schedule := schedules[0].(map[string]any)
	assert.Equal(t, "etl_schedule", schedule["name"])
	assert.Equal(t, "0 3 * * *", schedule["cron_schedule"])

	sensors, ok := etl.Metadata["sensors"].([]any)
	require.True(t, ok, "expected sensors on etl_job")
	require.Len(t, sensors, 1)
	assert.Equal(t, "etl_sensor", sensors[0].(map[string]any)["name"])
}

func TestE2E_DiscoversTheOpsOfTheOpJob(t *testing.T) {
	result := discoverE2E(t)

	for _, name := range []string{"etl_job/extract", "etl_job/transform", "etl_job/load"} {
		task := findAsset(t, result, name)
		assert.Equal(t, "Task", task.Type)
	}
}

func TestE2E_PipelineContainsItsTasks(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, containsEdge(result,
		"mrn://pipeline/dagster/etl_job",
		"mrn://task/dagster/etl_job-extract",
		"CONTAINS"))
}

func TestE2E_TasksDependOnTheOpFeedingThem(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, containsEdge(result,
		"mrn://task/dagster/etl_job-extract",
		"mrn://task/dagster/etl_job-transform",
		"DEPENDS_ON"))
	assert.True(t, containsEdge(result,
		"mrn://task/dagster/etl_job-extract",
		"mrn://task/dagster/etl_job-load",
		"DEPENDS_ON"))
}

func TestE2E_DiscoversTheThreeSoftwareDefinedAssets(t *testing.T) {
	result := discoverE2E(t)

	for _, name := range []string{"raw_orders", "clean_orders", "order_metrics"} {
		dataset := findAsset(t, result, name)
		assert.Equal(t, "Dataset", dataset.Type)
		assert.Equal(t, "mrn://dataset/dagster/"+name, *dataset.MRN)
	}
}

func TestE2E_DatasetsFeedTheirDownstreamAssets(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, containsEdge(result,
		"mrn://dataset/dagster/raw_orders",
		"mrn://dataset/dagster/clean_orders",
		"FEEDS"))
	assert.True(t, containsEdge(result,
		"mrn://dataset/dagster/clean_orders",
		"mrn://dataset/dagster/order_metrics",
		"FEEDS"))
}

func TestE2E_AssetJobProducesItsDatasets(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, containsEdge(result,
		"mrn://pipeline/dagster/orders_job",
		"mrn://dataset/dagster/clean_orders",
		"PRODUCES"))
}

func TestE2E_DatasetProducesTheWarehouseTableItNames(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, containsEdge(result,
		"mrn://dataset/dagster/clean_orders",
		"mrn://table/duckdb/clean_orders",
		"PRODUCES"))
}

func TestE2E_DatasetCarriesTheColumnsFromItsTableSchema(t *testing.T) {
	result := discoverE2E(t)

	clean := findAsset(t, result, "clean_orders")
	columns, ok := clean.Schema["columns"]
	require.True(t, ok, "expected columns on clean_orders")

	assert.Contains(t, columns, `"column_name":"id"`)
	assert.Contains(t, columns, `"data_type":"integer"`)
	assert.Contains(t, columns, `"column_name":"amount"`)
	assert.Contains(t, columns, `"data_type":"double"`)
}

func TestE2E_DatasetCarriesItsGroupAndComputeKind(t *testing.T) {
	result := discoverE2E(t)

	clean := findAsset(t, result, "clean_orders")
	assert.Equal(t, "duckdb", clean.Metadata["compute_kind"])
	assert.Equal(t, "staging", clean.Metadata["group"])
	assert.Equal(t, "analytics.staging.clean_orders", clean.Metadata["dagster_table_name"])
}

func TestE2E_DatasetRecordsItsLastMaterialization(t *testing.T) {
	result := discoverE2E(t)

	clean := findAsset(t, result, "clean_orders")
	assert.NotEmpty(t, clean.Metadata["last_materialized_at"])
	assert.NotEmpty(t, clean.Metadata["last_materialization_run"])
}

func TestE2E_CollectsRunHistoryForTheJobsThatRan(t *testing.T) {
	result := discoverE2E(t)
	require.NotEmpty(t, result.RunHistory)

	var etlHistory *pluginsdk.AssetRunHistory
	for i, history := range result.RunHistory {
		if history.AssetMRN == "mrn://pipeline/dagster/etl_job" {
			etlHistory = &result.RunHistory[i]
		}
	}
	require.NotNil(t, etlHistory, "expected run history for etl_job")
	require.NotEmpty(t, etlHistory.Runs)

	assert.Equal(t, "START", etlHistory.Runs[0].EventType)
	assert.Equal(t, "dagster", etlHistory.Runs[0].JobNamespace)
	assert.Equal(t, "etl_job", etlHistory.Runs[0].JobName)

	var completed bool
	for _, event := range etlHistory.Runs {
		if event.EventType == "COMPLETE" {
			completed = true
			// Facets cross the wire as JSON, so numbers arrive as float64.
			assert.Equal(t, float64(3), event.RunFacets["step_count"])
		}
	}
	assert.True(t, completed, "expected a COMPLETE event for the finished run")
}

func TestE2E_PipelineSummarisesItsRuns(t *testing.T) {
	result := discoverE2E(t)

	etl := findAsset(t, result, "etl_job")
	assert.Equal(t, "SUCCESS", etl.Metadata["last_run_status"])
	assert.Equal(t, float64(100), etl.Metadata["success_rate"])
}

func TestE2E_IncludeOpsFalseSkipsTasks(t *testing.T) {
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"host":        dagsterURL(t),
		"include_ops": false,
	})
	require.NoError(t, err)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Task", asset.Type)
	}
}

func TestE2E_CodeLocationsFilterSelectsNothingWhenUnmatched(t *testing.T) {
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"host":           dagsterURL(t),
		"code_locations": []any{"does-not-exist"},
	})
	require.NoError(t, err)

	assert.Empty(t, result.Assets)
}

func TestE2E_EveryAssetMRNMatchesItsOwnIdentity(t *testing.T) {
	result := discoverE2E(t)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)
		assert.Equal(t, "Dagster", asset.Providers[0])
	}
}
