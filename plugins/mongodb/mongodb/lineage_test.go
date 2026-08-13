package mongodb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

// collection and view mirror collection.go, which names both by the bare
// object name so the MRN matches what the server derives from Name. The
// database is not part of the identity; see mrn_test.go for why.
func collection(name string) pluginsdk.Asset {
	m := mrn.New("Collection", "MongoDB", name)
	return pluginsdk.Asset{Name: ptr(name), MRN: &m, Type: "Collection"}
}

func view(name, viewOn string) pluginsdk.Asset {
	m := mrn.New("View", "MongoDB", name)
	return pluginsdk.Asset{
		Name:     ptr(name),
		MRN:      &m,
		Type:     "View",
		Metadata: map[string]interface{}{"view_on": viewOn},
	}
}

func database(name string) pluginsdk.Asset {
	m := mrn.New("Database", "MongoDB", name)
	return pluginsdk.Asset{Name: ptr(name), MRN: &m, Type: "Database"}
}

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

func TestBuildCollectionLineage_ContainsEveryCollection(t *testing.T) {
	db := database("shop")
	colls := []pluginsdk.Asset{collection("orders"), collection("customers")}

	edges := buildCollectionLineage("shop", db, colls)

	require.Len(t, edges, 2)
	for _, e := range edges {
		assert.Equal(t, "CONTAINS", e.Type)
		assert.Equal(t, *db.MRN, e.Source)
	}
	assertLineageOnlyReferencesDiscoveredAssets(t, append(colls, db), edges)
}

// A view over a plain collection points at that collection's own MRN.
func TestBuildCollectionLineage_ViewOnCollection(t *testing.T) {
	db := database("shop")
	orders := collection("orders")
	recent := view("recent_orders", "orders")
	colls := []pluginsdk.Asset{orders, recent}

	edges := buildCollectionLineage("shop", db, colls)

	var viewOf []pluginsdk.LineageEdge
	for _, e := range edges {
		if e.Type == "VIEW_OF" {
			viewOf = append(viewOf, e)
		}
	}
	require.Len(t, viewOf, 1)
	assert.Equal(t, *orders.MRN, viewOf[0].Source)
	assert.Equal(t, *recent.MRN, viewOf[0].Target)
	assertLineageOnlyReferencesDiscoveredAssets(t, append(colls, db), edges)
}

// MongoDB allows a view defined on another view. The source is catalogued
// as a View, so the edge must not assume the type is Collection.
func TestBuildCollectionLineage_ViewOnViewUsesTheViewMRN(t *testing.T) {
	db := database("shop")
	orders := collection("orders")
	recent := view("recent_orders", "orders")
	todays := view("todays_orders", "recent_orders")
	colls := []pluginsdk.Asset{orders, recent, todays}

	edges := buildCollectionLineage("shop", db, colls)

	var source string
	for _, e := range edges {
		if e.Type == "VIEW_OF" && e.Target == *todays.MRN {
			source = e.Source
		}
	}
	assert.Equal(t, "mrn://view/mongodb/recent_orders", source,
		"the source is itself a view, so the edge must use the View MRN")
	assertLineageOnlyReferencesDiscoveredAssets(t, append(colls, db), edges)
}

// A view whose source was never discovered must not emit an edge at all.
func TestBuildCollectionLineage_UndiscoveredSourceEmitsNoEdge(t *testing.T) {
	db := database("shop")
	orphan := view("orphan_view", "collection_that_was_filtered_out")
	colls := []pluginsdk.Asset{orphan}

	edges := buildCollectionLineage("shop", db, colls)

	for _, e := range edges {
		assert.NotEqual(t, "VIEW_OF", e.Type, "no asset backs the source collection")
	}
	assertLineageOnlyReferencesDiscoveredAssets(t, append(colls, db), edges)
}
