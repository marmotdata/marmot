package vertexai

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assetMRN(assetType, name string) string {
	return mrnWithZone("", assetType, name)
}

func mrnWithZone(zone, assetType, name string) string {
	cfg := &Config{}
	cfg.Zone = zone
	return cfg.AssetMRN(assetType, provider, name)
}

func TestMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/model/vertex-ai@prod/churn-predictor", mrnWithZone("prod", "Model", "churn-predictor"))
}

// The provider loses its space: mrn.New lowercases every part and replaces
// slashes and spaces in all three.

func TestModelMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://asset/model/vertex-ai/churn-predictor", assetMRN("Model", "churn-predictor"))
}

func TestModelMRN_CarriesTheIDWhenDisplayNamesCollide(t *testing.T) {
	// The space between the display name and the id becomes a hyphen.
	assert.Equal(t,
		"mrn://asset/model/vertex-ai/fraud-detector-(2000000000000000002)",
		assetMRN("Model", "fraud-detector (2000000000000000002)"))
}

func TestEndpointMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://asset/endpoint/vertex-ai/churn-prod", assetMRN("Endpoint", "churn-prod"))
}

func TestDatasetMRN_IsTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dataset/vertex-ai/churn-training-data", assetMRN("Dataset", "churn-training-data"))
}

func TestFeatureGroupMRN_IsADatasetNamedAfterTheGroupID(t *testing.T) {
	// A feature group and a managed dataset are both Datasets, so a group
	// id equal to a dataset display name would resolve to one asset.
	assert.Equal(t, "mrn://asset/dataset/vertex-ai/customer_features", assetMRN("Dataset", "customer_features"))
}

func TestPipelineJobMRN_IsAJobNamedAfterTheDisplayName(t *testing.T) {
	assert.Equal(t, "mrn://asset/job/vertex-ai/churn-training", assetMRN("Job", "churn-training"))
}

func TestBigQueryTableMRN_MatchesWhatTheBigQueryPluginProduces(t *testing.T) {
	// Lineage into BigQuery has to use that plugin's own identity, project.dataset.table, or the server drops the edge.
	assert.Equal(t, "mrn://asset/table/bigquery/acme-ml.analytics.customer_events",
		mrn.Asset("Table", "BigQuery", bigQueryTable("bq://acme-ml.analytics.customer_events")...).String())
}

func TestGCSBucketMRN_MatchesWhatTheGCSPluginProduces(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/gcs/marmot-vertexai-artifacts",
		mrn.Asset("Bucket", "GCS", gcsBucket("gs://marmot-vertexai-artifacts/models/churn/")).String())
}

func TestEveryAssetMRN_IsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(asset.Type), parsed.Type)
		assert.Equal(t, "vertex-ai", parsed.Provider)
		assert.Empty(t, asset.PreviousMRNs)
	}
}

func TestEveryLineageEndpoint_IsAKindedMRN(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.ParseMRN(endpoint)
			require.NoError(t, err, endpoint)
			assert.False(t, parsed.Legacy, endpoint)
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
			parsed, err := mrn.ParseMRN(endpoint)
			require.NoError(t, err)
			if parsed.Provider != "vertex-ai" {
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
