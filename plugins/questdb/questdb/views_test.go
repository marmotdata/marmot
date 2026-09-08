package questdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewReferences_SimpleFrom(t *testing.T) {
	assert.Equal(t, []string{"trades"}, viewReferences("SELECT * FROM trades"))
}

func TestViewReferences_JoinWithAliases(t *testing.T) {
	sql := "SELECT t.ts, t.symbol, t.price, v.name FROM trades t JOIN venues v ON t.venue = v.name;"

	assert.Equal(t, []string{"trades", "venues"}, viewReferences(sql))
}

func TestViewReferences_ExplicitAsAliases(t *testing.T) {
	sql := "SELECT * FROM trades AS t LEFT JOIN venues AS v ON t.venue = v.name"

	assert.Equal(t, []string{"trades", "venues"}, viewReferences(sql))
}

func TestViewReferences_QuestDBJoinFlavours(t *testing.T) {
	sql := "SELECT * FROM trades ASOF JOIN quotes LT JOIN venues SPLICE JOIN books"

	assert.Equal(t, []string{"trades", "quotes", "venues", "books"}, viewReferences(sql))
}

func TestViewReferences_QuotedNames(t *testing.T) {
	assert.Equal(t, []string{"my table"}, viewReferences(`SELECT * FROM "my table"`))
	assert.Equal(t, []string{"trades"}, viewReferences(`SELECT * FROM 'trades'`))
}

func TestViewReferences_UnescapesDoubledQuotes(t *testing.T) {
	assert.Equal(t, []string{`say "hi"`}, viewReferences(`SELECT * FROM "say ""hi"""`))
}

func TestViewReferences_SkipsSubqueries(t *testing.T) {
	sql := "SELECT * FROM (SELECT ts, price FROM trades WHERE price > 1) WHERE ts > now()"

	assert.Equal(t, []string{"trades"}, viewReferences(sql))
}

func TestViewReferences_SkipsTableFunctions(t *testing.T) {
	assert.Empty(t, viewReferences("SELECT * FROM tables()"))
	assert.Empty(t, viewReferences("SELECT * FROM table_columns('trades')"))
}

func TestViewReferences_CommaSeparatedList(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, viewReferences("SELECT * FROM a, b x, c AS y WHERE a.id = b.id"))
}

func TestViewReferences_DedupesRepeatedTables(t *testing.T) {
	sql := "SELECT * FROM trades a JOIN trades b ON a.ts = b.ts JOIN TRADES c ON a.ts = c.ts"

	assert.Equal(t, []string{"trades"}, viewReferences(sql))
}

func TestViewReferences_StopsAtKeywords(t *testing.T) {
	assert.Equal(t, []string{"trades"}, viewReferences("SELECT * FROM trades WHERE price > 1"))
	assert.Equal(t, []string{"trades"}, viewReferences("SELECT ts, avg(price) FROM trades SAMPLE BY 1h"))
	assert.Equal(t, []string{"trades"}, viewReferences("SELECT * FROM trades LATEST ON ts PARTITION BY symbol"))
	assert.Equal(t, []string{"trades"}, viewReferences("SELECT * FROM trades TIMESTAMP(ts) LIMIT 10"))
}

func TestViewReferences_KeywordsAreCaseInsensitive(t *testing.T) {
	assert.Equal(t, []string{"trades", "venues"}, viewReferences("select * from trades t join venues v on t.venue = v.name"))
}

func TestViewReferences_IgnoresComments(t *testing.T) {
	sql := "SELECT * -- FROM nowhere\nFROM trades /* JOIN venues */ WHERE 1 = 1"

	assert.Equal(t, []string{"trades"}, viewReferences(sql))
}

func TestViewReferences_KeepsDottedNamesWhole(t *testing.T) {
	assert.Equal(t, []string{"sys.text_import_log"}, viewReferences("SELECT * FROM sys.text_import_log"))
}

func TestViewReferences_UnionReadsBothSides(t *testing.T) {
	assert.Equal(t, []string{"trades", "trades_archive"}, viewReferences("SELECT ts FROM trades UNION ALL SELECT ts FROM trades_archive"))
}

func TestViewReferences_EmptyOrKeywordOnlySQL(t *testing.T) {
	assert.Empty(t, viewReferences(""))
	assert.Empty(t, viewReferences("SELECT 1"))
	assert.Empty(t, viewReferences("SELECT * FROM"))
}

func TestStripComments_RemovesBothStyles(t *testing.T) {
	assert.Equal(t, "a \n b    c", stripComments("a -- line\n b /* block */  c"))
}

func TestStripComments_HandlesUnterminatedComment(t *testing.T) {
	assert.Equal(t, "a ", stripComments("a /* never closed"))
	assert.Equal(t, "a ", stripComments("a -- never ends"))
}
