package sagemaker

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every SageMaker resource has an account-unique name, so an asset's name is
// that name unqualified and its MRN is built from the name alone.

func TestModelMRN_IsTheModelName(t *testing.T) {
	assert.Equal(t, "mrn://model/sagemaker/churn-xgb", assetMRN("Model", "churn-xgb"))
}

func TestEndpointMRN_IsTheEndpointName(t *testing.T) {
	assert.Equal(t, "mrn://endpoint/sagemaker/churn-prod", assetMRN("Endpoint", "churn-prod"))
}

func TestFeatureGroupMRN_IsADatasetNamedAfterTheGroup(t *testing.T) {
	assert.Equal(t, "mrn://dataset/sagemaker/customer-features", assetMRN("Dataset", "customer-features"))
}

func TestTrainingJobMRN_IsAJobNamedAfterTheTrainingJob(t *testing.T) {
	assert.Equal(t, "mrn://job/sagemaker/churn-train-2026-09-01", assetMRN("Job", "churn-train-2026-09-01"))
}

func TestModelPackageGroupMRN_SharesTheModelNamespace(t *testing.T) {
	// A registry group is filed as a Model, so a group named the same as a
	// deployed model resolves to the same asset.
	assert.Equal(t, assetMRN("Model", "churn-registry"), "mrn://model/sagemaker/churn-registry")
}

func TestS3BucketMRN_MatchesWhatTheS3PluginProduces(t *testing.T) {
	// Lineage into S3 has to use the S3 plugin's own identity, or the
	// server drops the edge.
	assert.Equal(t, "mrn://bucket/s3/ml-artifacts", mrn.New("Bucket", "S3", "ml-artifacts"))
}

func TestGlueTableMRN_MatchesWhatTheGluePluginProduces(t *testing.T) {
	// The Glue plugin names a table asset with the bare table name.
	assert.Equal(t, "mrn://table/glue/customer_features", mrn.New("Table", "Glue", bareTableName("sagemaker_featurestore.customer_features")))
}

func TestModelMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := assetMRN("Model", "churn-xgb")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestJobMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Job", "churn-train-2026-09-01")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAssetMRN_MatchesTheServersDerivation(t *testing.T) {
	// The server rebuilds identity from an asset's type, first provider and
	// name, so any MRN that disagrees would create a second, orphaned asset.
	result := discoverWith(t, fullFake(), withTrainingJobs)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.NotEmpty(t, a.Providers)

		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestEveryLineageEndpoint_IsAnMRNTheServerCanParse(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.Parse(endpoint)
			require.NoError(t, err, endpoint)
			assert.Equal(t, endpoint, mrn.New(parsed.Type, parsed.Service, parsed.Name))
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
