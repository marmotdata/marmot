package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/lineage"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"sigs.k8s.io/yaml"
)

// TagsConfig, Filter, and ExternalLink are wire types shared with plugins.
// Marmot host code uses them as aliases of the SDK types so plugin authors
// and marmot always agree on the shape.
type TagsConfig = pluginsdk.TagsConfig
type Filter = pluginsdk.Filter
type ExternalLink = pluginsdk.ExternalLink

type Config struct {
	Name string      `json:"name" yaml:"name"`
	Runs []SourceRun `json:"runs" yaml:"runs"`
}

// SourceRun maps source names to their raw configurations
type SourceRun map[string]RawPluginConfig

// RawPluginConfig holds the raw JSON configuration for a plugin
// It uses a `map[string]interface{}` to unmarshal arbitrary JSON data
// for each plugin's specific config.
type RawPluginConfig map[string]interface{} // @name RawPluginConfig

// BaseConfig mirrors pluginsdk.BaseConfig; kept as a type alias so host
// code can embed it in configs without importing the SDK package name.
type BaseConfig = pluginsdk.BaseConfig

// PluginConfig combines base config with plugin-specific fields
type PluginConfig struct {
	BaseConfig `json:",inline"`
	Source     string `json:"source,omitempty"`
}

// DiscoveryResult contains all discovered assets, lineage, and documentation
type DiscoveryResult struct {
	Assets        []asset.Asset             `json:"assets"`
	Lineage       []lineage.LineageEdge     `json:"lineage"`
	Documentation []assetdocs.Documentation `json:"documentation"`
	Statistics    []Statistic               `json:"statistics"`
	RunHistory    []AssetRunHistory         `json:"run_history,omitempty"`
}

// AssetRunHistory contains run history events for an asset
type AssetRunHistory struct {
	AssetMRN string            `json:"asset_mrn"`
	Runs     []RunHistoryEvent `json:"runs"`
}

// RunHistoryEvent represents a single run event (START, COMPLETE, FAIL, etc.)
type RunHistoryEvent struct {
	RunID        string                 `json:"run_id"`
	JobNamespace string                 `json:"job_namespace"`
	JobName      string                 `json:"job_name"`
	EventType    string                 `json:"event_type"` // START, RUNNING, COMPLETE, FAIL, ABORT
	EventTime    time.Time              `json:"event_time"`
	RunFacets    map[string]interface{} `json:"run_facets,omitempty"`
	JobFacets    map[string]interface{} `json:"job_facets,omitempty"`
}

type Statistic struct {
	AssetMRN   string  `json:"asset_mrn"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

// Run represents a single run
type Run struct {
	ID           string          `json:"id"`
	PipelineName string          `json:"pipeline_name"`
	SourceName   string          `json:"source_name"`
	RunID        string          `json:"run_id"`
	Status       RunStatus       `json:"status"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	Config       RawPluginConfig `json:"config,omitempty"`
	Summary      *RunSummary     `json:"summary,omitempty"`
	CreatedBy    string          `json:"created_by"`
} // @name PluginRun

type RunStatus string // @name RunStatus

const (
	StatusRunning   RunStatus = "running"
	StatusCompleted RunStatus = "completed"
	StatusFailed    RunStatus = "failed"
	StatusCancelled RunStatus = "cancelled"
)

// RunSummary contains summary statistics for a run
type RunSummary struct {
	AssetsCreated      int `json:"assets_created"`
	AssetsUpdated      int `json:"assets_updated"`
	AssetsDeleted      int `json:"assets_deleted"`
	LineageCreated     int `json:"lineage_created"`
	LineageUpdated     int `json:"lineage_updated"`
	DocumentationAdded int `json:"documentation_added"`
	ErrorsCount        int `json:"errors_count"`
	TotalEntities      int `json:"total_entities"`
	DurationSeconds    int `json:"duration_seconds"`
} // @name RunSummary

// RunCheckpoint tracks what entities were processed in a run
type RunCheckpoint struct {
	ID           string    `json:"id"`
	RunID        string    `json:"run_id"`
	EntityType   string    `json:"entity_type"` // 'asset', 'lineage', 'documentation'
	EntityMRN    string    `json:"entity_mrn"`
	Operation    string    `json:"operation"`     // 'created', 'updated', 'deleted', 'skipped'
	SourceFields []string  `json:"source_fields"` // Which fields this source contributed
	CreatedAt    time.Time `json:"created_at"`
}

type Source interface {
	Validate(config RawPluginConfig) (RawPluginConfig, error)
	Discover(ctx context.Context, config RawPluginConfig) (*DiscoveryResult, error)
}

// DataFetcher is an optional interface that plugins can implement to support
// data preview functionality. Plugins that can query/fetch sample data from
// their data sources should implement this interface.
type DataFetcher interface {
	FetchSampleData(ctx context.Context, config RawPluginConfig, a *asset.Asset) (columnNames []string, rows [][]interface{}, err error)
}

// UnmarshalPluginConfig unmarshals raw config into a specific plugin config type
func UnmarshalPluginConfig[T any](raw RawPluginConfig) (*T, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling config: %w", err)
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling into plugin config: %w", err)
	}

	return &config, nil
}
