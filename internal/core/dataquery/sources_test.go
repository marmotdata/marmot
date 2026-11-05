package dataquery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Each preview is written the way that engine names a table, checked against what the plugin's discovery records.
func TestPreviewStatementsPerDialect(t *testing.T) {
	cases := []struct {
		plugin string
		meta   map[string]any
		want   string
	}{
		{"timescale", map[string]any{"schema": "metrics", "table_name": "cpu"}, "SELECT *\nFROM metrics.cpu\nLIMIT 10"},
		{"cockroachdb", map[string]any{"schema": "public", "table_name": "orders"}, "SELECT *\nFROM orders\nLIMIT 10"},
		{"mysql", map[string]any{"database": "shop", "table_name": "order"}, "SELECT *\nFROM `shop`.`order`\nLIMIT 10"},
		{"mariadb", map[string]any{"database": "shop", "table_name": "we`ird"}, "SELECT *\nFROM `shop`.`we``ird`\nLIMIT 10"},
		{"clickhouse", map[string]any{"database": "analytics", "table_name": "Events"}, "SELECT *\nFROM analytics.\"Events\"\nLIMIT 10"},
		{"oracle", map[string]any{"schema": "HR", "table_name": "EMPLOYEES"}, "SELECT *\nFROM \"HR\".\"EMPLOYEES\"\nFETCH FIRST 10 ROWS ONLY"},
		{"oracle", map[string]any{"schema": "hr", "table_name": "orders"}, "SELECT *\nFROM \"hr\".\"orders\"\nFETCH FIRST 10 ROWS ONLY"},
		{"sqlite", map[string]any{"table_name": "orders"}, "SELECT *\nFROM orders\nLIMIT 10"},
		{"duckdb", map[string]any{"schema": "main", "table_name": "orders"}, "SELECT *\nFROM orders\nLIMIT 10"},
		{"duckdb", map[string]any{"schema": "sales", "table_name": "revenue_by_country"}, "SELECT *\nFROM sales.revenue_by_country\nLIMIT 10"},
		{"bigquery", map[string]any{"project_id": "p-41j1", "dataset_id": "telemetry", "table_id": "tenants_daily"}, "SELECT *\nFROM `p-41j1.telemetry.tenants_daily`\nLIMIT 10"},
		{"bigquery", map[string]any{"project_id": "p", "dataset_id": "d", "table_id": "a`b"}, "SELECT *\nFROM `p.d.a\\`b`\nLIMIT 10"},
	}
	for _, c := range cases {
		locator, ok := sourceTableLocators[c.plugin]
		if assert.True(t, ok, c.plugin) {
			assert.Equal(t, c.want, locator.statement(c.meta, "ignored"), c.plugin)
		}
	}
}

// Sources only list the tables their queries can reach, defaulting the database as each plugin does.
func TestPreviewMetadataPins(t *testing.T) {
	assert.Equal(t, map[string]string{"database": "shop"}, sourceTableLocators["timescale"].metadata(map[string]any{"database": "shop"}),
		"timescale queries the discovered database when no query database is set")
	assert.Equal(t, map[string]string{"database": "postgres"}, sourceTableLocators["postgresql"].metadata(map[string]any{"database": "shop"}),
		"postgresql does not")
	assert.Equal(t, map[string]string{"database": "defaultdb"}, sourceTableLocators["cockroachdb"].metadata(map[string]any{}))
	assert.Equal(t, map[string]string{"database": "reporting"}, sourceTableLocators["mariadb"].metadata(map[string]any{"database": "shop", "query_database": "reporting"}))
	assert.Equal(t, map[string]string{"project_id": "p-41j1"}, sourceTableLocators["bigquery"].metadata(map[string]any{"project_id": "p-41j1"}))
}

// Every plugin that answers queries needs a locator, or its tables fall back to a bare name its engine may not resolve.
func TestEveryQueryablePluginHasALocator(t *testing.T) {
	files, err := filepath.Glob("../../../plugins/*/*/query.go")
	require.NoError(t, err)
	if len(files) == 0 {
		t.Skip("plugins are not in this tree")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		if !strings.Contains(string(src), ") PlanQuery(") {
			continue
		}
		id := filepath.Base(filepath.Dir(filepath.Dir(f)))
		_, ok := sourceTableLocators[id]
		assert.True(t, ok, "plugin %s answers queries but has no preview locator", id)
	}
}
