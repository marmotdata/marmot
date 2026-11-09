package metabase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every entry of engineMap is pinned against the identity the
// technology's own Marmot plugin produces, so a table read by a
// Metabase card lands on the asset that plugin creates.

func pgTable(engineName string, details map[string]any, schema, name string) string {
	return tableMRN(database{Engine: engineName, Details: details}, table{Schema: schema, Name: name})
}

func TestEngineMap_PostgresIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/public.orders", pgTable("postgres", nil, "public", "orders"))
}

func TestEngineMap_MySQLIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mysql/shop.orders", pgTable("mysql", map[string]any{"dbname": "shop"}, "shop", "orders"))
}

func TestEngineMap_MariaDBIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mariadb/shop.orders", pgTable("mariadb", map[string]any{"dbname": "shop"}, "shop", "orders"))
}

func TestEngineMap_BigQueryIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/bigquery/p.analytics.orders", pgTable("bigquery-cloud-sdk", map[string]any{"project-id": "p"}, "analytics", "orders"))
}

func TestEngineMap_ClickHouseIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/clickhouse/default.orders", pgTable("clickhouse", map[string]any{"dbname": "default"}, "default", "orders"))
}

func TestEngineMap_SQLiteIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sqlite/sample-database.orders", pgTable("sqlite", map[string]any{"db": "/plugins/sample-database.sqlite"}, "", "ORDERS"))
}

func TestEngineMap_AthenaIsCataloguedAsGlue(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/glue/lake.orders", pgTable("athena", nil, "lake", "orders"))
}

func TestEngineMap_SnowflakeIsDatabaseSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/snowflake/analytics.public.orders",
		pgTable("snowflake", map[string]any{"db": "ANALYTICS", "account": "acc"}, "PUBLIC", "ORDERS"))
}

func TestEngineMap_RedshiftIsDatabaseSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/redshift/dev.public.orders",
		pgTable("redshift", map[string]any{"db": "dev"}, "public", "orders"))
}

func TestEngineMap_SQLServerIsDatabaseSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.orders",
		pgTable("sqlserver", map[string]any{"db": "shop"}, "dbo", "orders"))
}

func TestEngineMap_OracleIsSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/oracle/orcl.shop.orders", pgTable("oracle", map[string]any{"service-name": "ORCL"}, "SHOP", "ORDERS"))
}

func TestEngineMap_DruidIsSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/druid/druid.wikipedia", pgTable("druid", nil, "druid", "wikipedia"))
}

func TestEngineMap_PrestoIsTrinoCatalogSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/trino/hive.default.orders",
		pgTable("presto-jdbc", map[string]any{"catalog": "hive"}, "default", "orders"))
}

func TestEngineMap_StarburstIsTrinoCatalogSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/trino/hive.default.orders",
		pgTable("starburst", map[string]any{"catalog": "hive"}, "default", "orders"))
}

func TestEngineMap_MongoIsDatabaseCollection(t *testing.T) {
	assert.Equal(t, "mrn://asset/collection/mongodb/shop.orders", pgTable("mongo", map[string]any{"dbname": "shop"}, "", "orders"))
	assert.Equal(t, "Collection", engineFor("mongo").Type)
}

func TestEngineMap_DatabricksIsCatalogSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/databricks/main.default.orders",
		pgTable("databricks", map[string]any{"catalog": "main"}, "default", "orders"))
}

func TestEngineMap_H2KeepsItsEngineNameSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/h2/public.orders", pgTable("h2", map[string]any{"db": "file:/sample"}, "PUBLIC", "ORDERS"))
	assert.Equal(t, "H2", engineFor("h2").Provider)
}

func TestEngineMap_UnknownEngineIsTitleCasedAndSchemaQualified(t *testing.T) {
	assert.Equal(t, "Vertica", engineFor("vertica").Provider)
	assert.Equal(t, "Table", engineFor("vertica").Type)
	assert.Equal(t, "mrn://asset/table/vertica/public.orders", pgTable("vertica", nil, "public", "orders"))
	assert.Equal(t, "mrn://asset/table/vertica/orders", pgTable("vertica", nil, "", "orders"), "no schema, bare name")
}

func TestEngineMap_DetailQualifiedNamesDropAMissingDetail(t *testing.T) {
	// A Snowflake connection configured without a db falls back to
	// schema.table rather than producing a leading dot.
	assert.Equal(t, "mrn://asset/table/snowflake/public.orders", pgTable("snowflake", nil, "PUBLIC", "ORDERS"))
}

func TestEngineMap_KnownEnginesDefaultToTheTableType(t *testing.T) {
	for name := range engineMap {
		assert.NotEmptyf(t, engineFor(name).Type, "engine %s has no type", name)
		assert.NotEmptyf(t, engineFor(name).Provider, "engine %s has no provider", name)
	}
}
