package metabase

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func refs(t *testing.T, sql string) []tableRef {
	t.Helper()
	found, _ := sqlTableRefs(sql)
	return found
}

func TestSQLTableRefs_FindsTablesAfterFromAndJoin(t *testing.T) {
	found := refs(t, "SELECT c.name, sum(o.total) FROM customers c JOIN orders o ON o.customer_id = c.id LEFT JOIN regions r ON r.id = o.region_id")

	assert.Equal(t, []tableRef{{Name: "customers"}, {Name: "orders"}, {Name: "regions"}}, found)
}

func TestSQLTableRefs_SplitsSchemaQualifiedNames(t *testing.T) {
	found := refs(t, "SELECT * FROM public.customers c JOIN sales.regions AS reg ON reg.id = c.region_id")

	assert.Equal(t, []tableRef{{Schema: "public", Name: "customers"}, {Schema: "sales", Name: "regions"}}, found)
}

func TestSQLTableRefs_TakesTheLastTwoPartsOfAThreePartName(t *testing.T) {
	found := refs(t, "SELECT * FROM analytics.public.orders")

	assert.Equal(t, []tableRef{{Schema: "public", Name: "orders"}}, found)
}

func TestSQLTableRefs_StripsDoubleQuotes(t *testing.T) {
	found := refs(t, `SELECT * FROM "public"."recent_orders" r JOIN "Weird Name" w ON w.id = r.id`)

	assert.Equal(t, []tableRef{{Schema: "public", Name: "recent_orders"}, {Name: "Weird Name"}}, found)
}

func TestSQLTableRefs_StripsBackticksAndBrackets(t *testing.T) {
	found := refs(t, "SELECT * FROM `shop`.`orders` o JOIN [dbo].[customers] c ON c.id = o.customer_id")

	assert.Equal(t, []tableRef{{Schema: "shop", Name: "orders"}, {Schema: "dbo", Name: "customers"}}, found)
}

func TestSQLTableRefs_IgnoresOptionalClauses(t *testing.T) {
	found := refs(t, "SELECT * FROM orders o [[JOIN customers c ON c.id = o.customer_id WHERE c.name = {{name}}]]")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_IgnoresTemplateTags(t *testing.T) {
	found := refs(t, "SELECT * FROM orders WHERE status = {{status}} AND total > {{min_total}}")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_IgnoresCTEs(t *testing.T) {
	found := refs(t, `WITH recent AS (SELECT * FROM "public"."recent_orders"), totals(id, n) AS (SELECT id, count(*) FROM recent GROUP BY id)
SELECT count(*) FROM recent r JOIN sales.regions AS reg ON reg.id = r.id JOIN totals t ON t.id = r.id`)

	assert.Equal(t, []tableRef{{Schema: "public", Name: "recent_orders"}, {Schema: "sales", Name: "regions"}}, found)
}

func TestSQLTableRefs_IgnoresSubqueries(t *testing.T) {
	found := refs(t, "SELECT * FROM (SELECT * FROM orders WHERE total > 10) big JOIN LATERAL (SELECT 1) x ON true")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_IgnoresFunctionCalls(t *testing.T) {
	found := refs(t, "SELECT * FROM generate_series(1, 10) g JOIN orders o ON o.id = g")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_IgnoresComments(t *testing.T) {
	found := refs(t, "SELECT * -- FROM old_orders\nFROM orders /* JOIN customers */ WHERE 1 = 1")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_IsCaseInsensitiveOnKeywords(t *testing.T) {
	found := refs(t, "select * from orders o inner join customers c on c.id = o.customer_id")

	assert.Equal(t, []tableRef{{Name: "orders"}, {Name: "customers"}}, found)
}

func TestSQLTableRefs_ReportsEachTableOnce(t *testing.T) {
	found := refs(t, "SELECT * FROM orders a JOIN orders b ON a.id = b.id")

	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_FindsReferencedCards(t *testing.T) {
	found, cards := sqlTableRefs("SELECT * FROM {{#42-customer-orders}} m JOIN {{ #7 }} q ON q.id = m.id JOIN orders o ON o.id = m.id")

	assert.Equal(t, []int{42, 7}, cards)
	assert.Equal(t, []tableRef{{Name: "orders"}}, found)
}

func TestSQLTableRefs_FindsNothingInEmptySQL(t *testing.T) {
	found, cards := sqlTableRefs("")

	assert.Empty(t, found)
	assert.Empty(t, cards)
}

func TestDatasetQuery_ReadsAPMBQLNativeStage(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"stages": [{"lib/type": "mbql.stage/native", "native": "SELECT 1", "template-tags": []}], "lib/type": "mbql/query", "database": 2}`))
	require.NoError(t, err)

	assert.True(t, q.isNative())
	assert.Equal(t, "SELECT 1", q.sql())
	assert.Empty(t, q.sources())
}

func TestDatasetQuery_ReadsANativeStageThatNestsTheQuery(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"stages": [{"lib/type": "mbql.stage/native", "native": {"query": "SELECT 2"}}]}`))
	require.NoError(t, err)

	assert.Equal(t, "SELECT 2", q.sql())
}

func TestDatasetQuery_ReadsAPMBQLSourceTableAndJoins(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"stages": [{"source-table": 10, "lib/type": "mbql.stage/mbql",
		"joins": [{"lib/type": "mbql/join", "alias": "Regions", "stages": [{"lib/type": "mbql.stage/mbql", "source-table": 12}]}]}],
		"lib/type": "mbql/query", "database": 2}`))
	require.NoError(t, err)

	assert.False(t, q.isNative())
	assert.Empty(t, q.sql())
	assert.Equal(t, []querySource{{TableID: 10}, {TableID: 12}}, q.sources())
}

func TestDatasetQuery_ReadsAPMBQLSourceCard(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"stages": [{"aggregation": [["count", {}]], "lib/type": "mbql.stage/mbql", "source-card": 42}], "lib/type": "mbql/query", "database": 2}`))
	require.NoError(t, err)

	assert.Equal(t, []querySource{{CardID: 42}}, q.sources())
}

