package athena

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shopIndex is the tableIndex a run over the AwsDataCatalog shop database
// and a federated sales catalog would build.
func shopIndex() *tableIndex {
	idx := newTableIndex()
	add := func(catalog, database string, t catalogTable) {
		mrnValue := ids.tableMRN(tableAssetType(t.TableType), catalog, database, t.Name)
		idx.add(database, t.Name, mrnValue)
		idx.addSource(catalog, database, t, mrnValue)
	}
	add("AwsDataCatalog", "shop", catalogTable{
		Name:      "orders",
		TableType: "EXTERNAL_TABLE",
		Columns: []catalogColumn{
			{Name: "order_id", Type: "bigint"},
			{Name: "customer_id", Type: "bigint"},
			{Name: "total", Type: "double"},
		},
		PartitionKeys: []catalogColumn{{Name: "dt", Type: "string"}},
	})
	add("AwsDataCatalog", "shop", catalogTable{
		Name:      "customers",
		TableType: "EXTERNAL_TABLE",
		Columns: []catalogColumn{
			{Name: "customer_id", Type: "bigint"},
			{Name: "first_name", Type: "string"},
			{Name: "last_name", Type: "string"},
		},
	})
	add("sales", "crm", catalogTable{
		Name:      "accounts",
		TableType: "EXTERNAL_TABLE",
		Columns: []catalogColumn{
			{Name: "customer_id", Type: "bigint"},
			{Name: "tier", Type: "string"},
		},
	})
	return idx
}

func addView(idx *tableIndex, name, sql string) string {
	mrnValue := ids.tableMRN(typeView, "AwsDataCatalog", "shop", name)
	idx.addView(viewDefinition{catalog: "AwsDataCatalog", database: "shop", name: name, mrn: mrnValue, sql: sql})
	return mrnValue
}

func edgeFrom(edges []pluginsdk.LineageEdge, source string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source {
			return &edges[i]
		}
	}
	return nil
}

func TestViewEdges_CarryColumnLineage(t *testing.T) {
	idx := shopIndex()
	view := addView(idx, "customer_orders", `SELECT o.order_id, c.first_name || ' ' || c.last_name AS full_name, o.total, o.dt
FROM "shop"."orders" o
JOIN customers c ON c.customer_id = o.customer_id`)

	edges := idx.viewEdges(t.Context())
	require.Len(t, edges, 2)

	orders := edgeFrom(edges, ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"))
	require.NotNil(t, orders)
	assert.Equal(t, view, orders.Target)
	assert.Equal(t, "VIEW_OF", orders.Type)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"order_id"}, ToColumn: "order_id", Confidence: 1},
		{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
		{FromColumns: []string{"dt"}, ToColumn: "dt", Confidence: 1},
	}, orders.ColumnLineage)

	customers := edgeFrom(edges, ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "customers"))
	require.NotNil(t, customers)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)
}

func TestViewEdges_ResolveAFederatedCatalog(t *testing.T) {
	idx := shopIndex()
	addView(idx, "tiers", `SELECT a.tier, c.first_name FROM sales.crm.accounts a JOIN awsdatacatalog.shop.customers c ON c.customer_id = a.customer_id`)

	edges := idx.viewEdges(t.Context())
	require.Len(t, edges, 2)

	accounts := edgeFrom(edges, ids.tableMRN(typeTable, "sales", "crm", "accounts"))
	require.NotNil(t, accounts)
	assert.Equal(t, []pluginsdk.ColumnEdge{{FromColumns: []string{"tier"}, ToColumn: "tier", Confidence: 1}}, accounts.ColumnLineage)
}

func TestViewEdges_ExpandSelectStar(t *testing.T) {
	idx := shopIndex()
	addView(idx, "all_customers", "SELECT * FROM customers")

	edges := idx.viewEdges(t.Context())

	require.Len(t, edges, 1)
	assert.Len(t, edges[0].ColumnLineage, 3)
}

func TestViewEdges_KeepTheTableEdgeWhenTheParserFails(t *testing.T) {
	idx := shopIndex()
	addView(idx, "broken", "SELECT order_id FROM orders WHERE (((")

	edges := idx.viewEdges(t.Context())

	require.Len(t, edges, 1)
	assert.Equal(t, ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"), edges[0].Source)
	assert.Equal(t, "VIEW_OF", edges[0].Type)
	assert.Empty(t, edges[0].ColumnLineage)
}

func TestViewEdges_AddTablesTheScanMisses(t *testing.T) {
	// The scan does not follow a comma-separated FROM list, the parser does.
	idx := shopIndex()
	addView(idx, "pairs", "SELECT o.order_id, c.first_name FROM orders o, customers c WHERE o.customer_id = c.customer_id")

	edges := idx.viewEdges(t.Context())

	require.Len(t, edges, 2)
	customers := edgeFrom(edges, ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "customers"))
	require.NotNil(t, customers)
	assert.Equal(t, []pluginsdk.ColumnEdge{{FromColumns: []string{"first_name"}, ToColumn: "first_name", Confidence: 1}}, customers.ColumnLineage)
}

func TestDiscover_ViewIsAViewOfTheTablesItReads(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage, edge(
		ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"),
		ids.tableMRN(typeView, "AwsDataCatalog", "shop", "order_totals"),
		"VIEW_OF",
	))
}
