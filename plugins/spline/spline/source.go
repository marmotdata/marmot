// Package spline discovers Spark applications and their table level lineage
// from a Spline server.
package spline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	UIHost string `json:"ui_host,omitempty" group:"Advanced" label:"UI Host" description:"Spline UI URL, used for links back to a run" validate:"omitempty,url"`

	Username string `json:"username,omitempty" description:"Username for basic authentication"`
	Password string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	Token    string `json:"token,omitempty" group:"Authentication" description:"Bearer token, as an alternative to basic authentication" sensitive:"true"`

	VerifySSL bool `json:"verify_ssl" group:"Authentication" label:"Verify SSL" description:"Verify the server's TLS certificate" default:"true"`

	Days      int `json:"days" group:"Advanced" description:"Only ingest Spark runs from the last N days" default:"7" validate:"omitempty,min=1"`
	MaxEvents int `json:"max_events" group:"Advanced" description:"Maximum number of Spark runs to read" default:"1000" validate:"omitempty,min=1"`
	PageSize  int `json:"page_size" group:"Advanced" description:"Number of runs to read per request" default:"100" validate:"omitempty,min=1,max=1000"`

	IncludeColumnLineage bool `json:"include_column_lineage" group:"Discovery" description:"Read column level lineage for each run" default:"true"`
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
		ID:            "spline",
		MarmotVersion: ">=0.13.0",
		Name:          "Spline",
		Description:   "Discover Spark applications and their lineage from a Spline server",
		Icon:          "spark",
		Category:      "orchestration",
		Status:        "experimental",
		Features:      []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:    pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(SplineFields{}, "Spline",
				"The metadata fields the Spline plugin emits for Pipeline assets."),
			pluginsdk.AssetSchemaOf(SplineRunFacetFields{}, "Run Facet",
				"The facets attached to each run history event."),
		},
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

	// One execution plan can be replayed by many runs, so plans and their
	// column lineage are fetched once and shared.
	plans := make(map[string]*LineageDetailed)
	planColumns := make(map[string]map[string][]pluginsdk.ColumnEdge)
	seenEdges := make(map[string]struct{})
	columnEdges := make(map[string]int)

	for _, app := range applications {
		pipelineMRN := s.config.AssetMRN("Pipeline", provider, app.Name)

		var inputURIs, outputURIs []string

		for _, planID := range app.PlanIDs {
			detailed, ok := plans[planID]
			if !ok {
				var err error
				detailed, err = s.client.GetLineageDetailed(ctx, planID)
				if err != nil {
					log.Warn().Err(err).Str("plan_id", planID).Msg("Failed to read execution plan, skipping its lineage")
					plans[planID] = nil
					continue
				}
				plans[planID] = detailed
			}
			if detailed == nil {
				continue
			}
			plan := &detailed.ExecutionPlan

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
				byInput, ok := planColumns[planID]
				if !ok {
					byInput = s.columnLineage(ctx, detailed)
					planColumns[planID] = byInput
				}
				for _, inputMRN := range sortedKeys(byInput) {
					if inputMRN == outputMRN {
						continue
					}
					addColumnEdge(&lineages, columnEdges, pluginsdk.LineageEdge{
						Source:        inputMRN,
						Target:        outputMRN,
						Type:          "DEPENDS_ON",
						JobMRN:        pipelineMRN,
						ColumnLineage: byInput[inputMRN],
					})
				}
			}
		}

		assets = append(assets, s.buildPipelineAsset(app, plans, inputURIs, outputURIs))
		runHistory = append(runHistory, buildRunHistory(pipelineMRN, app))
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("run_history", len(runHistory)).
		Msg("Spline discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:       assets,
		Lineage:      lineages,
		RunHistory:   runHistory,
		ZoneBindings: s.zoneBindings(),
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

func (s *Source) buildPipelineAsset(app application, plans map[string]*LineageDetailed, inputURIs, outputURIs []string) pluginsdk.Asset {
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

	if detailed := plans[latest.ExecutionPlanID]; detailed != nil {
		plan := detailed.ExecutionPlan
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

	var externalLinks []pluginsdk.AssetExternalLink
	if s.config.UIHost != "" {
		runURL := fmt.Sprintf("%s/app/events/overview/%s", s.config.UIHost, latest.ExecutionEventID)
		metadata["url"] = runURL
		externalLinks = append(externalLinks, pluginsdk.AssetExternalLink{Name: "Open in Spline", URL: runURL})
	}

	name := app.Name
	mrnValue := s.config.AssetMRN("Pipeline", provider, name)

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

// nativeMRN builds the MRN of an asset owned by another Marmot plugin, using that plugin's own provider and path. The host in the URI only hints at the owning plugin's zone, which Marmot resolves from the zone bindings that plugin reports.
func nativeMRN(ref dataSourceRef) string {
	return mrn.Asset(ref.Type, ref.Provider, ref.Path...).WithHost(ref.Host).String()
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

// zoneBindings names the Spline gateway host so other plugins can resolve it to this zone.
func (s *Source) zoneBindings() []pluginsdk.ZoneBinding {
	u, err := url.Parse(s.config.Host)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return []pluginsdk.ZoneBinding{{
		Provider: provider,
		Host:     strings.ToLower(u.Hostname()) + ":" + port,
		Zone:     s.config.Zone,
	}}
}
