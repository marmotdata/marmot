package gluepipeline

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This plugin shares the Glue provider with the Glue plugin, so a workflow
// step that runs a job has to land on the job asset that plugin already
// created rather than on a copy of it.

func TestPipelineMRN_IsTheWorkflowName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/glue/daily-etl", assetMRN("Pipeline", "daily-etl"))
}

func TestTaskMRN_IsTheWorkflowAndTheStep(t *testing.T) {
	// mrn.New turns the slash into a hyphen, so the task of a workflow reads
	// as one name.
	assert.Equal(t, "mrn://task/glue/daily-etl-load-orders", assetMRN("Task", taskName("daily-etl", "load-orders")))
}

func TestJobMRN_MatchesTheOneTheGluePluginCreates(t *testing.T) {
	// plugins/glue/glue/source.go createJobAsset: mrn.New("Job", "Glue", name)
	// over the bare job name. Run history is attached to this MRN, so if the
	// two ever disagree the runs land on nothing.
	assert.Equal(t, "mrn://job/glue/load-orders", assetMRN("Job", "load-orders"))
	assert.Equal(t, mrn.New("Job", "Glue", "load-orders"), assetMRN("Job", "load-orders"))
}

func TestCrawlerMRN_MatchesTheOneTheGluePluginCreates(t *testing.T) {
	// plugins/glue/glue/source.go createCrawlerAsset: type Crawler, bare name.
	assert.Equal(t, "mrn://crawler/glue/orders-crawler", assetMRN("Crawler", "orders-crawler"))
}

func TestDatabaseMRN_MatchesTheOneTheGluePluginCreates(t *testing.T) {
	// plugins/glue/glue/source.go createDatabaseAsset: type Database, bare name.
	assert.Equal(t, "mrn://database/glue/shop", assetMRN("Database", "shop"))
}

func TestBucketMRN_MatchesTheOneTheS3PluginCreates(t *testing.T) {
	assert.Equal(t, "mrn://bucket/s3/marmot-lake", mrn.New("Bucket", "S3", "marmot-lake"))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Pipeline", "daily-etl")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTaskMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Task", taskName("daily-etl", "load-orders"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAssetMRN_AgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so an
	// asset whose MRN says anything else is stored under a name nobody can
	// look up.
	result := discover(t, newSource(seededGlue()))
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		require.NotEmpty(t, asset.Providers)
		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}

func TestEveryLineageEndpoint_IsAnMRNThisRunCanName(t *testing.T) {
	// Edges that point at an asset another plugin owns are fine, but they
	// still have to be built with that plugin's identity, so every endpoint
	// must parse and round trip.
	result := discover(t, newSource(seededGlue()))
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.Parse(endpoint)
			require.NoError(t, err, endpoint)
			assert.Equal(t, endpoint, mrn.New(parsed.Type, parsed.Service, parsed.Name))
		}
	}
}

func TestRunHistoryMRN_PointsAtAnAssetIdentityNotACopy(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	var targets []string
	for _, history := range result.RunHistory {
		targets = append(targets, history.AssetMRN)
	}
	assert.Contains(t, targets, "mrn://job/glue/load-orders")
	assert.Contains(t, targets, "mrn://crawler/glue/orders-crawler")
	assert.Contains(t, targets, "mrn://pipeline/glue/daily-etl")
}
