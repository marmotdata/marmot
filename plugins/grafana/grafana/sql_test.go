package grafana

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// datasourceMap pins, per Grafana data source type, the provider and
// name shape of the Marmot plugin that catalogues the same tables.

func TestDatasourceMap_PostgreSQLIsBareTableName(t *testing.T) {
	src, ok := datasourceMap["grafana-postgresql-datasource"]
	require.True(t, ok)

	assert.Equal(t, "PostgreSQL", src.Provider)
	assert.Equal(t, "orders", src.tableName("shop", "public", "orders"))
}

func TestDatasourceMap_OlderGrafanaCallsPostgreSQLPostgres(t *testing.T) {
	src, ok := datasourceMap["postgres"]
	require.True(t, ok)

	assert.Equal(t, "PostgreSQL", src.Provider)
	assert.Equal(t, "orders", src.tableName("shop", "public", "orders"))
}

func TestDatasourceMap_MySQLIsBareTableName(t *testing.T) {
	src, ok := datasourceMap["mysql"]
	require.True(t, ok)

	assert.Equal(t, "MySQL", src.Provider)
	assert.Equal(t, "orders", src.tableName("shop", "", "orders"))
}

func TestDatasourceMap_SQLServerIsDatabaseSchemaTable(t *testing.T) {
	src, ok := datasourceMap["mssql"]
	require.True(t, ok)

	assert.Equal(t, "SQL Server", src.Provider)
	assert.Equal(t, "sales.dbo.orders", src.tableName("sales", "dbo", "orders"))
}

func TestDatasourceMap_ClickHouseIsBareTableName(t *testing.T) {
	src, ok := datasourceMap["grafana-clickhouse-datasource"]
	require.True(t, ok)

	assert.Equal(t, "ClickHouse", src.Provider)
	assert.Equal(t, "events", src.tableName("default", "default", "events"))
}

func TestDatasourceMap_CommunityClickHouseIsBareTableName(t *testing.T) {
	src, ok := datasourceMap["vertamedia-clickhouse-datasource"]
	require.True(t, ok)

	assert.Equal(t, "ClickHouse", src.Provider)
	assert.Equal(t, "events", src.tableName("default", "default", "events"))
}

func TestDatasourceMap_HasExactlyTheSQLSources(t *testing.T) {
	assert.Len(t, datasourceMap, 6)
	assert.NotContains(t, datasourceMap, "prometheus")
	assert.NotContains(t, datasourceMap, "elasticsearch")
}

func TestSQLServerName_DefaultsTheSchemaToDbo(t *testing.T) {
	assert.Equal(t, "sales.dbo.orders", sqlServerName("sales", "", "orders"))
}

func TestSQLServerName_NeedsADatabase(t *testing.T) {
	assert.Equal(t, "", sqlServerName("", "dbo", "orders"))
}

// extractTables

func TestExtractTables_BareTable(t *testing.T) {
	assert.Equal(t, []tableRef{{Table: "orders"}}, extractTables("SELECT * FROM orders"))
}

func TestExtractTables_SchemaQualifiedTable(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}},
		extractTables("SELECT * FROM public.orders"))
}

func TestExtractTables_ThreePartName(t *testing.T) {
	assert.Equal(t, []tableRef{{Database: "sales", Schema: "dbo", Table: "orders"}},
		extractTables("SELECT * FROM sales.dbo.orders"))
}

func TestExtractTables_FourPartNameKeepsTheLastThree(t *testing.T) {
	assert.Equal(t, []tableRef{{Database: "sales", Schema: "dbo", Table: "orders"}},
		extractTables("SELECT * FROM srv.sales.dbo.orders"))
}

func TestExtractTables_JoinedTables(t *testing.T) {
	sql := "SELECT o.created_at AS time, sum(o.total) FROM public.orders o JOIN public.customers c ON c.id = o.customer_id WHERE $__timeFilter(o.created_at) GROUP BY 1"

	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}, {Schema: "public", Table: "customers"}},
		extractTables(sql))
}

func TestExtractTables_EveryJoinKind(t *testing.T) {
	sql := "SELECT * FROM a LEFT OUTER JOIN b ON a.id = b.id INNER JOIN c USING (id) CROSS JOIN d"

	assert.Equal(t, []tableRef{{Table: "a"}, {Table: "b"}, {Table: "c"}, {Table: "d"}}, extractTables(sql))
}

func TestExtractTables_CommaSeparatedFromList(t *testing.T) {
	assert.Equal(t, []tableRef{{Table: "orders"}, {Table: "customers"}, {Table: "products"}},
		extractTables("SELECT * FROM orders o, customers AS c, products WHERE o.customer_id = c.id"))
}

func TestExtractTables_DoubleQuotedIdentifiersKeepTheirCase(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "Public", Table: "Order Lines"}},
		extractTables(`SELECT * FROM "Public"."Order Lines"`))
}

func TestExtractTables_BacktickIdentifiers(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "shop", Table: "orders"}},
		extractTables("SELECT * FROM `shop`.`orders`"))
}

func TestExtractTables_BracketIdentifiers(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "dbo", Table: "orders"}},
		extractTables("SELECT * FROM [dbo].[orders]"))
}

func TestExtractTables_IgnoresCTENames(t *testing.T) {
	sql := "WITH recent AS (SELECT * FROM orders WHERE created_at > now() - interval '1 day'), top AS (SELECT * FROM recent LIMIT 10) SELECT * FROM top JOIN customers c ON c.id = top.customer_id"

	assert.Equal(t, []tableRef{{Table: "orders"}, {Table: "customers"}}, extractTables(sql))
}

