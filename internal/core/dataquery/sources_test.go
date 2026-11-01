package dataquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuoteIdent(t *testing.T) {
	assert.Equal(t, "orders", quoteIdent("orders"))
	assert.Equal(t, `"Orders"`, quoteIdent("Orders"), "mixed case must be quoted to survive folding")
	assert.Equal(t, `"order lines"`, quoteIdent("order lines"))
	assert.Equal(t, `"a""b"`, quoteIdent(`a"b`), "embedded quotes are doubled")
}

func TestPostgresPreviewStatement(t *testing.T) {
	pg := sourceTableLocators["postgresql"]
	assert.Equal(t, "SELECT *\nFROM orders\nLIMIT 10",
		pg.statement(map[string]any{"schema": "public", "table_name": "orders"}, "orders"),
		"public needs no schema prefix")
	assert.Equal(t, "SELECT *\nFROM staging.\"Orders\"\nLIMIT 10",
		pg.statement(map[string]any{"schema": "staging", "table_name": "Orders"}, "Orders"))
	assert.Equal(t, map[string]string{"database": "shop"}, pg.metadata(map[string]any{"query_database": "shop"}))
	assert.Equal(t, map[string]string{"database": "postgres"}, pg.metadata(map[string]any{}),
		"the plugin queries the postgres database by default")
}

func TestTrinoPreviewStatement(t *testing.T) {
	trino := sourceTableLocators["trino"]
	assert.Equal(t, "SELECT *\nFROM postgresql.public.orders\nLIMIT 10",
		trino.statement(map[string]any{"catalog": "postgresql", "schema": "public", "table_name": "orders"}, "orders"))
}
