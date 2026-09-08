package presto

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prestoViewDefinition is what memory.information_schema.views returned
// on Presto 0.299 for CREATE VIEW ... AS SELECT * FROM memory.shop.orders.
const prestoViewDefinition = "SELECT *\nFROM\n  memory.shop.orders\nWHERE (totalprice > 100000)\n"

func TestExtractTableReferences_FullyQualifiedAcrossNewlines(t *testing.T) {
	refs := extractTableReferences(prestoViewDefinition)
	assert.Equal(t, [][]string{{"memory", "shop", "orders"}}, refs)
}

func TestExtractTableReferences_SchemaQualifiedAndBare(t *testing.T) {
	refs := extractTableReferences("SELECT * FROM shop.orders o JOIN customers c ON o.custkey = c.custkey")
	assert.Equal(t, [][]string{{"shop", "orders"}, {"customers"}}, refs)
}

func TestExtractTableReferences_JoinVariants(t *testing.T) {
	refs := extractTableReferences("SELECT 1 FROM a LEFT OUTER JOIN b ON true CROSS JOIN c")
	assert.Equal(t, [][]string{{"a"}, {"b"}, {"c"}}, refs)
}

func TestExtractTableReferences_LowercasesBareIdentifiers(t *testing.T) {
	refs := extractTableReferences("SELECT 1 FROM Shop.Orders")
	assert.Equal(t, [][]string{{"shop", "orders"}}, refs)
}

func TestExtractTableReferences_KeepsQuotedIdentifiersVerbatim(t *testing.T) {
	refs := extractTableReferences(`SELECT 1 FROM "memory"."Shop"."Big Orders"`)
	assert.Equal(t, [][]string{{"memory", "Shop", "Big Orders"}}, refs)
}

func TestExtractTableReferences_UnescapesDoubledQuotes(t *testing.T) {
	refs := extractTableReferences(`SELECT 1 FROM "we""ird"`)
	assert.Equal(t, [][]string{{`we"ird`}}, refs)
}

func TestExtractTableReferences_SkipsSubqueries(t *testing.T) {
	refs := extractTableReferences("SELECT 1 FROM (SELECT 1 FROM inner_table) x")
	assert.Equal(t, [][]string{{"inner_table"}}, refs)
}

func TestExtractTableReferences_NoReferences(t *testing.T) {
	assert.Empty(t, extractTableReferences("SELECT 1"))
}

func TestResolveReference_FillsMissingPartsFromTheView(t *testing.T) {
	assert.Equal(t, tableKey{"memory", "shop", "orders"}, resolveReference([]string{"memory", "shop", "orders"}, "hive", "default"))
	assert.Equal(t, tableKey{"hive", "shop", "orders"}, resolveReference([]string{"shop", "orders"}, "hive", "default"))
	assert.Equal(t, tableKey{"hive", "default", "orders"}, resolveReference([]string{"orders"}, "hive", "default"))
}

func tableAsset(catalog, schema, table string) pluginsdk.Asset {
	s := &Source{config: &Config{}}
	return s.createTableAsset(catalog, schema, table, "BASE TABLE", "memory", connectorInfoForName("memory"))
}

func viewAsset(catalog, schema, table, definition string) pluginsdk.Asset {
	s := &Source{config: &Config{}}
	a := s.createTableAsset(catalog, schema, table, "VIEW", "memory", connectorInfoForName("memory"))
	lang := "SQL"
	a.Query = &definition
	a.QueryLanguage = &lang
	return a
}

func TestViewLineage_EdgeRunsFromBaseTableToView(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableAsset("memory", "shop", "orders"),
		viewAsset("memory", "shop", "big_orders", prestoViewDefinition),
	}

	edges := viewLineage(assets)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://table/presto/memory.shop.orders", edges[0].Source)
	assert.Equal(t, "mrn://view/presto/memory.shop.big_orders", edges[0].Target)
	assert.Equal(t, "VIEW_OF", edges[0].Type)
}

func TestViewLineage_ResolvesBareNamesAgainstTheViewsSchema(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableAsset("memory", "shop", "orders"),
		tableAsset("memory", "other", "orders"),
		viewAsset("memory", "shop", "v", "SELECT * FROM orders"),
	}

	edges := viewLineage(assets)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://table/presto/memory.shop.orders", edges[0].Source)
}

func TestViewLineage_ReachesTablesInOtherCatalogs(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableAsset("tpch", "tiny", "nation"),
		viewAsset("memory", "shop", "v", "SELECT * FROM tpch.tiny.nation"),
	}

	edges := viewLineage(assets)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://table/presto/tpch.tiny.nation", edges[0].Source)
}

func TestViewLineage_DropsUndiscoveredReferences(t *testing.T) {
	assets := []pluginsdk.Asset{
		viewAsset("memory", "shop", "v", "SELECT * FROM hive.raw.events"),
	}

	assert.Empty(t, viewLineage(assets))
}

func TestViewLineage_DeduplicatesRepeatedReferences(t *testing.T) {
	assets := []pluginsdk.Asset{
		tableAsset("memory", "shop", "orders"),
		viewAsset("memory", "shop", "v", "SELECT * FROM orders UNION ALL SELECT * FROM shop.orders"),
	}

	assert.Len(t, viewLineage(assets), 1)
}

func TestViewLineage_ViewsCanStackOnViews(t *testing.T) {
	assets := []pluginsdk.Asset{
		viewAsset("memory", "shop", "v1", "SELECT 1"),
		viewAsset("memory", "shop", "v2", "SELECT * FROM v1"),
	}

	edges := viewLineage(assets)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://view/presto/memory.shop.v1", edges[0].Source)
	assert.Equal(t, "mrn://view/presto/memory.shop.v2", edges[0].Target)
}

func TestViewLineage_IgnoresViewsWithoutADefinition(t *testing.T) {
	s := &Source{config: &Config{}}
	view := s.createTableAsset("memory", "shop", "v", "VIEW", "memory", connectorInfoForName("memory"))

	assert.Empty(t, viewLineage([]pluginsdk.Asset{tableAsset("memory", "shop", "orders"), view}))
}
