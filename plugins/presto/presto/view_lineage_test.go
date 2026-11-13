package presto

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableWithColumns is a discovered table whose columns attachColumns has
// already set.
func tableWithColumns(t *testing.T, catalog, schema, table string, cols map[string]string) pluginsdk.Asset {
	t.Helper()
	a := tableAsset(catalog, schema, table)
	var list []column
	for name, typ := range cols {
		list = append(list, column{Column: pluginsdk.Column{Name: name, DataType: typ, Nullable: true}})
	}
	require.NoError(t, pluginsdk.SetColumns(&a, list))
	return a
}

func edgeBetween(edges []pluginsdk.LineageEdge, source, target string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}

func TestViewLineage_AttachesColumnLineageToTheViewEdge(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableWithColumns(t, "memory", "shop", "customers", map[string]string{"custkey": "bigint", "first_name": "varchar", "last_name": "varchar"}),
		tableWithColumns(t, "tpch", "tiny", "orders", map[string]string{"orderkey": "bigint", "custkey": "bigint", "totalprice": "double"}),
		viewAsset("memory", "shop", "customer_orders", `SELECT c.custkey, c.first_name || ' ' || c.last_name AS full_name, o.totalprice
FROM customers c
JOIN tpch.tiny.orders o ON o.custkey = c.custkey`),
	}

	edges := viewLineage(context.Background(), assets)
	require.Len(t, edges, 2)

	view := "mrn://asset/view/presto/memory.shop.customer_orders"

	customers := edgeBetween(edges, "mrn://asset/table/presto/memory.shop.customers", view)
	require.NotNil(t, customers)
	assert.Equal(t, "VIEW_OF", customers.Type)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"custkey"}, ToColumn: "custkey", Confidence: 1},
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)

	orders := edgeBetween(edges, "mrn://asset/table/presto/tpch.tiny.orders", view)
	require.NotNil(t, orders)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"totalprice"}, ToColumn: "totalprice", Confidence: 1},
	}, orders.ColumnLineage)
}

func TestViewLineage_ExpandsSelectStarFromDiscoveredColumns(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableWithColumns(t, "memory", "shop", "orders", map[string]string{"orderkey": "bigint", "totalprice": "double"}),
		viewAsset("memory", "shop", "big_orders", prestoViewDefinition),
	}

	edges := viewLineage(context.Background(), assets)

	require.Len(t, edges, 1)
	assert.ElementsMatch(t, []string{"orderkey", "totalprice"}, toColumns(edges[0].ColumnLineage))
}

func TestViewLineage_AddsTablesTheReferenceScanMisses(t *testing.T) {
	// A comma-separated FROM list names its second table after a comma,
	// which the FROM/JOIN scan does not follow but the parser does.
	assets := []pluginsdk.Asset{
		tableWithColumns(t, "memory", "shop", "a", map[string]string{"id": "bigint"}),
		tableWithColumns(t, "memory", "shop", "b", map[string]string{"id": "bigint", "name": "varchar"}),
		viewAsset("memory", "shop", "v", "SELECT a.id, b.name FROM a, b WHERE a.id = b.id"),
	}

	edges := viewLineage(context.Background(), assets)

	require.Len(t, edges, 2)
	b := edgeBetween(edges, "mrn://asset/table/presto/memory.shop.b", "mrn://asset/view/presto/memory.shop.v")
	require.NotNil(t, b)
	assert.Equal(t, "VIEW_OF", b.Type)
	assert.Equal(t, []string{"name"}, toColumns(b.ColumnLineage))
}

func TestViewLineage_KeepsTableEdgeWhenTheParserFails(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableWithColumns(t, "memory", "shop", "orders", map[string]string{"orderkey": "bigint"}),
		viewAsset("memory", "shop", "v", "SELECT orderkey FROM orders WHERE ((("),
	}

	edges := viewLineage(context.Background(), assets)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://asset/table/presto/memory.shop.orders", edges[0].Source)
	assert.Empty(t, edges[0].ColumnLineage)
}

func toColumns(edges []pluginsdk.ColumnEdge) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.ToColumn)
	}
	return out
}
