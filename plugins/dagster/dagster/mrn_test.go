package dagster

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Dagster job is addressed by its name, an op by the job it belongs to plus
// its handle, and a software-defined asset by its asset key. mrn.New turns the
// slash in the last two into a hyphen.

func TestPipelineMRN_IsTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/dagster/etl_job", assetMRN("Pipeline", "etl_job"))
}

func TestTaskMRN_CombinesTheJobAndTheOpHandle(t *testing.T) {
	assert.Equal(t, "mrn://task/dagster/etl_job-extract", assetMRN("Task", taskName("etl_job", "extract")))
}

func TestDatasetMRN_IsTheAssetKeyPath(t *testing.T) {
	name := datasetName(AssetKey{Path: []string{"raw", "orders"}})

	assert.Equal(t, "mrn://dataset/dagster/raw-orders", assetMRN("Dataset", name))
}

func TestDatasetMRN_ASingleSegmentKeyStaysBare(t *testing.T) {
	name := datasetName(AssetKey{Path: []string{"clean_orders"}})

	assert.Equal(t, "mrn://dataset/dagster/clean_orders", assetMRN("Dataset", name))
}

func TestTaskMRN_ANestedOpHandleKeepsEverySegment(t *testing.T) {
	// A handle for an op inside a graph is dotted, and the dot survives, so
	// two ops of the same name in different graphs stay distinct.
	assert.Equal(t, "mrn://task/dagster/etl_job-inner.extract", assetMRN("Task", taskName("etl_job", "inner.extract")))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Pipeline", "etl_job")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTaskMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Task", taskName("etl_job", "extract"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatasetMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Dataset", datasetName(AssetKey{Path: []string{"raw", "orders"}}))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestPipelineMRN_AQualifiedNameStillRoundTrips(t *testing.T) {
	original := assetMRN("Pipeline", "etl_job (beta)")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryDiscoveredAssetMRNMatchesItsOwnIdentity(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name) and ignores
	// the MRN on the wire, so the two must agree for every asset kind.
	result := discover(t, newFakeDagster(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name, "asset has no name")
		require.NotNil(t, asset.MRN, "asset %s has no MRN", *asset.Name)
		require.Len(t, asset.Providers, 1)

		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
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
	created["mrn://table/duckdb/clean_orders"] = struct{}{}

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
	// These are the exact strings the DuckDB, PostgreSQL, BigQuery and
	// Snowflake plugins produce, pinned so a change here is deliberate.
	duckdb, ok := warehouseTarget(AssetNode{
		ComputeKind: stringPtr("duckdb"),
		AssetKey:    AssetKey{Path: []string{"analytics", "staging", "clean_orders"}},
	})
	require.True(t, ok)
	assert.Equal(t, "mrn://table/duckdb/clean_orders", mrn.New("Table", duckdb.Provider, duckdb.Name))

	postgres, ok := warehouseTarget(AssetNode{
		ComputeKind: stringPtr("postgres"),
		AssetKey:    AssetKey{Path: []string{"app", "public", "orders"}},
	})
	require.True(t, ok)
	assert.Equal(t, "mrn://table/postgresql/orders", mrn.New("Table", postgres.Provider, postgres.Name))

	bigquery, ok := warehouseTarget(AssetNode{
		ComputeKind: stringPtr("bigquery"),
		AssetKey:    AssetKey{Path: []string{"project", "analytics", "orders"}},
	})
	require.True(t, ok)
	assert.Equal(t, "mrn://table/bigquery/orders", mrn.New("Table", bigquery.Provider, bigquery.Name))

	snowflake, ok := warehouseTarget(AssetNode{
		ComputeKind: stringPtr("snowflake"),
		AssetKey:    AssetKey{Path: []string{"ANALYTICS", "STAGING", "ORDERS"}},
	})
	require.True(t, ok)
	assert.Equal(t, "mrn://table/snowflake/analytics.staging.orders", mrn.New("Table", snowflake.Provider, snowflake.Name))
}

func TestAssetMRN_UsesTheExactProviderString(t *testing.T) {
	result := discover(t, newFakeDagster(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		assert.Equal(t, "Dagster", asset.Providers[0])
	}
}
