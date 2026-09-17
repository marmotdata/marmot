package mlflow

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every MLflow asset is addressed by its own MLflow name: a registered
// model's name is unique per registry, an experiment's and a dataset's per
// tracking server, so nothing needs qualifying.

func TestModelMRN_IsTheRegisteredModelName(t *testing.T) {
	assert.Equal(t, "mrn://model/mlflow/churn-predictor", assetMRN("Model", "churn-predictor"))
}

func TestExperimentMRN_IsTheExperimentName(t *testing.T) {
	assert.Equal(t, "mrn://experiment/mlflow/churn", assetMRN("Experiment", "churn"))
}

func TestDatasetMRN_IsTheDatasetName(t *testing.T) {
	assert.Equal(t, "mrn://dataset/mlflow/customers", assetMRN("Dataset", "customers"))
}

func TestModelMRN_SanitisesSpacesTheWayTheServerDoes(t *testing.T) {
	// The server rebuilds the MRN from the asset's Name with the same
	// mrn.New, so a name with spaces has to come out identical here.
	assert.Equal(t, "mrn://model/mlflow/it's-a-model", assetMRN("Model", "it's a model"))
}

func TestBucketEdge_UsesTheS3PluginsIdentity(t *testing.T) {
	// The S3 plugin catalogues a bucket as (Bucket, S3, <bucket name>).
	// The edge has to land on that asset, not on a bucket of our own.
	assert.Equal(t, "mrn://bucket/s3/ml-data", bucketMRN("s3://ml-data/customers.parquet"))
}

func TestBucketEdge_UsesTheGCSPluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://bucket/gcs/fraud-data", bucketMRN("gs://fraud-data/transactions.parquet"))
}

func TestBucketEdge_IsEmptyForNonObjectStorage(t *testing.T) {
	assert.Equal(t, "", bucketMRN("/data/customers.csv"))
	assert.Equal(t, "", bucketMRN("https://example.com/customers.csv"))
	assert.Equal(t, "", bucketMRN("hf://datasets/imdb"))
}

func TestModelMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Model", "churn-predictor")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server derives identity from (Type, Providers[0], Name); the MRN
	// the plugin sets has to be exactly that or edges miss their targets.
	result := discover(t, seeded(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
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
			assert.Containsf(t, e.Source, "mrn://bucket/", "only bucket edges may start outside this run: %s", e.Source)
		}
	}
}
