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
			assert.Equal(t, "mrn://asset/table/postgresql/analytics.raw.orders", e.Source)
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

// The CREATES edge must point at the same MRN the model's own asset is given, so the edge is not silently dropped. It is also the MRN plugins/postgresql emits for the table, because the adapter uses the provider string and the database.schema.table path that plugin uses.
func TestModelLineage_CreatesEdgeMatchesTheModelsOwnAsset(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	modelAssets, lineages := s.discoverModels()
	all := append(modelAssets, s.discoverSources()...)

	var creates []string
	for _, e := range lineages {
		if e.Type == "CREATES" {
			creates = append(creates, e.Target)
		}
	}
	require.Len(t, creates, 1)
	assert.Equal(t, "mrn://asset/table/postgresql/analytics.marts.orders_summary", creates[0])

	// The edge and the asset agree, which is the property that matters.
	assertLineageOnlyReferencesDiscoveredAssets(t, all, lineages)
}

func TestModelMRN_IsDatabaseSchemaAndName(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	assets, _ := s.discoverModels()

	var model pluginsdk.Asset
	for _, a := range assets {
		if a.Type == "Model" {
			model = a
		}
	}
	require.NotNil(t, model.MRN)
	assert.Equal(t, "mrn://asset/model/dbt/analytics.marts.orders_summary", *model.MRN)
	assert.Equal(t, "orders_summary", *model.Name)
}

func TestModelMRN_SameModelNameInDifferentSchemasDiffers(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	assert.NotEqual(t, s.modelMRN("analytics", "marts", "orders"), s.modelMRN("analytics", "staging", "orders"))
}

func TestModelMRN_CarriesTheZone(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)
	s.config.Zone = "shop"

	assert.Equal(t, "mrn://asset/model/dbt@shop/analytics.marts.orders", s.modelMRN("analytics", "marts", "orders"))
}

func TestTargetZoneAppliesToWarehouseTablesButNotToModels(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)
	s.config.TargetZone = "prod-pg"

	modelAssets, lineages := s.discoverModels()
	all := append(modelAssets, s.discoverSources()...)

	assertLineageOnlyReferencesDiscoveredAssets(t, all, lineages)
	for _, e := range lineages {
		switch e.Type {
		case "DEPENDS_ON":
			assert.Equal(t, "mrn://asset/table/postgresql@prod-pg/analytics.raw.orders", e.Source)
			assert.Equal(t, "mrn://asset/model/dbt/analytics.marts.orders_summary", e.Target)
		case "CREATES":
			assert.Equal(t, "mrn://asset/table/postgresql@prod-pg/analytics.marts.orders_summary", e.Target)
		}
	}
}

func TestWarehouseAssetsListTheirOldMRN(t *testing.T) {
	s := sourceWith(manifestWithModelOnSource(), true)

	for _, a := range s.discoverSources() {
		assert.Equal(t, []string{"mrn://asset/table/postgres/analytics.raw.orders"}, a.PreviousMRNs)
	}
}

func TestWarehousePath_MySQLDropsTheDatabase(t *testing.T) {
	assert.Equal(t, []string{"shop", "orders"}, warehousePath(GetAdapter("mysql"), "def", "shop", "orders"))
	assert.Equal(t, []string{"analytics", "marts", "orders"}, warehousePath(GetAdapter("postgres"), "analytics", "marts", "orders"))
}
