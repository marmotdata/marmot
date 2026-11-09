package sagemaker

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every SageMaker resource has an account-unique name, so an asset's name is
// that name unqualified and its MRN is built from the name alone.

func mrnFor(zone, assetType, name string) string {
	cfg := &Config{}
	cfg.Zone = zone
	return cfg.AssetMRN(assetType, provider, name)
}

func TestModelMRN_IsTheModelName(t *testing.T) {
	assert.Equal(t, "mrn://asset/model/sagemaker/churn-xgb", mrnFor("", "Model", "churn-xgb"))
}

func TestEndpointMRN_IsTheEndpointName(t *testing.T) {
	assert.Equal(t, "mrn://asset/endpoint/sagemaker/churn-prod", mrnFor("", "Endpoint", "churn-prod"))
}

func TestFeatureGroupMRN_IsADatasetNamedAfterTheGroup(t *testing.T) {
	assert.Equal(t, "mrn://asset/dataset/sagemaker/customer-features", mrnFor("", "Dataset", "customer-features"))
}

func TestTrainingJobMRN_IsAJobNamedAfterTheTrainingJob(t *testing.T) {
	assert.Equal(t, "mrn://asset/job/sagemaker/churn-train-2026-09-01", mrnFor("", "Job", "churn-train-2026-09-01"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/model/sagemaker@prod-eu/churn-xgb", mrnFor("prod-eu", "Model", "churn-xgb"))
}

func TestS3BucketMRN_MatchesWhatTheS3PluginProduces(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/ml-artifacts", mrn.Asset("Bucket", "S3", "ml-artifacts").String())
}

func TestGlueTableMRN_MatchesWhatTheGluePluginProduces(t *testing.T) {
	// The Glue plugin names a table asset database.table.
	assert.Equal(t, "mrn://asset/table/glue/sagemaker_featurestore.customer_features", glueTableMRN("sagemaker_featurestore.customer_features"))
}

func TestEveryAssetMRN_IsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, strings.ToLower(a.Providers[0]), parsed.Provider)
		assert.Empty(t, a.PreviousMRNs)
	}
}

func TestEveryLineageEndpoint_IsAKindedMRN(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.ParseMRN(endpoint)
			require.NoError(t, err, endpoint)
			assert.False(t, parsed.Legacy, endpoint)
		}
	}
}

func TestEveryRunHistoryMRN_BelongsToADiscoveredAsset(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)
	require.NotEmpty(t, result.RunHistory)

	known := map[string]bool{}
	for _, a := range result.Assets {
		known[*a.MRN] = true
	}

	for _, history := range result.RunHistory {
		assert.True(t, known[history.AssetMRN], history.AssetMRN)
	}
}

func TestDiscoveredAssets_CoverEveryTypeThePluginEmits(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	counts := map[string]int{}
	for _, a := range result.Assets {
		counts[a.Type]++
	}

	assert.Equal(t, 2, counts["Model"], "a deployed model and a registry group")
	assert.Equal(t, 1, counts["Endpoint"])
	assert.Equal(t, 1, counts["Dataset"])
	assert.Equal(t, 1, counts["Job"])
}
