package vertexai

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The provider loses its space: mrn.New lowercases every part and replaces
// slashes and spaces in all three.

func TestModelMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://model/vertex-ai/churn-predictor", assetMRN("Model", "churn-predictor"))
}

func TestModelMRN_CarriesTheIDWhenDisplayNamesCollide(t *testing.T) {
	// The space between the display name and the id becomes a hyphen.
	assert.Equal(t,
		"mrn://model/vertex-ai/fraud-detector-(2000000000000000002)",
		assetMRN("Model", "fraud-detector (2000000000000000002)"))
}

func TestEndpointMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://endpoint/vertex-ai/churn-prod", assetMRN("Endpoint", "churn-prod"))
}

func TestDatasetMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://dataset/vertex-ai/churn-training-data", assetMRN("Dataset", "churn-training-data"))
}

func TestFeatureGroupMRN_IsADatasetNamedAfterTheGroupID(t *testing.T) {
	// A feature group and a managed dataset are both Datasets, so a group
	// id equal to a dataset display name would resolve to one asset.
	assert.Equal(t, "mrn://dataset/vertex-ai/customer_features", assetMRN("Dataset", "customer_features"))
}

func TestPipelineJobMRN_IsAJobNamedAfterTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://job/vertex-ai/churn-training", assetMRN("Job", "churn-training"))
}

func TestBigQueryTableMRN_MatchesWhatTheBigQueryPluginProduces(t *testing.T) {
	// Lineage into BigQuery has to use that plugin's own identity, which
	// is the bare table name, or the server drops the edge.
	assert.Equal(t, "mrn://table/bigquery/customer_events",
		mrn.New("Table", "BigQuery", bigQueryTable("bq://acme-ml.analytics.customer_events")))
}

func TestGCSBucketMRN_MatchesWhatTheGCSPluginProduces(t *testing.T) {
	assert.Equal(t, "mrn://bucket/gcs/marmot-vertexai-artifacts",
		mrn.New("Bucket", "GCS", gcsBucket("gs://marmot-vertexai-artifacts/models/churn/")))
}

func TestModelMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := assetMRN("Model", "churn-predictor")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestQualifiedModelMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Model", "fraud-detector (2000000000000000002)")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestJobMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Job", "churn-training")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAssetMRN_MatchesTheServersDerivation(t *testing.T) {
	// The server rebuilds identity from an asset's type, first provider
	// and name, so any MRN that disagrees would create a second, orphaned
	// asset.
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.NotEmpty(t, asset.Providers)

		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}

func TestEveryLineageEndpoint_IsAnMRNTheServerCanParse(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.Parse(endpoint)
			require.NoError(t, err, endpoint)
			assert.Equal(t, endpoint, mrn.New(parsed.Type, parsed.Service, parsed.Name))
		}
	}
}

func TestEveryEdgeThisPluginOwnsBothEndsOf_PointsAtADiscoveredAsset(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	known := map[string]bool{}
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.Parse(endpoint)
			require.NoError(t, err)
			if parsed.Service != "vertex-ai" {
				// The other end belongs to the BigQuery or GCS plugin.
				continue
			}
			assert.True(t, known[endpoint], endpoint)
		}
	}
}

func TestEveryStatisticMRN_BelongsToADiscoveredAsset(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Statistics)

	known := map[string]bool{}
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, statistic := range result.Statistics {
		assert.True(t, known[statistic.AssetMRN], statistic.AssetMRN)
	}
}

func TestEveryRunHistoryMRN_BelongsToADiscoveredAsset(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.RunHistory)

	known := map[string]bool{}
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, history := range result.RunHistory {
		assert.True(t, known[history.AssetMRN], history.AssetMRN)
	}
}