func TestDatasetQuery_OnlyTheFirstStageNamesTheSource(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"stages": [{"lib/type": "mbql.stage/mbql", "source-table": 10}, {"lib/type": "mbql.stage/mbql", "filters": []}]}`))
	require.NoError(t, err)

	assert.Equal(t, []querySource{{TableID: 10}}, q.sources())
}

func TestDatasetQuery_ReadsALegacyNativeQuery(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"database": 2, "type": "native", "native": {"query": "SELECT 3", "template-tags": {}}}`))
	require.NoError(t, err)

	assert.True(t, q.isNative())
	assert.Equal(t, "SELECT 3", q.sql())
}

func TestDatasetQuery_ReadsALegacyQueryWithJoins(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"database": 2, "type": "query", "query": {"source-table": 10, "joins": [{"alias": "r", "source-table": 12}, {"alias": "m", "source-table": "card__7"}]}}`))
	require.NoError(t, err)

	assert.False(t, q.isNative())
	assert.Equal(t, []querySource{{TableID: 10}, {TableID: 12}, {CardID: 7}}, q.sources())
}

func TestDatasetQuery_ReadsALegacyCardSource(t *testing.T) {
	q, err := parseDatasetQuery(json.RawMessage(`{"database": 2, "type": "query", "query": {"source-table": "card__42"}}`))
	require.NoError(t, err)

	assert.Equal(t, []querySource{{CardID: 42}}, q.sources())
}

func TestDatasetQuery_ToleratesAnEmptyQuery(t *testing.T) {
	q, err := parseDatasetQuery(nil)
	require.NoError(t, err)

	assert.False(t, q.isNative())
	assert.Empty(t, q.sql())
	assert.Empty(t, q.sources())
}

func TestDatasetQuery_RejectsMalformedJSON(t *testing.T) {
	_, err := parseDatasetQuery(json.RawMessage(`{"stages": [`))

	require.Error(t, err)
}

func TestChartType_NormalisesMetabaseDisplays(t *testing.T) {
	assert.Equal(t, "Table", chartType("table"))
	assert.Equal(t, "Bar", chartType("bar"))
	assert.Equal(t, "Bar", chartType("row"))
	assert.Equal(t, "Bar", chartType("dist_bar"))
	assert.Equal(t, "Line", chartType("line"))
	assert.Equal(t, "Line", chartType("dual_line"))
	assert.Equal(t, "Pie", chartType("pie"))
	assert.Equal(t, "Area", chartType("area"))
	assert.Equal(t, "Area", chartType("treemap"))
	assert.Equal(t, "Scatter", chartType("scatter"))
	assert.Equal(t, "Map", chartType("map"))
	assert.Equal(t, "Gauge", chartType("gauge"))
	assert.Equal(t, "Text", chartType("scalar"))
	assert.Equal(t, "Text", chartType("smartscalar"))
	assert.Equal(t, "Text", chartType("progress"))
}

func TestChartType_FallsBackToOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("funnel"))
	assert.Equal(t, "Other", chartType("waterfall"))
	assert.Equal(t, "Other", chartType("combo"))
	assert.Equal(t, "Other", chartType("sankey"))
	assert.Equal(t, "Other", chartType(""))
}

func TestSlug_LowercasesAndHyphenatesEverythingElse(t *testing.T) {
	assert.Equal(t, "e-commerce-insights", slug("E-commerce Insights"))
	assert.Equal(t, "orders---people", slug("Orders + People"))
	assert.Equal(t, "revenue-by-customer", slug("Revenue by customer"))
	assert.Equal(t, "q3-2026", slug("Q3 2026"))
}

func TestCollectionID_ReadsIntegersAndRoot(t *testing.T) {
	var ids struct {
		A collectionID `json:"a"`
		B collectionID `json:"b"`
		C collectionID `json:"c"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a": 5, "b": "root", "c": null}`), &ids))

	assert.Equal(t, collectionID("5"), ids.A)
	assert.Equal(t, rootCollectionID, ids.B)
	assert.Equal(t, rootCollectionID, ids.C, "null collection_id means the root collection")
}

func TestListOf_ReadsBareArraysAndEnvelopes(t *testing.T) {
	var bare listOf[database]
	require.NoError(t, json.Unmarshal([]byte(`[{"id": 1, "name": "A"}]`), &bare))
	require.Len(t, bare, 1)
	assert.Equal(t, "A", bare[0].Name)

	var wrapped listOf[database]
	require.NoError(t, json.Unmarshal([]byte(`{"data": [{"id": 2, "name": "B"}], "total": 1}`), &wrapped))
	require.Len(t, wrapped, 1)
	assert.Equal(t, "B", wrapped[0].Name)
}

func TestDashboard_ReadsDashcardsOrOrderedCards(t *testing.T) {
	var current dashboard
	require.NoError(t, json.Unmarshal([]byte(`{"id": 1, "dashcards": [{"card_id": 40}, {"card_id": null}, {"card_id": 42}]}`), &current))
	assert.Equal(t, []int{40, 42}, current.cardIDs(), "text cards have no card_id")

	var older dashboard
	require.NoError(t, json.Unmarshal([]byte(`{"id": 1, "ordered_cards": [{"card_id": 7}]}`), &older))
	assert.Equal(t, []int{7}, older.cardIDs())
}
