package openmetadata

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const pipelineFields = "owners,tags,domains,dataProducts,tasks"

// discoverPipelines catalogues orchestration pipelines and their tasks.
// The naming matches Marmot's Airflow plugin: a pipeline is named after
// the DAG, a task after "<dag>.<task>", with CONTAINS from the pipeline
// to each task and DEPENDS_ON between tasks.
func (c *collector) discoverPipelines(ctx context.Context, client *client) error {
	pipelines, err := listAll[pipeline](ctx, client, "/v1/pipelines", pipelineFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing pipelines: %w", err)
	}

	discovered, taskCount := 0, 0
	for _, pl := range pipelines {
		if !c.wanted(pl.entityBase) {
			continue
		}

		p := projectionFor(pl.ServiceType)
		pipelineName := strings.Join(fqnBelowService(pl.FullyQualifiedName), ".")
		if pipelineName == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "schedule_interval", pl.ScheduleInterval)
		putIf(metadata, "concurrency", pl.Concurrency)
		putIf(metadata, "task_count", len(pl.Tasks))

		asset := c.newAsset(pl.entityBase, "pipeline", "Pipeline", p, c.mrnName(pipelineName, pl.FullyQualifiedName), metadata)
		c.add(pl.ID, asset)
		discovered++

		if c.config.IncludeRunHistory {
			c.addRunHistory(ctx, client, pl, *asset.MRN, pipelineName)
		}

		if !c.config.IncludeTasks {
			continue
		}

		taskMRNs := make(map[string]string, len(pl.Tasks))
		for _, task := range pl.Tasks {
			taskAsset := c.taskAsset(pl, task, p, pipelineName)
			c.add("", taskAsset)
			taskMRNs[task.Name] = *taskAsset.MRN
			taskCount++

			c.link(*asset.MRN, *taskAsset.MRN, "CONTAINS")
		}

		for _, task := range pl.Tasks {
			from, ok := taskMRNs[task.Name]
			if !ok {
				continue
			}
			for _, downstream := range task.DownstreamTasks {
				if to, ok := taskMRNs[downstream]; ok {
					c.link(from, to, "DEPENDS_ON")
				}
			}
		}
	}

	log.Debug().Int("pipelines", discovered).Int("tasks", taskCount).Msg("Discovered pipelines")
	return nil
}

// addRunHistory turns a pipeline's recent executions into run history.
// OpenMetadata records one row per execution with a status, so each row
// becomes a START event and a terminal event, matching the shape the
// Airflow plugin produces.
func (c *collector) addRunHistory(ctx context.Context, client *client, pl pipeline, assetMRN, pipelineName string) {
	since := c.now.AddDate(0, 0, -c.config.RunHistoryDays)

	runs, err := client.pipelineRuns(ctx, pl.FullyQualifiedName, since, c.config.RunHistoryLimit)
	if err != nil {
		log.Debug().Err(err).Str("pipeline", pipelineName).Msg("Failed to read pipeline runs")
		return
	}
	if len(runs) == 0 {
		return
	}

	namespace := strings.ToLower(projectionFor(pl.ServiceType).Provider)
	events := make([]pluginsdk.RunHistoryEvent, 0, len(runs)*2)

	for _, run := range runs {
		runID := run.identifier(pl.Name)
		start, end := runWindow(run)
		facets := map[string]interface{}{
			"pipeline": pipelineName,
			"status":   run.ExecutionStatus,
		}

		events = append(events, pluginsdk.RunHistoryEvent{
			RunID:        runID,
			JobNamespace: namespace,
			JobName:      pipelineName,
			EventType:    "START",
			EventTime:    start,
			RunFacets:    facets,
		})

		if eventType := runEventType(run.ExecutionStatus); eventType != "START" {
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        runID,
				JobNamespace: namespace,
				JobName:      pipelineName,
				EventType:    eventType,
				EventTime:    end,
				RunFacets:    facets,
			})
		}
	}

	c.runHistory = append(c.runHistory, pluginsdk.AssetRunHistory{AssetMRN: assetMRN, Runs: events})
}

// runWindow is when an execution started and finished. OpenMetadata
// times the execution itself only on its tasks, so the pipeline's own
// timestamp is the fallback for both ends.
func runWindow(run pipelineStatus) (time.Time, time.Time) {
	start, end := run.Timestamp, run.Timestamp

	for _, task := range run.TaskStatus {
		if task.StartTime > 0 && (start == run.Timestamp || task.StartTime < start) {
			start = task.StartTime
		}
		if task.EndTime > end {
			end = task.EndTime
		}
	}

	return time.UnixMilli(start).UTC(), time.UnixMilli(end).UTC()
}

// runEventType maps an OpenMetadata execution status onto the
// OpenLineage event types Marmot stores run history as.
func runEventType(status string) string {
	switch status {
	case "Successful":
		return "COMPLETE"
	case "Failed":
		return "FAIL"
	case "Skipped":
		return "ABORT"
	default:
		return "START"
	}
}

// taskAsset builds a task asset. Tasks are nested inside the pipeline in
// OpenMetadata rather than being entities of their own, so they have no
// id, no tags of their own worth carrying, and no OpenMetadata page.
func (c *collector) taskAsset(pl pipeline, task pipelineTask, p projection, pipelineName string) pluginsdk.Asset {
	name := pipelineName + "." + task.Name

	metadata := map[string]interface{}{}
	putIf(metadata, "pipeline", pipelineName)
	putIf(metadata, "task_type", task.TaskType)
	putIf(metadata, "downstream_tasks", task.DownstreamTasks)

	base := pl.entityBase
	base.ID = ""
	base.Name = task.Name
	base.DisplayName = task.DisplayName
	base.Description = task.Description
	base.FullyQualifiedName = pl.FullyQualifiedName + "." + task.Name
	base.SourceURL = task.SourceURL
	base.Tags = task.Tags
	base.Owners = task.Owners

	asset := c.newAsset(base, "", "Task", p, name, metadata)

	// The task's own OpenMetadata address is the pipeline it belongs to.
	if url := c.entityURL(pl.entityBase, "pipeline"); url != "" {
		asset.ExternalLinks = append([]pluginsdk.AssetExternalLink{{
			Name: "OpenMetadata",
			Icon: "mdi:database-search",
			URL:  url,
		}}, asset.ExternalLinks...)
	}

	return asset
}
