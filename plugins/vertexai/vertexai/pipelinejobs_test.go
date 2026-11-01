package vertexai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/aiplatform/v1"
)

func TestPipelineJob_IsNamedAfterItsDisplayName(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")

	assert.Equal(t, "mrn://job/vertex-ai/churn-training", *job.MRN)
}

func TestPipelineJob_RecordsItsStateAndTimestamps(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")

	assert.Equal(t, "PIPELINE_STATE_SUCCEEDED", job.Metadata["state"])
	assert.Equal(t, "2026-08-01T09:00:00Z", job.Metadata["start_time"])
	assert.Equal(t, "2026-08-01T09:45:00Z", job.Metadata["end_time"])
}

func TestPipelineJob_RecordsHowItWasRun(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")

	assert.Equal(t, "https://us-central1-kfp.pkg.dev/acme-ml/pipelines/churn/v3", job.Metadata["template_uri"])
	assert.Equal(t, "pipelines@acme-ml.iam.gserviceaccount.com", job.Metadata["service_account"])
	assert.Contains(t, job.Metadata["schedule_name"], "schedules/nightly")
}

func TestPipelineJob_RecordsWhyItFailed(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training-retry")

	assert.Equal(t, "training container exited with code 1", job.Metadata["error_message"])
}

func TestPipelineJob_LeavesOutThePipelineSpec(t *testing.T) {
	// The compiled pipeline is a whole document and would swamp the asset.
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")

	assert.NotContains(t, job.Metadata, "pipeline_spec")
	assert.NotContains(t, job.Metadata, "pipelineSpec")
}

func TestPipelineJob_EmitsAStartAndACompleteEvent(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
}

func TestPipelineJob_RunEventsCarryTheJobIdentity(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)
	require.NotEmpty(t, history.Runs)

	assert.Equal(t, trainJobID, history.Runs[0].RunID)
	assert.Equal(t, testProject, history.Runs[0].JobNamespace)
	assert.Equal(t, "churn-training", history.Runs[0].JobName)
}

func TestPipelineJob_RunEventsCarryTheRealTimestamps(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)
	require.Len(t, history.Runs, 2)

	assert.Equal(t, "2026-08-01T09:00:00Z", history.Runs[0].EventTime.UTC().Format(time.RFC3339))
	assert.Equal(t, "2026-08-01T09:45:00Z", history.Runs[1].EventTime.UTC().Format(time.RFC3339))
}

func TestPipelineJob_ClosesAFailedRunWithFail(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training-retry")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)
	require.Len(t, history.Runs, 2)

	assert.Equal(t, "FAIL", history.Runs[1].EventType)
}

func TestPipelineJob_ClosesACancelledRunWithAbort(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training-cancelled")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)
	require.Len(t, history.Runs, 2)

	assert.Equal(t, "ABORT", history.Runs[1].EventType)
}

func TestPipelineJob_ClosesAnUnrecognisedStateWithOther(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training-queued")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)
	require.Len(t, history.Runs, 2)

	assert.Equal(t, "OTHER", history.Runs[1].EventType)
}

func TestPipelineJob_LeavesARunningRunOpen(t *testing.T) {
	// A job still going has no end time, and only the start event is what
	// Marmot reads as a run in progress.
	result := discoverWith(t, fullFake(), withPipelineJobs)

	job := assetNamed(t, result, "Job", "churn-training-live")
	history := runHistoryFor(result, *job.MRN)
	require.NotNil(t, history)

	require.Len(t, history.Runs, 1)
	assert.Equal(t, "START", history.Runs[0].EventType)
}

func TestPipelineJob_SkipsAnEventWithAnUnreadableTimestamp(t *testing.T) {
	// Filing the event at the zero time would put a run in 1970.
	fake := fullFake()
	fake.pipelineJobs[testLocation] = []*aiplatform.GoogleCloudAiplatformV1PipelineJob{{
		Name:        pipelineJobName(trainJobID),
		DisplayName: "churn-training",
		State:       "PIPELINE_STATE_SUCCEEDED",
		StartTime:   "2026-08-01T09:00:00Z",
		EndTime:     "not a timestamp",
	}}

	result := discoverWith(t, fake, withPipelineJobs)

	history := runHistoryFor(result, "mrn://job/vertex-ai/churn-training")
	require.NotNil(t, history)
	require.Len(t, history.Runs, 1)
	assert.Equal(t, "START", history.Runs[0].EventType)
}

func TestPipelineJob_EmitsNoRunHistoryWithoutAnyTimestamp(t *testing.T) {
	fake := fullFake()
	fake.pipelineJobs[testLocation] = []*aiplatform.GoogleCloudAiplatformV1PipelineJob{{
		Name:        pipelineJobName(trainJobID),
		DisplayName: "churn-training",
		State:       "PIPELINE_STATE_QUEUED",
	}}

	result := discoverWith(t, fake, withPipelineJobs)

	assetNamed(t, result, "Job", "churn-training")
	assert.Nil(t, runHistoryFor(result, "mrn://job/vertex-ai/churn-training"))
}

func TestPipelineJobs_QualifyBothSidesOfADisplayNameCollision(t *testing.T) {
	// Scheduled pipelines run under the same display name every night.
	fake := fullFake()
	repeat := trainJob()
	repeat.Name = pipelineJobName("1400000000000000014")
	fake.pipelineJobs[testLocation] = append(fake.pipelineJobs[testLocation], repeat)

	result := discoverWith(t, fake, withPipelineJobs)

	assetNamed(t, result, "Job", "churn-training ("+trainJobID+")")
	assetNamed(t, result, "Job", "churn-training (1400000000000000014)")
}

func TestPipelineEventType_MapsEveryStateThePluginNames(t *testing.T) {
	assert.Equal(t, "COMPLETE", pipelineEventType("PIPELINE_STATE_SUCCEEDED"))
	assert.Equal(t, "FAIL", pipelineEventType("PIPELINE_STATE_FAILED"))
	assert.Equal(t, "ABORT", pipelineEventType("PIPELINE_STATE_CANCELLED"))
	assert.Equal(t, "RUNNING", pipelineEventType("PIPELINE_STATE_RUNNING"))
}

func TestPipelineEventType_MapsAnUnknownStateToOther(t *testing.T) {
	assert.Equal(t, "OTHER", pipelineEventType("PIPELINE_STATE_PAUSED"))
	assert.Equal(t, "OTHER", pipelineEventType(""))
}

func TestParseTimestamp_ReadsAnRFC3339Value(t *testing.T) {
	at, ok := parseTimestamp("2026-08-01T09:00:00Z", "job")

	require.True(t, ok)
	assert.Equal(t, 2026, at.Year())
}

func TestParseTimestamp_RejectsAnUnreadableValue(t *testing.T) {
	_, ok := parseTimestamp("yesterday", "job")

	assert.False(t, ok)
}

func TestParseTimestamp_RejectsAnEmptyValue(t *testing.T) {
	_, ok := parseTimestamp("", "job")

	assert.False(t, ok)
}

func TestPipelineJobs_StopAtTheConfiguredLimit(t *testing.T) {
	server := fullFake().start(t)

	source := &Source{}
	result, err := source.Discover(t.Context(), map[string]any{
		"project_id":            testProject,
		"locations":             []string{testLocation},
		"endpoint":              server.URL,
		"disable_auth":          true,
		"include_pipeline_jobs": true,
		"max_pipeline_jobs":     2,
	})
	require.NoError(t, err)

	jobs := 0
	for _, asset := range result.Assets {
		if asset.Type == "Job" {
			jobs++
		}
	}

	assert.Equal(t, 2, jobs)
}
