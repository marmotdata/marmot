// Package airflow ingests metadata from Apache Airflow, including DAGs,
// tasks, and dataset lineage.
package airflow

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Config for the Airflow plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"Airflow webserver URL (e.g., http://localhost:8080)" validate:"required,url"`
	Username string `json:"username,omitempty" description:"Username for basic authentication"`
	Password string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	APIToken string `json:"api_token,omitempty" group:"Authentication" label:"API Token" description:"API token for authentication (alternative to basic auth)" sensitive:"true"`

	DiscoverDAGs     bool `json:"discover_dags" group:"Discovery" label:"Discover DAGs" description:"Discover Airflow DAGs as Pipeline assets" default:"true"`
	DiscoverTasks    bool `json:"discover_tasks" group:"Discovery" description:"Discover tasks within DAGs" default:"true"`
	DiscoverDatasets bool `json:"discover_datasets" group:"Discovery" description:"Discover Airflow Datasets for lineage (requires Airflow 2.4+)" default:"true"`

	IncludeRunHistory bool `json:"include_run_history" group:"Discovery" description:"Include DAG run history in metadata" default:"true"`
	RunHistoryDays    int  `json:"run_history_days" group:"Advanced" description:"Number of days of run history to fetch" default:"7"`

	OnlyActive bool `json:"only_active" group:"Discovery" description:"Only discover active (unpaused) DAGs" default:"true"`
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:            "airflow",
		MarmotVersion: ">=0.13.0",
		Name:          "Apache Airflow",
		Description:   "Ingest metadata from Apache Airflow including DAGs, tasks, and dataset lineage",
		Icon:          "airflow",
		Category:      "orchestration",
		Federates:     true,
		ConfigSpec:    pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(AirflowDAGFields{}, "DAG",
				"The metadata fields Airflow emits for a DAG (Pipeline) asset."),
			pluginsdk.AssetSchemaOf(AirflowTaskFields{}, "Task",
				"The metadata fields for a Task asset."),
			pluginsdk.AssetSchemaOf(AirflowDatasetFields{}, "Dataset",
				"The metadata fields for a Dataset asset."),
			pluginsdk.AssetSchemaOf(AirflowDAGRunFields{}, "DAGRun",
				"The run facet fields emitted for DAG run history events."),
		},
	}
}

// Source implements the Airflow plugin.
type Source struct {
	config *Config
	client *Client
}

// Validate validates and normalizes the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	config.Host = strings.TrimSuffix(config.Host, "/")

	if config.RunHistoryDays <= 0 {
		config.RunHistoryDays = 7
	}

	if config.Username == "" && config.APIToken == "" {
		return nil, fmt.Errorf("authentication required: provide either username/password or api_token")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Airflow DAGs, tasks, and datasets.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config
	s.config.Host = strings.TrimSuffix(s.config.Host, "/")

	s.client = NewClient(ClientConfig{
		BaseURL:  s.config.Host,
		Username: s.config.Username,
		Password: s.config.Password,
		APIToken: s.config.APIToken,
	})

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	if s.config.DiscoverDAGs {
		dagAssets, dagLineages, dagRunHistory, err := s.discoverDAGs(ctx)
		if err != nil {
			return nil, fmt.Errorf("discovering DAGs: %w", err)
		}
		assets = append(assets, dagAssets...)
		lineages = append(lineages, dagLineages...)
		runHistory = append(runHistory, dagRunHistory...)
	}

	if s.config.DiscoverDatasets {
		// Dataset lineage points at the DAG and Task assets discovered
		// above. Those only exist when discover_dags is on, and a paused DAG
		// is filtered out by only_active while datasets still reference it,
		// so the edges have to be checked against what this run emitted.
		created := make(map[string]struct{}, len(assets))
		for _, a := range assets {
			if a.MRN != nil {
				created[*a.MRN] = struct{}{}
			}
		}

		datasetAssets, datasetLineages, err := s.discoverDatasets(ctx, created)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover datasets (requires Airflow 2.4+)")
		} else {
			assets = append(assets, datasetAssets...)
			lineages = append(lineages, datasetLineages...)
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("run_history", len(runHistory)).
		Msg("Airflow discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:       assets,
		Lineage:      lineages,
		RunHistory:   runHistory,
		ZoneBindings: zoneBindings(s.config),
	}, nil
}

// discoverDAGs discovers all DAGs and their tasks.
func (s *Source) discoverDAGs(ctx context.Context) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, []pluginsdk.AssetRunHistory, error) {
	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var allRunHistory []pluginsdk.AssetRunHistory

	dags, err := s.client.ListDAGs(ctx, s.config.OnlyActive)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing DAGs: %w", err)
	}

	log.Debug().Int("count", len(dags)).Msg("Found DAGs")

	for _, dag := range dags {
		dagAsset := s.createDAGAsset(dag)
		assets = append(assets, dagAsset)
		dagMRN := s.dagMRN(dag.DagID)

		var runHistory []DAGRun
		if s.config.IncludeRunHistory {
			runs, err := s.client.ListDAGRuns(ctx, dag.DagID, s.config.RunHistoryDays)
			if err != nil {
				log.Warn().Err(err).Str("dag_id", dag.DagID).Msg("Failed to fetch DAG runs")
			} else {
				runHistory = runs
			}
		}

		if len(runHistory) > 0 {
			s.enrichDAGAssetWithRunHistory(&assets[len(assets)-1], runHistory)
			assetRunHistory := s.convertDAGRunsToRunHistory(dagMRN, dag.DagID, runHistory)
			if len(assetRunHistory.Runs) > 0 {
				allRunHistory = append(allRunHistory, assetRunHistory)
			}
		}

		if s.config.DiscoverTasks {
			taskAssets, taskLineages, err := s.discoverTasks(ctx, dag.DagID)
			if err != nil {
				log.Warn().Err(err).Str("dag_id", dag.DagID).Msg("Failed to discover tasks")
				continue
			}
			assets = append(assets, taskAssets...)
			lineages = append(lineages, taskLineages...)
		}
	}

	return assets, lineages, allRunHistory, nil
}

// discoverTasks discovers tasks within a DAG.
func (s *Source) discoverTasks(ctx context.Context, dagID string) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, error) {
	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

	tasks, err := s.client.ListTasks(ctx, dagID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing tasks for DAG %s: %w", dagID, err)
	}

	taskMRNs := make(map[string]string)
	for _, task := range tasks {
		taskMRNs[task.TaskID] = s.taskMRN(dagID, task.TaskID)
	}

	dagMRN := s.dagMRN(dagID)

	for _, task := range tasks {
		taskAsset := s.createTaskAsset(dagID, task)
		assets = append(assets, taskAsset)

		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: dagMRN,
			Target: taskMRNs[task.TaskID],
			Type:   "CONTAINS",
		})

		for _, downstreamID := range task.DownstreamTaskIDs {
			if downstreamMRN, exists := taskMRNs[downstreamID]; exists {
				lineages = append(lineages, pluginsdk.LineageEdge{
					Source: taskMRNs[task.TaskID],
					Target: downstreamMRN,
					Type:   "DEPENDS_ON",
				})
			}
		}
	}

	return assets, lineages, nil
}

// discoverDatasets discovers Airflow Datasets and creates lineage.
func (s *Source) discoverDatasets(ctx context.Context, created map[string]struct{}) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, error) {
	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

	datasets, err := s.client.ListDatasets(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing datasets: %w", err)
	}

	log.Debug().Int("count", len(datasets)).Msg("Found datasets")

	for _, dataset := range datasets {
		datasetAsset := s.createDatasetAsset(dataset)
		assets = append(assets, datasetAsset)

		datasetMRN := s.datasetMRN(parseDatasetURI(dataset.URI))

		for _, consumer := range dataset.ConsumingDags {
			dagMRN := s.dagMRN(consumer.DagID)
			if _, ok := created[dagMRN]; !ok {
				log.Debug().Str("dag_id", consumer.DagID).Str("dataset", dataset.URI).
					Msg("Skipping FEEDS edge, consuming DAG not discovered")
				continue
			}
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: datasetMRN,
				Target: dagMRN,
				Type:   "FEEDS",
			})
		}

		for _, producer := range dataset.ProducingTasks {
			// Airflow names the producing task, not just its DAG, so the edge
			// starts at the Task asset when this run created one. Falling back
			// to the DAG keeps the edge useful when discover_tasks is off.
			taskMRN := s.taskMRN(producer.DagID, producer.TaskID)
			dagMRN := s.dagMRN(producer.DagID)

			var sourceMRN string
			switch {
			case containsMRN(created, taskMRN):
				sourceMRN = taskMRN
			case containsMRN(created, dagMRN):
				sourceMRN = dagMRN
			default:
				log.Debug().Str("dag_id", producer.DagID).Str("task_id", producer.TaskID).
					Str("dataset", dataset.URI).Msg("Skipping PRODUCES edge, producing DAG not discovered")
				continue
			}

			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: sourceMRN,
				Target: datasetMRN,
				Type:   "PRODUCES",
			})
		}
	}

	return assets, lineages, nil
}

// createDAGAsset creates a Pipeline asset from an Airflow DAG.
func (s *Source) createDAGAsset(dag DAG) pluginsdk.Asset {
	mrnValue := s.dagMRN(dag.DagID)

	var description *string
	if dag.Description != nil && *dag.Description != "" {
		description = dag.Description
	}

	metadata := map[string]interface{}{
		"dag_id":    dag.DagID,
		"file_path": dag.Fileloc,
		"is_paused": dag.IsPaused,
		"is_active": dag.IsActive,
	}

	if len(dag.Owners) > 0 {
		metadata["owners"] = strings.Join(dag.Owners, ", ")
	}

	if dag.ScheduleInterval != nil {
		metadata["schedule_interval"] = dag.ScheduleInterval.Value
	}

	if dag.NextDagRun != nil {
		metadata["next_run_date"] = *dag.NextDagRun
	}

	if dag.LastParsedTime != nil {
		metadata["last_parsed_time"] = *dag.LastParsedTime
	}

	cleanMetadata := s.cleanMetadata(metadata)

	return pluginsdk.Asset{
		Name:        &dag.DagID,
		MRN:         &mrnValue,
		Type:        "Pipeline",
		Providers:   []string{"Airflow"},
		Description: description,
		Metadata:    cleanMetadata,
		Tags:        s.config.Tags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "Airflow",
			LastSyncAt: time.Now(),
			Properties: cleanMetadata,
			Priority:   1,
		}},
	}
}

// enrichDAGAssetWithRunHistory adds run history metadata to a DAG asset.
func (s *Source) enrichDAGAssetWithRunHistory(a *pluginsdk.Asset, runs []DAGRun) {
	if len(runs) == 0 {
		return
	}

	latestRun := runs[0]
	a.Metadata["last_run_state"] = latestRun.State
	a.Metadata["last_run_id"] = latestRun.DagRunID

	if latestRun.ExecutionDate != "" {
		a.Metadata["last_run_date"] = latestRun.ExecutionDate
	}

	successCount := 0
	for _, run := range runs {
		if run.State == "success" {
			successCount++
		}
	}
	if len(runs) > 0 {
		a.Metadata["success_rate"] = float64(successCount) / float64(len(runs)) * 100
		a.Metadata["run_count"] = len(runs)
	}

	if len(a.Sources) > 0 {
		a.Sources[0].Properties = a.Metadata
	}
}

// convertDAGRunsToRunHistory converts Airflow DAG runs to plugin RunHistory events.
func (s *Source) convertDAGRunsToRunHistory(dagMRN, dagID string, runs []DAGRun) pluginsdk.AssetRunHistory {
	var events []pluginsdk.RunHistoryEvent

	for _, run := range runs {
		eventType := mapAirflowStateToEventType(run.State)

		var eventTime time.Time
		if run.StartDate != nil && *run.StartDate != "" {
			if t, err := time.Parse(time.RFC3339, *run.StartDate); err == nil {
				eventTime = t
			}
		}
		if eventTime.IsZero() && run.ExecutionDate != "" {
			if t, err := time.Parse(time.RFC3339, run.ExecutionDate); err == nil {
				eventTime = t
			}
		}
		if eventTime.IsZero() {
			eventTime = time.Now()
		}

		if run.StartDate != nil && *run.StartDate != "" {
			startTime, _ := time.Parse(time.RFC3339, *run.StartDate)
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        run.DagRunID,
				JobNamespace: "airflow",
				JobName:      dagID,
				EventType:    "START",
				EventTime:    startTime,
				RunFacets: map[string]interface{}{
					"run_type":     run.RunType,
					"dag_run_id":   run.DagRunID,
					"dag_id":       dagID,
					"logical_date": run.LogicalDate,
				},
			})
		}

		if eventType != "START" {
			var completionTime time.Time
			if run.EndDate != nil && *run.EndDate != "" {
				completionTime, _ = time.Parse(time.RFC3339, *run.EndDate)
			} else {
				completionTime = eventTime
			}

			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        run.DagRunID,
				JobNamespace: "airflow",
				JobName:      dagID,
				EventType:    eventType,
				EventTime:    completionTime,
				RunFacets: map[string]interface{}{
					"run_type":     run.RunType,
					"dag_run_id":   run.DagRunID,
					"dag_id":       dagID,
					"state":        run.State,
					"logical_date": run.LogicalDate,
				},
			})
		}
	}

	return pluginsdk.AssetRunHistory{
		AssetMRN: dagMRN,
		Runs:     events,
	}
}

// mapAirflowStateToEventType maps Airflow DAG run states to OpenLineage event types.
func mapAirflowStateToEventType(state string) string {
	switch state {
	case "success":
		return "COMPLETE"
	case "failed":
		return "FAIL"
	case "running":
		return "RUNNING"
	case "queued":
		return "START"
	default:
		return "OTHER"
	}
}

// createTaskAsset creates a Task asset from an Airflow task.
func (s *Source) createTaskAsset(dagID string, task Task) pluginsdk.Asset {
	taskName := task.TaskID
	mrnValue := s.taskMRN(dagID, task.TaskID)
	var previousMRNs []string
	if old := mrn.New("Task", "Airflow", fmt.Sprintf("%s.%s", dagID, task.TaskID)); old != mrnValue {
		previousMRNs = []string{old}
	}

	metadata := map[string]interface{}{
		"task_id":       task.TaskID,
		"dag_id":        dagID,
		"operator_name": task.OperatorName,
		"trigger_rule":  task.TriggerRule,
	}

	if task.Retries > 0 {
		metadata["retries"] = task.Retries
	}

	if task.Pool != "" {
		metadata["pool"] = task.Pool
	}

	if len(task.DownstreamTaskIDs) > 0 {
		metadata["downstream_tasks"] = task.DownstreamTaskIDs
	}

	cleanMetadata := s.cleanMetadata(metadata)

	return pluginsdk.Asset{
		Name:         &taskName,
		MRN:          &mrnValue,
		PreviousMRNs: previousMRNs,
		Type:         "Task",
		Providers:    []string{"Airflow"},
		Metadata:     cleanMetadata,
		Tags:         s.config.Tags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "Airflow",
			LastSyncAt: time.Now(),
			Properties: cleanMetadata,
			Priority:   1,
		}},
	}
}

// containsMRN reports whether this run emitted an asset with that MRN.
func containsMRN(created map[string]struct{}, mrnValue string) bool {
	_, ok := created[mrnValue]
	return ok
}

// datasetRef is the asset an Airflow Dataset URI points at. Provider, Type and Path follow the naming table of the plugin that owns the system, so the dataset lands on the asset that plugin catalogues. Host is the host:port the URI names, empty when it carries no server.
type datasetRef struct {
	Provider string
	Type     string
	Path     []string
	Host     string
	// Owned is true when no other plugin could own the dataset, so the asset belongs to Airflow and carries its zone.
	Owned bool
	URI   string
}

// Name is the dataset's own name: the last level of its path.
func (r datasetRef) Name() string { return r.Path[len(r.Path)-1] }

// parseDatasetURI parses an Airflow Dataset URI into the asset it names. The relational schemes follow what Airflow's own providers write, scheme://host:port/database/schema/table for PostgreSQL and Redshift, scheme://host:port/database/table for MySQL and scheme://project/dataset/table or scheme:project.dataset.table for BigQuery. Object storage stays at bucket granularity and Kafka at the topic.
func parseDatasetURI(uri string) datasetRef {
	ref := datasetRef{Provider: "Airflow", Type: "Dataset", Path: []string{uri}, Owned: true, URI: uri}

	scheme, rest, ok := strings.Cut(uri, ":")
	if !ok || scheme == "" {
		return ref
	}
	scheme = strings.ToLower(scheme)
	authority, segments := splitURI(rest)

	switch scheme {
	case "s3", "s3a", "s3n":
		return ownedBy(ref, "S3", "Bucket", nonEmptySegments([]string{authority}), "")
	case "gs", "gcs":
		return ownedBy(ref, "GCS", "Bucket", nonEmptySegments([]string{authority}), "")
	case "kafka":
		topic := segments
		if len(topic) == 0 {
			return ownedBy(ref, "Kafka", "Topic", nil, "")
		}
		return ownedBy(ref, "Kafka", "Topic", []string{topic[len(topic)-1]}, hostWithPort(authority, "9092"))
	case "postgresql", "postgres":
		return ownedBy(ref, "PostgreSQL", "Table", pgPath(segments), hostWithPort(authority, "5432"))
	case "mysql":
		return ownedBy(ref, "MySQL", "Table", lastN(segments, 2), hostWithPort(authority, "3306"))
	case "redshift":
		return ownedBy(ref, "Redshift", "Table", lastN(segments, 3), hostWithPort(authority, "5439"))
	case "snowflake":
		return ownedBy(ref, "Snowflake", "Table", lastN(segments, 3), "")
	case "bigquery", "bq":
		return ownedBy(ref, "BigQuery", "Table", bigQueryPath(authority, segments), "")
	case "http", "https":
		ref.Provider, ref.Type, ref.Owned = "HTTP", "Endpoint", false
		return ref
	case "file":
		ref.Provider, ref.Type, ref.Owned = "File", "File", false
		ref.Path = nonEmptySegments(joinNonEmpty(authority, segments))
		if len(ref.Path) == 0 {
			ref.Path = []string{uri}
		}
		return ref
	default:
		ref.Provider, ref.Owned = strings.ToUpper(scheme[:1])+scheme[1:], false
		ref.Path = nonEmptySegments(joinNonEmpty(authority, segments))
		if len(ref.Path) == 0 {
			ref.Path = []string{uri}
		}
		return ref
	}
}

// ownedBy fills in the system a URI belongs to. A URI that does not hold the levels the system's plugin names assets by stays an Airflow dataset instead of guessing.
func ownedBy(ref datasetRef, provider, assetType string, path []string, host string) datasetRef {
	if len(path) == 0 {
		return ref
	}
	ref.Provider, ref.Type, ref.Path, ref.Host, ref.Owned = provider, assetType, path, host, false
	return ref
}

// splitURI splits what follows the scheme into its authority and path segments. Both scheme://authority/a/b and scheme:a/b are read, with the second having no authority.
func splitURI(rest string) (string, []string) {
	if after, ok := strings.CutPrefix(rest, "//"); ok {
		authority, path, _ := strings.Cut(after, "/")
		path, _, _ = strings.Cut(path, "?")
		return authority, nonEmptySegments(strings.Split(path, "/"))
	}
	rest, _, _ = strings.Cut(rest, "?")
	return "", nonEmptySegments(strings.Split(rest, "/"))
}

func nonEmptySegments(segments []string) []string {
	var out []string
	for _, s := range segments {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func joinNonEmpty(authority string, segments []string) []string {
	return append([]string{authority}, segments...)
}

func lastN(segments []string, n int) []string {
	if len(segments) < n {
		return nil
	}
	return segments[len(segments)-n:]
}

// pgPath reads database/schema/table, or database/table in the default schema.
func pgPath(segments []string) []string {
	switch len(segments) {
	case 2:
		return []string{segments[0], "public", segments[1]}
	case 0, 1:
		return nil
	default:
		return segments[len(segments)-3:]
	}
}

// bigQueryPath reads project/dataset/table or the dotted project.dataset.table. A lone authority is the project of the dotted form.
func bigQueryPath(authority string, segments []string) []string {
	parts := segments
	if authority != "" {
		parts = append([]string{authority}, segments...)
	}
	if len(parts) == 1 {
		parts = strings.Split(parts[0], ".")
	}
	if len(parts) != 3 {
		return nil
	}
	return parts
}

// hostWithPort lowercases the first server of an authority and gives it the default port when it names none. It returns "" when the authority names no server.
func hostWithPort(authority, defaultPort string) string {
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	first, _, _ := strings.Cut(authority, ",")
	first = strings.ToLower(strings.TrimSpace(first))
	if first == "" {
		return ""
	}
	if h, p, err := net.SplitHostPort(first); err == nil {
		return net.JoinHostPort(h, p)
	}
	return net.JoinHostPort(strings.Trim(first, "[]"), defaultPort)
}

// dagMRN is the MRN of a DAG.
func (s *Source) dagMRN(dagID string) string {
	return s.config.AssetMRN("Pipeline", "Airflow", dagID)
}

// taskMRN is the MRN of a task, addressed by its DAG and its task id.
func (s *Source) taskMRN(dagID, taskID string) string {
	return s.config.AssetMRN("Task", "Airflow", dagID, taskID)
}

// datasetMRN is the MRN of the asset a Dataset URI points at. The systems behind most URIs belong to other plugins, so the MRN carries the host the URI names and leaves the zone for Marmot to resolve. Only a dataset that is nothing more than an Airflow dataset takes the plugin's zone.
func (s *Source) datasetMRN(ref datasetRef) string {
	if ref.Owned {
		return s.config.AssetMRN(ref.Type, ref.Provider, ref.Path...)
	}
	return mrn.Asset(ref.Type, ref.Provider, ref.Path...).WithHost(ref.Host).String()
}

// legacyDatasetURI parses an Airflow Dataset URI the way earlier releases did, which is what their MRNs were built from.
func legacyDatasetURI(uri string) (provider, assetType, name string) {
	provider = "Airflow"
	assetType = "Dataset"
	name = uri

	if idx := strings.Index(uri, "://"); idx > 0 {
		scheme := strings.ToLower(uri[:idx])
		path := uri[idx+3:]

		switch scheme {
		case "s3", "s3a", "s3n":
			provider = "S3"
			assetType = "Bucket"
			parts := strings.SplitN(path, "/", 2)
			name = parts[0]
		case "gs", "gcs":
			provider = "GCS"
			assetType = "Bucket"
			parts := strings.SplitN(path, "/", 2)
			name = parts[0]
		case "kafka":
			provider = "Kafka"
			assetType = "Topic"
			parts := strings.Split(path, "/")
			if len(parts) > 1 {
				name = parts[len(parts)-1]
			} else {
				name = path
			}
		case "postgresql", "postgres":
			provider = "PostgreSQL"
			assetType = "Table"
			name = path
		case "mysql":
			provider = "MySQL"
			assetType = "Table"
			name = path
		case "bigquery", "bq":
			provider = "BigQuery"
			assetType = "Table"
			name = path
		case "snowflake":
			provider = "Snowflake"
			assetType = "Table"
			name = path
		case "redshift":
			provider = "Redshift"
			assetType = "Table"
			name = path
		case "http", "https":
			provider = "HTTP"
			assetType = "Endpoint"
			name = uri
		case "file":
			provider = "File"
			assetType = "File"
			name = path
		default:
			provider = strings.ToUpper(scheme[:1]) + scheme[1:]
			name = path
		}
	}

	return provider, assetType, name
}

// createDatasetAsset creates a Dataset asset from an Airflow Dataset.
func (s *Source) createDatasetAsset(dataset Dataset) pluginsdk.Asset {
	ref := parseDatasetURI(dataset.URI)
	mrnValue := s.datasetMRN(ref)
	provider, assetType, name := ref.Provider, ref.Type, ref.Name()
	var previousMRNs []string
	legacyProvider, legacyType, legacyName := legacyDatasetURI(dataset.URI)
	if old := mrn.New(legacyType, legacyProvider, legacyName); old != mrnValue {
		previousMRNs = []string{old}
	}

	metadata := map[string]interface{}{
		"uri":             dataset.URI,
		"airflow_dataset": true,
		"created_at":      dataset.CreatedAt,
		"updated_at":      dataset.UpdatedAt,
		"producer_count":  len(dataset.ProducingTasks),
		"consumer_count":  len(dataset.ConsumingDags),
	}

	for k, v := range dataset.Extra {
		metadata[fmt.Sprintf("extra_%s", k)] = v
	}

	cleanMetadata := s.cleanMetadata(metadata)

	return pluginsdk.Asset{
		Name:         &name,
		MRN:          &mrnValue,
		PreviousMRNs: previousMRNs,
		Type:         assetType,
		Providers:    []string{provider},
		Metadata:     cleanMetadata,
		Tags:         s.config.Tags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "Airflow",
			LastSyncAt: time.Now(),
			Properties: cleanMetadata,
			Priority:   1,
		}},
	}
}

// cleanMetadata removes nil and empty values from metadata.
func (s *Source) cleanMetadata(metadata map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range metadata {
		if v == nil {
			continue
		}
		if str, ok := v.(string); ok && str == "" {
			continue
		}
		if slice, ok := v.([]interface{}); ok && len(slice) == 0 {
			continue
		}
		if m, ok := v.(map[string]interface{}); ok && len(m) == 0 {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

// zoneBindings tells Marmot which zone the webserver this run connected to is.
func zoneBindings(cfg *Config) []pluginsdk.ZoneBinding {
	u, err := url.Parse(cfg.Host)
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
	return []pluginsdk.ZoneBinding{{Provider: "Airflow", Host: net.JoinHostPort(strings.ToLower(u.Hostname()), port), Zone: cfg.Zone}}
}
