// Package spline discovers Spark applications and their table level lineage
// from a Spline server.
package spline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string Marmot stores for Spline assets.
const provider = "Spline"

// maxAttributesPerPlan caps the per-column lineage lookups, which cost one
// request each, so a wide table cannot turn one Spark job into thousands of
// calls.
const maxAttributesPerPlan = 200

// maxRecentIDs is how many recent Spark application ids and execution plan ids
// are kept in a pipeline's metadata.
const maxRecentIDs = 10

// Config for the Spline plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host   string `json:"host" description:"Spline REST gateway URL, for example http://spline:8080" validate:"required,url"`
	UIHost string `json:"ui_host,omitempty" label:"UI Host" description:"Spline UI URL, used for links back to a run" validate:"omitempty,url"`

	Username string `json:"username,omitempty" description:"Username for basic authentication"`
	Password string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	Token    string `json:"token,omitempty" description:"Bearer token, as an alternative to basic authentication" sensitive:"true"`

	VerifySSL bool `json:"verify_ssl" label:"Verify SSL" description:"Verify the server's TLS certificate" default:"true"`

	Days      int `json:"days" description:"Only ingest Spark runs from the last N days" default:"7" validate:"omitempty,min=1"`
	MaxEvents int `json:"max_events" description:"Maximum number of Spark runs to read" default:"1000" validate:"omitempty,min=1"`
	PageSize  int `json:"page_size" description:"Number of runs to read per request" default:"100" validate:"omitempty,min=1,max=1000"`

	IncludeColumnLineage bool `json:"include_column_lineage" description:"Read column level lineage for each run" default:"true"`
}

