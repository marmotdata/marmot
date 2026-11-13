package redash

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableMRNFor is what the plugin does for one table reference: look up the
// Marmot identity behind a Redash data source type and build the table MRN
// its owning plugin would build.
func tableMRNFor(t *testing.T, dsType string, options map[string]any, sql string) string {
	t.Helper()

	target, ok := targetForDataSource(dsType)
	require.True(t, ok, "no target for data source type %q", dsType)

	refs := extractTableRefs(sql)
	require.Len(t, refs, 1)

	path, ok := target.assetPath(options, refs[0])
	require.True(t, ok, "could not name %q for %q", refs[0].String(), dsType)

	return mrn.Asset("Table", target.Provider, path...).WithHost(target.host(options)).String()
}

// One test per entry in dataSourceTargets. A wrong provider or a wrong name
// shape produces an edge pointing at an asset that does not exist, which the
// Marmot server silently drops, so each entry is pinned to an exact MRN.

func TestDataSourceMap_PgIsAPostgreSQLTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders",
		tableMRNFor(t, "pg", map[string]any{"dbname": "shop"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_PostgresAliasIsPostgreSQL(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql@host=db.internal:6432/shop.public.orders",
		tableMRNFor(t, "postgres", map[string]any{"dbname": "shop", "host": "DB.internal", "port": float64(6432)}, "SELECT * FROM orders"))
}

func TestDataSourceMap_RedshiftPostgresIsPostgreSQL(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders",
		tableMRNFor(t, "redshift_postgres", map[string]any{"dbname": "shop"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_MysqlIsAMySQLTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mysql@host=my:3306/shop.orders",
		tableMRNFor(t, "mysql", map[string]any{"db": "shop", "host": "my"}, "SELECT * FROM shop.orders"))
}

func TestDataSourceMap_RdsMysqlIsMySQL(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mysql/shop.orders",
		tableMRNFor(t, "rds_mysql", map[string]any{"db": "shop"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_MysqlRdsAliasIsMySQL(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mysql/shop.orders",
		tableMRNFor(t, "mysql_rds", map[string]any{"db": "shop"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_MariadbIsAMariaDBTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mariadb/shop.orders",
		tableMRNFor(t, "mariadb", map[string]any{"db": "shop"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_ClickhouseIsATable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/clickhouse/logs.events",
		tableMRNFor(t, "clickhouse", map[string]any{"dbname": "logs"}, "SELECT * FROM logs.events"))
}

func TestDataSourceMap_SqliteIsABareTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sqlite/orders",
		tableMRNFor(t, "sqlite", map[string]any{"dbpath": "/data/shop.db"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_BigqueryIsAProjectDatasetTable(t *testing.T) {
	// plugins/bigquery names a table project.dataset.table.
	assert.Equal(t, "mrn://asset/table/bigquery/shop-prod.analytics.orders",
		tableMRNFor(t, "bigquery", map[string]any{"projectId": "shop-prod"}, "SELECT * FROM analytics.orders"))
}

func TestDataSourceMap_AthenaIsCataloguedAsGlue(t *testing.T) {
	// Athena has no catalog of its own: it reads the Glue Data Catalog,
	// which plugins/glue already owns.
	assert.Equal(t, "mrn://asset/table/glue/analytics.orders",
		tableMRNFor(t, "athena", map[string]any{"schema": "analytics"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_SnowflakeIsFullyQualifiedFromTheDatabaseOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/snowflake/analytics.public.orders",
		tableMRNFor(t, "snowflake", map[string]any{"database": "ANALYTICS"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_RedshiftUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/redshift/warehouse.public.orders",
		tableMRNFor(t, "redshift", map[string]any{"dbname": "warehouse"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_RedshiftIamUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/redshift/warehouse.public.orders",
		tableMRNFor(t, "redshift_iam", map[string]any{"dbname": "warehouse"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_MssqlUsesTheDbOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.orders",
		tableMRNFor(t, "mssql", map[string]any{"db": "shop"}, "SELECT * FROM dbo.orders"))
}

func TestDataSourceMap_MssqlOdbcUsesTheDbOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.orders",
		tableMRNFor(t, "mssql_odbc", map[string]any{"db": "shop"}, "SELECT * FROM [dbo].[orders]"))
}

func TestDataSourceMap_CockroachUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/cockroachdb/shop.public.orders",
		tableMRNFor(t, "cockroach", map[string]any{"dbname": "shop"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_TrinoUsesTheCatalogOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/trino/hive.analytics.orders",
		tableMRNFor(t, "trino", map[string]any{"catalog": "hive", "schema": "analytics"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_PrestoIsItsOwnProvider(t *testing.T) {
	// Marmot models Presto and Trino as separate providers, matching
	// plugins/openmetadata's projection table.
	assert.Equal(t, "mrn://asset/table/presto/hive.analytics.orders",
		tableMRNFor(t, "presto", map[string]any{"catalog": "hive"}, "SELECT * FROM analytics.orders"))
}

func TestDataSourceMap_DatabricksUsesTheCatalogOption(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/databricks/main.sales.orders",
		tableMRNFor(t, "databricks", map[string]any{"catalog": "main"}, "SELECT * FROM sales.orders"))
}

func TestDataSourceMap_OracleIsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/oracle/orcl.sales.orders",
		tableMRNFor(t, "oracle", map[string]any{"servicename": "ORCL"}, "SELECT * FROM sales.orders"))
}

func TestDataSourceMap_DruidFallsBackToItsOwnSchema(t *testing.T) {
	// Druid datasources all live in the schema called druid, and a Druid
	// query never spells it out.
	assert.Equal(t, "mrn://asset/table/druid/druid.wikipedia",
		tableMRNFor(t, "druid", nil, "SELECT * FROM wikipedia"))
}

func TestDataSourceMap_ElasticsearchIsABareIndex(t *testing.T) {
	// Marmot's elasticsearch plugin catalogues an index as a Table named by
	// the index alone.
	assert.Equal(t, "mrn://asset/table/elasticsearch/logs-2026",
		tableMRNFor(t, "elasticsearch", nil, `SELECT * FROM "logs-2026"`))
}

// Lookup and qualification rules

func TestDataSourceMap_HasNoTargetForAnUnknownType(t *testing.T) {
	_, ok := targetForDataSource("google_spreadsheets")

	assert.False(t, ok)
}

func TestDataSourceMap_LookupIsCaseInsensitive(t *testing.T) {
	target, ok := targetForDataSource("PG")

	require.True(t, ok)
	assert.Equal(t, "PostgreSQL", target.Provider)
}

func TestDataSourceMap_LookupIgnoresSurroundingSpace(t *testing.T) {
	_, ok := targetForDataSource(" pg ")

	assert.True(t, ok)
}

func TestDataSourceTarget_ThreePartReferenceOverridesTheConnectionOptions(t *testing.T) {
	// A query that spells out the database wins over the data source's
	// default, because that is the database it actually read.
	assert.Equal(t, "mrn://asset/table/snowflake/other.public.orders",
		tableMRNFor(t, "snowflake", map[string]any{"database": "ANALYTICS"}, "SELECT * FROM other.public.orders"))
}

func TestDataSourceTarget_SkipsWhenTheDatabaseIsUnknown(t *testing.T) {
	// Databricks connections in Redash carry no catalog option, so a
	// two-part reference cannot be qualified. Half a name would address a
	// different asset, so no edge is better than a wrong one.
	target, ok := targetForDataSource("databricks")
	require.True(t, ok)

	_, named := target.assetPath(map[string]any{"host": "dbc.example.com"}, tableRef{Parts: []string{"sales", "orders"}})

	assert.False(t, named)
}

func TestDataSourceTarget_SkipsWhenTheSchemaIsUnknown(t *testing.T) {
	target, ok := targetForDataSource("oracle")
	require.True(t, ok)

	_, named := target.assetPath(nil, tableRef{Parts: []string{"orders"}})

	assert.False(t, named)
}

func TestDataSourceTarget_TableOnlyShapeNeedsNoOptionsAtAll(t *testing.T) {
	target, ok := targetForDataSource("elasticsearch")
	require.True(t, ok)

	path, named := target.assetPath(nil, tableRef{Parts: []string{"orders"}})

	require.True(t, named)
	assert.Equal(t, []string{"orders"}, path)
}

func TestDataSourceTarget_SchemaOptionFillsAMissingSchema(t *testing.T) {
	target, ok := targetForDataSource("trino")
	require.True(t, ok)

	path, named := target.assetPath(map[string]any{"catalog": "hive", "schema": "analytics"}, tableRef{Parts: []string{"orders"}})

	require.True(t, named)
	assert.Equal(t, []string{"hive", "analytics", "orders"}, path)
}

func TestDataSourceTarget_IgnoresABlankConnectionOption(t *testing.T) {
	target, ok := targetForDataSource("snowflake")
	require.True(t, ok)

	_, named := target.assetPath(map[string]any{"database": "   "}, tableRef{Parts: []string{"public", "orders"}})

	assert.False(t, named)
}

func TestDataSourceTargets_EveryEntryNamesAProviderAndAKnownShape(t *testing.T) {
	for dsType, target := range dataSourceTargets {
		assert.NotEmpty(t, target.Provider, "data source type %q has no provider", dsType)
		assert.Contains(t, []nameShape{tableOnly, databaseTable, schemaName, databaseName}, target.Shape, "data source type %q has an unknown shape", dsType)
		if target.Shape == databaseName {
			assert.NotEmpty(t, target.DatabaseKeys, "data source type %q needs a database but names no option key", dsType)
		}
	}
}

func TestDataSourceMap_SameTableInTwoSchemasDiffers(t *testing.T) {
	options := map[string]any{"dbname": "shop"}
	assert.NotEqual(t,
		tableMRNFor(t, "pg", options, "SELECT * FROM public.orders"),
		tableMRNFor(t, "pg", options, "SELECT * FROM archive.orders"))
}
