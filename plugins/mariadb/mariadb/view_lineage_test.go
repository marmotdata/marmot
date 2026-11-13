package mariadb

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testObject builds a table or view asset the way discoverObjects and
// attachColumns do, with each column's type as its schema entry.
func testObject(t *testing.T, cfg *Config, assetType, name string, columns map[string]string) pluginsdk.Asset {
	t.Helper()
	mrnValue := assetMRN(cfg, assetType, cfg.Database, name)
	a := pluginsdk.Asset{
		Name:     &name,
		MRN:      &mrnValue,
		Type:     assetType,
		Metadata: map[string]interface{}{"database": cfg.Database},
		Schema:   map[string]string{},
	}
	var cols []column
	for colName, dataType := range columns {
		cols = append(cols, column{Column: pluginsdk.Column{Name: colName, DataType: dataType}})
	}
	require.NoError(t, pluginsdk.SetColumns(&a, cols))
	return a
}

func viewOf(source, target pluginsdk.Asset) pluginsdk.LineageEdge {
	return pluginsdk.LineageEdge{Source: *source.MRN, Target: *target.MRN, Type: "VIEW_OF"}
}

// The definitions are in the normalised form information_schema.VIEWS
// hands back: every name backticked and qualified with the database.
func TestAttachViewColumnLineage(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}

	orders := testObject(t, cfg, "Table", "orders", map[string]string{"id": "int(11)", "customer_id": "int(11)", "total": "decimal(10,2)"})
	customers := testObject(t, cfg, "Table", "customers", map[string]string{"id": "int(11)", "first_name": "varchar(50)", "last_name": "varchar(50)"})
	customerOrders := testObject(t, cfg, "View", "customer_orders", map[string]string{"full_name": "varchar(101)", "total": "decimal(10,2)"})
	customerOrders.Query = new("select concat(`c`.`first_name`,' ',`c`.`last_name`) AS `full_name`,`o`.`total` AS `total` " +
		"from (`shop`.`customers` `c` join `shop`.`orders` `o` on(`o`.`customer_id` = `c`.`id`))")
	broken := testObject(t, cfg, "View", "broken", nil)
	broken.Query = new("select from where `shop`.`orders`")

	assets := []pluginsdk.Asset{orders, customers, customerOrders, broken}
	// orders -> customer_orders is deliberately missing: the parser finds
	// it, but only references the regex found get column lineage.
	edges := []pluginsdk.LineageEdge{
		viewOf(customers, customerOrders),
		viewOf(orders, broken),
	}

	attachViewColumnLineage(context.Background(), "shop", assets, edges)

	require.Len(t, edges, 2, "no edges are added")
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
	assert.Equal(t, "VIEW_OF", edges[0].Type)
	assert.Empty(t, edges[1].ColumnLineage, "an unparseable view keeps its table-level edge")
	assert.Equal(t, *orders.MRN, edges[1].Source)
}

func TestAttachViewColumnLineage_ViewOnAView(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}

	orders := testObject(t, cfg, "Table", "orders", map[string]string{"customer_id": "int(11)", "total": "decimal(10,2)"})
	spend := testObject(t, cfg, "View", "spend", map[string]string{"customer_id": "int(11)", "total_spend": "decimal(32,2)"})
	spend.Query = new("select `shop`.`orders`.`customer_id` AS `customer_id`,sum(`shop`.`orders`.`total`) AS `total_spend` " +
		"from `shop`.`orders` group by `shop`.`orders`.`customer_id`")
	bigSpenders := testObject(t, cfg, "View", "big_spenders", map[string]string{"customer_id": "int(11)"})
	bigSpenders.Query = new("select `shop`.`spend`.`customer_id` AS `customer_id` from `shop`.`spend` where `shop`.`spend`.`total_spend` > 100")
	seq := pluginsdk.Asset{Name: new("order_seq"), MRN: new(assetMRN(cfg, "Sequence", "shop", "order_seq")), Type: "Sequence"}

	assets := []pluginsdk.Asset{orders, spend, bigSpenders, seq}
	edges := []pluginsdk.LineageEdge{viewOf(orders, spend), viewOf(spend, bigSpenders)}

	attachViewColumnLineage(context.Background(), "shop", assets, edges)

	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "customer_id", Confidence: 1},
		{FromColumns: []string{"total"}, ToColumn: "total_spend", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_id"}, ToColumn: "customer_id", Confidence: 1},
	}, edges[1].ColumnLineage)
}

// MariaDB (checked on 11.x) folds a declared column list into the stored
// definition: CREATE VIEW v (a, b) AS SELECT x, y + 1 FROM t is stored with
// the SELECT aliased to a and b, so the edges already target the view's
// own columns without any renaming.
func TestAttachViewColumnLineage_DeclaredColumnList(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 3306, Database: "shop"}

	tbl := testObject(t, cfg, "Table", "t", map[string]string{"x": "int(11)", "y": "int(11)"})
	v := testObject(t, cfg, "View", "v", map[string]string{"a": "int(11)", "b": "bigint(12)"})
	v.Query = new("select `shop`.`t`.`x` AS `a`,`shop`.`t`.`y` + 1 AS `b` from `shop`.`t`")

	edges := []pluginsdk.LineageEdge{viewOf(tbl, v)}
	attachViewColumnLineage(context.Background(), "shop", []pluginsdk.Asset{tbl, v}, edges)

	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"x"}, ToColumn: "a", Confidence: 1},
		{FromColumns: []string{"y"}, ToColumn: "b", Transform: "expression", Confidence: 1},
	}, edges[0].ColumnLineage)
}

func TestDecodeColumns_IgnoresMalformedSchema(t *testing.T) {
	assert.Empty(t, decodeColumns(""))
	assert.Empty(t, decodeColumns("not json"))
	assert.Equal(t, map[string]string{"id": "int"}, decodeColumns(`[{"column_name":"id","data_type":"int"},{"data_type":"text"}]`))
}
