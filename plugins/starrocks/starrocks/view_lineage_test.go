package starrocks

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lineageObject builds a table or view asset the way objectAsset does,
// with the columns, in order, as its schema and query as the view SQL.
func lineageObject(t *testing.T, cfg *Config, assetType, database, name, query string, columns [][2]string) pluginsdk.Asset {
	t.Helper()
	mrnValue := assetMRN(cfg, assetType, database, name)
	a := pluginsdk.Asset{
		Name:     &name,
		MRN:      &mrnValue,
		Type:     assetType,
		Metadata: map[string]any{"database": database, "table_name": name},
		Schema:   map[string]string{},
	}
	if query != "" {
		a.Query = &query
	}
	var cols []column
	for _, c := range columns {
		cols = append(cols, column{Column: pluginsdk.Column{Name: c[0], DataType: c[1]}})
	}
	require.NoError(t, pluginsdk.SetColumns(&a, cols))
	return a
}

func viewOfEdge(source, target pluginsdk.Asset) pluginsdk.LineageEdge {
	return pluginsdk.LineageEdge{Source: *source.MRN, Target: *target.MRN, Type: "VIEW_OF"}
}

func shopTables(t *testing.T, cfg *Config) (orders, customers pluginsdk.Asset) {
	orders = lineageObject(t, cfg, "Table", "shop", "orders", "", [][2]string{{"order_id", "bigint"}, {"order_date", "date"}, {"customer_id", "int"}, {"amount", "decimal(10,2)"}})
	customers = lineageObject(t, cfg, "Table", "shop", "customers", "", [][2]string{{"customer_id", "int"}, {"country", "varchar(2)"}})
	return orders, customers
}

func TestAttachViewColumnLineage_ViewAndMaterializedView(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: defaultCatalog}
	orders, customers := shopTables(t, cfg)
	dailySales := lineageObject(t, cfg, "View", "shop", "daily_sales", viewQuery(dailySalesViewDDL), [][2]string{{"order_date", "date"}, {"country", "varchar(2)"}, {"total", "decimal(38,2)"}})
	mvSales := lineageObject(t, cfg, "View", "shop", "mv_sales", viewQuery(mvSalesDDL), [][2]string{{"order_date", "date"}, {"total", "decimal(38,2)"}})

	assets := []pluginsdk.Asset{orders, customers, dailySales, mvSales}
	edges := []pluginsdk.LineageEdge{
		{Source: "mrn://asset/database/starrocks/shop", Target: *orders.MRN, Type: "CONTAINS"},
		viewOfEdge(orders, dailySales),
		viewOfEdge(customers, dailySales),
		viewOfEdge(orders, mvSales),
	}

	attachViewColumnLineage(context.Background(), cfg.Catalog, assets, edges)

	require.Len(t, edges, 4, "no edges are added")
	assert.Empty(t, edges[0].ColumnLineage, "only VIEW_OF edges get column lineage")
	for _, i := range []int{1, 3} {
		assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
			{FromColumns: []string{"order_date"}, ToColumn: "order_date", Confidence: 1},
			{FromColumns: []string{"amount"}, ToColumn: "total", Transform: "expression", Confidence: 1},
		}, edges[i].ColumnLineage, "orders -> %s", edges[i].Target)
	}
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"country"}, ToColumn: "country", Confidence: 1},
	}, edges[2].ColumnLineage)
}

// StarRocks can store catalog-qualified names in a view, and a view may
// read from another database.
func TestAttachViewColumnLineage_CatalogQualifiedCrossDatabase(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: defaultCatalog}
	orders, _ := shopTables(t, cfg)
	cents := lineageObject(t, cfg, "View", "reports", "order_cents",
		"SELECT `default_catalog`.`shop`.`orders`.`order_id`, `default_catalog`.`shop`.`orders`.`amount` * 100 AS `amount_cents` "+
			"FROM `default_catalog`.`shop`.`orders`",
		[][2]string{{"order_id", "bigint"}, {"amount_cents", "decimal(12,2)"}})

	edges := []pluginsdk.LineageEdge{viewOfEdge(orders, cents)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{orders, cents}, edges)

	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"order_id"}, ToColumn: "order_id", Confidence: 1},
		{FromColumns: []string{"amount"}, ToColumn: "amount_cents", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
}

