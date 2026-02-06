// +marmot:name=Airflow
// +marmot:description=Ingests metadata from Apache Airflow including DAGs, tasks, and dataset lineage.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage, Run History
package airflow

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for Airflow plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"Airflow webserver URL (e.g., http://localhost:8080)" validate:"required,url"`
	Username string `json:"username,omitempty" description:"Username for basic authentication"`
	Password string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	APIToken string `json:"api_token,omitempty" description:"API token for authentication (alternative to basic auth)" sensitive:"true"`

	DiscoverDAGs     bool `json:"discover_dags" description:"Discover Airflow DAGs as Pipeline assets" default:"true"`
	DiscoverTasks    bool `json:"discover_tasks" description:"Discover tasks within DAGs" default:"true"`
	DiscoverDatasets bool `json:"discover_datasets" description:"Discover Airflow Datasets for lineage (requires Airflow 2.4+)" default:"true"`

	IncludeRunHistory bool `json:"include_run_history" description:"Include DAG run history in metadata" default:"true"`
	RunHistoryDays    int  `json:"run_history_days" description:"Number of days of run history to fetch" default:"7"`

	OnlyActive bool `json:"only_active" description:"Only discover active (unpaused) DAGs" default:"true"`
}

// +marmot:example-config
var _ = `
host: "http://localhost:8080"
username: "admin"
password: "${AIRFLOW_PASSWORD}"
discover_dags: true
discover_tasks: true
discover_datasets: true
include_run_history: true
run_history_days: 7
only_active: true
filter:
  include:
    - "^analytics_.*"
  exclude:
    - ".*_test$"
tags:
  - "airflow"
  - "orchestration"
