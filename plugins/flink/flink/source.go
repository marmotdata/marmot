// Package flink discovers jobs and their vertices from an Apache Flink
// JobManager.
package flink

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Flink"

// errorLimit caps the failure text kept on a failed job. Flink hands back
// a full stack trace, which is far more than a catalog entry needs.
const errorLimit = 500

// Config for the Flink plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"JobManager REST URL, for example http://localhost:8081" validate:"required,url"`
	Username string `json:"username,omitempty" description:"Username for basic auth, when a proxy in front of the JobManager requires it"`
	Password string `json:"password,omitempty" description:"Password for basic auth" sensitive:"true"`
	Token    string `json:"token,omitempty" description:"Bearer token, when a proxy in front of the JobManager requires it" sensitive:"true"`

	VerifySSL         bool `json:"verify_ssl" label:"Verify SSL" description:"Verify the JobManager's TLS certificate" default:"true"`
	IncludeTasks      bool `json:"include_tasks" description:"Discover each job vertex as a Task asset" default:"true"`
	IncludeRunHistory bool `json:"include_run_history" description:"Record each job's state changes as run history" default:"true"`
	IncludeCompleted  bool `json:"include_completed" description:"Include the finished, failed and cancelled jobs the JobManager still lists" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "http://flink-jobmanager.internal:8081"
include_tasks: true
include_run_history: true
include_completed: true
tags:
  - "flink"
  - "streaming"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "flink",
		Name:        "Flink",
		Description: "Discover jobs and their vertices from an Apache Flink JobManager",
		Icon:        "flink",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Flink plugin.
type Source struct {
	config *Config
	client *client
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimSuffix(strings.TrimSpace(config.Host), "/")

	if config.Username != "" && config.Token != "" {
		return nil, fmt.Errorf("username and token are mutually exclusive: configure one of them")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Flink jobs as Pipelines and their vertices as Tasks.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	s.client = newClient(s.config.Host, s.config.Username, s.config.Password, s.config.Token, s.config.VerifySSL)

	cluster, err := s.client.clusterConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading cluster config: %w", err)
	}
	log.Debug().Str("host", s.config.Host).Str("version", cluster.FlinkVersion).Msg("Connected to Flink")

	jobs, err := s.client.jobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	if !s.config.IncludeCompleted {
		active := jobs[:0]
		for _, job := range jobs {
			if !isCompleted(job.State) {
				active = append(active, job)
			}
		}
		jobs = active
	}
	log.Debug().Int("count", len(jobs)).Msg("Found jobs")

	names := pipelineNames(jobs)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	for _, job := range jobs {
		jobAssets, jobLineages, runs, err := s.discoverJob(ctx, job, names[job.JID], cluster.FlinkVersion)
		if err != nil {
			log.Warn().Err(err).Str("jid", job.JID).Str("name", job.Name).Msg("Failed to discover job")
			continue
		}
		assets = append(assets, jobAssets...)
		lineages = append(lineages, jobLineages...)
		if runs != nil {
			runHistory = append(runHistory, *runs)
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("run_history", len(runHistory)).
		Msg("Flink discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		RunHistory: runHistory,
	}, nil
}

// discoverJob builds one job's Pipeline, its Tasks, the edges between them
// and its run history. The job details are required; the config and the
// failure cause only enrich the result, so losing them is a warning.
func (s *Source) discoverJob(ctx context.Context, job jobSummary, name, version string) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, *pluginsdk.AssetRunHistory, error) {
	details, err := s.client.job(ctx, job.JID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading job: %w", err)
	}

	config, err := s.client.config(ctx, job.JID)
	if err != nil {
		log.Warn().Err(err).Str("jid", job.JID).Msg("Failed to read job config")
		config = &jobConfig{}
	}

	var cause string
	if details.State == "FAILED" {
		exceptions, err := s.client.exceptions(ctx, job.JID)
		if err != nil {
			log.Warn().Err(err).Str("jid", job.JID).Msg("Failed to read job exceptions")
		} else {
			cause = truncate(exceptions.rootCause(), errorLimit)
		}
	}

	pipeline := s.pipelineAsset(job, details, config.ExecutionConfig, name, version, cause)
	assets := []pluginsdk.Asset{pipeline}

	var lineages []pluginsdk.LineageEdge
	if s.config.IncludeTasks {
		tasks, edges := s.taskAssets(name, details)
		assets = append(assets, tasks...)
		lineages = append(lineages, edges...)
	}

	var runs *pluginsdk.AssetRunHistory
	if s.config.IncludeRunHistory {
		history := runHistory(*pipeline.MRN, details, config.ExecutionConfig.JobParallelism)
		if len(history.Runs) > 0 {
			runs = &history
		}
	}

	return assets, lineages, runs, nil
}

func (s *Source) pipelineAsset(job jobSummary, details *jobDetails, config executionConfig, name, version, cause string) pluginsdk.Asset {
	mrnValue := assetMRN("Pipeline", name)
	url := jobURL(s.config.Host, details.JID, details.State)

	metadata := map[string]interface{}{
		"jid":           details.JID,
		"state":         details.State,
		"duration_ms":   details.Duration,
		"is_stoppable":  details.IsStoppable,
		"vertex_count":  len(details.Vertices),
		"flink_version": version,
		"url":           url,
	}
	putTime(metadata, "start_time", details.StartTime)
	putTime(metadata, "end_time", details.EndTime)
	if details.MaxParallelism > 0 {
		metadata["max_parallelism"] = details.MaxParallelism
	}
	if config.JobParallelism > 0 {
		metadata["parallelism"] = config.JobParallelism
	}
	if config.ExecutionMode != "" {
		metadata["execution_mode"] = config.ExecutionMode
	}
	if config.RestartStrategy != "" {
		metadata["restart_strategy"] = config.RestartStrategy
	}
	if len(job.Tasks) > 0 {
		counts := make(map[string]interface{}, len(job.Tasks))
		for state, count := range job.Tasks {
			counts[state] = count
		}
		metadata["task_counts"] = counts
	}
	if cause != "" {
		metadata["error"] = cause
	}

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Pipeline",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Flink",
			URL:  url,
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// taskAssets builds a Task per vertex, a CONTAINS edge from the pipeline
// to each, and a DEPENDS_ON edge for every input in the plan, from the
// vertex read to the vertex reading it.
func (s *Source) taskAssets(pipelineName string, details *jobDetails) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	pipelineMRN := assetMRN("Pipeline", pipelineName)
	url := jobURL(s.config.Host, details.JID, details.State)

	nodes := make(map[string]planNode, len(details.Plan.Nodes))
	for _, node := range details.Plan.Nodes {
		nodes[node.ID] = node
	}

	names := vertexNames(details.Vertices)
	taskMRNs := make(map[string]string, len(details.Vertices))
	for _, vertex := range details.Vertices {
		taskMRNs[vertex.ID] = assetMRN("Task", pipelineName+"/"+names[vertex.ID])
	}

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

	for _, vertex := range details.Vertices {
		name := pipelineName + "/" + names[vertex.ID]
		mrnValue := taskMRNs[vertex.ID]

		metadata := map[string]interface{}{
			"jid":           details.JID,
			"pipeline":      pipelineName,
			"vertex_id":     vertex.ID,
			"status":        vertex.Status,
			"parallelism":   vertex.Parallelism,
			"duration_ms":   vertex.Duration,
			"read_records":  vertex.Metrics.ReadRecords,
			"write_records": vertex.Metrics.WriteRecords,
			"read_bytes":    vertex.Metrics.ReadBytes,
			"write_bytes":   vertex.Metrics.WriteBytes,
		}
		if vertex.MaxParallelism > 0 {
			metadata["max_parallelism"] = vertex.MaxParallelism
		}
		putTime(metadata, "start_time", vertex.StartTime)
		putTime(metadata, "end_time", vertex.EndTime)

		node, ok := nodes[vertex.ID]
		if ok {
			if node.Operator != "" {
				metadata["operator"] = node.Operator
			}
			if description := cleanDescription(node.Description); description != "" {
				metadata["description"] = description
			}
		}

		assets = append(assets, pluginsdk.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      "Task",
			Providers: []string{provider},
			Metadata:  metadata,
			Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
			ExternalLinks: []pluginsdk.AssetExternalLink{{
				Name: "Open in Flink",
				URL:  url,
			}},
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: pipelineMRN,
			Target: mrnValue,
			Type:   "CONTAINS",
		})

		for _, input := range node.Inputs {
			upstream, ok := taskMRNs[input.ID]
			if !ok {
				log.Debug().Str("jid", details.JID).Str("vertex", vertex.ID).Str("input", input.ID).
					Msg("Skipping plan input that names no vertex")
				continue
			}
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: upstream,
				Target: mrnValue,
				Type:   "DEPENDS_ON",
			})
		}
	}

	return assets, lineages
}

// runHistory turns the job's state timestamps into one run, identified by
// the job id. Flink records when the job entered each state, so a job
// that has finished yields its START, RUNNING and COMPLETE events at
// once, and a job still running yields only the first two.
func runHistory(assetMRN string, details *jobDetails, parallelism int) pluginsdk.AssetRunHistory {
	facets := map[string]interface{}{
		"state":       details.State,
		"duration_ms": details.Duration,
	}
	if parallelism > 0 {
		facets["parallelism"] = parallelism
	}

	var events []pluginsdk.RunHistoryEvent
	event := func(eventType string, at int64) {
		if at <= 0 {
			return
		}
		events = append(events, pluginsdk.RunHistoryEvent{
			RunID:        details.JID,
			JobNamespace: "flink",
			JobName:      details.Name,
			EventType:    eventType,
			EventTime:    time.UnixMilli(at).UTC(),
			RunFacets:    facets,
		})
	}

	timestamps := details.Timestamps
	started := timestamps["CREATED"]
	if started <= 0 {
		started = timestamps["RUNNING"]
	}
	if started <= 0 {
		started = details.StartTime
	}

	event("START", started)
	event("RUNNING", timestamps["RUNNING"])
	event("COMPLETE", timestamps["FINISHED"])
	event("FAIL", timestamps["FAILED"])
	event("ABORT", timestamps["CANCELED"])

	return pluginsdk.AssetRunHistory{
		AssetMRN: assetMRN,
		Runs:     events,
	}
}

// pipelineNames picks the asset name for every job, keyed by job id.
// Flink lets any number of jobs share a name (resubmitting the same jar
// does exactly that), so when they do the most recently started job keeps
// the bare name and the others carry their id.
func pipelineNames(jobs []jobSummary) map[string]string {
	newest := make(map[string]jobSummary, len(jobs))
	for _, job := range jobs {
		current, ok := newest[job.Name]
		if !ok || job.StartTime > current.StartTime || (job.StartTime == current.StartTime && job.JID > current.JID) {
			newest[job.Name] = job
		}
	}

	names := make(map[string]string, len(jobs))
	for _, job := range jobs {
		switch {
		case job.Name == "":
			names[job.JID] = job.JID
		case newest[job.Name].JID == job.JID:
			names[job.JID] = job.Name
		default:
			names[job.JID] = fmt.Sprintf("%s (%s)", job.Name, job.JID)
		}
	}
	return names
}

// vertexNames picks the name of every vertex within one job, keyed by
// vertex id. Two vertices of a job can carry the same operator name, so
// the first keeps the bare name and the rest carry their id.
func vertexNames(vertices []jobVertex) map[string]string {
	seen := make(map[string]struct{}, len(vertices))
	names := make(map[string]string, len(vertices))
	for _, vertex := range vertices {
		name := vertex.Name
		if name == "" {
			name = vertex.ID
		}
		if _, dup := seen[name]; dup {
			names[vertex.ID] = fmt.Sprintf("%s (%s)", name, vertex.ID)
			continue
		}
		seen[name] = struct{}{}
		names[vertex.ID] = name
	}
	return names
}

// isCompleted reports whether the job has reached a state it cannot leave.
func isCompleted(state string) bool {
	switch state {
	case "FINISHED", "FAILED", "CANCELED":
		return true
	}
	return false
}

// jobURL is the job's page in the Flink web UI, which files jobs under
// "running" or "completed" depending on their state.
func jobURL(host, jid, state string) string {
	switch {
	case state == "RUNNING":
		return host + "/#/job/running/" + jid + "/overview"
	case isCompleted(state):
		return host + "/#/job/completed/" + jid + "/overview"
	default:
		return host + "/#/overview"
	}
}

// putTime records an epoch millisecond timestamp as RFC3339. Flink uses
// -1 (end-time) and 0 (timestamps) for "not yet", both of which are
// left out.
func putTime(metadata map[string]interface{}, key string, millis int64) {
	if millis <= 0 {
		return
	}
	metadata[key] = time.UnixMilli(millis).UTC().Format(time.RFC3339)
}

// cleanDescription flattens the plan's HTML-ish operator description
// ("counter<br/>+- Sink: print-sink<br/>") into one line.
func cleanDescription(description string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(description, "<br/>", " ")), " ")
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

// assetMRN is the single place a Flink MRN is built, so the asset pass,
// the lineage pass and the run history can never drift into addressing
// the same job or vertex differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
