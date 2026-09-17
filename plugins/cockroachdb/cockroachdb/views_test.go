package cockroachdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The definition CockroachDB stores for the customer_totals view in the e2e
// fixture: every name fully qualified, aliases introduced with AS.
const customerTotalsDefinition = "SELECT c.id, c.name, sum(o.total) AS total FROM shop.public.orders AS o JOIN shop.public.customers AS c ON c.id = o.customer_id GROUP BY c.id, c.name"

func TestViewReferences_ReadsEveryFullyQualifiedNameInAJoin(t *testing.T) {
	refs := viewReferences(customerTotalsDefinition)

	assert.Equal(t, [][]string{
		{"shop", "public", "orders"},
		{"shop", "public", "customers"},
	}, refs)
}

func TestViewReferences_ReadsABareName(t *testing.T) {
	assert.Equal(t, [][]string{{"orders"}}, viewReferences("SELECT id FROM orders"))
}

func TestViewReferences_ReadsASchemaQualifiedName(t *testing.T) {
	assert.Equal(t, [][]string{{"sales", "regions"}}, viewReferences("SELECT code FROM sales.regions WHERE code = 'eu'"))
}

func TestViewReferences_FollowsACommaSeparatedFromList(t *testing.T) {
	refs := viewReferences("SELECT * FROM shop.public.orders o, shop.public.customers AS c, sales.regions WHERE o.customer_id = c.id")

	assert.Equal(t, [][]string{
		{"shop", "public", "orders"},
		{"shop", "public", "customers"},
		{"sales", "regions"},
	}, refs)
}

func TestViewReferences_HandlesJoinsWithoutAliases(t *testing.T) {
	refs := viewReferences("SELECT * FROM shop.public.orders LEFT JOIN shop.public.customers ON customers.id = orders.customer_id")

	assert.Equal(t, [][]string{
		{"shop", "public", "orders"},
		{"shop", "public", "customers"},
	}, refs)
}

func TestViewReferences_SkipsSubqueries(t *testing.T) {
	refs := viewReferences("SELECT * FROM (SELECT id FROM shop.public.orders) AS sub JOIN shop.public.customers AS c ON c.id = sub.id")

	// The inner FROM is still scanned, the parenthesised subquery itself
	// is not a name.
	assert.Equal(t, [][]string{
		{"shop", "public", "orders"},
		{"shop", "public", "customers"},
	}, refs)
}

func TestViewReferences_SkipsFunctionCalls(t *testing.T) {
	refs := viewReferences("SELECT * FROM generate_series(1, 10) AS g JOIN shop.public.orders AS o ON o.id = g")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, refs)
}

func TestViewReferences_SkipsLateralSubqueries(t *testing.T) {
	refs := viewReferences("SELECT * FROM shop.public.orders AS o, LATERAL (SELECT 1) AS x")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, refs)
}

func TestViewReferences_KeepsCaseOfQuotedIdentifiers(t *testing.T) {
	refs := viewReferences(`SELECT * FROM shop."Sales"."Regions"`)

	assert.Equal(t, [][]string{{"shop", "Sales", "Regions"}}, refs)
}

func TestViewReferences_UnescapesDoubledQuotes(t *testing.T) {
	refs := viewReferences(`SELECT * FROM "odd""name"`)

	assert.Equal(t, [][]string{{`odd"name`}}, refs)
}

func TestViewReferences_LowercasesUnquotedNames(t *testing.T) {
	// CockroachDB folds unquoted identifiers to lowercase, so the catalog
	// key for Orders is orders.
	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, viewReferences("SELECT * FROM Shop.Public.Orders"))
}

func TestViewReferences_IgnoresKeywordsInsideStringLiterals(t *testing.T) {
	refs := viewReferences("SELECT * FROM shop.public.orders WHERE note = 'from nowhere join x'")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, refs)
}

func TestViewReferences_IgnoresComments(t *testing.T) {
	refs := viewReferences("SELECT * -- from shop.public.customers\nFROM shop.public.orders /* join sales.regions */")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, refs)
}

