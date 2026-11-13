package mysql

import (
	"context"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testObject builds a table or view asset the way discoverTablesAndViews
// does, so the lineage code sees the same names and MRNs.
func testObject(cfg *Config, assetType, dbName, name string) pluginsdk.Asset {
	mrnValue := assetMRN(cfg, assetType, dbName, name)
	return pluginsdk.Asset{
		Name: &name,
		MRN:  &mrnValue,
		Type: assetType,
		Metadata: map[string]interface{}{
			"database":   dbName,
			"schema":     dbName,
			"table_name": name,
		},
	}
}

// The definitions are in the normalised form information_schema.VIEWS
// hands back: every name backticked and qualified with the database.
func TestViewColumnLineage(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}
	assets := []pluginsdk.Asset{
		testObject(cfg, "Table", "shop", "orders"),
		testObject(cfg, "Table", "shop", "customers"),
		testObject(cfg, "View", "shop", "orders_by_customer"),
		testObject(cfg, "View", "shop", "customer_orders"),
		testObject(cfg, "View", "shop", "broken"),
	}
	columns := map[string][]tableColumn{
		"orders":             {{"id", "int"}, {"customer_id", "int"}, {"total", "decimal"}},
		"customers":          {{"id", "int"}, {"first_name", "varchar"}, {"last_name", "varchar"}},
		"orders_by_customer": {{"customer_id", "int"}, {"total_spend", "decimal"}, {"n", "bigint"}},
		"customer_orders":    {{"full_name", "varchar"}, {"total", "decimal"}},
	}
	viewDefs := map[string]string{
		"orders_by_customer": "select `shop`.`orders`.`customer_id` AS `customer_id`,sum(`shop`.`orders`.`total`) AS `total_spend`,count(0) AS `n` " +
			"from `shop`.`orders` group by `shop`.`orders`.`customer_id`",
		"customer_orders": "select concat(`c`.`first_name`,' ',`c`.`last_name`) AS `full_name`,`o`.`total` AS `total` " +
			"from (`shop`.`customers` `c` join `shop`.`orders` `o` on((`o`.`customer_id` = `c`.`id`)))",
		"broken": "select from where",
	}

	edges := viewColumnLineage(context.Background(), "shop", assets, viewDefs, columns)
	require.Len(t, edges, 3, "orders→orders_by_customer, orders→customer_orders, customers→customer_orders")

	ordersMRN := *assets[0].MRN
	customersMRN := *assets[1].MRN
	byCustomerMRN := *assets[2].MRN
	customerOrdersMRN := *assets[3].MRN

	agg := findEdge(edges, ordersMRN, byCustomerMRN)
	require.NotNil(t, agg)
	assert.Equal(t, "DEPENDS_ON", agg.Type)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "customer_id", Confidence: 1},
		{FromColumns: []string{"total"}, ToColumn: "total_spend", Transform: "expression", Confidence: 1},
	}, agg.ColumnLineage)

	fromCustomers := findEdge(edges, customersMRN, customerOrdersMRN)
	require.NotNil(t, fromCustomers)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, fromCustomers.ColumnLineage)

	fromOrders := findEdge(edges, ordersMRN, customerOrdersMRN)
	require.NotNil(t, fromOrders)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
	}, fromOrders.ColumnLineage)

	for _, e := range edges {
		assert.False(t, strings.HasSuffix(e.Target, "broken"), "an unparseable view yields no edges")
	}
}

// CREATE VIEW v (a, b) AS SELECT x, y names the view's columns a and b.
// Edges must land on those, matched by position, whatever the SELECT
// calls its outputs; here the declared names also swap the SELECT's.
func TestViewColumnLineage_DeclaredColumnList(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}
	assets := []pluginsdk.Asset{
		testObject(cfg, "Table", "shop", "orders"),
		testObject(cfg, "View", "shop", "renamed"),
		testObject(cfg, "View", "shop", "miscounted"),
	}
	columns := map[string][]tableColumn{
		"orders":     {{"id", "int"}, {"customer_id", "int"}, {"total", "decimal"}},
		"renamed":    {{"total", "int"}, {"buyer", "int"}, {"spend", "decimal"}},
		"miscounted": {{"a", "int"}},
	}
	viewDefs := map[string]string{
		"renamed": "select `shop`.`orders`.`customer_id` AS `customer_id`,`shop`.`orders`.`id` AS `id`,(`shop`.`orders`.`total` * 2) AS `total` " +
			"from `shop`.`orders`",
		"miscounted": "select `shop`.`orders`.`id` AS `id`,`shop`.`orders`.`total` AS `total` from `shop`.`orders`",
	}

	edges := viewColumnLineage(context.Background(), "shop", assets, viewDefs, columns)
	require.Len(t, edges, 1, "a view whose column count disagrees with its SELECT gets no edges")

	edge := findEdge(edges, *assets[0].MRN, *assets[1].MRN)
	require.NotNil(t, edge)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "total", Confidence: 1},
		{FromColumns: []string{"id"}, ToColumn: "buyer", Confidence: 1},
		{FromColumns: []string{"total"}, ToColumn: "spend", Transform: "expression", Confidence: 1},
	}, edge.ColumnLineage)
}

// MySQL 8.4 keeps the declared list out of VIEW_DEFINITION: for
// CREATE VIEW v (a, b) AS SELECT x, y + 1 FROM t it stores the SELECT's
// own aliases, while information_schema.COLUMNS reports a and b.
func TestViewColumnLineage_DeclaredColumnListAsMySQLStoresIt(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}
	assets := []pluginsdk.Asset{testObject(cfg, "Table", "shop", "t"), testObject(cfg, "View", "shop", "v")}
	columns := map[string][]tableColumn{
		"t": {{"x", "int"}, {"y", "int"}},
		"v": {{"a", "int"}, {"b", "bigint"}},
	}
	viewDefs := map[string]string{"v": "select `shop`.`t`.`x` AS `x`,(`shop`.`t`.`y` + 1) AS `y + 1` from `shop`.`t`"}

	edges := viewColumnLineage(context.Background(), "shop", assets, viewDefs, columns)
	require.Len(t, edges, 1)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"x"}, ToColumn: "a", Confidence: 1},
		{FromColumns: []string{"y"}, ToColumn: "b", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
}

func TestWithViewColumns(t *testing.T) {
	assert.Equal(t, "select 1", withViewColumns("select 1", nil))
	assert.Equal(t, "WITH `__marmot_view` (`a`, `b``c`) AS (\nselect 1, 2 -- note\n) SELECT * FROM `__marmot_view`",
		withViewColumns(" select 1, 2 -- note\n; ", []string{"a", "b`c"}))
}

func TestViewColumnLineage_NoTables(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}
	assets := []pluginsdk.Asset{testObject(cfg, "Database", "shop", "shop")}
	edges := viewColumnLineage(context.Background(), "shop", assets, map[string]string{"v": "select 1"}, nil)
	assert.Empty(t, edges)
}

func findEdge(edges []pluginsdk.LineageEdge, source, target string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}
