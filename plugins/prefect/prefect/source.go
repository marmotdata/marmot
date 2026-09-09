// Package prefect discovers flows, tasks and run history from Prefect
// Cloud or a self-hosted Prefect server.
package prefect

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service name every Prefect asset is filed under.
const provider = "Prefect"

// Config for the Prefect plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host       string `json:"host" description:"Prefect API URL, for example http://localhost:4200/api" validate:"required,url"`
	APIKey     string `json:"api_key" label:"API Key" description:"Prefect Cloud API key" sensitive:"true"`
	AuthString string `json:"auth_string" description:"Self-hosted server credentials, as user:password" sensitive:"true"`

	VerifySSL bool `json:"verify_ssl" label:"Verify SSL" description:"Check the server's TLS certificate" default:"true"`

	IncludeTasks       bool `json:"include_tasks" description:"Discover the tasks of each flow's most recent run" default:"true"`
	IncludeDeployments bool `json:"include_deployments" description:"Read deployments for schedules, tags and descriptions" default:"true"`
	IncludeRunHistory  bool `json:"include_run_history" description:"Record recent flow runs as run history" default:"true"`
	// Not omitempty: Prefect answers a limit of 0 with an empty list, so an
	// explicit 0 would quietly discover nothing instead of failing.
	RunHistoryLimit int `json:"run_history_limit" description:"How many recent runs to read per flow" default:"10" validate:"min=1,max=100"`
}