var _ = `
host: "http://spline.internal:8080"
ui_host: "http://spline.internal:9090"
days: 7
max_events: 1000
include_column_lineage: true
tags:
  - "spline"
  - "spark"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "spline",
		Name:        "Spline",
		Description: "Discover Spark applications and their lineage from a Spline server",
		Icon:        "spark",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Spline plugin.
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

	config.Host = normaliseHost(config.Host)
	config.UIHost = strings.TrimSuffix(config.UIHost, "/")

	if config.Token != "" && config.Username != "" {
		return nil, fmt.Errorf("token and username are mutually exclusive: set one or the other")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// normaliseHost turns whatever root the user pasted into the gateway root the
// client appends its own paths to. People routinely copy the URL of the
// consumer or producer API out of their Spline deployment, and both sit one
// level below the root the plugin needs.
func normaliseHost(host string) string {
	host = strings.TrimSuffix(strings.TrimSpace(host), "/")
	for _, suffix := range []string{"/consumer", "/producer"} {
		if strings.HasSuffix(strings.ToLower(host), suffix) {
			host = host[:len(host)-len(suffix)]
		}
	}
	return strings.TrimSuffix(host, "/")
}

// Discover reads recent Spark runs from Spline and turns them into one
// Pipeline per application, with run history and lineage to the tables and
// buckets the runs read and wrote.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot rely
	// on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	s.client = NewClient(ClientConfig{
		BaseURL:   s.config.Host,
		Username:  s.config.Username,
		Password:  s.config.Password,
		Token:     s.config.Token,
		VerifySSL: s.config.VerifySSL,
	})

	if version := s.client.ServerVersion(ctx); version != "" {
		log.Debug().Str("version", version).Msg("Connected to Spline")
	}

	end := time.Now()
	start := end.AddDate(0, 0, -s.config.Days)

	events, err := s.client.ListExecutionEvents(ctx, start, end, s.config.PageSize, s.config.MaxEvents)
	if err != nil {
		return nil, fmt.Errorf("listing execution events: %w", err)
	}
	log.Debug().Int("count", len(events)).Int("days", s.config.Days).Msg("Read execution events")

	applications := groupByApplication(events)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	// One execution plan can be replayed by many runs, so plans are fetched
	// once and shared.
	plans := make(map[string]*ExecutionPlanInfo)
	seenEdges := make(map[string]struct{})

	for _, app := range applications {
		pipelineMRN := assetMRN("Pipeline", app.Name)

		columnLineage := make(map[string][]ColumnLineageEntry)
		var inputURIs, outputURIs []string

		for _, planID := range app.PlanIDs {
			plan, ok := plans[planID]
			if !ok {
				detailed, err := s.client.GetLineageDetailed(ctx, planID)
				if err != nil {
					log.Warn().Err(err).Str("plan_id", planID).Msg("Failed to read execution plan, skipping its lineage")
					plans[planID] = nil
					continue
				}
				plan = &detailed.ExecutionPlan
				plans[planID] = plan
			}
			if plan == nil {
				continue
			}

			for _, input := range plan.Inputs {
				if input.Source == "" {
					continue
				}
				inputURIs = appendUnique(inputURIs, input.Source)
				ref, ok := parseDataSourceURI(input.Source)
				if !ok {
					log.Debug().Str("uri", input.Source).Msg("No Marmot asset for input data source")
					continue
				}
				addEdge(&lineages, seenEdges, pluginsdk.LineageEdge{
					Source: nativeMRN(ref),
					Target: pipelineMRN,
					Type:   "FEEDS",
				})
			}

			if plan.Output == nil || plan.Output.Source == "" {
				continue
			}
			outputURIs = appendUnique(outputURIs, plan.Output.Source)

			ref, ok := parseDataSourceURI(plan.Output.Source)
			if !ok {
				log.Debug().Str("uri", plan.Output.Source).Msg("No Marmot asset for output data source")
				continue
			}
			outputMRN := nativeMRN(ref)
			addEdge(&lineages, seenEdges, pluginsdk.LineageEdge{
				Source: pipelineMRN,
				Target: outputMRN,
				Type:   "PRODUCES",
			})

			if s.config.IncludeColumnLineage {
				if entries := s.columnLineage(ctx, plan); len(entries) > 0 {
					columnLineage[outputMRN] = append(columnLineage[outputMRN], entries...)
				}
			}
		}

		assets = append(assets, s.buildPipelineAsset(app, plans, inputURIs, outputURIs, columnLineage))
		runHistory = append(runHistory, buildRunHistory(pipelineMRN, app))
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("run_history", len(runHistory)).
		Msg("Spline discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		RunHistory: runHistory,
	}, nil
}

// application is every run Spline recorded for one Spark application name.
type application struct {
	Name string
	// Events are ordered newest first.
	Events []ExecutionEvent
	// PlanIDs are the distinct execution plans these runs used, newest first.
	PlanIDs []string
}

// groupByApplication collects runs by the Spark application name. Grouping by
// application rather than by run is what keeps a nightly job as one pipeline
// asset instead of a new one for every execution.
func groupByApplication(events []ExecutionEvent) []application {
	// Events arrive newest first and that order is what "last run" and the
	// capped id lists mean, so grouping preserves it.
	sorted := make([]ExecutionEvent, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Timestamp > sorted[j].Timestamp })

	index := make(map[string]int)
	var applications []application

	for _, event := range sorted {
		name := applicationName(event)
		if name == "" {
			log.Warn().Str("event_id", event.ExecutionEventID).Msg("Execution event has no application name or plan id, skipping")
			continue
		}

		position, ok := index[name]
		if !ok {
			index[name] = len(applications)
			applications = append(applications, application{Name: name})
			position = len(applications) - 1
		}

		app := &applications[position]
		app.Events = append(app.Events, event)
		app.PlanIDs = appendUnique(app.PlanIDs, event.ExecutionPlanID)
	}

	return applications
}

// applicationName is the pipeline's identity. Spline fills applicationName
// from the Spark app name; when an agent did not report one the plan id is the
// only stable identity left.
func applicationName(event ExecutionEvent) string {
	if name := strings.TrimSpace(event.ApplicationName); name != "" {
		return name
	}
	if name, ok := event.Extra["appName"].(string); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(event.ExecutionPlanID)
}

func (s *Source) buildPipelineAsset(app application, plans map[string]*ExecutionPlanInfo, inputURIs, outputURIs []string, columnLineage map[string][]ColumnLineageEntry) pluginsdk.Asset {
	latest := app.Events[0]

	metadata := map[string]any{
		"execution_count":   len(app.Events),
		"last_execution_at": time.UnixMilli(latest.Timestamp).UTC().Format(time.RFC3339),
		"last_execution_id": latest.ExecutionEventID,
	}
	setIfNotEmpty(metadata, "framework", latest.FrameworkName)
	setIfNotEmpty(metadata, "last_error", errorMessage(latest.Error))

	if latest.DurationNs != nil {
		metadata["last_duration_ms"] = *latest.DurationNs / int64(time.Millisecond)
	}

	var applicationIDs []string
	for _, event := range app.Events {
		if event.ApplicationID != "" {
			applicationIDs = appendUnique(applicationIDs, event.ApplicationID)
		}
	}
	if len(applicationIDs) > 0 {
		metadata["application_ids"] = capIDs(applicationIDs)
	}
	metadata["execution_plan_ids"] = capIDs(app.PlanIDs)

	if plan := plans[latest.ExecutionPlanID]; plan != nil {
		setIfNotEmpty(metadata, "system_name", plan.SystemInfo.Name)
		setIfNotEmpty(metadata, "system_version", plan.SystemInfo.Version)
		setIfNotEmpty(metadata, "agent_name", plan.AgentInfo.Name)
		setIfNotEmpty(metadata, "agent_version", plan.AgentInfo.Version)
	}

	if len(inputURIs) > 0 {
		metadata["inputs"] = inputURIs
	}
	if len(outputURIs) > 0 {
		metadata["outputs"] = outputURIs
	}

	// Marmot's lineage edges carry no column information, so the column map is
	// stored on the pipeline as JSON keyed by the MRN of the table it produced.
	if len(columnLineage) > 0 {
		if encoded, err := json.Marshal(columnLineage); err == nil {
			metadata["column_lineage"] = string(encoded)
		} else {
			log.Warn().Err(err).Str("application", app.Name).Msg("Failed to encode column lineage")
		}
	}

	var externalLinks []pluginsdk.AssetExternalLink
	if s.config.UIHost != "" {
		runURL := fmt.Sprintf("%s/app/events/overview/%s", s.config.UIHost, latest.ExecutionEventID)
		metadata["url"] = runURL
		externalLinks = append(externalLinks, pluginsdk.AssetExternalLink{Name: "Open in Spline", URL: runURL})
	}

	name := app.Name
	mrnValue := assetMRN("Pipeline", name)

	return pluginsdk.Asset{
		Name:          &name,
		MRN:           &mrnValue,
		Type:          "Pipeline",
		Providers:     []string{provider},
		Metadata:      metadata,
		Tags:          pluginsdk.InterpolateTags(s.config.Tags, metadata),
		ExternalLinks: externalLinks,
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// buildRunHistory turns every execution event of an application into one run,
// so a pipeline keeps the history of its Spark runs instead of only the last.
func buildRunHistory(pipelineMRN string, app application) pluginsdk.AssetRunHistory {
	runs := make([]pluginsdk.RunHistoryEvent, 0, len(app.Events))

	for _, event := range app.Events {
		eventType := "COMPLETE"
		if errorMessage(event.Error) != "" {
			eventType = "FAIL"
		}

		facets := map[string]any{
			"execution_plan_id": event.ExecutionPlanID,
			"append":            event.Append,
		}
		setIfNotEmpty(facets, "application_id", event.ApplicationID)
		setIfNotEmpty(facets, "output", event.DataSourceURI)
		if event.DurationNs != nil {
			facets["duration_ms"] = *event.DurationNs / int64(time.Millisecond)
		}

		runs = append(runs, pluginsdk.RunHistoryEvent{
			RunID:        event.ExecutionEventID,
			JobNamespace: "spline",
			JobName:      app.Name,
			EventType:    eventType,
			EventTime:    time.UnixMilli(event.Timestamp).UTC(),
			RunFacets:    facets,
		})
	}

	return pluginsdk.AssetRunHistory{AssetMRN: pipelineMRN, Runs: runs}
}

// ColumnLineageEntry records which columns one output column was derived from.
type ColumnLineageEntry struct {
	ToColumn    string   `json:"to_column"`
	FromColumns []string `json:"from_columns"`
}

// columnLineage resolves, for every column an execution plan produced, the
// columns it was derived from. Spline answers one column per request, so the
// number of lookups is capped.
func (s *Source) columnLineage(ctx context.Context, plan *ExecutionPlanInfo) []ColumnLineageEntry {
	attributes := plan.Extra.Attributes
	if len(attributes) > maxAttributesPerPlan {
		log.Warn().
			Str("plan_id", plan.ID).
			Int("attributes", len(attributes)).
			Int("limit", maxAttributesPerPlan).
			Msg("Execution plan has more columns than the lookup limit, column lineage is truncated")
		attributes = attributes[:maxAttributesPerPlan]
	}

	var entries []ColumnLineageEntry

	for _, attribute := range attributes {
		if attribute.ID == "" || attribute.Name == "" {
			continue
		}

		result, err := s.client.GetAttributeLineage(ctx, attribute.ID)
		if err != nil {
			log.Warn().Err(err).Str("attribute", attribute.Name).Msg("Failed to read column lineage")
			continue
		}

		sources := sourceColumns(result.Lineage, attribute.ID)
		if len(sources) == 0 {
			// A column read straight from an input has no derivation to record.
			continue
		}
		entries = append(entries, ColumnLineageEntry{ToColumn: attribute.Name, FromColumns: sources})
	}

	return entries
}

// sourceColumns walks the attribute graph from rootID and returns the names of
// every column it depends on. Spline points an edge from a column to the
// column it was derived from, so following edges forward finds its sources.
func sourceColumns(graph AttributeGraph, rootID string) []string {
	names := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		names[node.ID] = node.Name
	}

	outgoing := make(map[string][]string, len(graph.Edges))
	for _, edge := range graph.Edges {
		outgoing[edge.Source] = append(outgoing[edge.Source], edge.Target)
	}

	visited := map[string]struct{}{rootID: {}}
	queue := []string{rootID}
	var sources []string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range outgoing[current] {
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
			if name := names[next]; name != "" {
				sources = appendUnique(sources, name)
			}
		}
	}

	sort.Strings(sources)
	return sources
}

// assetMRN is the single place a Spline MRN is built, so the pipeline assets
// and the lineage edges that point at them can never drift apart.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// nativeMRN builds the MRN of an asset owned by another Marmot plugin, using
// that plugin's own provider string and name shape.
func nativeMRN(ref dataSourceRef) string {
	return mrn.New(ref.Type, ref.Provider, ref.Name)
}

func addEdge(edges *[]pluginsdk.LineageEdge, seen map[string]struct{}, edge pluginsdk.LineageEdge) {
	key := edge.Source + "|" + edge.Target + "|" + edge.Type
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*edges = append(*edges, edge)
}

// errorMessage renders whatever the Spark agent reported as an error. Spline
// stores it as free-form JSON, usually an object with a message.
func errorMessage(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case map[string]any:
		for _, key := range []string{"message", "msg", "error"} {
			if text, ok := typed[key].(string); ok && text != "" {
				return text
			}
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) == "null" || string(encoded) == "{}" {
		return ""
	}
	return string(encoded)
}

func setIfNotEmpty(target map[string]any, key, value string) {
	if value != "" {
		target[key] = value
	}
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func capIDs(ids []string) []string {
	if len(ids) > maxRecentIDs {
		return ids[:maxRecentIDs]
	}
	return ids
}
