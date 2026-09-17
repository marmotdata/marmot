package gluepipeline

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// namespace groups every run this plugin reports, the way OpenLineage
// namespaces a job.
const namespace = "glue"

// workflowRunHistory converts workflow runs into run events on the Pipeline
// asset.
func workflowRunHistory(assetMRN, workflow string, runs []types.WorkflowRun) pluginsdk.AssetRunHistory {
	history := pluginsdk.AssetRunHistory{AssetMRN: assetMRN}

	for _, run := range runs {
		runID := safeStr(run.WorkflowRunId)
		if runID == "" {
			continue
		}

		facets := map[string]interface{}{"status": string(run.Status)}
		if stats := runStatistics(run.Statistics); stats != nil {
			facets["statistics"] = stats
		}
		if message := safeStr(run.ErrorMessage); message != "" {
			facets["error_message"] = message
		}

		started := timeOrNow(run.StartedOn)
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      workflow,
			EventType:    "START",
			EventTime:    started,
			RunFacets:    facets,
		})
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      workflow,
			EventType:    workflowEventType(run.Status),
			EventTime:    completionTime(run.CompletedOn, started),
			RunFacets:    facets,
		})
	}

	return history
}

// jobRunHistory converts job runs into run events on the Job asset the Glue
// plugin creates.
func jobRunHistory(assetMRN, job string, runs []types.JobRun) pluginsdk.AssetRunHistory {
	history := pluginsdk.AssetRunHistory{AssetMRN: assetMRN}

	for _, run := range runs {
		runID := safeStr(run.Id)
		if runID == "" {
			continue
		}

		facets := map[string]interface{}{"state": string(run.JobRunState)}
		if run.ExecutionTime != 0 {
			facets["execution_time_seconds"] = run.ExecutionTime
		}
		if run.Attempt != 0 {
			facets["attempt"] = run.Attempt
		}
		if trigger := safeStr(run.TriggerName); trigger != "" {
			facets["trigger"] = trigger
		}
		if run.WorkerType != "" {
			facets["worker_type"] = string(run.WorkerType)
		}
		if run.NumberOfWorkers != nil {
			facets["number_of_workers"] = *run.NumberOfWorkers
		}
		if message := safeStr(run.ErrorMessage); message != "" {
			facets["error_message"] = message
		}

		started := timeOrNow(run.StartedOn)
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      job,
			EventType:    "START",
			EventTime:    started,
			RunFacets:    facets,
		})
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      job,
			EventType:    jobEventType(run.JobRunState),
			EventTime:    completionTime(run.CompletedOn, started),
			RunFacets:    facets,
		})
	}

	return history
}

// crawlerRunHistory converts crawls into run events on the Crawler asset the
// Glue plugin creates.
func crawlerRunHistory(assetMRN, crawler string, crawls []types.CrawlerHistory) pluginsdk.AssetRunHistory {
	history := pluginsdk.AssetRunHistory{AssetMRN: assetMRN}

	for _, crawl := range crawls {
		runID := safeStr(crawl.CrawlId)
		if runID == "" {
			continue
		}

		facets := map[string]interface{}{"state": string(crawl.State)}
		if crawl.DPUHour != 0 {
			facets["dpu_hour"] = crawl.DPUHour
		}
		if summary := safeStr(crawl.Summary); summary != "" {
			facets["summary"] = summary
		}
		if group := safeStr(crawl.LogGroup); group != "" {
			facets["log_group"] = group
		}
		if message := safeStr(crawl.ErrorMessage); message != "" {
			facets["error_message"] = message
		}

		started := timeOrNow(crawl.StartTime)
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      crawler,
			EventType:    "START",
			EventTime:    started,
			RunFacets:    facets,
		})
		history.Runs = append(history.Runs, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      crawler,
			EventType:    crawlerEventType(crawl.State),
			EventTime:    completionTime(crawl.EndTime, started),
			RunFacets:    facets,
		})
	}

	return history
}

// workflowEventType maps a workflow run status to an OpenLineage event type.
func workflowEventType(status types.WorkflowRunStatus) string {
	switch status {
	case types.WorkflowRunStatusCompleted:
		return "COMPLETE"
	case types.WorkflowRunStatusError:
		return "FAIL"
	case types.WorkflowRunStatusStopped:
		return "ABORT"
	case types.WorkflowRunStatusRunning, types.WorkflowRunStatusStopping:
		return "RUNNING"
	default:
		return "OTHER"
	}
}

// jobEventType maps a job run state to an OpenLineage event type.
func jobEventType(state types.JobRunState) string {
	switch state {
	case types.JobRunStateSucceeded:
		return "COMPLETE"
	case types.JobRunStateFailed, types.JobRunStateError, types.JobRunStateTimeout, types.JobRunStateExpired:
		return "FAIL"
	case types.JobRunStateStopped:
		return "ABORT"
	case types.JobRunStateRunning, types.JobRunStateStarting, types.JobRunStateStopping, types.JobRunStateWaiting:
		return "RUNNING"
	default:
		return "OTHER"
	}
}

// crawlerEventType maps a crawl state to an OpenLineage event type.
func crawlerEventType(state types.CrawlerHistoryState) string {
	switch state {
	case types.CrawlerHistoryStateCompleted:
		return "COMPLETE"
	case types.CrawlerHistoryStateFailed:
		return "FAIL"
	case types.CrawlerHistoryStateStopped:
		return "ABORT"
	case types.CrawlerHistoryStateRunning:
		return "RUNNING"
	default:
		return "OTHER"
	}
}

// runStatistics flattens the action counters of a workflow run.
func runStatistics(stats *types.WorkflowRunStatistics) map[string]interface{} {
	if stats == nil {
		return nil
	}
	return map[string]interface{}{
		"total_actions":     stats.TotalActions,
		"succeeded_actions": stats.SucceededActions,
		"failed_actions":    stats.FailedActions,
		"errored_actions":   stats.ErroredActions,
		"running_actions":   stats.RunningActions,
		"stopped_actions":   stats.StoppedActions,
		"timeout_actions":   stats.TimeoutActions,
		"waiting_actions":   stats.WaitingActions,
	}
}

func timeOrNow(t *time.Time) time.Time {
	if t == nil {
		return time.Now()
	}
	return *t
}

// completionTime falls back to the start time for a run that is still going,
// so every event has a time the server can order.
func completionTime(completed *time.Time, started time.Time) time.Time {
	if completed == nil {
		return started
	}
	return *completed
}
