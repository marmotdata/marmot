// Package dagster discovers jobs, ops and software-defined assets from a
// Dagster webserver.
package dagster

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string every Dagster asset is filed under.
const provider = "Dagster"

// Dagster generates an implicit job for materialising assets and names it with
// a double underscore prefix. It is an internal detail of the asset machinery
// rather than something a user wrote, so discovery skips it.
const internalJobPrefix = "__"

// Config for the Dagster plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host  string `json:"host" description:"Dagster webserver URL, for example http://localhost:3000" validate:"required,url"`
	Token string `json:"token,omitempty" description:"Dagster+ API token" sensitive:"true"`

	VerifySSL bool `json:"verify_ssl" label:"Verify SSL" description:"Check the server's TLS certificate" default:"true"`

	IncludeOps        bool `json:"include_ops" description:"Discover the ops inside each job as Task assets" default:"true"`
	IncludeAssets     bool `json:"include_assets" description:"Discover software-defined assets as Dataset assets" default:"true"`
	IncludeRunHistory bool `json:"include_run_history" description:"Collect recent runs of each job" default:"true"`
	RunHistoryLimit   int  `json:"run_history_limit" description:"How many recent runs to read per job" default:"10" validate:"min=1,max=100"`

	CodeLocations []string `json:"code_locations,omitempty" description:"Only discover these code locations. Empty means all of them"`
}

