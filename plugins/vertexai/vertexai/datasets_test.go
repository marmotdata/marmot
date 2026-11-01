package vertexai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/aiplatform/v1"
)

func TestDataset_IsNamedAfterItsDisplayName(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	assert.Equal(t, "mrn://dataset/vertex-ai/churn-training-data", *dataset.MRN)
}

func TestDataset_CarriesItsDescription(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	require.NotNil(t, dataset.Description)
	assert.Equal(t, "Labelled churn events", *dataset.Description)
}

func TestDataset_RecordsItsKindFromTheMetadataSchema(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "receipt-scans")

	assert.Equal(t, "image_1.0.0", dataset.Metadata["dataset_kind"])
}

func TestDataset_LeavesOutTheKindWhenTheSchemaURIIsNotOne(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "experimental-corpus")

	assert.NotContains(t, dataset.Metadata, "dataset_kind")
}

func TestDataset_RecordsItsCounts(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	assert.Equal(t, int64(125000), dataset.Metadata["data_item_count"])
	assert.Equal(t, 1, dataset.Metadata["saved_query_count"])
}

func TestDataset_EmitsItsItemCountAsAStatistic(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	value, ok := statisticFor(result, *dataset.MRN, "asset.data_item_count")
	require.True(t, ok)
	assert.Equal(t, float64(125000), value)
}

func TestDataset_EmitsAZeroItemCountToo(t *testing.T) {
	// An empty dataset is worth knowing about, and a missing statistic
	// would read as an unmeasured one.
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "receipt-scans")

	value, ok := statisticFor(result, *dataset.MRN, "asset.data_item_count")
	require.True(t, ok)
	assert.Equal(t, float64(0), value)
}

func TestDataset_LinksTheBigQueryTableItReads(t *testing.T) {
	result := discoverWith(t, fullFake())

	assert.True(t, hasEdge(result,
		"mrn://table/bigquery/"+testTable,
		"mrn://dataset/vertex-ai/churn-training-data",
		"FEEDS"))
}

func TestDataset_RecordsTheURIItReads(t *testing.T) {
	result := discoverWith(t, fullFake())

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	assert.Equal(t, "bq://"+testProject+".analytics."+testTable, dataset.Metadata["source_uris"])
}

func TestDataset_LinksTheBucketHoldingItsBlobs(t *testing.T) {
	// The image, text and video schemas name the bucket with no scheme.
	result := discoverWith(t, fullFake())

	assert.True(t, hasEdge(result,
		"mrn://bucket/gcs/"+testBucket,
		"mrn://dataset/vertex-ai/receipt-scans",
		"FEEDS"))
}

func TestDataset_EmitsNoEdgeForAMetadataShapeItDoesNotRecognise(t *testing.T) {
	// A wrong edge is worse than a missing one, so an unfamiliar shape
	// produces nothing at all.
	result := discoverWith(t, fullFake())

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "mrn://dataset/vertex-ai/experimental-corpus", edge.Target)
	}
}

func TestDataset_SurvivesAMetadataShapeItDoesNotRecognise(t *testing.T) {
	result := discoverWith(t, fullFake())

	assetNamed(t, result, "Dataset", "experimental-corpus")
}

func TestDatasets_AreSkippedWhenTheConfigTurnsThemOff(t *testing.T) {
	server := fullFake().start(t)

	source := &Source{}
	result, err := source.Discover(t.Context(), map[string]any{
		"project_id":       testProject,
		"locations":        []string{testLocation},
		"endpoint":         server.URL,
		"disable_auth":     true,
		"include_datasets": false,
	})
	require.NoError(t, err)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "churn-training-data", *asset.Name)
	}
}

func TestDatasetKind_DropsTheSchemaSuffix(t *testing.T) {
	assert.Equal(t, "tabular_1.0.0",
		datasetKind("gs://google-cloud-aiplatform/schema/dataset/metadata/tabular_1.0.0.yaml"))
}

func TestDatasetKind_IsEmptyForAURIThatIsNotASchema(t *testing.T) {
	assert.Empty(t, datasetKind("gs://acme-schemas/private/corpus.json"))
}

func TestDatasetKind_IsEmptyForNoURI(t *testing.T) {
	assert.Empty(t, datasetKind(""))
}

