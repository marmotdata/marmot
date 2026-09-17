package hive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewReferences_QualifiesABareTableWithTheViewsDatabase(t *testing.T) {
	assert.Equal(t, []string{"sales.orders"},
		viewReferences("SELECT * FROM orders", "sales"))
}

func TestViewReferences_KeepsAnExplicitDatabase(t *testing.T) {
	assert.Equal(t, []string{"finance.ledger"},
		viewReferences("SELECT * FROM finance.ledger", "sales"))
}

func TestViewReferences_StripsBackticksFromTheExpandedQuery(t *testing.T) {
	query := "SELECT `c`.`name` FROM `sales`.`orders` `o` JOIN `sales`.`customers` `c` ON `o`.`customer_id` = `c`.`id`"

	assert.Equal(t, []string{"sales.orders", "sales.customers"},
		viewReferences(query, "sales"))
}

func TestViewReferences_FollowsEveryJoin(t *testing.T) {
	query := `SELECT 1 FROM a
		LEFT OUTER JOIN b ON a.id = b.id
		INNER JOIN c ON a.id = c.id
		JOIN d ON a.id = d.id`

	assert.Equal(t, []string{"sales.a", "sales.b", "sales.c", "sales.d"},
		viewReferences(query, "sales"))
}

func TestViewReferences_FollowsACommaSeparatedFromList(t *testing.T) {
	assert.Equal(t, []string{"sales.a", "sales.b", "sales.c"},
		viewReferences("SELECT 1 FROM a x, b AS y, c WHERE x.id = y.id", "sales"))
}

func TestViewReferences_SkipsSubqueries(t *testing.T) {
	query := "SELECT 1 FROM (SELECT id FROM orders) o JOIN customers c ON o.id = c.id"

	assert.Equal(t, []string{"sales.orders", "sales.customers"},
		viewReferences(query, "sales"))
}

func TestViewReferences_IgnoresKeywordsInsideStringLiterals(t *testing.T) {
	assert.Equal(t, []string{"sales.orders"},
		viewReferences("SELECT 'FROM nowhere' AS label FROM orders", "sales"))
}

func TestViewReferences_IgnoresComments(t *testing.T) {
	query := "SELECT 1 -- FROM ghost\nFROM orders /* JOIN phantom */"

	assert.Equal(t, []string{"sales.orders"},
		viewReferences(query, "sales"))
}

func TestViewReferences_DeduplicatesRepeatedTables(t *testing.T) {
	assert.Equal(t, []string{"sales.orders"},
		viewReferences("SELECT 1 FROM orders a JOIN orders b ON a.id = b.parent_id", "sales"))
}

func TestViewReferences_LowercasesNames(t *testing.T) {
	assert.Equal(t, []string{"sales.orders"},
		viewReferences("SELECT 1 FROM Sales.Orders", "other"))
}

func TestViewReferences_KeepsTheLastTwoPartsOfAThreePartName(t *testing.T) {
	assert.Equal(t, []string{"sales.orders"},
		viewReferences("SELECT 1 FROM hive.sales.orders", "other"))
}

func TestViewReferences_EmptyQueryHasNoReferences(t *testing.T) {
	assert.Empty(t, viewReferences("", "sales"))
}

func TestViewReferences_IsNotFooledByAnAliasThatLooksLikeATable(t *testing.T) {
	// "o" and "c" are aliases, not tables, so they must not be reported.
	query := "SELECT c.name FROM sales.orders o JOIN sales.customers c ON o.customer_id = c.id GROUP BY c.name"

	assert.Equal(t, []string{"sales.orders", "sales.customers"},
		viewReferences(query, "sales"))
}

func TestQualify_UsesTheDefaultDatabaseForBareNames(t *testing.T) {
	assert.Equal(t, "sales.orders", qualify("orders", "sales"))
	assert.Equal(t, "sales.orders", qualify("ORDERS", "Sales"))
}

func TestQualify_KeepsAQualifiedName(t *testing.T) {
	assert.Equal(t, "finance.ledger", qualify("finance.ledger", "sales"))
}

func TestQualify_EmptyNameIsDropped(t *testing.T) {
	assert.Equal(t, "", qualify("", "sales"))
}

func TestTokenize_KeepsDottedBacktickedIdentifiersTogether(t *testing.T) {
	tokens := tokenize("FROM `sales`.`orders` o")

	assert.Equal(t, "FROM", tokens[0].text)
	assert.Equal(t, "sales.orders", tokens[1].text)
	assert.True(t, tokens[1].quoted)
	assert.Equal(t, "o", tokens[2].text)
}

func TestTokenize_UnescapesDoubledBackticks(t *testing.T) {
	tokens := tokenize("FROM `odd``name`")

	assert.Equal(t, "odd`name", tokens[1].text)
}
