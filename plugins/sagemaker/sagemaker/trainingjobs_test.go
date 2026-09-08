package sagemaker

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTrainingJobs is the config that opts a run into training jobs.
var withTrainingJobs = pluginsdk.RawConfig{"include_training_jobs": true}

func TestDiscoverTrainingJobs_AreSkippedByDefault(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	for _, a := range result.Assets {
		assert.NotEqual(t, "Job", a.Type)
	}
}

func TestDiscoverTrainingJobs_CatalogsTheJob(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	assert.Equal(t, []string{"SageMaker"}, job.Providers)
	assert.Equal(t, "Completed", job.Metadata["status"])
}

func TestDiscoverTrainingJobs_RecordsTheTrainingImage(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", job.Metadata["algorithm_image"])
}

func TestDiscoverTrainingJobs_RecordsTheHyperparameters(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")
	params, ok := job.Metadata["hyperparameters"].(map[string]any)
	require.True(t, ok, "expected a hyperparameter map")

	assert.Equal(t, "5", params["max_depth"])
}

func TestDiscoverTrainingJobs_RecordsTheHardware(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	assert.Equal(t, "ml.m5.large", job.Metadata["instance_type"])
	assert.Equal(t, int32(1), job.Metadata["instance_count"])
}

func TestDiscoverTrainingJobs_RecordsTheInputChannels(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")
	channels, ok := job.Metadata["input_channels"].(map[string]any)
	require.True(t, ok, "expected an input channel map")

	assert.Equal(t, "s3://ml-data/train", channels["train"])
}

func TestDiscoverTrainingJobs_RecordsTheOutputAndArtifactLocations(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	assert.Equal(t, "s3://ml-artifacts/churn", job.Metadata["output_s3_path"])
	assert.Equal(t, "s3://ml-artifacts/churn/model.tar.gz", job.Metadata["model_artifacts_s3_uri"])
}

func TestDiscoverTrainingJobs_RecordsTheFinalMetrics(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")
	metrics, ok := job.Metadata["final_metrics"].(map[string]any)
	require.True(t, ok, "expected a metric map")

	assert.Equal(t, float32(0.91), metrics["validation:auc"])
}

func TestDiscoverTrainingJobs_RecordsTheRunWindow(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	assert.Equal(t, "2026-09-01T10:00:00Z", job.Metadata["started_at"])
	assert.Equal(t, "2026-09-01T10:30:00Z", job.Metadata["ended_at"])
}

func TestDiscoverTrainingJobs_LinksToTheAWSConsole(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")

	require.Len(t, job.ExternalLinks, 1)
	assert.Equal(t,
		"https://us-east-1.console.aws.amazon.com/sagemaker/home?region=us-east-1#/jobs/churn-train-2026-09-01",
		job.ExternalLinks[0].URL)
}

func TestDiscoverTrainingJobs_LinksTheTrainingDataBucketToTheJob(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-data", "mrn://job/sagemaker/churn-train-2026-09-01", "FEEDS"))
}

func TestDiscoverTrainingJobs_LinksTheJobToTheModelItProduced(t *testing.T) {
	// SageMaker connects the two by artifact: the model points at the
	// tarball the job wrote.
	result := discoverWith(t, fullFake(), withTrainingJobs)

	assert.True(t, hasEdge(result, "mrn://job/sagemaker/churn-train-2026-09-01", "mrn://model/sagemaker/churn-xgb", "PRODUCES"))
}

func TestDiscoverTrainingJobs_SkipsTheModelEdgeWhenNoArtifactMatches(t *testing.T) {
	fake := fullFake()
	fake.trainingDetails["churn-train-2026-09-01"].ModelArtifacts.S3ModelArtifacts = aws.String("s3://ml-artifacts/other/model.tar.gz")

	result := discoverWith(t, fake, withTrainingJobs)

	assert.False(t, hasEdge(result, "mrn://job/sagemaker/churn-train-2026-09-01", "mrn://model/sagemaker/churn-xgb", "PRODUCES"))
}

