package starrocks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryReferences_FindsTheTableAfterFrom(t *testing.T) {
	refs := queryReferences("SELECT a, b FROM orders")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_FindsBothSidesOfAJoin(t *testing.T) {
	refs := queryReferences("SELECT o.order_date, c.country FROM orders o JOIN customers c ON o.customer_id = c.customer_id")

	assert.Equal(t, []string{"orders", "customers"}, refs)
}

func TestQueryReferences_SkipsTheAliasAfterATableName(t *testing.T) {
	// A bare alias must not be mistaken for a second table.
	refs := queryReferences("SELECT * FROM orders o, customers c")

	assert.Equal(t, []string{"orders", "customers"}, refs)
}

func TestQueryReferences_SkipsAnAliasIntroducedByAs(t *testing.T) {
	refs := queryReferences("SELECT * FROM orders AS o")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_KeepsTheDatabaseQualifier(t *testing.T) {
	refs := queryReferences("SELECT * FROM shop.orders")

	assert.Equal(t, []string{"shop.orders"}, refs)
}

func TestQueryReferences_KeepsTheCatalogQualifier(t *testing.T) {
	refs := queryReferences("SELECT * FROM default_catalog.shop.orders")

	assert.Equal(t, []string{"default_catalog.shop.orders"}, refs)
}

func TestQueryReferences_StripsBackticks(t *testing.T) {
	refs := queryReferences("SELECT * FROM `shop`.`orders`")

	assert.Equal(t, []string{"shop.orders"}, refs)
}

func TestQueryReferences_DoesNotStopAtAWhereClause(t *testing.T) {
	// WHERE is reserved, so it must not be consumed as an alias and hide
	// a later JOIN.
	refs := queryReferences("SELECT * FROM orders WHERE amount > 0")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_IgnoresTableNamesInsideStringLiterals(t *testing.T) {
	refs := queryReferences("SELECT * FROM orders WHERE note = 'from customers'")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_IgnoresCommentedOutTables(t *testing.T) {
	refs := queryReferences("SELECT * FROM orders -- FROM customers\n")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_IgnoresBlockCommentedTables(t *testing.T) {
	refs := queryReferences("SELECT * FROM orders /* FROM customers */")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_ReturnsEachTableOnce(t *testing.T) {
	refs := queryReferences("SELECT * FROM orders a JOIN orders b ON a.id = b.id")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_SkipsASubqueryInsteadOfGuessing(t *testing.T) {
	// A parenthesised derived table has no name to point at, but the
	// tables inside it still do.
	refs := queryReferences("SELECT * FROM (SELECT id FROM orders) t")

	assert.Equal(t, []string{"orders"}, refs)
}

func TestQueryReferences_FindsTablesAcrossSeveralJoins(t *testing.T) {
	refs := queryReferences("SELECT * FROM a LEFT JOIN b ON a.id = b.id INNER JOIN c ON b.id = c.id")

	assert.Equal(t, []string{"a", "b", "c"}, refs)
}

func TestQueryReferences_EmptyQueryYieldsNothing(t *testing.T) {
	assert.Empty(t, queryReferences(""))
}

func TestResolveReference_BareNameUsesTheCurrentDatabase(t *testing.T) {
	db, object, ok := resolveReference("orders", "default_catalog", "shop")

	require.True(t, ok)
	assert.Equal(t, "shop", db)
	assert.Equal(t, "orders", object)
}

func TestResolveReference_TwoPartNameCarriesItsDatabase(t *testing.T) {
	db, object, ok := resolveReference("warehouse.orders", "default_catalog", "shop")

	require.True(t, ok)
	assert.Equal(t, "warehouse", db)
	assert.Equal(t, "orders", object)
}

func TestResolveReference_ThreePartNameInTheConfiguredCatalogResolves(t *testing.T) {
	db, object, ok := resolveReference("default_catalog.shop.orders", "default_catalog", "shop")

	require.True(t, ok)
	assert.Equal(t, "shop", db)
	assert.Equal(t, "orders", object)
}

func TestResolveReference_NameInAnotherCatalogIsOutOfScope(t *testing.T) {
	// This run only discovered one catalog, so an object in a different
	// one is not an asset we can point an edge at.
	_, _, ok := resolveReference("hive_catalog.shop.orders", "default_catalog", "shop")

	assert.False(t, ok)
}

func TestResolveReference_EmptyNameIsNotAReference(t *testing.T) {
	_, _, ok := resolveReference("", "default_catalog", "shop")

	assert.False(t, ok)
}