// Example configuration for the plugin
var _ = `
host: "http://localhost:3000"
token: "${DAGSTER_CLOUD_API_TOKEN}"
verify_ssl: true
include_ops: true
include_assets: true
include_run_history: true
run_history_limit: 10
code_locations:
  - "analytics"
filter:
  include:
    - "^orders_.*"
  exclude:
    - ".*_test$"
tags:
  - "dagster"
  - "orchestration"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "dagster",
		Name:        "Dagster",
		Description: "Discover jobs, ops and software-defined assets from Dagster",
		Icon:        "dagster",
		Category:    "orchestration",
		Status:      "experimental",
		// Discover emits CONTAINS, DEPENDS_ON, FEEDS and PRODUCES edges and
		// converts job runs into run history, so all three are declared.
		Features:   []string{"Assets", "Lineage", "Run History"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Dagster plugin.
type Source struct {
	config *Config
	client *Client
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// The GraphQL endpoint is appended to the host, so a trailing slash would
	// produce a double slash that some proxies reject.
	config.Host = strings.TrimSuffix(config.Host, "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Dagster jobs, ops and software-defined assets.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	s.client = NewClient(ClientConfig{
		Host:      s.config.Host,
		Token:     s.config.Token,
		VerifySSL: s.config.VerifySSL,
	})

	repositories, err := s.client.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing repositories: %w", err)
	}

	repositories = s.selectedRepositories(repositories)
	if len(repositories) == 0 {
		log.Warn().Strs("code_locations", s.config.CodeLocations).Msg("No Dagster code locations matched")
	}

	names := pipelineNames(repositories)

	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	for _, repo := range repositories {
		jobAssets, jobLineage, jobRuns := s.discoverJobs(ctx, repo, names)
		assets = append(assets, jobAssets...)
		lineage = append(lineage, jobLineage...)
		runHistory = append(runHistory, jobRuns...)

		if !s.config.IncludeAssets {
			continue
		}

		nodes, err := s.client.RepositoryAssetNodes(ctx, repo)
		if err != nil {
			log.Warn().Err(err).Str("code_location", repo.Location.Name).Msg("Failed to discover software-defined assets")
			continue
		}

		datasetAssets, datasetLineage := s.discoverDatasets(repo, nodes, names)
		assets = append(assets, datasetAssets...)
		lineage = append(lineage, datasetLineage...)
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineage)).
		Int("run_histories", len(runHistory)).
		Msg("Dagster discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineage,
		RunHistory: runHistory,
	}, nil
}

// selectedRepositories keeps only the repositories whose code location the
// user asked for. An empty code_locations list means every location.
func (s *Source) selectedRepositories(repositories []Repository) []Repository {
	if len(s.config.CodeLocations) == 0 {
		return repositories
	}

	wanted := make(map[string]struct{}, len(s.config.CodeLocations))
	for _, name := range s.config.CodeLocations {
		wanted[name] = struct{}{}
	}

	var selected []Repository
	for _, repo := range repositories {
		if _, ok := wanted[repo.Location.Name]; ok {
			selected = append(selected, repo)
		}
	}
	return selected
}

// discoverJobs turns every job of one repository into a Pipeline asset, its
// Task assets and its run history.
func (s *Source) discoverJobs(ctx context.Context, repo Repository, names map[string]string) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, []pluginsdk.AssetRunHistory) {
	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	for _, job := range repo.Jobs {
		if strings.HasPrefix(job.Name, internalJobPrefix) {
			continue
		}

		name, ok := names[jobKey(repo, job.Name)]
		if !ok {
			continue
		}

		var handles []SolidHandle
		if s.config.IncludeOps {
			fetched, err := s.client.JobOpHandles(ctx, repo, job.Name)
			if err != nil {
				log.Warn().Err(err).Str("job", job.Name).Msg("Failed to read job op graph")
			} else {
				handles = fetched
			}
		}

		var runs []Run
		if s.config.IncludeRunHistory {
			// Runs are filtered by job name only, so two code locations that
			// share a job name see each other's runs. Dagster's runs filter
			// has no code location field, so this is as narrow as it gets.
			fetched, err := s.client.JobRuns(ctx, job.Name, s.config.RunHistoryLimit)
			if err != nil {
				log.Warn().Err(err).Str("job", job.Name).Msg("Failed to read job runs")
			} else {
				runs = fetched
			}
		}

		pipeline := s.pipelineAsset(repo, job, name, handles, runs)
		assets = append(assets, pipeline)

		taskAssets, taskLineage := s.taskAssets(repo, name, handles)
		assets = append(assets, taskAssets...)
		lineage = append(lineage, taskLineage...)

		if history := runHistoryFor(*pipeline.MRN, job.Name, runs); len(history.Runs) > 0 {
			runHistory = append(runHistory, history)
		}
	}

	return assets, lineage, runHistory
}

// pipelineAsset builds the Pipeline asset for one job.
func (s *Source) pipelineAsset(repo Repository, job Job, name string, handles []SolidHandle, runs []Run) pluginsdk.Asset {
	jobURL := fmt.Sprintf("%s/locations/%s/jobs/%s", s.config.Host, repo.Location.Name, job.Name)

	metadata := map[string]any{
		"job_id":        job.ID,
		"repository":    repo.Name,
		"code_location": repo.Location.Name,
		"is_asset_job":  job.IsAssetJob,
		"url":           jobURL,
	}

	if job.Description != nil {
		metadata["description"] = *job.Description
	}
	if tags := tagMap(job.Tags); len(tags) > 0 {
		metadata["tags"] = tags
	}
	if len(job.Schedules) > 0 {
		schedules := make([]map[string]any, 0, len(job.Schedules))
		for _, schedule := range job.Schedules {
			entry := map[string]any{"name": schedule.Name, "cron_schedule": schedule.CronSchedule}
			if schedule.ScheduleState != nil {
				entry["status"] = schedule.ScheduleState.Status
			}
			schedules = append(schedules, entry)
		}
		metadata["schedules"] = schedules
	}
	if len(job.Sensors) > 0 {
		sensors := make([]map[string]any, 0, len(job.Sensors))
		for _, sensor := range job.Sensors {
			entry := map[string]any{"name": sensor.Name}
			if sensor.SensorState != nil {
				entry["status"] = sensor.SensorState.Status
			}
			sensors = append(sensors, entry)
		}
		metadata["sensors"] = sensors
	}
	// op_count comes from the op graph, which is only fetched when the user
	// asked for Task assets.
	if s.config.IncludeOps {
		metadata["op_count"] = len(handles)
	}
	for key, value := range runSummary(runs) {
		metadata[key] = value
	}

	metadata = cleanMetadata(metadata)
	mrnValue := assetMRN("Pipeline", name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Pipeline",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Dagster",
			URL:  jobURL,
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if job.Description != nil && *job.Description != "" {
		asset.Description = job.Description
	}

	return asset
}

// taskAssets builds a Task asset per op and the edges around it: the job
// contains every op, and an op depends on the ops feeding its inputs.
func (s *Source) taskAssets(repo Repository, pipelineName string, handles []SolidHandle) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge

	pipelineMRN := assetMRN("Pipeline", pipelineName)

	// An op is addressed by its handle, so upstream names from dependsOn are
	// resolved through this map rather than assumed to be top level handles.
	handleByOp := make(map[string]string, len(handles))
	for _, handle := range handles {
		handleByOp[handle.Solid.Name] = handle.HandleID
	}

	for _, handle := range handles {
		name := taskName(pipelineName, handle.HandleID)
		taskURL := fmt.Sprintf("%s/locations/%s/jobs/%s/%s", s.config.Host, repo.Location.Name, splitPipelineName(pipelineName), handle.HandleID)

		metadata := map[string]any{
			"op":           handle.Solid.Definition.Name,
			"handle_id":    handle.HandleID,
			"job":          pipelineName,
			"input_count":  len(handle.Solid.Inputs),
			"output_count": len(handle.Solid.Outputs),
			"url":          taskURL,
		}
		if handle.Solid.Definition.Description != nil {
			metadata["description"] = *handle.Solid.Definition.Description
		}

		metadata = cleanMetadata(metadata)
		mrnValue := assetMRN("Task", name)

		asset := pluginsdk.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      "Task",
			Providers: []string{provider},
			Metadata:  metadata,
			Schema:    make(map[string]string),
			Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
			ExternalLinks: []pluginsdk.AssetExternalLink{{
				Name: "Open in Dagster",
				URL:  taskURL,
			}},
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}
		if handle.Solid.Definition.Description != nil && *handle.Solid.Definition.Description != "" {
			asset.Description = handle.Solid.Definition.Description
		}

		assets = append(assets, asset)

		lineage = append(lineage, pluginsdk.LineageEdge{
			Source: pipelineMRN,
			Target: mrnValue,
			Type:   "CONTAINS",
		})

		seen := make(map[string]struct{})
		for _, input := range handle.Solid.Inputs {
			for _, dependency := range input.DependsOn {
				upstreamHandle, ok := handleByOp[dependency.Solid.Name]
				if !ok {
					continue
				}
				upstreamMRN := assetMRN("Task", taskName(pipelineName, upstreamHandle))
				if upstreamMRN == mrnValue {
					continue
				}
				if _, done := seen[upstreamMRN]; done {
					continue
				}
				seen[upstreamMRN] = struct{}{}

				lineage = append(lineage, pluginsdk.LineageEdge{
					Source: upstreamMRN,
					Target: mrnValue,
					Type:   "DEPENDS_ON",
				})
			}
		}
	}

	return assets, lineage
}

// discoverDatasets turns software-defined assets into Dataset assets, links
// them to each other and to the jobs that materialise them, and points at the
// warehouse table an asset writes when it names one.
func (s *Source) discoverDatasets(repo Repository, nodes []AssetNode, names map[string]string) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge

	// Only assets declared in this repository become Marmot assets, so an
	// upstream dependency on an asset from another location is dropped rather
	// than turned into an edge the server would discard.
	known := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		known[datasetName(node.AssetKey)] = struct{}{}
	}

	for _, node := range nodes {
		name := datasetName(node.AssetKey)
		assetURL := fmt.Sprintf("%s/assets/%s", s.config.Host, name)

		metadata := map[string]any{
			"asset_key":      node.AssetKey.Path,
			"code_location":  repo.Location.Name,
			"repository":     repo.Name,
			"is_partitioned": node.IsPartitioned,
			"url":            assetURL,
		}

		// Dagster's own metadata entries are written first so the fields this
		// plugin documents always win a name clash.
		var columns []pluginsdk.Column
		for _, entry := range node.MetadataEntries {
			if entry.Typename == "TableSchemaMetadataEntry" {
				columns = append(columns, tableSchemaColumns(entry)...)
				continue
			}
			key, value, ok := flattenMetadataEntry(entry)
			if !ok {
				continue
			}
			metadata[key] = value
		}

		if node.Description != nil {
			metadata["description"] = *node.Description
		}
		if node.ComputeKind != nil {
			metadata["compute_kind"] = *node.ComputeKind
		}
		if node.GroupName != nil {
			metadata["group"] = *node.GroupName
		}
		if len(node.OpNames) > 0 {
			metadata["op_names"] = node.OpNames
		}
		if jobs := userJobNames(node.JobNames); len(jobs) > 0 {
			metadata["job_names"] = jobs
		}
		if len(node.AssetMaterializations) > 0 {
			latest := node.AssetMaterializations[0]
			if at, ok := millisToTime(latest.Timestamp); ok {
				metadata["last_materialized_at"] = at.UTC().Format(time.RFC3339)
			}
			if latest.RunID != "" {
				metadata["last_materialization_run"] = latest.RunID
			}
		}

		metadata = cleanMetadata(metadata)
		mrnValue := assetMRN("Dataset", name)

		asset := pluginsdk.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      "Dataset",
			Providers: []string{provider},
			Metadata:  metadata,
			Schema:    make(map[string]string),
			Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
			ExternalLinks: []pluginsdk.AssetExternalLink{{
				Name: "Open in Dagster",
				URL:  assetURL,
			}},
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}
		if node.Description != nil && *node.Description != "" {
			asset.Description = node.Description
		}
		if len(columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("asset", name).Msg("Failed to attach columns")
			}
		}

		assets = append(assets, asset)

		for _, dependency := range node.Dependencies {
			upstream := datasetName(dependency.Asset.AssetKey)
			if _, ok := known[upstream]; !ok {
				continue
			}
			if upstream == name {
				continue
			}
			lineage = append(lineage, pluginsdk.LineageEdge{
				Source: assetMRN("Dataset", upstream),
				Target: mrnValue,
				Type:   "FEEDS",
			})
		}

		for _, jobName := range node.JobNames {
			pipelineName, ok := names[jobKey(repo, jobName)]
			if !ok {
				continue
			}
			lineage = append(lineage, pluginsdk.LineageEdge{
				Source: assetMRN("Pipeline", pipelineName),
				Target: mrnValue,
				Type:   "PRODUCES",
			})
		}

		if target, ok := warehouseTarget(node); ok {
			lineage = append(lineage, pluginsdk.LineageEdge{
				Source: mrnValue,
				Target: mrn.New("Table", target.Provider, target.Name),
				Type:   "PRODUCES",
			})
		}
	}

	return assets, lineage
}

// WarehouseTable is the native table a software-defined asset writes to.
type WarehouseTable struct {
	Provider string
	Name     string
}

// warehouseProviders maps a Dagster compute kind onto the provider string the
// matching Marmot plugin files its tables under. Only warehouses whose table
// naming is unambiguous are listed; anything else stays unlinked rather than
// producing an edge that points at nothing.
var warehouseProviders = map[string]string{
	"snowflake":  "Snowflake",
	"bigquery":   "BigQuery",
	"postgres":   "PostgreSQL",
	"postgresql": "PostgreSQL",
	"duckdb":     "DuckDB",
}

// warehouseTarget resolves the warehouse table an asset writes to, when the
// asset names one. It needs both a compute kind naming a known warehouse and a
// full database/schema/table triple, so an asset that only half describes its
// destination is left alone.
func warehouseTarget(node AssetNode) (WarehouseTable, bool) {
	if node.ComputeKind == nil {
		return WarehouseTable{}, false
	}
	target, ok := warehouseProviders[strings.ToLower(*node.ComputeKind)]
	if !ok {
		return WarehouseTable{}, false
	}

	database, schema, table, ok := tableTriple(node)
	if !ok {
		return WarehouseTable{}, false
	}

	// Each plugin names its tables its own way, and an edge only lands if the
	// name matches exactly.
	switch target {
	case "Snowflake":
		return WarehouseTable{Provider: target, Name: fmt.Sprintf("%s.%s.%s", database, schema, table)}, true
	default:
		return WarehouseTable{Provider: target, Name: table}, true
	}
}

// tableTriple reads the database, schema and table an asset writes to. Dagster
// has no single standard for this, so three explicit forms are accepted in
// order of how directly they state it.
func tableTriple(node AssetNode) (database, schema, table string, ok bool) {
	text := make(map[string]string)
	for _, entry := range node.MetadataEntries {
		if entry.Typename == "TextMetadataEntry" {
			text[entry.Label] = entry.Text
		}
	}

	if text["database"] != "" && text["schema"] != "" && text["table"] != "" {
		return text["database"], text["schema"], text["table"], true
	}

	// dagster/table_name is the convention the official warehouse integrations
	// write, holding a dotted database.schema.table.
	if parts := strings.Split(text["dagster/table_name"], "."); len(parts) == 3 {
		return parts[0], parts[1], parts[2], true
	}

	if len(node.AssetKey.Path) == 3 {
		p := node.AssetKey.Path
		return p[0], p[1], p[2], true
	}

	return "", "", "", false
}

// runHistoryFor converts a job's runs into run-history events. Every run that
// started emits a START, and a run that is no longer pending emits the event
// its status maps to.
func runHistoryFor(pipelineMRN, jobName string, runs []Run) pluginsdk.AssetRunHistory {
	var events []pluginsdk.RunHistoryEvent

	for _, run := range runs {
		facets := runFacets(run)

		if start, ok := secondsToTime(run.StartTime); ok {
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        run.RunID,
				JobNamespace: "dagster",
				JobName:      jobName,
				EventType:    "START",
				EventTime:    start,
				RunFacets:    facets,
			})
		}

		eventType := mapRunStatusToEventType(run.Status)
		if eventType == "START" {
			continue
		}

		// A finished run has an end time; one still going only has the moment
		// it was last touched.
		end, ok := secondsToTime(run.EndTime)
		if !ok {
			end, ok = secondsToTime(run.UpdateTime)
		}
		if !ok {
			end, ok = secondsToTime(run.StartTime)
		}
		if !ok {
			continue
		}

		events = append(events, pluginsdk.RunHistoryEvent{
			RunID:        run.RunID,
			JobNamespace: "dagster",
			JobName:      jobName,
			EventType:    eventType,
			EventTime:    end,
			RunFacets:    facets,
		})
	}

	return pluginsdk.AssetRunHistory{AssetMRN: pipelineMRN, Runs: events}
}

// mapRunStatusToEventType maps a Dagster run status to the event types Marmot
// records. A run that has not started yet is reported as START so it is not
// mistaken for a finished one.
func mapRunStatusToEventType(status string) string {
	switch status {
	case "SUCCESS":
		return "COMPLETE"
	case "FAILURE":
		return "FAIL"
	case "CANCELED":
		return "ABORT"
	case "STARTED", "STARTING", "CANCELING":
		return "RUNNING"
	case "QUEUED", "NOT_STARTED":
		return "START"
	default:
		return "OTHER"
	}
}

// runFacets records the detail of a run that does not fit the event itself.
func runFacets(run Run) map[string]any {
	facets := map[string]any{
		"status":     run.Status,
		"step_count": len(run.StepStats),
	}

	var succeeded, failed int
	for _, step := range run.StepStats {
		switch step.Status {
		case "SUCCESS":
			succeeded++
		case "FAILURE":
			failed++
		}
	}
	if len(run.StepStats) > 0 {
		facets["step_success_count"] = succeeded
		facets["step_failure_count"] = failed
	}

	if tags := tagMap(run.Tags); len(tags) > 0 {
		facets["tags"] = tags
	}

	return facets
}

// runSummary describes a job's recent runs. It is empty when there are none,
// so a job that has never run carries no misleading zero success rate.
func runSummary(runs []Run) map[string]any {
	if len(runs) == 0 {
		return nil
	}

	summary := map[string]any{"run_count": len(runs)}

	// Dagster returns runs newest first, so the head is the latest one.
	summary["last_run_status"] = runs[0].Status
	if at, ok := secondsToTime(runs[0].StartTime); ok {
		summary["last_run_at"] = at.UTC().Format(time.RFC3339)
	}

	var finished, succeeded int
	for _, run := range runs {
		switch run.Status {
		case "SUCCESS":
			finished++
			succeeded++
		case "FAILURE", "CANCELED":
			finished++
		}
	}
	if finished > 0 {
		rate := float64(succeeded) / float64(finished) * 100
		summary["success_rate"] = math.Round(rate*100) / 100
	}

	return summary
}

// pipelineNames resolves the display name of every job in a run. A job name is
// normally unique, so the bare name is used. When two code locations define
// the same name, every occurrence after the first is qualified with its
// location so the pipelines stay distinct assets.
func pipelineNames(repositories []Repository) map[string]string {
	ordered := make([]Repository, len(repositories))
	copy(ordered, repositories)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Location.Name != ordered[j].Location.Name {
			return ordered[i].Location.Name < ordered[j].Location.Name
		}
		return ordered[i].Name < ordered[j].Name
	})

	seen := make(map[string]int)
	names := make(map[string]string)

	for _, repo := range ordered {
		jobs := make([]Job, len(repo.Jobs))
		copy(jobs, repo.Jobs)
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })

		for _, job := range jobs {
			if strings.HasPrefix(job.Name, internalJobPrefix) {
				continue
			}
			seen[job.Name]++
			if seen[job.Name] == 1 {
				names[jobKey(repo, job.Name)] = job.Name
				continue
			}
			names[jobKey(repo, job.Name)] = fmt.Sprintf("%s (%s)", job.Name, repo.Location.Name)
		}
	}

	return names
}

// jobKey identifies one job across every code location in a run.
func jobKey(repo Repository, jobName string) string {
	return repo.Location.Name + "\x00" + repo.Name + "\x00" + jobName
}

// taskName addresses an op by the job it belongs to and its handle, which is
// unique within that job even for ops nested inside a graph.
func taskName(pipelineName, handleID string) string {
	return pipelineName + "/" + handleID
}

// splitPipelineName recovers the Dagster job name from a pipeline name that
// was qualified with its code location, so deep links stay correct.
func splitPipelineName(pipelineName string) string {
	if idx := strings.LastIndex(pipelineName, " ("); idx > 0 && strings.HasSuffix(pipelineName, ")") {
		return pipelineName[:idx]
	}
	return pipelineName
}

// datasetName joins an asset key into the slash separated form Dagster shows
// in its own UI.
func datasetName(key AssetKey) string {
	return strings.Join(key.Path, "/")
}

// userJobNames drops Dagster's implicit asset job from a list of job names.
func userJobNames(jobNames []string) []string {
	var names []string
	for _, name := range jobNames {
		if strings.HasPrefix(name, internalJobPrefix) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// flattenMetadataEntry turns one Dagster metadata entry into a metadata key
// and value. Entry kinds with no plain representation are skipped.
func flattenMetadataEntry(entry MetadataEntry) (string, any, bool) {
	key := metadataKey(entry.Label)
	if key == "" {
		return "", nil, false
	}

	switch entry.Typename {
	case "TextMetadataEntry":
		return key, entry.Text, entry.Text != ""
	case "UrlMetadataEntry":
		return key, entry.URL, entry.URL != ""
	case "PathMetadataEntry":
		return key, entry.Path, entry.Path != ""
	case "JsonMetadataEntry":
		return key, entry.JSONString, entry.JSONString != ""
	default:
		return "", nil, false
	}
}

// metadataKey turns a Dagster label into a metadata key. Labels are free text
// and often namespaced with a slash, as in dagster/table_name.
func metadataKey(label string) string {
	key := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, label)

	return strings.Trim(key, "_")
}

// tableSchemaColumns converts a Dagster TableSchema entry into Marmot columns.
func tableSchemaColumns(entry MetadataEntry) []pluginsdk.Column {
	if entry.Schema == nil {
		return nil
	}

	columns := make([]pluginsdk.Column, 0, len(entry.Schema.Columns))
	for _, column := range entry.Schema.Columns {
		converted := pluginsdk.Column{
			Name:     column.Name,
			DataType: column.Type,
		}
		if column.Description != nil {
			converted.Description = *column.Description
		}
		if column.Constraints != nil {
			converted.Nullable = column.Constraints.Nullable
		}
		columns = append(columns, converted)
	}

	return columns
}

// tagMap turns a Dagster tag list into a map, dropping the hidden tags
// Dagster prefixes with a dot for its own bookkeeping.
func tagMap(tags []Tag) map[string]string {
	result := make(map[string]string, len(tags))
	for _, tag := range tags {
		if tag.Key == "" || strings.HasPrefix(tag.Key, ".") {
			continue
		}
		result[tag.Key] = tag.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// secondsToTime converts a Dagster epoch-seconds timestamp, which is null
// while a run has not reached that point yet.
func secondsToTime(value *float64) (time.Time, bool) {
	if value == nil || *value <= 0 {
		return time.Time{}, false
	}
	seconds, fraction := math.Modf(*value)
	return time.Unix(int64(seconds), int64(fraction*float64(time.Second))), true
}

// millisToTime converts a materialization timestamp, which Dagster serialises
// as epoch milliseconds inside a string.
func millisToTime(value string) (time.Time, bool) {
	millis, err := strconv.ParseInt(value, 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis), true
}

// cleanMetadata removes empty values so an asset card shows only fields that
// carry information.
func cleanMetadata(metadata map[string]any) map[string]any {
	cleaned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if v == "" {
				continue
			}
		case []string:
			if len(v) == 0 {
				continue
			}
		case map[string]string:
			if len(v) == 0 {
				continue
			}
		case []map[string]any:
			if len(v) == 0 {
				continue
			}
		}
		cleaned[key] = value
	}
	return cleaned
}

// assetMRN is the single place a Dagster MRN is built. Asset creation and
// every lineage edge go through it so the two can never drift into addressing
// the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
