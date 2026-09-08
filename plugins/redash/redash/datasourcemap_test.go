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

	name, ok := target.assetName(options, refs[0])
	require.True(t, ok, "could not name %q for %q", refs[0].String(), dsType)

	return mrn.New("Table", target.Provider, name)
}

// One test per entry in dataSourceTargets. A wrong provider or a wrong name
// shape produces an edge pointing at an asset that does not exist, which the
// Marmot server silently drops, so each entry is pinned to an exact MRN.

func TestDataSourceMap_PgIsABarePostgreSQLTable(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders",
		tableMRNFor(t, "pg", map[string]any{"dbname": "shop"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_PostgresAliasIsPostgreSQL(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders",
		tableMRNFor(t, "postgres", nil, "SELECT * FROM orders"))
}

func TestDataSourceMap_RedshiftPostgresIsPostgreSQL(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders",
		tableMRNFor(t, "redshift_postgres", nil, "SELECT * FROM orders"))
}

func TestDataSourceMap_MysqlIsABareMySQLTable(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/orders",
		tableMRNFor(t, "mysql", map[string]any{"db": "shop"}, "SELECT * FROM shop.orders"))
}

func TestDataSourceMap_RdsMysqlIsMySQL(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/orders",
		tableMRNFor(t, "rds_mysql", nil, "SELECT * FROM orders"))
}

func TestDataSourceMap_MysqlRdsAliasIsMySQL(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/orders",
		tableMRNFor(t, "mysql_rds", nil, "SELECT * FROM orders"))
}

func TestDataSourceMap_MariadbIsABareMariaDBTable(t *testing.T) {
	assert.Equal(t, "mrn://table/mariadb/orders",
		tableMRNFor(t, "mariadb", nil, "SELECT * FROM orders"))
}

func TestDataSourceMap_ClickhouseIsABareTable(t *testing.T) {
	assert.Equal(t, "mrn://table/clickhouse/events",
		tableMRNFor(t, "clickhouse", map[string]any{"dbname": "logs"}, "SELECT * FROM logs.events"))
}

