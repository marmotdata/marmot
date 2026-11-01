package gluepipeline

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobEventType_SucceededIsComplete(t *testing.T) {
	assert.Equal(t, "COMPLETE", jobEventType(types.JobRunStateSucceeded))
}

func TestJobEventType_FailedIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", jobEventType(types.JobRunStateFailed))
}

func TestJobEventType_TimeoutIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", jobEventType(types.JobRunStateTimeout))
}

func TestJobEventType_ErrorIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", jobEventType(types.JobRunStateError))
}

// An expired run never finished, so it counts as a failure rather than a
// deliberate stop.
func TestJobEventType_ExpiredIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", jobEventType(types.JobRunStateExpired))
}

func TestJobEventType_StoppedIsAbort(t *testing.T) {
	assert.Equal(t, "ABORT", jobEventType(types.JobRunStateStopped))
}

func TestJobEventType_StartingIsRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", jobEventType(types.JobRunStateStarting))
}

func TestJobEventType_WaitingIsRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", jobEventType(types.JobRunStateWaiting))
}

func TestJobEventType_UnknownIsOther(t *testing.T) {
	assert.Equal(t, "OTHER", jobEventType(types.JobRunState("SOMETHING_NEW")))
}

func TestWorkflowEventType_CompletedIsComplete(t *testing.T) {
	assert.Equal(t, "COMPLETE", workflowEventType(types.WorkflowRunStatusCompleted))
}

func TestWorkflowEventType_ErrorIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", workflowEventType(types.WorkflowRunStatusError))
}

func TestWorkflowEventType_StoppedIsAbort(t *testing.T) {
	assert.Equal(t, "ABORT", workflowEventType(types.WorkflowRunStatusStopped))
}

func TestWorkflowEventType_StoppingIsRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", workflowEventType(types.WorkflowRunStatusStopping))
}

func TestCrawlerEventType_CompletedIsComplete(t *testing.T) {
	assert.Equal(t, "COMPLETE", crawlerEventType(types.CrawlerHistoryStateCompleted))
}

func TestCrawlerEventType_FailedIsFail(t *testing.T) {
	assert.Equal(t, "FAIL", crawlerEventType(types.CrawlerHistoryStateFailed))
}

func TestCrawlerEventType_StoppedIsAbort(t *testing.T) {
	assert.Equal(t, "ABORT", crawlerEventType(types.CrawlerHistoryStateStopped))
}

func TestJobRunHistory_EmitsAStartAndAnEndEvent(t *testing.T) {
	history := jobRunHistory("mrn://job/glue/load-orders", "load-orders", []types.JobRun{{
		Id:          aws.String("jr_1"),
		JobRunState: types.JobRunStateSucceeded,
		StartedOn:   &runStart,
		CompletedOn: &runFinish,
	}})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "mrn://job/glue/load-orders", history.AssetMRN)
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, runStart, history.Runs[0].EventTime)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
	assert.Equal(t, runFinish, history.Runs[1].EventTime)
}

func TestJobRunHistory_RecordsTheRunFacets(t *testing.T) {
	history := jobRunHistory("mrn://job/glue/load-orders", "load-orders", []types.JobRun{{
		Id:              aws.String("jr_1"),
		JobRunState:     types.JobRunStateSucceeded,
		StartedOn:       &runStart,
		CompletedOn:     &runFinish,
		ExecutionTime:   720,
		Attempt:         2,
		TriggerName:     aws.String("after-crawl"),
		WorkerType:      types.WorkerTypeG1x,
		NumberOfWorkers: aws.Int32(4),
	}})

	require.Len(t, history.Runs, 2)
	facets := history.Runs[1].RunFacets
	assert.Equal(t, "SUCCEEDED", facets["state"])
	assert.Equal(t, int32(720), facets["execution_time_seconds"])
	assert.Equal(t, int32(2), facets["attempt"])
	assert.Equal(t, "after-crawl", facets["trigger"])
	assert.Equal(t, "G.1X", facets["worker_type"])
	assert.Equal(t, int32(4), facets["number_of_workers"])
}

