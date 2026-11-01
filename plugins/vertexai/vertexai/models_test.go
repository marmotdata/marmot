package vertexai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModel_IsNamedAfterItsDisplayName(t *testing.T) {
	// A model's unique id is a meaningless number, so the display name is
	// what makes the catalog readable.
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "mrn://model/vertex-ai/churn-predictor", *model.MRN)
}

func TestModel_CarriesItsDescription(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	require.NotNil(t, model.Description)
	assert.Equal(t, "Predicts subscription churn", *model.Description)
}

func TestModel_RecordsWhereItLives(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, churnModelID, model.Metadata["resource_id"])
	assert.Equal(t, testLocation, model.Metadata["location"])
	assert.Equal(t, testProject, model.Metadata["project_id"])
}

func TestModel_RecordsItsTimestamps(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "2026-08-01T10:00:00Z", model.Metadata["create_time"])
	assert.Equal(t, "2026-08-02T11:30:00Z", model.Metadata["update_time"])
}

func TestModel_RecordsEachLabelUnderItsOwnKey(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "growth", model.Metadata["label_team"])
	assert.Equal(t, "gold", model.Metadata["label_tier"])
}

func TestModel_RecordsItsServingContainer(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "us-docker.pkg.dev/vertex-ai/prediction/sklearn-cpu.1-5:latest", model.Metadata["container_image"])
	assert.Equal(t, "gs://"+testBucket+"/models/churn/", model.Metadata["artifact_uri"])
}

func TestModel_RecordsItsPredictionSchemata(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "gs://"+testBucket+"/schema/instance.yaml", model.Metadata["predict_schemata_instance"])
	assert.Equal(t, "gs://"+testBucket+"/schema/parameters.yaml", model.Metadata["predict_schemata_parameters"])
	assert.Equal(t, "gs://"+testBucket+"/schema/prediction.yaml", model.Metadata["predict_schemata_prediction"])
}

func TestModel_RecordsItsVersion(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, "1", model.Metadata["version_id"])
	assert.Equal(t, []string{"default"}, model.Metadata["version_aliases"])
	assert.Equal(t, "First release", model.Metadata["version_description"])
	assert.Equal(t, "2026-08-01T10:00:00Z", model.Metadata["version_create_time"])
}

func TestModel_RecordsTheFormatsItSupports(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, []string{"DEDICATED_RESOURCES"}, model.Metadata["supported_deployment_resources_types"])
	assert.Equal(t, []string{"jsonl", "bigquery"}, model.Metadata["supported_input_storage_formats"])
	assert.Equal(t, []string{"jsonl"}, model.Metadata["supported_output_storage_formats"])
}

func TestModel_ReducesTheProducingJobToItsBareID(t *testing.T) {
	// The API gives a full resource name; the bare id is what the job
	// asset is named after and what a person reads.
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, trainJobID, model.Metadata["pipeline_job"])
}

func TestModel_CountsTheEndpointsItIsDeployedTo(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, 1, model.Metadata["deployed_model_count"])
}

func TestModel_RecordsTheModelGardenModelItCameFrom(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDOne+")")

	assert.Equal(t, "publishers/google/models/gemma", model.Metadata["base_model_source"])
}

func TestModel_LeavesOutAFieldTheAPIDidNotSet(t *testing.T) {
	// An empty key in the catalog reads as "the model has no version",
	// which is a different claim from "the API did not say".
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDTwo+")")

	assert.NotContains(t, model.Metadata, "version_id")
	assert.NotContains(t, model.Metadata, "container_image")
	assert.NotContains(t, model.Metadata, "base_model_source")
}

func TestModels_QualifyBothSidesOfADisplayNameCollision(t *testing.T) {
	// Vertex AI lets two models share a display name, and a name that
	// depended on API ordering would move between runs.
	result := discoverWith(t, fullFake())

	first := assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDOne+")")
	second := assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDTwo+")")

	assert.Equal(t, fraudModelIDOne, first.Metadata["resource_id"])
	assert.Equal(t, fraudModelIDTwo, second.Metadata["resource_id"])
}

func TestModels_KeepTheUnqualifiedDisplayNameInMetadata(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDOne+")")

	assert.Equal(t, "fraud-detector", model.Metadata["display_name"])
}

func TestModels_AreReadAcrossEveryPage(t *testing.T) {
	// The fake returns two models on the first page and one on the second.
	result := discoverWith(t, fullFake())

	assetNamed(t, result, "Model", "churn-predictor")
	assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDOne+")")
	assetNamed(t, result, "Model", "fraud-detector ("+fraudModelIDTwo+")")
}

func TestModel_LinksTheArtifactBucketToTheModel(t *testing.T) {
	result := discoverWith(t, fullFake())

	assert.True(t, hasEdge(result,
		"mrn://bucket/gcs/"+testBucket,
		"mrn://model/vertex-ai/churn-predictor",
		"FEEDS"))
}

func TestModel_LinksThePipelineJobThatProducedIt(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	assert.True(t, hasEdge(result,
		"mrn://job/vertex-ai/churn-training",
		"mrn://model/vertex-ai/churn-predictor",
		"PRODUCES"))
}

func TestModel_HasNoProducingJobEdgeWhenJobsAreNotDiscovered(t *testing.T) {
	// The job's asset name comes from its display name, so without the
	// job there is nothing to point at.
	result := discoverWith(t, fullFake())

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "PRODUCES", edge.Type)
	}
}

func TestModels_EachGetTheirOwnEdgeFromTheSharedArtifactBucket(t *testing.T) {
	// Several models sit in one bucket, and each of them is a separate
	// fact about that bucket.
	result := discoverWith(t, fullFake())

	count := 0
	for _, edge := range result.Lineage {
		if edge.Source == "mrn://bucket/gcs/"+testBucket && edge.Type == "FEEDS" {
			count++
		}
	}

	assert.Equal(t, 4, count, "three models and the image dataset read from the same bucket")
}

func TestBaseModelSource_IsEmptyWithoutASource(t *testing.T) {
	assert.Empty(t, baseModelSource(nil))
}
