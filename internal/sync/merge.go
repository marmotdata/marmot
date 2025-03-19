package sync

import (
	"sort"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
)

// mergeSources merges source information, preferring sources with higher priority
func mergeSources(existing, new []asset.AssetSource) []asset.AssetSource {
	sourceMap := make(map[string]asset.AssetSource)

	// Helper function to merge properties
	mergeProperties := func(existing, new map[string]interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		// Copy existing properties
		for k, v := range existing {
			result[k] = v
		}
		// Update/add new properties
		for k, v := range new {
			result[k] = v
		}
		return result
	}

	// Index existing sources, strictly skip empty names
	for _, src := range existing {
		if src.Name == "" {
			continue
		}
		sourceMap[src.Name] = src
	}

	// Update or add new sources, strictly skip empty names
	for _, src := range new {
		if src.Name == "" {
			continue
		}

		if existing, ok := sourceMap[src.Name]; ok {
			// Merge properties if both exist
			if src.Properties != nil {
				existing.Properties = mergeProperties(existing.Properties, src.Properties)
			}
			// Update timestamp only if newer
			if src.LastSyncAt.After(existing.LastSyncAt) {
				existing.LastSyncAt = src.LastSyncAt
			}
			// Update priority if higher
			if src.Priority > existing.Priority {
				existing.Priority = src.Priority
			}
			sourceMap[src.Name] = existing
		} else {
			// Add new source
			sourceMap[src.Name] = src
		}
	}

	// Convert map back to slice, sorting by priority
	result := make([]asset.AssetSource, 0, len(sourceMap))
	for _, src := range sourceMap {
		result = append(result, src)
	}

	// Sort by priority (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})

	return result
}

// UpdateSource updates or adds a source to an asset's sources
func UpdateSource(sources []asset.AssetSource, newSource asset.AssetSource) []asset.AssetSource {
	// Don't process sources with empty names
	if newSource.Name == "" {
		return sources
	}

	// Find existing source index
	index := -1
	for i, src := range sources {
		if src.Name == newSource.Name {
			index = i
			break
		}
	}

	if index >= 0 {
		// Update existing source
		sources[index].Properties = newSource.Properties
		sources[index].LastSyncAt = time.Now()
		sources[index].Priority = newSource.Priority
	} else {
		// Add new source
		sources = append(sources, newSource)
	}

	// Sort by priority
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Priority > sources[j].Priority
	})

	return sources
}
