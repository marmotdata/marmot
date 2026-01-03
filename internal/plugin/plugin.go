package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Name string      `json:"name" yaml:"name"`
	Runs []SourceRun `json:"runs" yaml:"runs"`
}

// SourceRun maps source names to their raw configurations
type SourceRun map[string]RawPluginConfig

// RawPluginConfig holds the raw JSON configuration for a plugin
// It uses a `map[string]interface{}` to unmarshal arbitrary JSON data
// for each plugin's specific config.
type RawPluginConfig map[string]interface{}

type BaseConfig struct {
	Tags          TagsConfig     `json:"tags,omitempty" description:"Tags to apply to discovered assets"`
	ExternalLinks []ExternalLink `json:"external_links,omitempty" description:"External links to show on all assets"`
}

// PluginConfig combines base config with plugin-specific fields
type PluginConfig struct {
	BaseConfig `json:",inline"`
	Source     string `json:"source,omitempty"`
}

// ExternalLink defines an external resource link
type ExternalLink struct {
	Name string `json:"name" description:"Display name for the link" validate:"required"`
	Icon string `json:"icon,omitempty" description:"Icon identifier for the link"`
	URL  string `json:"url" description:"URL to the external resource" validate:"required,url"`
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
}

type RunStatus string

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
}

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

// StatefulRunContext provides context for stateful operations
type StatefulRunContext struct {
	PipelineName       string
	SourceName         string
	LastRunCheckpoints map[string]*RunCheckpoint // entity_mrn -> checkpoint
	CurrentRunID       string
}

type Source interface {
	Validate(config RawPluginConfig) (RawPluginConfig, error)
	Discover(ctx context.Context, config RawPluginConfig) (*DiscoveryResult, error)
}

// StatefulSource extends Source with stateful capabilities
type StatefulSource interface {
	Source
	SupportsStatefulIngestion() bool
}

// GetConfigType attempts to extract the config type from a source by unmarshaling into an empty interface and using reflection
func GetConfigType(raw RawPluginConfig, source Source) interface{} {
	validated, err := source.Validate(raw)
	if err == nil && validated != nil {
		return validated
	}
	return raw
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
