package mssql

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	customersMRN   = "mrn://asset/table/sql-server/shop.dbo.customers"
	ordersMRN      = "mrn://asset/table/sql-server/shop.dbo.orders"
	orderTotalsMRN = "mrn://asset/view/sql-server/shop.dbo.order_totals"
)

// discoveredWithColumns is discoveredNames with the column types discovery
// records, which the parser needs to resolve column references.
func discoveredWithColumns() map[string]objectRef {
	objects := discoveredNames()
	set := func(key string, columns map[string]string) {
		ref := objects[key]
		ref.Columns = columns
		objects[key] = ref
	}
	set("shop.dbo.customers", map[string]string{"customer_id": "int", "first_name": "nvarchar(50)", "last_name": "nvarchar(50)"})
	set("shop.dbo.orders", map[string]string{"order_id": "int", "customer_id": "int", "total": "decimal(10,2)"})
	set("shop.dbo.order_totals", map[string]string{"customer_name": "nvarchar(101)", "total_spend": "decimal(38,2)"})
	return objects
}

func edgeFrom(t *testing.T, edges []pluginsdk.LineageEdge, source string) pluginsdk.LineageEdge {
	t.Helper()
	for _, e := range edges {
		if e.Source == source {
			return e
		}
	}
	require.Failf(t, "edge not found", "no edge from %s in %v", source, edges)
	return pluginsdk.LineageEdge{}
}

func TestViewOfEdges_CarryColumnLineage(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": `-- spend per customer
CREATE VIEW [dbo].[order_totals] WITH SCHEMABINDING AS
SELECT c.first_name + ' ' + c.last_name AS customer_name,
       SUM(o.total) AS total_spend
FROM dbo.customers c
JOIN shop.dbo.orders o ON o.customer_id = c.customer_id
GROUP BY c.first_name, c.last_name`,
	}

	edges := viewOfEdges(context.Background(), testSource().config, "shop", viewAssets(t), definitions, discoveredWithColumns())

	require.Len(t, edges, 2, "column lineage rides on the existing VIEW_OF edges rather than adding new ones")

	customers := edgeFrom(t, edges, customersMRN)
	assert.Equal(t, "VIEW_OF", customers.Type)
	assert.Equal(t, orderTotalsMRN, customers.Target)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "customer_name", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)

	orders := edgeFrom(t, edges, ordersMRN)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total_spend", Transform: "expression", Confidence: 1},
	}, orders.ColumnLineage)
}

func TestViewOfEdges_ColumnListRenamesTheTargets(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": "CREATE OR ALTER VIEW dbo.order_totals ([customer_name], total_spend) AS SELECT c.first_name, o.total FROM dbo.customers c JOIN dbo.orders o ON o.customer_id = c.customer_id WITH CHECK OPTION;",
	}

	edges := viewOfEdges(context.Background(), testSource().config, "shop", viewAssets(t), definitions, discoveredWithColumns())

	require.Len(t, edges, 2)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"first_name"}, ToColumn: "customer_name", Confidence: 1},
	}, edgeFrom(t, edges, customersMRN).ColumnLineage)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total_spend", Confidence: 1},
	}, edgeFrom(t, edges, ordersMRN).ColumnLineage)
}

func TestViewOfEdges_StarExpandsFromTheDiscoveredColumns(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT * FROM orders",
	}

	edges := viewOfEdges(context.Background(), testSource().config, "shop", viewAssets(t), definitions, discoveredWithColumns())

	require.Len(t, edges, 1)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "customer_id", Confidence: 1},
		{FromColumns: []string{"order_id"}, ToColumn: "order_id", Confidence: 1},
		{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
	}, edges[0].ColumnLineage)
}

func TestViewOfEdges_KeepTableEdgesWhenTheDefinitionDoesNotParse(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT FROM dbo.orders WHERE (",
	}

	edges := viewOfEdges(context.Background(), testSource().config, "shop", viewAssets(t), definitions, discoveredWithColumns())

	require.Len(t, edges, 1)
	assert.Equal(t, ordersMRN, edges[0].Source)
	assert.Empty(t, edges[0].ColumnLineage)
}

func TestViewQuery(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		want       string
		ok         bool
	}{
		{
			name:       "plain header",
			definition: "CREATE VIEW dbo.v AS SELECT id FROM t",
			want:       "SELECT id FROM t",
			ok:         true,
		},
		{
			name:       "comments, brackets, attributes and trailer",
			definition: "/* audit */\n-- owner: data\ncreate view [shop].[dbo].[v] with schemabinding, view_metadata\nas\nSELECT id FROM dbo.t\nWITH CHECK OPTION\n",
			want:       "SELECT id FROM dbo.t",
			ok:         true,
		},
		{
			name:       "column list wraps the select",
			definition: "CREATE OR ALTER VIEW v (a, [b c]) AS SELECT x, y FROM t;",
			want:       "SELECT * FROM (SELECT x, y FROM t) AS [v]([a], [b c])",
			ok:         true,
		},
		{
			name:       "not a view",
			definition: "SELECT id FROM t",
			ok:         false,
		},
		{
			name:       "empty body",
			definition: "CREATE VIEW v AS ",
			ok:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := viewQuery(tt.definition)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