func TestViewReferences_DedupesRepeatedNames(t *testing.T) {
	refs := viewReferences("SELECT * FROM shop.public.orders AS a JOIN shop.public.orders AS b ON a.id = b.id")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}}, refs)
}

func TestViewReferences_ReportsCTENamesForTheResolverToDrop(t *testing.T) {
	refs := viewReferences("WITH recent AS (SELECT * FROM shop.public.orders) SELECT * FROM recent")

	assert.Equal(t, [][]string{{"shop", "public", "orders"}, {"recent"}}, refs)
}

func TestViewReferences_SurvivesTypeAnnotations(t *testing.T) {
	// CockroachDB writes casts as value:::TYPE in stored definitions.
	refs := viewReferences("SELECT id FROM shop.public.customers WHERE created_at > (now():::TIMESTAMPTZ - '1 day':::INTERVAL)")

	assert.Equal(t, [][]string{{"shop", "public", "customers"}}, refs)
}

func TestViewReferences_EmptyDefinitionHasNoReferences(t *testing.T) {
	assert.Empty(t, viewReferences(""))
}

func TestViewReferences_TrailingFromIsNotAName(t *testing.T) {
	assert.Empty(t, viewReferences("SELECT 1 FROM"))
}

func knownRelations() map[string]relation {
	return map[string]relation{
		"public.orders":  {Schema: "public", Name: "orders", Kind: relkindTable},
		"sales.regions":  {Schema: "sales", Name: "regions", Kind: relkindTable},
		"sales.orders":   {Schema: "sales", Name: "orders", Kind: relkindTable},
		"public.regions": {Schema: "public", Name: "regions", Kind: relkindView},
	}
}

func TestResolveReference_ThreePartNameInThisDatabase(t *testing.T) {
	r, ok := resolveReference([]string{"shop", "sales", "regions"}, "shop", "public", knownRelations())

	require.True(t, ok)
	assert.Equal(t, "sales.regions", r.key())
}

func TestResolveReference_ThreePartNameInAnotherDatabaseIsNotFound(t *testing.T) {
	// A view cannot read across databases in CockroachDB, and even if the
	// name matched, the relation lives in another discovery pass.
	_, ok := resolveReference([]string{"analytics", "public", "orders"}, "shop", "public", knownRelations())

	assert.False(t, ok)
}

func TestResolveReference_TwoPartNameIsSchemaAndTable(t *testing.T) {
	r, ok := resolveReference([]string{"sales", "orders"}, "shop", "public", knownRelations())

	require.True(t, ok)
	assert.Equal(t, "sales.orders", r.key())
}

func TestResolveReference_TwoPartNameFallsBackToDatabaseAndPublicTable(t *testing.T) {
	r, ok := resolveReference([]string{"shop", "orders"}, "shop", "sales", knownRelations())

	require.True(t, ok)
	assert.Equal(t, "public.orders", r.key())
}

func TestResolveReference_BareNamePrefersTheViewsOwnSchema(t *testing.T) {
	r, ok := resolveReference([]string{"orders"}, "shop", "sales", knownRelations())

	require.True(t, ok)
	assert.Equal(t, "sales.orders", r.key())
}

func TestResolveReference_BareNameFallsBackToPublic(t *testing.T) {
	r, ok := resolveReference([]string{"orders"}, "shop", "reporting", knownRelations())

	require.True(t, ok)
	assert.Equal(t, "public.orders", r.key())
}

func TestResolveReference_UnknownNameIsNotFound(t *testing.T) {
	_, ok := resolveReference([]string{"nowhere"}, "shop", "public", knownRelations())

	assert.False(t, ok)
}

func TestResolveReference_TooManyPartsIsNotFound(t *testing.T) {
	_, ok := resolveReference([]string{"a", "b", "c", "d"}, "shop", "public", knownRelations())

	assert.False(t, ok)
}