func TestExtractTables_IgnoresRecursiveCTENames(t *testing.T) {
	sql := "WITH RECURSIVE tree(id, parent) AS (SELECT id, parent FROM categories UNION ALL SELECT c.id, c.parent FROM categories c JOIN tree ON tree.id = c.parent) SELECT * FROM tree"

	assert.Equal(t, []tableRef{{Table: "categories"}}, extractTables(sql))
}

func TestExtractTables_ASchemaQualifiedNameIsNeverACTE(t *testing.T) {
	sql := "WITH orders AS (SELECT 1) SELECT * FROM public.orders"

	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}}, extractTables(sql))
}

func TestExtractTables_LooksInsideSubqueries(t *testing.T) {
	sql := "SELECT * FROM (SELECT customer_id FROM orders) o JOIN customers c ON c.id = o.customer_id"

	assert.Equal(t, []tableRef{{Table: "orders"}, {Table: "customers"}}, extractTables(sql))
}

func TestExtractTables_IgnoresLateralSubqueries(t *testing.T) {
	sql := "SELECT * FROM customers c, LATERAL (SELECT * FROM orders WHERE customer_id = c.id) o"

	assert.Equal(t, []tableRef{{Table: "customers"}, {Table: "orders"}}, extractTables(sql))
}

func TestExtractTables_IgnoresFunctionCalls(t *testing.T) {
	sql := "SELECT * FROM generate_series(1, 10) g JOIN unnest(ARRAY[1,2]) u ON true"

	assert.Empty(t, extractTables(sql))
}

func TestExtractTables_IgnoresTemplateVariables(t *testing.T) {
	assert.Empty(t, extractTables("SELECT * FROM ${table}"))
	assert.Empty(t, extractTables("SELECT * FROM $table"))
	assert.Empty(t, extractTables("SELECT * FROM [[table]]"))
	assert.Empty(t, extractTables("SELECT * FROM public.$table"))
	assert.Empty(t, extractTables("SELECT * FROM public.${table:raw}"))
}

func TestExtractTables_IgnoresGrafanaMacros(t *testing.T) {
	assert.Empty(t, extractTables("SELECT $__timeGroup(created_at, '1h') FROM $__table"))
}

func TestExtractTables_TemplateVariableInAStringIsNotATable(t *testing.T) {
	sql := "SELECT name, total FROM public.customer_totals WHERE name = '${customer}' ORDER BY total DESC"

	assert.Equal(t, []tableRef{{Schema: "public", Table: "customer_totals"}}, extractTables(sql))
}

func TestExtractTables_IgnoresFromInsideStringLiterals(t *testing.T) {
	sql := "SELECT 'hello from nowhere' AS greeting, 'it''s from here' FROM orders"

	assert.Equal(t, []tableRef{{Table: "orders"}}, extractTables(sql))
}

func TestExtractTables_IgnoresFromInsideComments(t *testing.T) {
	sql := "-- pulled from legacy\nSELECT * /* join staging */ FROM orders"

	assert.Equal(t, []tableRef{{Table: "orders"}}, extractTables(sql))
}

func TestExtractTables_DeduplicatesRepeatedTables(t *testing.T) {
	sql := "SELECT * FROM orders a JOIN orders b ON a.id = b.parent_id"

	assert.Equal(t, []tableRef{{Table: "orders"}}, extractTables(sql))
}

func TestExtractTables_ClickHouseArrayJoinIsNotATable(t *testing.T) {
	sql := "SELECT arr FROM events ARRAY JOIN tags AS arr FINAL"

	assert.Equal(t, []tableRef{{Table: "events"}}, extractTables(sql))
}

func TestExtractTables_KeywordAfterFromIsNotATable(t *testing.T) {
	assert.Empty(t, extractTables("SELECT * FROM ONLY"))
	assert.Empty(t, extractTables("INSERT INTO t SELECT * FROM VALUES (1)"))
}

func TestExtractTables_EmptyQuery(t *testing.T) {
	assert.Empty(t, extractTables(""))
	assert.Empty(t, extractTables("SELECT 1"))
}

// sqlTableMRNs

func TestSQLTableMRNs_NamesPostgreSQLTablesBare(t *testing.T) {
	sql := "SELECT * FROM public.orders o JOIN public.customers c ON c.id = o.customer_id"

	assert.Equal(t, []string{"mrn://table/postgresql/orders", "mrn://table/postgresql/customers"},
		sqlTableMRNs("grafana-postgresql-datasource", "shop", sql))
}

func TestSQLTableMRNs_UsesTheDataSourcesDatabaseForSQLServer(t *testing.T) {
	assert.Equal(t, []string{"mrn://table/sql-server/sales.dbo.orders"},
		sqlTableMRNs("mssql", "sales", "SELECT * FROM orders"))
}

func TestSQLTableMRNs_QueryDatabaseWinsOverTheDataSourcesForSQLServer(t *testing.T) {
	assert.Equal(t, []string{"mrn://table/sql-server/archive.dbo.orders"},
		sqlTableMRNs("mssql", "sales", "SELECT * FROM archive.dbo.orders"))
}

func TestSQLTableMRNs_SQLServerWithoutADatabaseYieldsNothing(t *testing.T) {
	assert.Empty(t, sqlTableMRNs("mssql", "", "SELECT * FROM orders"))
}

func TestSQLTableMRNs_NonSQLDataSourceYieldsNothing(t *testing.T) {
	assert.Empty(t, sqlTableMRNs("prometheus", "", "SELECT * FROM orders"))
	assert.Empty(t, sqlTableMRNs("", "", "SELECT * FROM orders"))
}

func TestSQLTableMRNs_TemplateVariableYieldsNothing(t *testing.T) {
	assert.Empty(t, sqlTableMRNs("grafana-postgresql-datasource", "shop", "SELECT * FROM ${table}"))
}