func TestDatasetSources_ReadsATabularBigQuerySource(t *testing.T) {
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{
			"type": "bigquery_source",
			"uri":  "bq://acme.analytics.events",
		},
	})

	assert.Equal(t, []string{"bq://acme.analytics.events"}, input.bigQueryURIs)
	assert.Empty(t, input.gcsURIs)
}

func TestDatasetSources_ReadsATabularCloudStorageSourceList(t *testing.T) {
	// The tabular schema spells a Cloud Storage source as a list of URIs
	// and a BigQuery source as one string, under the same key.
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{
			"type": "gcs_source",
			"uri":  []any{"gs://acme-data/train.csv", "gs://acme-data/test.csv"},
		},
	})

	assert.Equal(t, []string{"gs://acme-data/train.csv", "gs://acme-data/test.csv"}, input.gcsURIs)
}

func TestDatasetSources_ReadsTheTimeSeriesSpelling(t *testing.T) {
	// The time series schema names the two sources under separate keys.
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{
			"type":          "bigquery_source",
			"bigquery_uri":  "bq://acme.analytics.readings",
			"gcs_uri":       "gs://acme-data/readings.csv",
			"timeColumn":    "reading_time",
			"unknownFuture": 42,
		},
	})

	assert.Equal(t, []string{"bq://acme.analytics.readings"}, input.bigQueryURIs)
	assert.Equal(t, []string{"gs://acme-data/readings.csv"}, input.gcsURIs)
}

func TestDatasetSources_ReadsTheBucketOfABlobDataset(t *testing.T) {
	input := datasetSources(map[string]any{"gcsBucket": "acme-images"})

	assert.Equal(t, []string{"acme-images"}, input.buckets)
}

func TestDatasetSources_ClassifiesBySchemeNotByTheDeclaredType(t *testing.T) {
	// Trusting the type field would emit an edge into the wrong
	// technology when the two disagree.
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{
			"type": "gcs_source",
			"uri":  "bq://acme.analytics.events",
		},
	})

	assert.Equal(t, []string{"bq://acme.analytics.events"}, input.bigQueryURIs)
	assert.Empty(t, input.gcsURIs)
}

func TestDatasetSources_IgnoresAURIWithAnUnknownScheme(t *testing.T) {
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{"uri": "s3://acme-data/train.csv"},
	})

	assert.True(t, input.empty())
}

func TestDatasetSources_IgnoresMetadataThatIsNotAnObject(t *testing.T) {
	assert.True(t, datasetSources("just a string").empty())
}

func TestDatasetSources_IgnoresNoMetadataAtAll(t *testing.T) {
	assert.True(t, datasetSources(nil).empty())
}

func TestDatasetSources_IgnoresAnInputConfigThatIsNotAnObject(t *testing.T) {
	assert.True(t, datasetSources(map[string]any{"inputConfig": "gs://acme-data"}).empty())
}

func TestDatasetSources_IgnoresAURIListHoldingSomethingElse(t *testing.T) {
	input := datasetSources(map[string]any{
		"inputConfig": map[string]any{"uri": []any{42, nil, "gs://acme-data/train.csv"}},
	})

	assert.Equal(t, []string{"gs://acme-data/train.csv"}, input.gcsURIs)
}

func TestDatasetSources_IgnoresABucketThatIsNotAString(t *testing.T) {
	assert.True(t, datasetSources(map[string]any{"gcsBucket": 42}).empty())
}

func TestDataset_LinksTwoObjectsInOneBucketOnlyOnce(t *testing.T) {
	fake := fullFake()
	fake.datasets[testLocation] = []*aiplatform.GoogleCloudAiplatformV1Dataset{tabularDataset()}
	fake.datasets[testLocation][0].Metadata = map[string]any{
		"inputConfig": map[string]any{
			"type": "gcs_source",
			"uri":  []any{"gs://acme-data/train.csv", "gs://acme-data/test.csv"},
		},
	}

	result := discoverWith(t, fake)

	count := 0
	for _, edge := range result.Lineage {
		if edge.Target == "mrn://dataset/vertex-ai/churn-training-data" {
			count++
		}
	}

	assert.Equal(t, 1, count)
}
