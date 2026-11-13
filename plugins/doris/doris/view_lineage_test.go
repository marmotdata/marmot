package doris

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lineageObject builds a table or view asset the way buildObject does,
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

func shopTables(t *testing.T, cfg *Config) (events, customers pluginsdk.Asset) {
	events = lineageObject(t, cfg, "Table", "shop", "events", "", [][2]string{{"event_date", "date"}, {"event_id", "bigint"}, {"customer_id", "int"}, {"event_type", "varchar(32)"}, {"amount", "decimalv3(9, 2)"}})
	customers = lineageObject(t, cfg, "Table", "shop", "customers", "", [][2]string{{"customer_id", "int"}, {"email", "varchar(128)"}, {"country", "varchar(2)"}})
	return events, customers
}

func TestAttachViewColumnLineage_ViewAndMaterializedView(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: "internal"}
	events, customers := shopTables(t, cfg)
	dailySales := lineageObject(t, cfg, "View", "shop", "daily_sales", extractViewQuery(dailySalesViewDDL), [][2]string{{"event_date", "date"}, {"country", "varchar(2)"}, {"revenue", "decimalv3(38, 2)"}, {"purchases", "bigint"}})
	mvSales := lineageObject(t, cfg, "View", "shop", "mv_sales", mvSalesQuery, [][2]string{{"event_date", "date"}, {"country", "varchar(2)"}, {"revenue", "decimalv3(38, 2)"}})

	assets := []pluginsdk.Asset{events, customers, dailySales, mvSales}
	edges := []pluginsdk.LineageEdge{
		{Source: "mrn://asset/database/doris/shop", Target: *events.MRN, Type: "CONTAINS"},
		viewOfEdge(events, dailySales),
		viewOfEdge(customers, dailySales),
		viewOfEdge(events, mvSales),
		viewOfEdge(customers, mvSales),
	}

	attachViewColumnLineage(context.Background(), cfg.Catalog, assets, edges)

	require.Len(t, edges, 5, "no edges are added")
	assert.Empty(t, edges[0].ColumnLineage, "only VIEW_OF edges get column lineage")
	for _, i := range []int{1, 3} {
		assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
			{FromColumns: []string{"event_date"}, ToColumn: "event_date", Confidence: 1},
			{FromColumns: []string{"amount"}, ToColumn: "revenue", Transform: "expression", Confidence: 1},
		}, edges[i].ColumnLineage, "events -> %s", edges[i].Target)
	}
	for _, i := range []int{2, 4} {
		assert.Equal(t, []pluginsdk.ColumnEdge{
			{FromColumns: []string{"country"}, ToColumn: "country", Confidence: 1},
		}, edges[i].ColumnLineage, "customers -> %s", edges[i].Target)
	}
}

// Newer Doris releases write catalog-qualified names into a stored view,
// and a view may read from another database.
func TestAttachViewColumnLineage_CatalogQualifiedCrossDatabase(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: "internal"}
	events, _ := shopTables(t, cfg)
	purchases := lineageObject(t, cfg, "View", "reports", "purchases",
		"SELECT `internal`.`shop`.`events`.`customer_id` AS `customer_id`, `internal`.`shop`.`events`.`amount` * 100 AS `amount_cents` "+
			"FROM `internal`.`shop`.`events` WHERE `internal`.`shop`.`events`.`event_type` = 'purchase'",
		[][2]string{{"customer_id", "int"}, {"amount_cents", "decimalv3(12, 2)"}})

	edges := []pluginsdk.LineageEdge{viewOfEdge(events, purchases)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{events, purchases}, edges)

	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "customer_id", Confidence: 1},
		{FromColumns: []string{"amount"}, ToColumn: "amount_cents", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
}

func TestAttachViewColumnLineage_KeepsTableEdgesWhenParsingFails(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: "internal"}
	events, customers := shopTables(t, cfg)
	broken := lineageObject(t, cfg, "View", "shop", "broken", "SELECT FROM WHERE shop.events", nil)
	// The parser sees customers but the reference scanner did not, so no
	// edge exists to attach to and none is added.
	partial := lineageObject(t, cfg, "View", "shop", "partial", "SELECT e.amount, c.country FROM shop.events e, shop.customers c", nil)

	edges := []pluginsdk.LineageEdge{viewOfEdge(events, broken), viewOfEdge(events, partial)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{events, customers, broken, partial}, edges)

	require.Len(t, edges, 2)
	assert.Empty(t, edges[0].ColumnLineage)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"amount"}, ToColumn: "amount", Confidence: 1},
	}, edges[1].ColumnLineage)
}

// CREATE VIEW v (a, b) AS SELECT x, y names the view's columns a and b,
// but the query the asset carries has lost that list. Edges must land on
// the view's own columns, matched by position.
func TestAttachViewColumnLineage_DeclaredColumnList(t *testing.T) {
	cfg := &Config{Host: "localhost", Catalog: "internal"}
	events, _ := shopTables(t, cfg)
	const renamedDDL = "CREATE VIEW `renamed` (`day` COMMENT '', `takings` COMMENT 'sum of amount') COMMENT 'VIEW' AS " +
		"SELECT `e`.`event_date` AS `event_date`, sum(`e`.`amount`) AS `revenue` FROM `internal`.`shop`.`events` e GROUP BY `e`.`event_date`;"
	renamed := lineageObject(t, cfg, "View", "shop", "renamed", extractViewQuery(renamedDDL),
		[][2]string{{"day", "date"}, {"takings", "decimalv3(38, 2)"}})
	// A view whose columns do not line up with its SELECT is left with its
	// table-level edge rather than guessed at.
	miscounted := lineageObject(t, cfg, "View", "shop", "miscounted", "SELECT event_date, amount FROM shop.events",
		[][2]string{{"event_date", "date"}})

	edges := []pluginsdk.LineageEdge{viewOfEdge(events, renamed), viewOfEdge(events, miscounted)}
	attachViewColumnLineage(context.Background(), cfg.Catalog, []pluginsdk.Asset{events, renamed, miscounted}, edges)

	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"event_date"}, ToColumn: "day", Confidence: 1},
		{FromColumns: []string{"amount"}, ToColumn: "takings", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
	assert.Empty(t, edges[1].ColumnLineage)
}

func TestStripCatalog(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"quoted table", "SELECT 1 FROM `internal`.`shop`.`orders`", "SELECT 1 FROM `shop`.`orders`"},
		{"quoted column", "SELECT `internal`.`shop`.`orders`.`id`", "SELECT `shop`.`orders`.`id`"},
		{"bare and mixed case", "SELECT 1 FROM Internal.shop.orders", "SELECT 1 FROM shop.orders"},
		{"two-part name is a table", "SELECT internal.id FROM internal", "SELECT internal.id FROM internal"},
		{"longer word", "SELECT 1 FROM my_internal.shop.orders", "SELECT 1 FROM my_internal.shop.orders"},
		{"other catalog", "SELECT 1 FROM hive.shop.orders", "SELECT 1 FROM hive.shop.orders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripCatalog(tt.in, "internal"))
		})
	}
	assert.Equal(t, "SELECT 1 FROM internal.a.b", stripCatalog("SELECT 1 FROM internal.a.b", ""))
}
