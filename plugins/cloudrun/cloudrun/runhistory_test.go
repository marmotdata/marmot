package cloudrun

import (
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	run "google.golang.org/api/run/v2"
)

const jobMRN = "mrn://job/cloud-run/europe-west1-nightly-export"

func jobRunHistory(t *testing.T, fake *fakeAPI) pluginsdk.AssetRunHistory {
	t.Helper()

	result := discover(t, fake, nil)
	require.Len(t, result.RunHistory, 1)
	return result.RunHistory[0]
}

func eventsOfType(history pluginsdk.AssetRunHistory, eventType string) []pluginsdk.RunHistoryEvent {
	var events []pluginsdk.RunHistoryEvent
	for _, event := range history.Runs {
		if event.EventType == eventType {
			events = append(events, event)
		}
	}
	return events
}

func TestRunHistory_IsAttachedToTheJobAsset(t *testing.T) {
	history := jobRunHistory(t, newFakeAPI())

	assert.Equal(t, jobMRN, history.AssetMRN)
}

func TestRunHistory_EmitsAStartForEveryExecution(t *testing.T) {
	history := jobRunHistory(t, newFakeAPI())

	starts := eventsOfType(history, "START")
	require.Len(t, starts, 3)
	assert.Equal(t, "nightly-export-x9k2t", starts[0].RunID)
	assert.Equal(t, time.Date(2026, 8, 30, 7, 0, 4, 0, time.UTC), starts[0].EventTime.UTC())
}

func TestRunHistory_NamesTheJobByItsProjectAndAssetName(t *testing.T) {
	history := jobRunHistory(t, newFakeAPI())

	require.NotEmpty(t, history.Runs)
	assert.Equal(t, "acme", history.Runs[0].JobNamespace)
	assert.Equal(t, "europe-west1/nightly-export", history.Runs[0].JobName)
}

func TestRunHistory_MarksAnExecutionWithNoFailuresComplete(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{succeededExecution()},
	}

	history := jobRunHistory(t, fake)

	completes := eventsOfType(history, "COMPLETE")
	require.Len(t, completes, 1)
	assert.Equal(t, "nightly-export-x9k2t", completes[0].RunID)
	assert.Equal(t, time.Date(2026, 8, 30, 7, 11, 48, 0, time.UTC), completes[0].EventTime.UTC())
}

func TestRunHistory_MarksAnExecutionWithFailedTasksFailed(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{failedExecution()},
	}

	history := jobRunHistory(t, fake)

	fails := eventsOfType(history, "FAIL")
	require.Len(t, fails, 1)
	assert.Equal(t, "nightly-export-b4m1p", fails[0].RunID)
}

func TestRunHistory_MarksAnExecutionWithCancelledTasksAborted(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{cancelledExecution()},
	}

	history := jobRunHistory(t, fake)

	aborts := eventsOfType(history, "ABORT")
	require.Len(t, aborts, 1)
	assert.Equal(t, "nightly-export-q7w3z", aborts[0].RunID)
}

func TestRunHistory_MarksAnExecutionWithNoCompletionTimeRunning(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{runningExecution()},
	}

	history := jobRunHistory(t, fake)

	running := eventsOfType(history, "RUNNING")
	require.Len(t, running, 1)
	assert.Equal(t, "nightly-export-r1n2g", running[0].RunID)
	assert.Empty(t, eventsOfType(history, "COMPLETE"))
}

func TestRunHistory_CarriesTheTaskCountersAsFacets(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{failedExecution()},
	}

	history := jobRunHistory(t, fake)

	require.NotEmpty(t, history.Runs)
	facets := history.Runs[0].RunFacets
	assert.Equal(t, int64(4), facets["task_count"])
	assert.Equal(t, int64(3), facets["succeeded_count"])
	assert.Equal(t, int64(1), facets["failed_count"])
	assert.Equal(t, int64(0), facets["cancelled_count"])
	assert.Equal(t, int64(2), facets["retried_count"])
}

func TestRunHistory_CarriesTheLogLinkWhenTheAPIProvidesOne(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{succeededExecution()},
	}

	history := jobRunHistory(t, fake)

	require.NotEmpty(t, history.Runs)
	assert.Equal(t, "https://console.cloud.google.com/logs/viewer?project=acme", history.Runs[0].RunFacets["log_uri"])
}

func TestRunHistory_OmitsTheLogLinkWhenTheAPIHasNone(t *testing.T) {
	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{failedExecution()},
	}

	history := jobRunHistory(t, fake)

	require.NotEmpty(t, history.Runs)
	assert.NotContains(t, history.Runs[0].RunFacets, "log_uri")
}

func TestRunHistory_SkipsAnEventWhoseTimestampDoesNotParse(t *testing.T) {
	// A zero time would sort the run to 1970 in the history view, which reads
	// as a real event that happened long ago. Dropping it is honest.
	execution := succeededExecution()
	execution.CompletionTime = "not a timestamp"

	fake := newFakeAPI()
	fake.executions["nightly-export"] = &run.GoogleCloudRunV2ListExecutionsResponse{
		Executions: []*run.GoogleCloudRunV2Execution{execution},
	}

	history := jobRunHistory(t, fake)

	require.Len(t, history.Runs, 1)
	assert.Equal(t, "START", history.Runs[0].EventType)
}

func TestRunHistory_ReadsAtMostTheConfiguredNumberOfExecutions(t *testing.T) {
	fake := newFakeAPI()

	result := discover(t, fake, pluginsdk.RawConfig{"max_executions_per_job": 2})

	require.Len(t, result.RunHistory, 1)
	assert.Len(t, eventsOfType(result.RunHistory[0], "START"), 2)
}

func TestRunHistory_IsSkippedWhenExecutionsAreTurnedOff(t *testing.T) {
	result := discover(t, newFakeAPI(), pluginsdk.RawConfig{"include_executions": false})

	assert.Empty(t, result.RunHistory)
	assert.NotEmpty(t, result.Statistics, "the job asset and its statistic are still discovered")
}

func TestRunHistory_IsOmittedForAJobThatHasNeverRun(t *testing.T) {
	fake := newFakeAPI()
	fake.executions = nil

	result := discover(t, fake, nil)

	assert.Empty(t, result.RunHistory)
}
