package mlflow

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every MLflow asset is addressed by its own MLflow name: a registered
// model's name is unique per registry, an experiment's and a dataset's per
// tracking server, so nothing needs qualifying.

func assetMRN(assetType, name string) string {
	return mrn.Asset(assetType, provider, name).String()
}

func TestModelMRN_IsTheRegisteredModelName(t *testing.T) {
	assert.Equal(t, "mrn://asset/model/mlflow/churn-predictor", assetMRN("Model", "churn-predictor"))
}

func TestExperimentMRN_IsTheExperimentName(t *testing.T) {
	assert.Equal(t, "mrn://asset/experiment/mlflow/churn", assetMRN("Experiment", "churn"))
}

func TestDatasetMRN_IsTheDatasetName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dataset/mlflow/customers", assetMRN("Dataset", "customers"))
}

func TestModelMRN_SanitisesSpacesTheWayTheServerDoes(t *testing.T) {
	// The server rebuilds the MRN from the asset's Name with the same
	// mrn.New, so a name with spaces has to come out identical here.
	assert.Equal(t, "mrn://asset/model/mlflow/it's-a-model", assetMRN("Model", "it's a model"))
}

func TestBucketEdge_UsesTheS3PluginsIdentity(t *testing.T) {
	// The S3 plugin catalogues a bucket as (Bucket, S3, <bucket name>).
	// The edge has to land on that asset, not on a bucket of our own.
	assert.Equal(t, "mrn://asset/bucket/s3/ml-data", bucketMRN("s3://ml-data/customers.parquet"))
}

func TestBucketEdge_UsesTheGCSPluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/gcs/fraud-data", bucketMRN("gs://fraud-data/transactions.parquet"))
}

func TestBucketEdge_IsEmptyForNonObjectStorage(t *testing.T) {
	assert.Equal(t, "", bucketMRN("/data/customers.csv"))
	assert.Equal(t, "", bucketMRN("https://example.com/customers.csv"))
	assert.Equal(t, "", bucketMRN("hf://datasets/imdb"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod"

	assert.Equal(t, "mrn://asset/model/mlflow@prod/churn-predictor", cfg.AssetMRN("Model", provider, "churn-predictor"))
}

func TestEveryAsset_MRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, seeded(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, strings.ToLower(a.Providers[0]), parsed.Provider)
		assert.Empty(t, a.PreviousMRNs, "names are unchanged, so the server's own guess covers the old MRN")
	}
}

func TestEveryEdge_EndsOnAnAssetOfThisRunOrAnotherPluginsBucket(t *testing.T) {
	result := discover(t, seeded(), nil)

	created := make(map[string]struct{})
	for _, a := range result.Assets {
		created[*a.MRN] = struct{}{}
	}

	for _, e := range result.Lineage {
		_, ok := created[e.Target]
		assert.Truef(t, ok, "edge target %s was not created in this run", e.Target)

		if _, ok := created[e.Source]; !ok {
			assert.Containsf(t, e.Source, "mrn://asset/bucket/", "only bucket edges may start outside this run: %s", e.Source)
		}
	}
}