func TestDataSourceMap_SqliteIsABareTable(t *testing.T) {
	assert.Equal(t, "mrn://table/sqlite/orders",
		tableMRNFor(t, "sqlite", map[string]any{"dbpath": "/data/shop.db"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_BigqueryIsABareTable(t *testing.T) {
	// plugins/bigquery names a table by its bare name and puts the dataset
	// on a Dataset asset, so the dataset is dropped here too.
	assert.Equal(t, "mrn://table/bigquery/orders",
		tableMRNFor(t, "bigquery", map[string]any{"projectId": "shop-prod"}, "SELECT * FROM analytics.orders"))
}

func TestDataSourceMap_AthenaIsCataloguedAsGlue(t *testing.T) {
	// Athena has no catalog of its own: it reads the Glue Data Catalog,
	// which plugins/glue already owns.
	assert.Equal(t, "mrn://table/glue/orders",
		tableMRNFor(t, "athena", map[string]any{"schema": "analytics"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_SnowflakeIsFullyQualifiedFromTheDatabaseOption(t *testing.T) {
	assert.Equal(t, "mrn://table/snowflake/analytics.public.orders",
		tableMRNFor(t, "snowflake", map[string]any{"database": "ANALYTICS"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_RedshiftUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://table/redshift/warehouse.public.orders",
		tableMRNFor(t, "redshift", map[string]any{"dbname": "warehouse"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_RedshiftIamUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://table/redshift/warehouse.public.orders",
		tableMRNFor(t, "redshift_iam", map[string]any{"dbname": "warehouse"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_MssqlUsesTheDbOption(t *testing.T) {
	assert.Equal(t, "mrn://table/sql server/shop.dbo.orders",
		tableMRNFor(t, "mssql", map[string]any{"db": "shop"}, "SELECT * FROM dbo.orders"))
}

func TestDataSourceMap_MssqlOdbcUsesTheDbOption(t *testing.T) {
	assert.Equal(t, "mrn://table/sql server/shop.dbo.orders",
		tableMRNFor(t, "mssql_odbc", map[string]any{"db": "shop"}, "SELECT * FROM [dbo].[orders]"))
}

func TestDataSourceMap_CockroachUsesTheDbnameOption(t *testing.T) {
	assert.Equal(t, "mrn://table/cockroachdb/shop.public.orders",
		tableMRNFor(t, "cockroach", map[string]any{"dbname": "shop"}, "SELECT * FROM public.orders"))
}

func TestDataSourceMap_TrinoUsesTheCatalogOption(t *testing.T) {
	assert.Equal(t, "mrn://table/trino/hive.analytics.orders",
		tableMRNFor(t, "trino", map[string]any{"catalog": "hive", "schema": "analytics"}, "SELECT * FROM orders"))
}

func TestDataSourceMap_PrestoIsItsOwnProvider(t *testing.T) {
	// Marmot models Presto and Trino as separate providers, matching
	// plugins/openmetadata's projection table.
	assert.Equal(t, "mrn://table/presto/hive.analytics.orders",
		tableMRNFor(t, "presto", map[string]any{"catalog": "hive"}, "SELECT * FROM analytics.orders"))
}

func TestDataSourceMap_DatabricksUsesTheCatalogOption(t *testing.T) {
	assert.Equal(t, "mrn://table/databricks/main.sales.orders",
		tableMRNFor(t, "databricks", map[string]any{"catalog": "main"}, "SELECT * FROM sales.orders"))
}

func TestDataSourceMap_OracleIsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/oracle/sales.orders",
		tableMRNFor(t, "oracle", map[string]any{"servicename": "ORCL"}, "SELECT * FROM sales.orders"))
}

func TestDataSourceMap_DruidFallsBackToItsOwnSchema(t *testing.T) {
	// Druid datasources all live in the schema called druid, and a Druid
	// query never spells it out.
	assert.Equal(t, "mrn://table/druid/druid.wikipedia",
		tableMRNFor(t, "druid", nil, "SELECT * FROM wikipedia"))
}

func TestDataSourceMap_ElasticsearchIsABareIndex(t *testing.T) {
	// Marmot's elasticsearch plugin catalogues an index as a Table named by
	// the index alone.
	assert.Equal(t, "mrn://table/elasticsearch/logs-2026",
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
	assert.Equal(t, "mrn://table/snowflake/other.public.orders",
		tableMRNFor(t, "snowflake", map[string]any{"database": "ANALYTICS"}, "SELECT * FROM other.public.orders"))
}

func TestDataSourceTarget_SkipsWhenTheDatabaseIsUnknown(t *testing.T) {
	// Databricks connections in Redash carry no catalog option, so a
	// two-part reference cannot be qualified. Half a name would address a
	// different asset, so no edge is better than a wrong one.
	target, ok := targetForDataSource("databricks")
	require.True(t, ok)

	_, named := target.assetName(map[string]any{"host": "dbc.example.com"}, tableRef{Parts: []string{"sales", "orders"}})

	assert.False(t, named)
}

func TestDataSourceTarget_SkipsWhenTheSchemaIsUnknown(t *testing.T) {
	target, ok := targetForDataSource("oracle")
	require.True(t, ok)

	_, named := target.assetName(nil, tableRef{Parts: []string{"orders"}})

	assert.False(t, named)
}

func TestDataSourceTarget_BareShapeNeedsNoOptionsAtAll(t *testing.T) {
	target, ok := targetForDataSource("pg")
	require.True(t, ok)

	name, named := target.assetName(nil, tableRef{Parts: []string{"orders"}})

	require.True(t, named)
	assert.Equal(t, "orders", name)
}

func TestDataSourceTarget_SchemaOptionFillsAMissingSchema(t *testing.T) {
	target, ok := targetForDataSource("trino")
	require.True(t, ok)

	name, named := target.assetName(map[string]any{"catalog": "hive", "schema": "analytics"}, tableRef{Parts: []string{"orders"}})

	require.True(t, named)
	assert.Equal(t, "hive.analytics.orders", name)
}

func TestDataSourceTarget_IgnoresABlankConnectionOption(t *testing.T) {
	target, ok := targetForDataSource("snowflake")
	require.True(t, ok)

	_, named := target.assetName(map[string]any{"database": "   "}, tableRef{Parts: []string{"public", "orders"}})

	assert.False(t, named)
}

func TestDataSourceTargets_EveryEntryNamesAProviderAndAKnownShape(t *testing.T) {
	for dsType, target := range dataSourceTargets {
		assert.NotEmpty(t, target.Provider, "data source type %q has no provider", dsType)
		assert.Contains(t, []nameShape{bareName, schemaName, databaseName}, target.Shape, "data source type %q has an unknown shape", dsType)
		if target.Shape == databaseName {
			assert.NotEmpty(t, target.DatabaseKeys, "data source type %q needs a database but names no option key", dsType)
		}
	}
}
