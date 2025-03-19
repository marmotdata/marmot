package plugin

import (
	"context"
	"fmt"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"sigs.k8s.io/yaml"
)

// Config represents the top-level configuration
type Config struct {
	Runs []SourceRun `json:"runs"`
}

// SourceRun maps source names to their raw configurations
type SourceRun map[string]RawPluginConfig

// RawPluginConfig holds the raw JSON configuration for a plugin
// It uses a `map[string]interface{}` to unmarshal arbitrary JSON data
// for each plugin's specific config.
type RawPluginConfig map[string]interface{}

// BaseConfig contains configuration fields common to all plugins
type BaseConfig struct {
	GlobalDocumentation         []string       `json:"global_documentation,omitempty"`
	GlobalDocumentationPosition string         `json:"global_documentation_position,omitempty"`
	Metadata                    MetadataConfig `json:"metadata,omitempty"`
	Tags                        TagsConfig     `json:"tags,omitempty"`
	Merge                       MergeConfig    `json:"merge,omitempty"`
	ExternalLinks               []ExternalLink `json:"external_links,omitempty"`
	AWSConfig                   *AWSConfig     `json:"aws,omitempty"`
}

// PluginConfig combines base config with plugin-specific fields
type PluginConfig struct {
	BaseConfig `json:",inline"`
	Source     string `json:"source,omitempty"`
}

// MetadataConfig defines allowable metadata fields
type MetadataConfig struct {
	Allow []string `json:"allow,omitempty"`
}

// MergeConfig defines how different asset fields should be merged
type MergeConfig struct {
	Tags        string `json:"tags,omitempty"`        // append, append_only, overwrite, keep_first
	Metadata    string `json:"metadata,omitempty"`    // merge, overwrite, keep_first
	Name        string `json:"name,omitempty"`        // keep_first, overwrite
	Description string `json:"description,omitempty"` // keep_first, overwrite
}

// ExternalLink defines an external resource link

type ExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

// MetadataFilter defines metadata filtering rules
type MetadataFilter struct {
	Mode     string   `json:"mode"`     // "allow_all", "allow_list", or "deny_list"
	Props    []string `json:"props"`    // Properties to allow or deny based on mode
	Prefixes []string `json:"prefixes"` // Property prefixes to allow or deny
}

// MergePolicy defines how assets should be merged during sync
type MergePolicy struct {
	Strategy        MergeStrategy               `json:"strategy"`
	Fields          map[string]FieldMergeConfig `json:"fields,omitempty"`
	MetadataMapping []MetadataMapping           `json:"metadata_mapping,omitempty"`
	MetadataRules   MetadataRules               `json:"metadata_rules,omitempty"`
}

// FieldMergeConfig defines merge behavior for a specific field
type FieldMergeConfig struct {
	Strategy MergeStrategy  `json:"strategy"`
	Priority int            `json:"priority,omitempty"`
	Filter   MetadataFilter `json:"filter,omitempty"`
}

// MetadataRules defines rules for metadata handling
type MetadataRules struct {
	Merge       MetadataFilter `json:"merge"`       // Controls which fields get merged between assets
	Environment MetadataFilter `json:"environment"` // Controls which fields get copied to environments
}

// MetadataMapping defines how metadata should be mapped during merge
type MetadataMapping struct {
	Source       string `json:"source"`            // JSON path in source object
	Target       string `json:"target"`            // Target metadata path
	Type         string `json:"type"`              // "asset" or "environment"
	DefaultValue any    `json:"default,omitempty"` // Optional default value
}

// MergeStrategy defines different merge strategies
type MergeStrategy string

const (
	MergeStrategyOverwrite MergeStrategy = "overwrite"  // Replace existing value
	MergeStrategyAppend    MergeStrategy = "append"     // Append to existing values
	MergeStrategyKeepFirst MergeStrategy = "keep_first" // Keep existing value if present
	MergeStrategyMerge     MergeStrategy = "merge"      // Deep merge for maps
)

// DiscoveryResult contains all discovered assets, lineage, and documentation
type DiscoveryResult struct {
	Assets        []asset.Asset             `json:"assets"`
	Lineage       []lineage.LineageEdge     `json:"lineage"`
	Documentation []assetdocs.Documentation `json:"documentation"`
}

// Source defines the interface that all plugins must implement
type Source interface {
	// Validate checks if the provided configuration is valid
	Validate(config RawPluginConfig) error

	// Discover performs the discovery of assets, lineage, and documentation
	Discover(ctx context.Context, config RawPluginConfig) (*DiscoveryResult, error)
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
