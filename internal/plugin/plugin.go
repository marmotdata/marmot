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
	Tags        string `json:"tags,omitempty"`
	Metadata    string `json:"metadata,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ExternalLink defines an external resource link
type ExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

// MetadataFilter defines metadata filtering rules
type MetadataFilter struct {
	Mode     string   `json:"mode"`
	Props    []string `json:"props"`
	Prefixes []string `json:"prefixes"`
}

// MetadataRules defines rules for metadata handling
type MetadataRules struct {
	Merge       MetadataFilter `json:"merge"`
	Environment MetadataFilter `json:"environment"`
}

// MetadataMapping defines how metadata should be mapped during merge
type MetadataMapping struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	Type         string `json:"type"`
	DefaultValue any    `json:"default,omitempty"`
}

// DiscoveryResult contains all discovered assets, lineage, and documentation
type DiscoveryResult struct {
	Assets        []asset.Asset             `json:"assets"`
	Lineage       []lineage.LineageEdge     `json:"lineage"`
	Documentation []assetdocs.Documentation `json:"documentation"`
}

type Source interface {
	Validate(config RawPluginConfig) error
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
