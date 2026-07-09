package plugin

import (
	"regexp"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/lineage"
)

// Filter filters discovered assets by name. Filtering is applied by the
// Marmot host after discovery; plugins only carry the config.
type Filter struct {
	Include []string `json:"include,omitempty" description:"Include patterns for resource names (regex)"`
	Exclude []string `json:"exclude,omitempty" description:"Exclude patterns for resource names (regex)"`
}

// ShouldIncludeResource reports whether a resource name passes the filter.
func ShouldIncludeResource(name string, filter Filter) bool {
	if len(filter.Include) == 0 && len(filter.Exclude) == 0 {
		return true
	}

	for _, pattern := range filter.Exclude {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return false
		}
	}

	if len(filter.Include) == 0 {
		return true
	}

	for _, pattern := range filter.Include {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// FilterDiscoveryResult filters a DiscoveryResult based on the Filter in the config.
// It filters assets by name, then removes lineage, documentation, statistics, and
// run history entries that reference excluded assets.
func FilterDiscoveryResult(result *DiscoveryResult, rawConfig RawPluginConfig) {
	if result == nil {
		return
	}

	base, err := UnmarshalPluginConfig[BaseConfig](rawConfig)
	if err != nil || base.Filter == nil {
		return
	}

	filter := *base.Filter
	if len(filter.Include) == 0 && len(filter.Exclude) == 0 {
		return
	}

	// Filter assets by name and collect included MRNs
	includedMRNs := make(map[string]struct{})
	filteredAssets := make([]asset.Asset, 0, len(result.Assets))
	for _, a := range result.Assets {
		name := ""
		if a.Name != nil {
			name = *a.Name
		}
		if ShouldIncludeResource(name, filter) {
			filteredAssets = append(filteredAssets, a)
			if a.MRN != nil {
				includedMRNs[*a.MRN] = struct{}{}
			}
		}
	}
	result.Assets = filteredAssets

	// Filter lineage: keep edges where both source and target are included
	filteredLineage := make([]lineage.LineageEdge, 0, len(result.Lineage))
	for _, edge := range result.Lineage {
		_, srcOK := includedMRNs[edge.Source]
		_, tgtOK := includedMRNs[edge.Target]
		if srcOK && tgtOK {
			filteredLineage = append(filteredLineage, edge)
		}
	}
	result.Lineage = filteredLineage

	// Filter documentation
	filteredDocs := make([]assetdocs.Documentation, 0, len(result.Documentation))
	for _, doc := range result.Documentation {
		if _, ok := includedMRNs[doc.MRN]; ok {
			filteredDocs = append(filteredDocs, doc)
		}
	}
	result.Documentation = filteredDocs

	// Filter statistics
	filteredStats := make([]Statistic, 0, len(result.Statistics))
	for _, stat := range result.Statistics {
		if _, ok := includedMRNs[stat.AssetMRN]; ok {
			filteredStats = append(filteredStats, stat)
		}
	}
	result.Statistics = filteredStats

	// Filter run history
	filteredHistory := make([]AssetRunHistory, 0, len(result.RunHistory))
	for _, rh := range result.RunHistory {
		if _, ok := includedMRNs[rh.AssetMRN]; ok {
			filteredHistory = append(filteredHistory, rh)
		}
	}
	result.RunHistory = filteredHistory
}