`

// Source implements the Airflow plugin.
type Source struct {
	config *Config
	client *Client
}

// Validate validates and normalizes the plugin configuration.
func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
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

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Airflow DAGs, tasks, and datasets.
func (s *Source) Discover(ctx context.Context, rawConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
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

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var runHistory []plugin.AssetRunHistory

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
		datasetAssets, datasetLineages, err := s.discoverDatasets(ctx)
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

	return &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		RunHistory: runHistory,
	}, nil
}

// discoverDAGs discovers all DAGs and their tasks.
func (s *Source) discoverDAGs(ctx context.Context) ([]asset.Asset, []lineage.LineageEdge, []plugin.AssetRunHistory, error) {
	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var allRunHistory []plugin.AssetRunHistory

	dags, err := s.client.ListDAGs(ctx, s.config.OnlyActive)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing DAGs: %w", err)
	}

	log.Debug().Int("count", len(dags)).Msg("Found DAGs")

	for _, dag := range dags {
		dagAsset := s.createDAGAsset(dag)
		assets = append(assets, dagAsset)
		dagMRN := mrn.New("Pipeline", "Airflow", dag.DagID)

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
func (s *Source) discoverTasks(ctx context.Context, dagID string) ([]asset.Asset, []lineage.LineageEdge, error) {
	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	tasks, err := s.client.ListTasks(ctx, dagID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing tasks for DAG %s: %w", dagID, err)
	}

	taskMRNs := make(map[string]string)
	for _, task := range tasks {
		taskMRN := mrn.New("Task", "Airflow", fmt.Sprintf("%s.%s", dagID, task.TaskID))
		taskMRNs[task.TaskID] = taskMRN
	}

	dagMRN := mrn.New("Pipeline", "Airflow", dagID)

	for _, task := range tasks {
		taskAsset := s.createTaskAsset(dagID, task)
		assets = append(assets, taskAsset)

		lineages = append(lineages, lineage.LineageEdge{
			Source: dagMRN,
			Target: taskMRNs[task.TaskID],
			Type:   "CONTAINS",
		})

		for _, downstreamID := range task.DownstreamTaskIDs {
			if downstreamMRN, exists := taskMRNs[downstreamID]; exists {
				lineages = append(lineages, lineage.LineageEdge{
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
func (s *Source) discoverDatasets(ctx context.Context) ([]asset.Asset, []lineage.LineageEdge, error) {
	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	datasets, err := s.client.ListDatasets(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing datasets: %w", err)
	}

	log.Debug().Int("count", len(datasets)).Msg("Found datasets")

	for _, dataset := range datasets {
		datasetAsset := s.createDatasetAsset(dataset)
		assets = append(assets, datasetAsset)

		provider, assetType, name := parseDatasetURI(dataset.URI)
		datasetMRN := mrn.New(assetType, provider, name)

		for _, consumer := range dataset.ConsumingDags {
			dagMRN := mrn.New("Pipeline", "Airflow", consumer.DagID)
			lineages = append(lineages, lineage.LineageEdge{
				Source: datasetMRN,
				Target: dagMRN,
				Type:   "FEEDS",
			})
		}

		for _, producer := range dataset.ProducingTasks {
			dagMRN := mrn.New("Pipeline", "Airflow", producer.DagID)
			lineages = append(lineages, lineage.LineageEdge{
				Source: dagMRN,
				Target: datasetMRN,
				Type:   "PRODUCES",
			})
		}
	}

	return assets, lineages, nil
}

// createDAGAsset creates a Pipeline asset from an Airflow DAG.
func (s *Source) createDAGAsset(dag DAG) asset.Asset {
	mrnValue := mrn.New("Pipeline", "Airflow", dag.DagID)

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

	return asset.Asset{
		Name:        &dag.DagID,
		MRN:         &mrnValue,
		Type:        "Pipeline",
		Providers:   []string{"Airflow"},
		Description: description,
		Metadata:    cleanMetadata,
		Tags:        s.config.Tags,
		Sources: []asset.AssetSource{{
			Name:       "Airflow",
			LastSyncAt: time.Now(),
			Properties: cleanMetadata,
			Priority:   1,
		}},
	}
}

// enrichDAGAssetWithRunHistory adds run history metadata to a DAG asset.
func (s *Source) enrichDAGAssetWithRunHistory(a *asset.Asset, runs []DAGRun) {
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
func (s *Source) convertDAGRunsToRunHistory(dagMRN, dagID string, runs []DAGRun) plugin.AssetRunHistory {
	var events []plugin.RunHistoryEvent

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
			events = append(events, plugin.RunHistoryEvent{
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

			events = append(events, plugin.RunHistoryEvent{
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

	return plugin.AssetRunHistory{
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
func (s *Source) createTaskAsset(dagID string, task Task) asset.Asset {
	taskName := fmt.Sprintf("%s.%s", dagID, task.TaskID)
	mrnValue := mrn.New("Task", "Airflow", taskName)

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

	return asset.Asset{
		Name:      &taskName,
		MRN:       &mrnValue,
		Type:      "Task",
		Providers: []string{"Airflow"},
		Metadata:  cleanMetadata,
		Tags:      s.config.Tags,
		Sources: []asset.AssetSource{{
			Name:       "Airflow",
			LastSyncAt: time.Now(),
			Properties: cleanMetadata,
			Priority:   1,
		}},
	}
}

// parseDatasetURI parses an Airflow Dataset URI and returns provider, asset type, and name.
func parseDatasetURI(uri string) (provider, assetType, name string) {
	provider = "Airflow"
	assetType = "Dataset"
	name = uri

	if idx := strings.Index(uri, "://"); idx != -1 {
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
func (s *Source) createDatasetAsset(dataset Dataset) asset.Asset {
	provider, assetType, name := parseDatasetURI(dataset.URI)
	mrnValue := mrn.New(assetType, provider, name)

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

	return asset.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  cleanMetadata,
		Tags:      s.config.Tags,
		Sources: []asset.AssetSource{{
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

func init() {
	meta := plugin.PluginMeta{
		ID:          "airflow",
		Name:        "Airflow",
		Description: "Ingest metadata from Apache Airflow including DAGs, tasks, and dataset lineage",
		Icon:        "airflow",
		Category:    "orchestration",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Airflow plugin")
	}
}