func TestAttachViewColumnLineage_KeepsTableEdgesWhenParsingFails(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: defaultCatalog}
	orders, customers := shopTables(t, cfg)
	broken := lineageObject(t, cfg, "View", "shop", "broken", "SELECT FROM WHERE orders", nil)
	// The parser sees customers but no VIEW_OF edge was resolved for it,
	// so nothing is added.
	partial := lineageObject(t, cfg, "View", "shop", "partial", "SELECT o.amount, c.country FROM orders o, customers c", nil)

	edges := []pluginsdk.LineageEdge{viewOfEdge(orders, broken), viewOfEdge(orders, partial)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{orders, customers, broken, partial}, edges)

	require.Len(t, edges, 2)
	assert.Empty(t, edges[0].ColumnLineage)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"amount"}, ToColumn: "amount", Confidence: 1},
	}, edges[1].ColumnLineage)
}

// CREATE VIEW v (a, b) AS SELECT x, y names the view's columns a and b,
// and viewQuery drops that list. Edges must land on the view's own
// columns, matched by position, for views and materialized views alike.
func TestAttachViewColumnLineage_DeclaredColumnList(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: defaultCatalog}
	orders, _ := shopTables(t, cfg)
	const renamedDDL = "CREATE VIEW `renamed` (`day`, `takings`)\nCOMMENT \"renamed\" SECURITY NONE AS " +
		"SELECT o.order_date, SUM(o.amount) AS total FROM orders o GROUP BY o.order_date;"
	const renamedMVDDL = "CREATE MATERIALIZED VIEW `mv_renamed` (`day`, `takings`)\nDISTRIBUTED BY HASH(`day`) BUCKETS 2 \nREFRESH ASYNC\n" +
		"AS SELECT order_date, SUM(amount) AS total FROM orders GROUP BY order_date;"
	renamed := lineageObject(t, cfg, "View", "shop", "renamed", viewQuery(renamedDDL),
		[][2]string{{"day", "date"}, {"takings", "decimal(38,2)"}})
	mvRenamed := lineageObject(t, cfg, "View", "shop", "mv_renamed", viewQuery(renamedMVDDL),
		[][2]string{{"day", "date"}, {"takings", "decimal(38,2)"}})
	// A view whose columns do not line up with its SELECT is left with its
	// table-level edge rather than guessed at.
	miscounted := lineageObject(t, cfg, "View", "shop", "miscounted", "SELECT order_date, amount FROM orders",
		[][2]string{{"order_date", "date"}})

	edges := []pluginsdk.LineageEdge{viewOfEdge(orders, renamed), viewOfEdge(orders, mvRenamed), viewOfEdge(orders, miscounted)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{orders, renamed, mvRenamed, miscounted}, edges)

	for _, i := range []int{0, 1} {
		assert.Equal(t, []pluginsdk.ColumnEdge{
			{FromColumns: []string{"order_date"}, ToColumn: "day", Confidence: 1},
			{FromColumns: []string{"amount"}, ToColumn: "takings", Transform: "expression", Confidence: 1},
		}, edges[i].ColumnLineage, "orders -> %s", edges[i].Target)
	}
	assert.Empty(t, edges[2].ColumnLineage)
}

func TestStripCatalog(t *testing.T) {
	assert.Equal(t, "SELECT `shop`.`orders`.`id` FROM `shop`.`orders`",
		stripCatalog("SELECT `default_catalog`.`shop`.`orders`.`id` FROM `default_catalog`.`shop`.`orders`", defaultCatalog))
	assert.Equal(t, "SELECT 1 FROM hive_catalog.shop.orders",
		stripCatalog("SELECT 1 FROM hive_catalog.shop.orders", defaultCatalog), "another catalog is left alone")
	assert.Equal(t, "SELECT default_catalog.id FROM default_catalog",
		stripCatalog("SELECT default_catalog.id FROM default_catalog", defaultCatalog), "a two-part name is a table")
}
