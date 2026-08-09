package dbt

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertLineageOnlyReferencesDiscoveredAssets is the guard for the bug
// class this plugin family kept reproducing: an edge naming an MRN the
// same run never emits is silently dropped by the server, so the lineage
// disappears instead of failing loudly.
func assertLineageOnlyReferencesDiscoveredAssets(t *testing.T, assets []pluginsdk.Asset, edges []pluginsdk.LineageEdge) {
	t.Helper()

	emitted := make(map[string]struct{}, len(assets))
	for _, a := range assets {
		if a.MRN != nil {
			emitted[*a.MRN] = struct{}{}
		}
	}

	for _, edge := range edges {
		assert.Contains(t, emitted, edge.Source,
			"lineage edge source %q has no asset behind it", edge.Source)
		assert.Contains(t, emitted, edge.Target,
			"lineage edge target %q has no asset behind it", edge.Target)
	}
}

// manifestWithModelOnSource builds the smallest project that exercises the
// model-depends-on-source path: one source table and one model reading it.
func manifestWithModelOnSource() *DBTManifest {
	return &DBTManifest{
		Metadata: ManifestMetadata{AdapterType: "postgres"},
		Nodes: map[string]ManifestNode{
			"model.shop.orders_summary": {
				UniqueID:     "model.shop.orders_summary",
				Name:         "orders_summary",
				ResourceType: "model",
				Database:     "analytics",
				Schema:       "marts",
				Materialized: "table",
				DependsOn:    NodeDependency{Nodes: []string{"source.shop.raw.orders"}},
			},
		},
		Sources: map[string]ManifestNode{
			"source.shop.raw.orders": {
				UniqueID:     "source.shop.raw.orders",
				Name:         "orders",
				ResourceType: "source",
				Database:     "analytics",
				Schema:       "raw",
			},
		},
	}
}

func sourceWith(manifest *DBTManifest, discoverSources bool) *Source {
	return &Source{
		manifest: manifest,
		config: &Config{
			ProjectName:     "shop",
			DiscoverModels:  true,
			DiscoverSources: discoverSources,
		},
	}
}

// With discover_sources on, the source table becomes an asset and the
// DEPENDS_ON edge has something to point at.
func TestModelLineage_SourceEdgeKeptWhenSourcesAreDiscovered(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	modelAssets, lineages := s.discoverModels()
	sourceAssets := s.discoverSources()
	all := append(modelAssets, sourceAssets...)

	assertLineageOnlyReferencesDiscoveredAssets(t, all, lineages)

	var dependsOn int
	for _, e := range lineages {
		if e.Type == "DEPENDS_ON" {
			dependsOn++
			assert.Equal(t, "mrn://table/postgresql/raw.orders", e.Source)
		}
	}
	assert.Equal(t, 1, dependsOn)
}

// With discover_sources off no source asset is ever created, so the edge
// must be dropped rather than left pointing at nothing.
func TestModelLineage_SourceEdgeDroppedWhenSourcesAreNotDiscovered(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), false)

	modelAssets, lineages := s.discoverModels()

	assertLineageOnlyReferencesDiscoveredAssets(t, modelAssets, lineages)

	for _, e := range lineages {
		assert.NotEqual(t, "DEPENDS_ON", e.Type,
			"no source asset exists, so the dependency edge must be dropped")
	}
}

// The materialized table a model creates must land on the MRN the native
// plugin uses, or dbt files the same physical table twice.
func TestModelLineage_CreatesEdgeUsesTheNativeTableMRN(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	_, lineages := s.discoverModels()

	var creates []string
	for _, e := range lineages {
		if e.Type == "CREATES" {
			creates = append(creates, e.Target)
		}
	}
	require.Len(t, creates, 1)
	assert.Equal(t, "mrn://table/postgresql/marts.orders_summary", creates[0],
		"plugins/postgresql addresses this table as schema.table under PostgreSQL")
}
