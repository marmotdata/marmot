package vertexai

import (
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// pipelineJobAssets catalogues recent pipeline runs and turns each one
// into a run history entry. It returns the jobs keyed by resource name, so
// a model produced by one of them can point back at it.
func (s *Source) pipelineJobAssets(items []scanned[*aiplatform.GoogleCloudAiplatformV1PipelineJob]) ([]pluginsdk.Asset, index, []pluginsdk.AssetRunHistory) {
	displayNames := make([]string, 0, len(items))
	for _, item := range items {
		displayNames = append(displayNames, item.resource.DisplayName)
	}
	naming := newNames(displayNames)

	assets := make([]pluginsdk.Asset, 0, len(items))
	jobs := index{}
	var runs []pluginsdk.AssetRunHistory

	for _, item := range items {
		job := item.resource
		id := s.locate(job.Name, item.location)

		name := naming.resolve(job.DisplayName, id.id)
		if name == "" {
			log.Warn().Str("resource_name", job.Name).Msg("Skipping a pipeline job with no display name or id")
			continue
		}

		asset := s.newAsset("Job", name, "", s.pipelineJobMetadata(job, id))
		assets = append(assets, asset)
		jobs[job.Name] = name

		if history := runHistory(job, *asset.MRN, name, id); history != nil {
			runs = append(runs, *history)
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered pipeline jobs")
	return assets, jobs, runs
}

func (s *Source) pipelineJobMetadata(job *aiplatform.GoogleCloudAiplatformV1PipelineJob, id resourceName) map[string]any {
	metadata := commonMetadata(id, job.CreateTime, job.UpdateTime, job.Labels)

	putString(metadata, "display_name", job.DisplayName)
	putString(metadata, "state", job.State)
	putString(metadata, "start_time", job.StartTime)
	putString(metadata, "end_time", job.EndTime)
	putString(metadata, "schedule_name", job.ScheduleName)
	putString(metadata, "template_uri", job.TemplateUri)
	putString(metadata, "service_account", job.ServiceAccount)

	if job.Error != nil {
		putString(metadata, "error_message", job.Error.Message)
	}

	// PipelineSpec is deliberately left out: it is the whole compiled
	// pipeline document and would swamp the asset.

	return metadata
}

// runHistory turns a pipeline job's state and timestamps into the events
// Marmot shows on the asset's run history.
func runHistory(job *aiplatform.GoogleCloudAiplatformV1PipelineJob, assetMRN, name string, id resourceName) *pluginsdk.AssetRunHistory {
	facets := map[string]any{"resource_id": id.id}
	if job.State != "" {
		facets["state"] = job.State
	}

	event := func(eventType, timestamp string) *pluginsdk.RunHistoryEvent {
		at, ok := parseTimestamp(timestamp, job.Name)
		if !ok {
			return nil
		}
		return &pluginsdk.RunHistoryEvent{
			RunID:        id.id,
			JobNamespace: id.project,
			JobName:      name,
			EventType:    eventType,
			EventTime:    at,
			RunFacets:    facets,
		}
	}

	var events []pluginsdk.RunHistoryEvent
	if start := event("START", job.StartTime); start != nil {
		events = append(events, *start)
	}
	if final := event(pipelineEventType(job.State), job.EndTime); final != nil {
		events = append(events, *final)
	}

	if len(events) == 0 {
		return nil
	}

	return &pluginsdk.AssetRunHistory{AssetMRN: assetMRN, Runs: events}
}

// pipelineEventType maps a pipeline state onto the event that closes its
// run.
func pipelineEventType(state string) string {
	switch state {
	case "PIPELINE_STATE_SUCCEEDED":
		return "COMPLETE"
	case "PIPELINE_STATE_FAILED":
		return "FAIL"
	case "PIPELINE_STATE_CANCELLED":
		return "ABORT"
	case "PIPELINE_STATE_RUNNING":
		return "RUNNING"
	default:
		return "OTHER"
	}
}

// parseTimestamp reads one of the API's RFC 3339 timestamps. A job that has
// not started or finished simply has none, which is not a problem; a value
// that will not parse is, because the alternative is filing the event at
// the zero time.
func parseTimestamp(value, job string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Warn().Err(err).Str("pipeline_job", job).Str("timestamp", value).
			Msg("Skipping a run history event with an unreadable timestamp")
		return time.Time{}, false
	}

	return at, true
}
