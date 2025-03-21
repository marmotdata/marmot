package sync

import (
	"sort"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
)

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
