package dagster

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMRNSource(zone string) *Source {
	s := &Source{config: &Config{}}
	s.config.Zone = zone
	return s
}

func TestPipelineMRN_IsTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/dagster/etl_job", newMRNSource("").pipelineMRN("etl_job"))
}

func TestTaskMRN_IsTheJobAndTheOpHandle(t *testing.T) {
	assert.Equal(t, "mrn://asset/task/dagster/etl_job.extract", newMRNSource("").taskMRN("etl_job", "extract"))
}

func TestTaskMRN_ANestedOpHandleStaysOneLevel(t *testing.T) {
	// A handle for an op inside a graph is dotted, so it is quoted to keep two ops of the same name in different graphs distinct.
	assert.Equal(t, `mrn://asset/task/dagster/etl_job."inner.extract"`, newMRNSource("").taskMRN("etl_job", "inner.extract"))
}

func TestDatasetMRN_IsTheAssetKeyPath(t *testing.T) {
	assert.Equal(t, "mrn://asset/dataset/dagster/raw.orders", newMRNSource("").datasetMRN(AssetKey{Path: []string{"raw", "orders"}}))
	assert.Equal(t, "mrn://asset/dataset/dagster/clean_orders", newMRNSource("").datasetMRN(AssetKey{Path: []string{"clean_orders"}}))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/dagster@prod/etl_job", newMRNSource("prod").pipelineMRN("etl_job"))
}

func TestTaskMRN_SameOpNameInDifferentJobsDiffers(t *testing.T) {
	s := newMRNSource("")
	assert.NotEqual(t, s.taskMRN("a", "extract"), s.taskMRN("b", "extract"))
}

func TestEveryLineageEndpointIsAnAssetFromThisRunOrAKnownWarehouse(t *testing.T) {
	// The server drops an edge whose endpoint does not exist, so an edge may
	// only point at an asset created here or at one another plugin owns.
	result := discover(t, newFakeDagster(), nil)

	created := make(map[string]struct{}, len(result.Assets))
	for _, asset := range result.Assets {
		created[*asset.MRN] = struct{}{}
	}
	// The only edge that leaves this plugin is the warehouse table, named
	// exactly as the DuckDB plugin names its tables.
	created["mrn://asset/table/duckdb/analytics.staging.clean_orders"] = struct{}{}

	for _, edge := range result.Lineage {
		assert.Contains(t, created, edge.Source, "edge source is unknown")
		assert.Contains(t, created, edge.Target, "edge target is unknown")
	}
}

func TestRunHistoryPointsAtAPipelineFromThisRun(t *testing.T) {
	result := discover(t, newFakeDagster(), nil)

	pipelines := make(map[string]struct{})
	for _, asset := range result.Assets {
		if asset.Type == "Pipeline" {
			pipelines[*asset.MRN] = struct{}{}
		}
	}

	require.NotEmpty(t, result.RunHistory)
	for _, history := range result.RunHistory {
		assert.Contains(t, pipelines, history.AssetMRN)
	}
}

func TestWarehouseMRN_MatchesEachPluginsOwnTableNaming(t *testing.T) {
	// These are the strings the DuckDB, PostgreSQL, BigQuery and Snowflake plugins produce, pinned so a change here is deliberate.
	cases := []struct {
		kind string
		path []string
		want string
	}{
		{"duckdb", []string{"analytics", "staging", "clean_orders"}, "mrn://asset/table/duckdb/analytics.staging.clean_orders"},
		{"postgres", []string{"app", "public", "orders"}, "mrn://asset/table/postgresql/app.public.orders"},
		{"bigquery", []string{"project", "analytics", "orders"}, "mrn://asset/table/bigquery/project.analytics.orders"},
		{"snowflake", []string{"ANALYTICS", "STAGING", "ORDERS"}, "mrn://asset/table/snowflake/analytics.staging.orders"},
	}
	for _, tc := range cases {
		target, ok := warehouseTarget(AssetNode{ComputeKind: stringPtr(tc.kind), AssetKey: AssetKey{Path: tc.path}})
		require.True(t, ok, tc.kind)
		assert.Equal(t, tc.want, mrn.Asset("Table", target.Provider, target.Path...).String())
	}
}

func TestAssetMRN_UsesTheExactProviderString(t *testing.T) {
	result := discover(t, newFakeDagster(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		assert.Equal(t, "Dagster", asset.Providers[0])
	}
}

func TestEveryDiscoveredAssetMRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, newFakeDagster(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(asset.Type), parsed.Type)
		assert.Equal(t, strings.ToLower(asset.Providers[0]), parsed.Provider)
	}
}

func TestDiscoveredTaskKeepsTheOldMRNAsAPreviousMRN(t *testing.T) {
	result := discover(t, newFakeDagster(), nil)
	for _, asset := range result.Assets {
		if asset.MRN != nil && *asset.MRN == "mrn://asset/task/dagster/etl_job.extract" {
			assert.Contains(t, asset.PreviousMRNs, "mrn://asset/task/dagster/etl_job-extract")
			return
		}
	}
	t.Fatal("task not discovered")
}
