package redash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refStrings renders the extracted references so a test can assert on them
// as plain dotted names.
func refStrings(sql string) []string {
	var out []string
	for _, ref := range extractTableRefs(sql) {
		out = append(out, ref.String())
	}
	return out
}

func TestExtractTableRefs_FindsTheTableAfterFrom(t *testing.T) {
	assert.Equal(t, []string{"orders"}, refStrings("SELECT * FROM orders"))
}

func TestExtractTableRefs_FindsASchemaQualifiedTable(t *testing.T) {
	assert.Equal(t, []string{"public.orders"}, refStrings("SELECT * FROM public.orders"))
}

func TestExtractTableRefs_FindsAThreePartName(t *testing.T) {
	assert.Equal(t, []string{"analytics.public.orders"}, refStrings("SELECT * FROM analytics.public.orders"))
}

func TestExtractTableRefs_FindsAJoinedTable(t *testing.T) {
	assert.Equal(t, []string{"orders", "customers"},
		refStrings("SELECT * FROM orders o JOIN customers c ON c.id = o.customer_id"))
}

func TestExtractTableRefs_FindsTablesAcrossJoinFlavours(t *testing.T) {
	sql := "SELECT * FROM a LEFT OUTER JOIN b ON true INNER JOIN c ON true CROSS JOIN d"
	assert.Equal(t, []string{"a", "b", "c", "d"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresCommonTableExpressionNames(t *testing.T) {
	sql := "WITH recent AS (SELECT * FROM orders) SELECT * FROM recent"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_FindsTablesInsideEveryCTEBody(t *testing.T) {
	sql := "WITH a AS (SELECT * FROM orders), b AS (SELECT * FROM customers) SELECT * FROM a JOIN b ON true"
	assert.Equal(t, []string{"orders", "customers"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresDerivedTables(t *testing.T) {
	sql := "SELECT * FROM (SELECT id FROM orders) t"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresFunctionCalls(t *testing.T) {
	sql := "SELECT * FROM generate_series(1, 10)"
	assert.Empty(t, refStrings(sql))
}

func TestExtractTableRefs_IgnoresSchemaQualifiedFunctionCalls(t *testing.T) {
	sql := "SELECT * FROM public.my_function(1)"
	assert.Empty(t, refStrings(sql))
}

func TestExtractTableRefs_IgnoresTheFromInsideExtract(t *testing.T) {
	// extract(year from created_at) spells an argument with FROM. Reading it
	// as a clause would catalogue the column as a table.
	sql := "SELECT extract(year FROM created_at) FROM orders"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresTheFromInsideSubstring(t *testing.T) {
	sql := "SELECT substring(name FROM 1 FOR 3) FROM customers"
	assert.Equal(t, []string{"customers"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresRedashParameterPlaceholders(t *testing.T) {
	sql := "SELECT * FROM orders WHERE total > {{min_total}} AND day = '{{ day }}'"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresRedashTemplateBlocks(t *testing.T) {
	sql := "SELECT * FROM orders {% if x %} WHERE 1=1 {% endif %}"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_ReadsDoubleQuotedIdentifiers(t *testing.T) {
	assert.Equal(t, []string{"My Schema.Order Items"}, refStrings(`SELECT * FROM "My Schema"."Order Items"`))
}

func TestExtractTableRefs_ReadsBacktickIdentifiers(t *testing.T) {
	assert.Equal(t, []string{"shop.orders"}, refStrings("SELECT * FROM `shop`.`orders`"))
}

func TestExtractTableRefs_ReadsBracketIdentifiers(t *testing.T) {
	assert.Equal(t, []string{"dbo.orders"}, refStrings("SELECT * FROM [dbo].[orders]"))
}

func TestExtractTableRefs_IgnoresLineComments(t *testing.T) {
	sql := "SELECT * FROM orders -- FROM secrets\n WHERE 1=1"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresBlockComments(t *testing.T) {
	sql := "SELECT * /* FROM secrets */ FROM orders"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresStringLiterals(t *testing.T) {
	sql := "SELECT date_trunc('day', created_at) FROM orders WHERE note = 'from nowhere'"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresEscapedQuotesInsideLiterals(t *testing.T) {
	sql := "SELECT * FROM orders WHERE note = 'it''s from here'"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_DeduplicatesRepeatedTables(t *testing.T) {
	sql := "SELECT * FROM orders UNION ALL SELECT * FROM orders"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_IgnoresTheOnlyKeyword(t *testing.T) {
	assert.Equal(t, []string{"orders"}, refStrings("SELECT * FROM ONLY orders"))
}

func TestExtractTableRefs_IgnoresLateralJoins(t *testing.T) {
	sql := "SELECT * FROM orders o, LATERAL unnest(o.items) i"
	assert.Equal(t, []string{"orders"}, refStrings(sql))
}

func TestExtractTableRefs_ReadsTheRealRedashQueryFromTheFixture(t *testing.T) {
	assert.Equal(t, []string{"public.orders", "public.customers"}, refStrings(ordersSQL))
}

func TestExtractTableRefs_FindsNothingInANonSQLQuery(t *testing.T) {
	assert.Empty(t, refStrings(`{"query": {"match_all": {}}}`))
}

func TestTableRef_SplitsIntoDatabaseSchemaAndTable(t *testing.T) {
	refs := extractTableRefs("SELECT * FROM analytics.public.orders")

	require.Len(t, refs, 1)
	assert.Equal(t, "analytics", refs[0].Database())
	assert.Equal(t, "public", refs[0].Schema())
	assert.Equal(t, "orders", refs[0].Table())
}

func TestTableRef_LeavesTheSchemaEmptyForABareName(t *testing.T) {
	refs := extractTableRefs("SELECT * FROM orders")

	require.Len(t, refs, 1)
	assert.Equal(t, "", refs[0].Schema())
	assert.Equal(t, "", refs[0].Database())
}