var _ = `
host: "http://localhost:4200/api"
auth_string: "admin:password"
include_tasks: true
include_run_history: true
run_history_limit: 10
tags:
  - "prefect"
  - "orchestration"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "prefect",
		Name:        "Prefect",
		Description: "Discover flows, tasks and run history from Prefect Cloud or a self-hosted Prefect server",
		Icon:        "prefect",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(PrefectPipelineFields{}, "Pipeline",
				"The metadata fields the Prefect plugin emits for a flow (Pipeline) asset."),
			pluginsdk.AssetSchemaOf(PrefectTaskFields{}, "Task",
				"The metadata fields for a Task asset."),
			pluginsdk.AssetSchemaOf(PrefectRunFacetFields{}, "Run Facet",
				"The facets attached to each run-history event of a Pipeline asset."),
		},
	}
}

// Source represents the Prefect plugin.
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

	host, err := normaliseHost(config.Host)
	if err != nil {
		return nil, err
	}
	config.Host = host

	if config.APIKey != "" && config.AuthString != "" {
		return nil, errors.New("set either api_key (Prefect Cloud) or auth_string (self-hosted), not both")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Prefect flows, their tasks and their recent runs.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	s.client = NewClient(ClientConfig{
		BaseURL:    s.config.Host,
		APIKey:     s.config.APIKey,
		AuthString: s.config.AuthString,
		VerifySSL:  s.config.VerifySSL,
	})

	if err := s.client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("reaching the Prefect API: %w", err)
	}

	flows, err := s.client.ListFlows(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing flows: %w", err)
	}

	log.Debug().Int("count", len(flows)).Msg("Found Prefect flows")

	run := &discovery{source: s}
	for _, flow := range flows {
		run.flow(ctx, flow)
	}

	log.Info().
		Int("assets", len(run.assets)).
		Int("lineages", len(run.lineage)).
		Int("run_history", len(run.runHistory)).
		Msg("Prefect discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     run.assets,
		Lineage:    run.lineage,
		RunHistory: run.runHistory,
	}, nil
}

// discovery accumulates the result of one Discover call.
type discovery struct {
	source     *Source
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	runHistory []pluginsdk.AssetRunHistory

	// assetsAPIMissing is set once a flow run's materializations answer
	// 404. Only Prefect Cloud has an Assets API, and a self-hosted server
	// answers 404 for every flow run, so one 404 is enough to stop asking.
	assetsAPIMissing bool
}

// flow discovers one flow: its Pipeline asset, the tasks of its latest
// run, and its recent runs. A failure reading one flow's detail is logged
// and skipped so the rest of the workspace still lands in the catalog.
func (d *discovery) flow(ctx context.Context, flow Flow) {
	config := d.source.config

	var deployments []Deployment
	if config.IncludeDeployments {
		found, err := d.source.client.ListDeployments(ctx, flow.ID)
		if err != nil {
			log.Warn().Err(err).Str("flow", flow.Name).Msg("Failed to list deployments")
		} else {
			deployments = found
		}
	}

	var runs []FlowRun
	if limit := d.source.runsToFetch(); limit > 0 {
		found, err := d.source.client.ListFlowRuns(ctx, flow.ID, limit)
		if err != nil {
			log.Warn().Err(err).Str("flow", flow.Name).Msg("Failed to list flow runs")
		} else {
			runs = found
		}
	}

	pipeline := d.source.pipelineAsset(flow, deployments, runs)
	d.assets = append(d.assets, pipeline)
	pipelineMRN := *pipeline.MRN

	if config.IncludeTasks && len(runs) > 0 {
		taskRuns, err := d.source.client.ListTaskRuns(ctx, runs[0].ID)
		if err != nil {
			log.Warn().Err(err).Str("flow", flow.Name).Str("flow_run", runs[0].ID).
				Msg("Failed to list task runs")
		} else {
			assets, edges := d.source.taskAssets(flow.Name, pipelineMRN, taskRuns)
			d.assets = append(d.assets, assets...)
			d.lineage = append(d.lineage, edges...)
		}
	}

	if config.IncludeRunHistory && len(runs) > 0 {
		history := d.source.runHistory(pipelineMRN, flow.Name, deployments, runs)
		if len(history.Runs) > 0 {
			d.runHistory = append(d.runHistory, history)
		}
	}

	if len(runs) > 0 && !d.assetsAPIMissing {
		d.lineage = append(d.lineage, d.materializationLineage(ctx, pipelineMRN, runs[0].ID)...)
	}
}

// materializationLineage reads the assets one flow run wrote and turns
// them into edges to the tables and buckets other Marmot plugins own.
func (d *discovery) materializationLineage(ctx context.Context, pipelineMRN, flowRunID string) []pluginsdk.LineageEdge {
	materializations, err := d.source.client.ListAssetMaterializations(ctx, flowRunID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			d.assetsAPIMissing = true
			log.Debug().Msg("No Assets API on this server, skipping materialization lineage")
			return nil
		}
		log.Warn().Err(err).Str("flow_run", flowRunID).Msg("Failed to read asset materializations")
		return nil
	}

	return materializationEdges(pipelineMRN, materializations)
}

// runsToFetch is how many recent runs a flow needs. Tasks are read from
// the latest run, so one run is still fetched when run history is off.
func (s *Source) runsToFetch() int {
	if s.config.IncludeRunHistory {
		return s.config.RunHistoryLimit
	}
	if s.config.IncludeTasks {
		return 1
	}
	return 0
}

// assetMRN builds the MRN for one of this plugin's assets. Every MRN,
// including the ends of lineage edges, goes through here so it always
// matches what the Marmot server rebuilds from Type, Providers[0] and
// Name.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// pipelineAsset builds the Pipeline asset for one flow.
func (s *Source) pipelineAsset(flow Flow, deployments []Deployment, runs []FlowRun) pluginsdk.Asset {
	metadata := map[string]any{
		"flow_id": flow.ID,
	}
	addNonEmpty(metadata, "created", flow.Created)
	addNonEmpty(metadata, "updated", flow.Updated)
	if len(flow.Labels) > 0 {
		metadata["labels"] = flow.Labels
	}

	tags := append([]string{}, flow.Tags...)

	var description *string
	if len(deployments) > 0 {
		// Deployments come back newest first, so the first one is the
		// current shape of the flow: its description and its code path.
		newest := deployments[0]

		metadata["deployment_count"] = len(deployments)
		metadata["deployments"] = deploymentNames(deployments)
		metadata["paused"] = newest.Paused
		addNonEmpty(metadata, "entrypoint", newest.Entrypoint)

		if schedules := activeSchedules(deployments); len(schedules) > 0 {
			metadata["schedules"] = schedules
		}
		if pools := workPools(deployments); len(pools) > 0 {
			metadata["work_pools"] = pools
		}
		if newest.Description != "" {
			description = &newest.Description
		}
		for _, deployment := range deployments {
			tags = append(tags, deployment.Tags...)
		}
	}

	if len(runs) > 0 {
		latest := runs[0]
		addNonEmpty(metadata, "last_run_state", stateOf(latest))
		addNonEmpty(metadata, "last_run_at", firstNonEmpty(latest.StartTime, latest.ExpectedStartTime))
		metadata["run_count"] = len(runs)
		metadata["success_rate"] = successRate(runs)
	}

	tags = unique(tags)
	if len(tags) > 0 {
		metadata["tags"] = tags
	}

	link := flowURL(s.config.Host, flow.ID)
	addNonEmpty(metadata, "url", link)

	name := flow.Name
	mrnValue := assetMRN("Pipeline", name)

	asset := pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Pipeline",
		Providers:   []string{provider},
		Description: description,
		Metadata:    metadata,
		Tags:        append(tags, pluginsdk.InterpolateTags(s.config.Tags, metadata)...),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if link != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in Prefect", URL: link}}
	}

	return asset
}

// discoveredTask is one task of a flow, folded together from every run of
// that task in a single flow run.
type discoveredTask struct {
	Name      string
	Key       string
	LastState string
	LastRunAt string
	RunCount  int
	Tags      []string
}

// taskAssets builds a Task asset per distinct task in one flow run, plus
// the edges that hold the flow together: the Pipeline contains each task,
// and a task that consumed another task's result depends on it.
func (s *Source) taskAssets(flowName, pipelineMRN string, taskRuns []TaskRun) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	tasks, namesByRunID := foldTaskRuns(flowName, taskRuns)

	assets := make([]pluginsdk.Asset, 0, len(tasks))
	edges := make([]pluginsdk.LineageEdge, 0, len(tasks))

	for _, task := range tasks {
		assets = append(assets, s.taskAsset(flowName, task))
		edges = append(edges, pluginsdk.LineageEdge{
			Source: pipelineMRN,
			Target: assetMRN("Task", task.Name),
			Type:   "CONTAINS",
		})
	}

	return assets, append(edges, dependencyEdges(taskRuns, namesByRunID)...)
}

// taskAsset builds the Task asset for one task of a flow.
func (s *Source) taskAsset(flowName string, task discoveredTask) pluginsdk.Asset {
	metadata := map[string]any{
		"task_key":  task.Key,
		"flow":      flowName,
		"run_count": task.RunCount,
	}
	addNonEmpty(metadata, "last_state", task.LastState)
	addNonEmpty(metadata, "last_run_at", task.LastRunAt)
	if len(task.Tags) > 0 {
		metadata["tags"] = task.Tags
	}

	name := task.Name
	mrnValue := assetMRN("Task", name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Task",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      append(append([]string{}, task.Tags...), pluginsdk.InterpolateTags(s.config.Tags, metadata)...),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// runHistory converts a flow's recent runs into run-history events.
func (s *Source) runHistory(pipelineMRN, flowName string, deployments []Deployment, runs []FlowRun) pluginsdk.AssetRunHistory {
	deploymentNames := make(map[string]string, len(deployments))
	for _, deployment := range deployments {
		deploymentNames[deployment.ID] = deployment.Name
	}

	var events []pluginsdk.RunHistoryEvent

	for _, run := range runs {
		facets := map[string]any{
			"run_name":        run.Name,
			"total_run_time":  run.TotalRunTime,
			"parameter_count": len(run.Parameters),
		}
		addNonEmpty(facets, "state_name", run.StateName)
		addNonEmpty(facets, "deployment", deploymentNames[run.DeploymentID])

		started := parseTime(firstNonEmpty(run.StartTime, run.ExpectedStartTime))
		finished := parseTime(run.EndTime)
		if finished.IsZero() && run.State != nil {
			finished = parseTime(run.State.Timestamp)
		}
		if finished.IsZero() {
			finished = started
		}

		if started.IsZero() && finished.IsZero() {
			log.Debug().Str("flow_run", run.ID).Msg("Skipping run with no timestamps")
			continue
		}

		newEvent := func(eventType string, at time.Time) pluginsdk.RunHistoryEvent {
			return pluginsdk.RunHistoryEvent{
				RunID:        run.ID,
				JobNamespace: "prefect",
				JobName:      flowName,
				EventType:    eventType,
				EventTime:    at,
				RunFacets:    facets,
			}
		}

		if !started.IsZero() {
			events = append(events, newEvent("START", started))
		}
		if !finished.IsZero() {
			events = append(events, newEvent(eventTypeForState(stateOf(run)), finished))
		}
	}

	return pluginsdk.AssetRunHistory{AssetMRN: pipelineMRN, Runs: events}
}

// taskKeyHash matches the hash Prefect appends to a task key.
var taskKeyHash = regexp.MustCompile(`-[0-9a-f]{8}$`)

// taskName turns a Prefect task key into the name a person reads.
//
// Prefect writes a task key as "<function name>-<8 hex digits>", where the
// digits are derived from the task's source. Keeping them would rename the
// asset every time somebody edits the task, so they are dropped. Two tasks
// whose functions share a name therefore land on one asset; their task
// keys stay in metadata.
func taskName(flowName, taskKey string) string {
	return flowName + "/" + taskKeyHash.ReplaceAllString(taskKey, "")
}

// foldTaskRuns groups a flow run's task runs into one entry per task,
// and maps each task run id to the task name it belongs to so inputs can
// be resolved. A task called several times in one run contributes one
// task, the way a flow stays one Pipeline across many runs.
func foldTaskRuns(flowName string, taskRuns []TaskRun) ([]discoveredTask, map[string]string) {
	namesByRunID := make(map[string]string, len(taskRuns))
	byName := make(map[string]*discoveredTask, len(taskRuns))
	var order []string

	for _, taskRun := range taskRuns {
		key := firstNonEmpty(taskRun.TaskKey, taskRun.Name, taskRun.ID)
		name := taskName(flowName, key)
		namesByRunID[taskRun.ID] = name

		task, seen := byName[name]
		if !seen {
			task = &discoveredTask{Name: name, Key: key}
			byName[name] = task
			order = append(order, name)
		}

		task.RunCount++
		task.Tags = unique(append(task.Tags, taskRun.Tags...))

		// The API returns task runs newest first, so the first run seen for
		// a task is its most recent one.
		if task.LastState == "" {
			task.LastState = taskRun.StateType
			task.LastRunAt = firstNonEmpty(taskRun.StartTime, taskRun.ExpectedStartTime)
		}
	}

	tasks := make([]discoveredTask, 0, len(order))
	for _, name := range order {
		tasks = append(tasks, *byName[name])
	}

	return tasks, namesByRunID
}

// dependencyEdges reads each task run's inputs and emits an edge from the
// task that produced a value to the task that consumed it.
func dependencyEdges(taskRuns []TaskRun, namesByRunID map[string]string) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, taskRun := range taskRuns {
		target, ok := namesByRunID[taskRun.ID]
		if !ok {
			continue
		}

		for _, refs := range taskRun.TaskInputs {
			for _, ref := range refs {
				// A constant or a flow parameter is also listed as an input,
				// but only another task run carries an id.
				source, ok := namesByRunID[ref.ID]
				if !ok || source == target {
					continue
				}

				pair := source + "->" + target
				if _, done := seen[pair]; done {
					continue
				}
				seen[pair] = struct{}{}

				edges = append(edges, pluginsdk.LineageEdge{
					Source: assetMRN("Task", source),
					Target: assetMRN("Task", target),
					Type:   "DEPENDS_ON",
				})
			}
		}
	}

	return edges
}

// eventTypeForState maps a Prefect run state to the run-history event
// types Marmot stores.
func eventTypeForState(state string) string {
	switch strings.ToUpper(state) {
	case "COMPLETED":
		return "COMPLETE"
	case "FAILED", "CRASHED":
		return "FAIL"
	case "CANCELLED", "CANCELLING":
		return "ABORT"
	case "RUNNING", "PENDING", "PAUSED":
		return "RUNNING"
	default:
		return "OTHER"
	}
}

// stateOf reads a run's state, preferring the flat field and falling back
// to the nested state object.
func stateOf(run FlowRun) string {
	if run.StateType != "" {
		return run.StateType
	}
	if run.State != nil {
		return run.State.Type
	}
	return ""
}

// successRate is the percentage of the fetched runs that completed.
func successRate(runs []FlowRun) float64 {
	if len(runs) == 0 {
		return 0
	}

	completed := 0
	for _, run := range runs {
		if strings.EqualFold(stateOf(run), "COMPLETED") {
			completed++
		}
	}

	return float64(completed) / float64(len(runs)) * 100
}

// deploymentNames lists the deployments of a flow, newest first.
func deploymentNames(deployments []Deployment) []string {
	names := make([]string, 0, len(deployments))
	for _, deployment := range deployments {
		if deployment.Name != "" {
			names = append(names, deployment.Name)
		}
	}
	return names
}

// activeSchedules describes the schedules that will actually fire, as the
// cron, interval or recurrence rule a person would recognise.
func activeSchedules(deployments []Deployment) []string {
	var schedules []string

	for _, deployment := range deployments {
		for _, entry := range deployment.Schedules {
			if !entry.Active || entry.Schedule == nil {
				continue
			}
			switch {
			case entry.Schedule.Cron != "":
				schedules = append(schedules, entry.Schedule.Cron)
			case entry.Schedule.Interval > 0:
				schedules = append(schedules, fmt.Sprintf("every %gs", entry.Schedule.Interval))
			case entry.Schedule.RRule != "":
				schedules = append(schedules, entry.Schedule.RRule)
			}
		}
	}

	return unique(schedules)
}

// workPools lists the work pools the flow's deployments run on.
func workPools(deployments []Deployment) []string {
	pools := make([]string, 0, len(deployments))
	for _, deployment := range deployments {
		if deployment.WorkPoolName != "" {
			pools = append(pools, deployment.WorkPoolName)
		}
	}
	return unique(pools)
}

// cloudWorkspace matches the account and workspace ids in a Prefect Cloud
// API URL.
var cloudWorkspace = regexp.MustCompile(`/accounts/([^/]+)/workspaces/([^/]+)`)

// flowURL is the address of a flow in the Prefect UI. Cloud serves its UI
// on a different host to its API, so the account and workspace ids are
// read back out of the configured API URL.
func flowURL(host, flowID string) string {
	if match := cloudWorkspace.FindStringSubmatch(host); match != nil {
		return fmt.Sprintf("https://app.prefect.cloud/account/%s/workspace/%s/flows/flow/%s",
			match[1], match[2], flowID)
	}

	base := strings.TrimSuffix(host, "/api")
	if base == "" {
		return ""
	}

	return base + "/flows/flow/" + flowID
}

// normaliseHost drops a trailing slash and adds the /api path people
// leave off when they copy the address of the Prefect UI.
func normaliseHost(host string) (string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(host), "/")
	if trimmed == "" {
		// Leave it empty so the required check reports the missing field.
		return "", nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("host is not a URL: %w", err)
	}

	// go-playground's url check passes "localhost:4200", reading the name
	// as the scheme, so the scheme is checked here instead. Without it the
	// failure only surfaces on the first request.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("host must start with http:// or https://, got %q", host)
	}

	if strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "api") {
		return trimmed, nil
	}

	return trimmed + "/api", nil
}

// nameShape says how much of a table's path belongs in its Marmot name.
// Marmot names a PostgreSQL table by the bare table name but a Snowflake
// table by database.schema.table, so an edge has to follow whichever
// shape the owning plugin uses or the server drops it.
type nameShape int

const (
	bareName nameShape = iota
	qualifiedName
)

// assetKeySchemes maps a Prefect asset key scheme to the Marmot provider
// that owns the table and the name shape that provider uses.
var assetKeySchemes = map[string]struct {
	provider string
	shape    nameShape
}{
	"postgres":   {"PostgreSQL", bareName},
	"postgresql": {"PostgreSQL", bareName},
	"mysql":      {"MySQL", bareName},
	"mariadb":    {"MariaDB", bareName},
	"clickhouse": {"ClickHouse", bareName},
	"bigquery":   {"BigQuery", bareName},
	"snowflake":  {"Snowflake", qualifiedName},
	"redshift":   {"Redshift", qualifiedName},
	"databricks": {"Databricks", qualifiedName},
}

// bucketSchemes maps an object store scheme to its Marmot provider.
var bucketSchemes = map[string]struct{ provider, assetType string }{
	"s3":  {"S3", "Bucket"},
	"gs":  {"GCS", "Bucket"},
	"gcs": {"GCS", "Bucket"},
}

// assetKeyTarget is the asset another Marmot plugin owns that a Prefect
// asset key points at.
type assetKeyTarget struct {
	Provider string
	Type     string
	Name     string
}

// MRN is the identity the owning plugin gives this asset.
func (t assetKeyTarget) MRN() string {
	return mrn.New(t.Type, t.Provider, t.Name)
}

// parseAssetKey turns a Prefect asset key URI into the asset it names.
// A scheme this plugin does not know, or a path too short to identify a
// table, is skipped rather than guessed at: a wrong name produces an edge
// the server silently drops.
func parseAssetKey(key string) (assetKeyTarget, bool) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(key), "://")
	if !found || rest == "" {
		return assetKeyTarget{}, false
	}
	scheme = strings.ToLower(scheme)

	if bucket, ok := bucketSchemes[scheme]; ok {
		name, _, _ := strings.Cut(rest, "/")
		if name == "" {
			return assetKeyTarget{}, false
		}
		return assetKeyTarget{Provider: bucket.provider, Type: bucket.assetType, Name: name}, true
	}

	table, ok := assetKeySchemes[scheme]
	if !ok {
		return assetKeyTarget{}, false
	}

	// A database asset key is <scheme>://<host>/<database>/<schema>/<table>,
	// so the first segment is the server and not part of the identity.
	segments := nonEmpty(strings.Split(rest, "/")[1:])
	if len(segments) == 0 {
		return assetKeyTarget{}, false
	}

	if table.shape == qualifiedName {
		if len(segments) != 3 {
			return assetKeyTarget{}, false
		}
		return assetKeyTarget{Provider: table.provider, Type: "Table", Name: strings.Join(segments, ".")}, true
	}

	return assetKeyTarget{Provider: table.provider, Type: "Table", Name: segments[len(segments)-1]}, true
}

// materializationEdges turns the assets a flow run read and wrote into
// lineage: what the run read feeds it, what it wrote it produces.
func materializationEdges(pipelineMRN string, materializations []AssetMaterialization) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	add := func(source, target, edgeType string) {
		pair := source + "->" + target
		if _, done := seen[pair]; done {
			return
		}
		seen[pair] = struct{}{}
		edges = append(edges, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
	}

	for _, materialization := range materializations {
		for _, upstream := range materialization.UpstreamAssets {
			if target, ok := parseAssetKey(upstream); ok {
				add(target.MRN(), pipelineMRN, "FEEDS")
			} else {
				log.Debug().Str("asset_key", upstream).Msg("Skipping upstream asset key Marmot cannot name")
			}
		}

		if target, ok := parseAssetKey(materialization.AssetKey); ok {
			add(pipelineMRN, target.MRN(), "PRODUCES")
		} else if materialization.AssetKey != "" {
			log.Debug().Str("asset_key", materialization.AssetKey).
				Msg("Skipping materialized asset key Marmot cannot name")
		}
	}

	return edges
}

// addNonEmpty sets a metadata key only when there is something to show,
// so the asset card has no blank rows.
func addNonEmpty(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

// firstNonEmpty returns the first value that is set.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// unique removes duplicates and blanks, keeping the original order.
func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, done := seen[value]; done {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

// nonEmpty drops blank path segments.
func nonEmpty(segments []string) []string {
	result := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			result = append(result, segment)
		}
	}
	return result
}

// parseTime reads a Prefect timestamp, returning the zero time when the
// field was absent or unparseable.
func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Debug().Str("value", value).Msg("Ignoring unparseable timestamp")
		return time.Time{}
	}

	return parsed
}
