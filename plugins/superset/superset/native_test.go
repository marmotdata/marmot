package superset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every backendMap entry is pinned here: the provider and name shape are
// what the technology's own Marmot plugin produces, and a drift on
// either side silently disconnects Superset from the tables it reads.

func TestBackendMap_Postgres(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders", nativeTableMRN("postgresql", "shop", "public", "orders"))
}

func TestBackendMap_MySQL(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/orders", nativeTableMRN("mysql", "shop", "shop", "orders"))
}

func TestBackendMap_MariaDB(t *testing.T) {
	assert.Equal(t, "mrn://table/mariadb/orders", nativeTableMRN("mariadb", "shop", "shop", "orders"))
}

func TestBackendMap_BigQuery(t *testing.T) {
	assert.Equal(t, "mrn://table/bigquery/events", nativeTableMRN("bigquery", "my-project", "analytics", "events"))
}

func TestBackendMap_ClickHouse(t *testing.T) {
	assert.Equal(t, "mrn://table/clickhouse/hits", nativeTableMRN("clickhouse", "default", "default", "hits"))
}

func TestBackendMap_ClickHouseConnect(t *testing.T) {
	// The clickhouse-connect dialect reports itself as clickhousedb.
	assert.Equal(t, "mrn://table/clickhouse/hits", nativeTableMRN("clickhousedb", "default", "default", "hits"))
}

func TestBackendMap_SQLite(t *testing.T) {
	assert.Equal(t, "mrn://table/sqlite/orders", nativeTableMRN("sqlite", "", "main", "orders"))
}

func TestBackendMap_DuckDB(t *testing.T) {
	assert.Equal(t, "mrn://table/duckdb/orders", nativeTableMRN("duckdb", "", "main", "orders"))
}

func TestBackendMap_AthenaIsCataloguedAsGlue(t *testing.T) {
	assert.Equal(t, "mrn://table/glue/orders", nativeTableMRN("awsathena", "awsdatacatalog", "sales", "orders"))
	assert.Equal(t, "mrn://table/glue/orders", nativeTableMRN("athena", "awsdatacatalog", "sales", "orders"))
}

func TestBackendMap_SnowflakeIsFullyQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/snowflake/sales.public.orders", nativeTableMRN("snowflake", "SALES", "PUBLIC", "ORDERS"))
}

func TestBackendMap_RedshiftIsFullyQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/redshift/dev.public.orders", nativeTableMRN("redshift", "dev", "public", "orders"))
}

func TestBackendMap_SQLServerIsFullyQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", nativeTableMRN("mssql", "shop", "dbo", "orders"))
}

func TestBackendMap_CockroachDBIsFullyQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/cockroachdb/shop.public.orders", nativeTableMRN("cockroachdb", "shop", "public", "orders"))
}

func TestBackendMap_OracleIsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/oracle/hr.employees", nativeTableMRN("oracle", "ORCL", "HR", "EMPLOYEES"))
}

func TestBackendMap_DruidIsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/druid/druid.wikipedia", nativeTableMRN("druid", "", "druid", "wikipedia"))
}

func TestBackendMap_TrinoIsCatalogQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/trino/hive.default.orders", nativeTableMRN("trino", "hive", "default", "orders"))
}

func TestBackendMap_DatabricksIsCatalogQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/databricks/main.sales.orders", nativeTableMRN("databricks", "main", "sales", "orders"))
}

func TestBackendMap_CoversExactlyTheBackendsPinnedAbove(t *testing.T) {
	// A new entry needs a pin of its own; a removed one needs its pin gone.
	assert.Len(t, backendMap, 18)
}

func TestNativeTableMRN_UnknownBackendYieldsNothing(t *testing.T) {
	assert.Equal(t, "", nativeTableMRN("teradatasql", "dw", "dw", "facts"))
}

func TestNativeTableMRN_IsCaseInsensitiveOnTheBackend(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders", nativeTableMRN("PostgreSQL", "shop", "public", "orders"))
}

