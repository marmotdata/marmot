package gluepipeline

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSource(zone string) *Source {
	cfg := &Config{}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestPipelineMRN_IsTheWorkflowName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/glue/daily-etl", testSource("").assetMRN("Pipeline", "daily-etl"))
}

func TestTaskMRN_IsWorkflowThenStep(t *testing.T) {
	assert.Equal(t, "mrn://asset/task/glue/daily-etl.load-orders", testSource("").assetMRN("Task", "daily-etl", "load-orders"))
}

func TestTaskMRN_SameStepNameInTwoWorkflowsDiffers(t *testing.T) {
	s := testSource("")
	assert.NotEqual(t, s.assetMRN("Task", "etl-a", "load"), s.assetMRN("Task", "etl-b", "load"))
}

func TestPreviousTaskMRN_IsTheFoldedLegacyName(t *testing.T) {
	assert.Equal(t, "mrn://task/glue/daily-etl-load-orders", previousTaskMRN("daily-etl", "load-orders"))
}

func TestGlueOwnedMRNsMatchTheGluePlugin(t *testing.T) {
	// These assets belong to plugins/glue, so the edges have to name them the way it does.
	s := testSource("")
	assert.Equal(t, "mrn://asset/job/glue/load-orders", s.assetMRN("Job", "load-orders"))
	assert.Equal(t, "mrn://asset/crawler/glue/orders-crawler", s.assetMRN("Crawler", "orders-crawler"))
	assert.Equal(t, "mrn://asset/database/glue/shop", s.assetMRN("Database", "shop"))
}

func TestMRNs_CarryTheZone(t *testing.T) {
	s := testSource("prod-eu")
	assert.Equal(t, "mrn://asset/pipeline/glue@prod-eu/daily-etl", s.assetMRN("Pipeline", "daily-etl"))
	assert.Equal(t, "mrn://asset/job/glue@prod-eu/load-orders", s.assetMRN("Job", "load-orders"))
}

func TestBucketMRN_MatchesTheS3PluginIdentity(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/marmot-lake", mrn.Asset("Bucket", "S3", "marmot-lake").String())
}

func TestPipelineMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := testSource("").assetMRN("Task", "daily-etl", "load-orders")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestEveryAsset_NameIsItsOwnAndMRNsAreUnique(t *testing.T) {
	result := discover(t, newSource(seededGlue()))
	require.NotEmpty(t, result.Assets)

	seen := map[string]bool{}
	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		require.NotEmpty(t, asset.Providers)
		assert.False(t, seen[*asset.MRN], *asset.MRN)
		seen[*asset.MRN] = true

		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err)
		path := parsed.Path()
		assert.Equal(t, mrn.Asset(asset.Type, asset.Providers[0], path...).String(), *asset.MRN)
		assert.Equal(t, mrn.Asset(asset.Type, asset.Providers[0], *asset.Name).String(), mrn.Asset(asset.Type, asset.Providers[0], path[len(path)-1]).String(), "the name is the last level of the path")
	}
}

func TestTaskAssets_CarryTheirPreviousMRN(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	var tasks int
	for _, asset := range result.Assets {
		if asset.Type != "Task" {
			continue
		}
		tasks++
		require.Len(t, asset.PreviousMRNs, 1)
		assert.Contains(t, asset.PreviousMRNs[0], "mrn://task/glue/")
	}
	assert.NotZero(t, tasks)
}

func TestEveryLineageEndpoint_IsAnMRNThisRunCanName(t *testing.T) {
	// Edges that point at an asset another plugin owns are fine, but they still have to be built with that plugin's identity, so every endpoint must parse and round trip.
	result := discover(t, newSource(seededGlue()))
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.ParseMRN(endpoint)
			require.NoError(t, err, endpoint)
			assert.Equal(t, endpoint, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
		}
	}
}

func TestRunHistoryMRN_PointsAtAnAssetIdentityNotACopy(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	var targets []string
	for _, history := range result.RunHistory {
		targets = append(targets, history.AssetMRN)
	}
	assert.Contains(t, targets, "mrn://asset/job/glue/load-orders")
	assert.Contains(t, targets, "mrn://asset/crawler/glue/orders-crawler")
	assert.Contains(t, targets, "mrn://asset/pipeline/glue/daily-etl")
}