func TestJobRunHistory_RecordsTheErrorOfAFailedRun(t *testing.T) {
	history := jobRunHistory("mrn://job/glue/load-orders", "load-orders", []types.JobRun{{
		Id:           aws.String("jr_2"),
		JobRunState:  types.JobRunStateFailed,
		StartedOn:    &runStart,
		CompletedOn:  &runFinish,
		ErrorMessage: aws.String("OutOfMemoryError"),
	}})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "FAIL", history.Runs[1].EventType)
	assert.Equal(t, "OutOfMemoryError", history.Runs[1].RunFacets["error_message"])
}

// A run still in flight has no completion time, so the second event has to
// borrow the start time to stay orderable.
func TestJobRunHistory_UsesTheStartTimeWhileARunIsInFlight(t *testing.T) {
	history := jobRunHistory("mrn://job/glue/load-orders", "load-orders", []types.JobRun{{
		Id:          aws.String("jr_3"),
		JobRunState: types.JobRunStateRunning,
		StartedOn:   &runStart,
	}})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "RUNNING", history.Runs[1].EventType)
	assert.Equal(t, runStart, history.Runs[1].EventTime)
}

func TestJobRunHistory_SkipsRunsWithoutAnID(t *testing.T) {
	history := jobRunHistory("mrn://job/glue/load-orders", "load-orders", []types.JobRun{{
		JobRunState: types.JobRunStateSucceeded,
		StartedOn:   &runStart,
	}})

	assert.Empty(t, history.Runs)
}

func TestWorkflowRunHistory_RecordsTheActionCounters(t *testing.T) {
	history := workflowRunHistory("mrn://pipeline/glue/daily-etl", "daily-etl", []types.WorkflowRun{{
		WorkflowRunId: aws.String("wr_1"),
		Status:        types.WorkflowRunStatusCompleted,
		StartedOn:     &runStart,
		CompletedOn:   &runFinish,
		Statistics:    &types.WorkflowRunStatistics{TotalActions: 3, SucceededActions: 2, FailedActions: 1},
	}})

	require.Len(t, history.Runs, 2)
	statistics, ok := history.Runs[1].RunFacets["statistics"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, int32(3), statistics["total_actions"])
	assert.Equal(t, int32(2), statistics["succeeded_actions"])
	assert.Equal(t, int32(1), statistics["failed_actions"])
}

func TestWorkflowRunHistory_NamesTheJobAfterTheWorkflow(t *testing.T) {
	history := workflowRunHistory("mrn://pipeline/glue/daily-etl", "daily-etl", []types.WorkflowRun{{
		WorkflowRunId: aws.String("wr_1"),
		Status:        types.WorkflowRunStatusRunning,
		StartedOn:     &runStart,
	}})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "glue", history.Runs[0].JobNamespace)
	assert.Equal(t, "daily-etl", history.Runs[0].JobName)
	assert.Equal(t, "RUNNING", history.Runs[1].EventType)
}

func TestCrawlerRunHistory_RecordsTheSummary(t *testing.T) {
	history := crawlerRunHistory("mrn://crawler/glue/orders-crawler", "orders-crawler", []types.CrawlerHistory{{
		CrawlId:   aws.String("crawl_1"),
		State:     types.CrawlerHistoryStateCompleted,
		StartTime: &runStart,
		EndTime:   &runFinish,
		Summary:   aws.String("Added 1 table"),
		LogGroup:  aws.String("/aws-glue/crawlers"),
	}})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "Added 1 table", history.Runs[1].RunFacets["summary"])
	assert.Equal(t, "/aws-glue/crawlers", history.Runs[1].RunFacets["log_group"])
}

func TestRunStatistics_IsNilWithoutStatistics(t *testing.T) {
	assert.Nil(t, runStatistics(nil))
}