func TestNativeTableMRN_NeedsATable(t *testing.T) {
	assert.Equal(t, "", nativeTableMRN("postgresql", "shop", "public", ""))
}

func TestNativeTableMRN_DropsEmptyLevelsFromAQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://table/snowflake/public.orders", nativeTableMRN("snowflake", "", "PUBLIC", "ORDERS"))
}

// tableRefs is deliberately conservative: it takes what plainly follows
// FROM and JOIN and nothing else.

func TestTableRefs_FindsABareTable(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}}, tableRefs("SELECT * FROM orders", "public"))
}

func TestTableRefs_KeepsAnExplicitSchema(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "sales", Table: "orders"}}, tableRefs("SELECT * FROM sales.orders o", "public"))
}

func TestTableRefs_KeepsAThreePartName(t *testing.T) {
	assert.Equal(t, []tableRef{{Database: "shop", Schema: "sales", Table: "orders"}}, tableRefs("SELECT * FROM shop.sales.orders", "public"))
}

func TestTableRefs_FollowsJoins(t *testing.T) {
	refs := tableRefs("SELECT * FROM public.orders o JOIN public.customers c ON c.id = o.customer_id LEFT OUTER JOIN payments p ON p.order_id = o.id", "public")
	assert.Equal(t, []tableRef{
		{Schema: "public", Table: "orders"},
		{Schema: "public", Table: "customers"},
		{Schema: "public", Table: "payments"},
	}, refs)
}

func TestTableRefs_UnquotesIdentifiers(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "Sales Data", Table: "Order Lines"}},
		tableRefs(`SELECT * FROM "Sales Data"."Order Lines"`, "public"))
	assert.Equal(t, []tableRef{{Schema: "sales", Table: "orders"}},
		tableRefs("SELECT * FROM `sales`.`orders`", "public"))
	assert.Equal(t, []tableRef{{Schema: "dbo", Table: "Orders"}},
		tableRefs("SELECT * FROM [dbo].[Orders]", "public"))
}

func TestTableRefs_IgnoresCommonTableExpressions(t *testing.T) {
	sql := `WITH recent AS (SELECT * FROM orders WHERE ordered_at > now() - interval '7 days'),
	             big AS (SELECT * FROM recent WHERE total > 100)
	        SELECT * FROM big JOIN customers c ON c.id = big.customer_id`
	assert.Equal(t, []tableRef{
		{Schema: "public", Table: "orders"},
		{Schema: "public", Table: "customers"},
	}, tableRefs(sql, "public"))
}

func TestTableRefs_IgnoresSubqueries(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}},
		tableRefs("SELECT * FROM (SELECT * FROM orders) sub", "public"))
}

func TestTableRefs_IgnoresTableFunctions(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}},
		tableRefs("SELECT * FROM generate_series(1, 10) g JOIN orders o ON o.id = g", "public"))
}

func TestTableRefs_IgnoresComments(t *testing.T) {
	sql := "SELECT * -- FROM commented_out\nFROM orders /* JOIN also_commented */"
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}}, tableRefs(sql, "public"))
}

func TestTableRefs_IsCaseInsensitiveOnKeywords(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}, {Schema: "public", Table: "customers"}},
		tableRefs("select * from orders o join customers c on c.id = o.customer_id", "public"))
}

func TestTableRefs_ReportsEachTableOnce(t *testing.T) {
	assert.Equal(t, []tableRef{{Schema: "public", Table: "orders"}},
		tableRefs("SELECT * FROM orders a JOIN orders b ON a.id = b.parent_id", "public"))
}

func TestTableRefs_LeavesTheSchemaEmptyWhenNothingSuppliesOne(t *testing.T) {
	assert.Equal(t, []tableRef{{Table: "orders"}}, tableRefs("SELECT * FROM orders", ""))
}

func TestTableRefs_EmptySQLHasNoTables(t *testing.T) {
	assert.Empty(t, tableRefs("", "public"))
}
