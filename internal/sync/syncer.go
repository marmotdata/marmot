package sync

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"sigs.k8s.io/yaml"
)

type AssetSyncer struct{}

func NewAssetSyncer() *AssetSyncer {
	return &AssetSyncer{}
}

// ApplyConfig applies the plugin configuration to an asset
func (s *AssetSyncer) ApplyConfig(existing, new *asset.Asset, rawConfig plugin.RawPluginConfig) *asset.Asset {
	// 1. Extract BaseConfig from rawConfig:
	baseConfig, err := extractBaseConfig(rawConfig)
	if err != nil {
		// Handle error appropriately, e.g., log and return the new asset as is
		fmt.Printf("Error extracting BaseConfig: %v\n", err)
		return new
	}

	if existing == nil {
		// For new assets, just apply metadata filtering
		return s.applyNewAssetConfig(new, baseConfig)
	}

	result := *existing

	// Apply metadata merge strategy
	if baseConfig.Metadata.Allow != nil {
		result.Metadata = s.mergeMetadata(existing.Metadata, new.Metadata, baseConfig.Metadata.Allow)
	}

	// Apply tag merge strategy based on configuration
	switch baseConfig.Merge.Tags {
	case "append":
		// Combine tags and remove duplicates
		result.Tags = uniqueAppend(existing.Tags, new.Tags)
	case "append_only":
		// Only append new tags that don't exist
		result.Tags = uniqueAppend(existing.Tags, new.Tags)
	case "overwrite":
		// Replace all tags with new ones
		result.Tags = new.Tags
	case "keep_first":
		// Keep existing tags
		result.Tags = existing.Tags
	default:
		// Default to overwrite behavior
		result.Tags = new.Tags
	}

	// Apply name merge strategy
	if new.Name != nil {
		switch baseConfig.Merge.Name {
		case "overwrite":
			result.Name = new.Name
		case "keep_first":
			// Do nothing, keep existing name
		}
	}

	// Apply description merge strategy
	if new.Description != nil {
		switch baseConfig.Merge.Description {
		case "overwrite":
			result.Description = new.Description
		case "keep_first":
			// Do nothing, keep existing description
		}
	}

	// Apply external links
	if len(baseConfig.ExternalLinks) > 0 {
		result.ExternalLinks = s.processExternalLinks(new, baseConfig.ExternalLinks)
	}

	return &result
}

func (s *AssetSyncer) ProcessExternalLinks(ast *asset.Asset, links []plugin.ExternalLink) []asset.ExternalLink {
	result := make([]asset.ExternalLink, len(links))
	for i, link := range links {
		processedLink := asset.ExternalLink{
			Name: link.Name,
			Icon: link.Icon,
			URL:  link.URL,
		}

		// Process URL template
		if ast.Metadata != nil {
			for key, value := range ast.Metadata {
				if strVal, ok := value.(string); ok {
					placeholder := "{" + key + "}"
					processedLink.URL = strings.ReplaceAll(processedLink.URL, placeholder, strVal)
				}
			}
		}

		result[i] = processedLink
	}

	return result
}

// applyNewAssetConfig applies configuration to a new asset
func (s *AssetSyncer) applyNewAssetConfig(ast *asset.Asset, config plugin.BaseConfig) *asset.Asset {
	result := *ast

	// Filter metadata based on allowed fields
	if config.Metadata.Allow != nil {
		filteredMetadata := make(map[string]interface{})
		for _, allowed := range config.Metadata.Allow {
			if val, ok := ast.Metadata[allowed]; ok {
				filteredMetadata[allowed] = val
			}
		}
		result.Metadata = filteredMetadata
	}

	// Process external links
	if len(config.ExternalLinks) > 0 {
		result.ExternalLinks = s.processExternalLinks(ast, config.ExternalLinks)
	}

	return &result
}

// mergeMetadata merges metadata fields based on allowed fields
func (s *AssetSyncer) mergeMetadata(existing, new map[string]interface{}, allowed []string) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy existing allowed fields
	for _, field := range allowed {
		if val, ok := existing[field]; ok {
			result[field] = val
		}
	}

	// Update/add new allowed fields
	for _, field := range allowed {
		if val, ok := new[field]; ok {
			result[field] = val
		}
	}

	return result
}

// processExternalLinks processes external link templates
func (s *AssetSyncer) processExternalLinks(ast *asset.Asset, linkConfigs []plugin.ExternalLink) []asset.ExternalLink {
	result := make([]asset.ExternalLink, len(linkConfigs))
	for i, config := range linkConfigs {
		link := asset.ExternalLink{
			Name: config.Name,
			Icon: config.Icon,
			URL:  config.URL,
		}

		// Process URL template
		if ast.Metadata != nil {
			link.URL = s.processTemplate(link.URL, ast.Metadata)
		}

		result[i] = link
	}
	return result
}

// processTemplate processes a template string with metadata values
func (s *AssetSyncer) processTemplate(template string, metadata map[string]interface{}) string {
	for key, value := range metadata {
		placeholder := "{" + key + "}"
		if stringValue, ok := value.(string); ok {
			template = strings.ReplaceAll(template, placeholder, stringValue)
		}
	}
	return template
}

// Helper function to append unique items to a slice
func uniqueAppend(existing, new []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(existing)+len(new))

	// Add existing items
	for _, item := range existing {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	// Add new items
	for _, item := range new {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// extractBaseConfig is a helper function to unmarshal the BaseConfig from RawPluginConfig
func extractBaseConfig(rawConfig plugin.RawPluginConfig) (plugin.BaseConfig, error) {
	var baseConfig plugin.BaseConfig
	rawBaseConfig, ok := rawConfig["aws"]
	if !ok {
		return baseConfig, nil // It's okay if BaseConfig is not present
	}

	// Marshal the raw BaseConfig back into JSON for Unmarshal to work correctly
	baseConfigJSON, err := yaml.Marshal(rawBaseConfig)
	if err != nil {
		return baseConfig, fmt.Errorf("marshaling raw base config: %w", err)
	}

	if err := yaml.Unmarshal(baseConfigJSON, &baseConfig); err != nil {
		return baseConfig, fmt.Errorf("unmarshaling into BaseConfig: %w", err)
	}

	return baseConfig, nil
}