func TestDiscoverTrainingJobs_EmitsAStartAndACompleteEvent(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	require.Len(t, result.RunHistory, 1)
	history := result.RunHistory[0]

	assert.Equal(t, "mrn://job/sagemaker/churn-train-2026-09-01", history.AssetMRN)
	require.Len(t, history.Runs, 2)
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
}

func TestDiscoverTrainingJobs_TimesTheRunEventsFromTheJob(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	runs := result.RunHistory[0].Runs

	assert.Equal(t, seedTime, runs[0].EventTime.UTC())
	assert.Equal(t, seedTime.Add(30*time.Minute), runs[1].EventTime.UTC())
}

func TestDiscoverTrainingJobs_NamesTheRunAfterTheJob(t *testing.T) {
	result := discoverWith(t, fullFake(), withTrainingJobs)

	run := result.RunHistory[0].Runs[0]

	assert.Equal(t, "churn-train-2026-09-01", run.RunID)
	assert.Equal(t, "churn-train-2026-09-01", run.JobName)
	assert.Equal(t, "sagemaker", run.JobNamespace)
}

func TestDiscoverTrainingJobs_EmitsOnlyAStartForARunningJob(t *testing.T) {
	// A run with no closing event is what tells Marmot the job is still
	// going.
	fake := fullFake()
	detail := fake.trainingDetails["churn-train-2026-09-01"]
	detail.TrainingJobStatus = types.TrainingJobStatusInProgress
	detail.TrainingEndTime = nil

	result := discoverWith(t, fake, withTrainingJobs)

	require.Len(t, result.RunHistory, 1)
	require.Len(t, result.RunHistory[0].Runs, 1)
	assert.Equal(t, "START", result.RunHistory[0].Runs[0].EventType)
}

func TestDiscoverTrainingJobs_RecordsTheFailureReason(t *testing.T) {
	fake := fullFake()
	detail := fake.trainingDetails["churn-train-2026-09-01"]
	detail.TrainingJobStatus = types.TrainingJobStatusFailed
	detail.FailureReason = aws.String("AlgorithmError: exit code 1")

	result := discoverWith(t, fake, withTrainingJobs)

	job := assetNamed(t, result, "Job", "churn-train-2026-09-01")
	assert.Equal(t, "AlgorithmError: exit code 1", job.Metadata["failure_reason"])
	assert.Equal(t, "FAIL", result.RunHistory[0].Runs[1].EventType)
}

func TestDiscoverTrainingJobs_SkipsAJobItCannotDescribe(t *testing.T) {
	fake := fullFake()
	fake.trainingJobs = append(fake.trainingJobs, types.TrainingJobSummary{TrainingJobName: aws.String("ghost")})

	result := discoverWith(t, fake, withTrainingJobs)

	for _, a := range result.Assets {
		assert.NotEqual(t, "ghost", *a.Name)
	}
	assetNamed(t, result, "Job", "churn-train-2026-09-01")
}

func TestDiscoverTrainingJobs_FailureKeepsTheRestOfTheRun(t *testing.T) {
	fake := fullFake()
	fake.errs = map[string]error{"ListTrainingJobs": errors.New("access denied")}

	result := discoverWith(t, fake, withTrainingJobs)

	assetNamed(t, result, "Model", "churn-xgb")
}

func TestTrainingEventType_ClosesACompletedJob(t *testing.T) {
	assert.Equal(t, "COMPLETE", trainingEventType(types.TrainingJobStatusCompleted))
}

func TestTrainingEventType_ClosesAFailedJob(t *testing.T) {
	assert.Equal(t, "FAIL", trainingEventType(types.TrainingJobStatusFailed))
}

func TestTrainingEventType_ClosesAStoppedJob(t *testing.T) {
	assert.Equal(t, "ABORT", trainingEventType(types.TrainingJobStatusStopped))
}

func TestTrainingEventType_LeavesARunningJobOpen(t *testing.T) {
	assert.Empty(t, trainingEventType(types.TrainingJobStatusInProgress))
}
