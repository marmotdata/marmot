package amundsen

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
)

// The MRN a table gets is the whole contract of this plugin: it decides
// whether an asset imported from Amundsen is the same asset the
// technology's own Marmot plugin discovers, or a duplicate of it.

func TestProjection_PostgresTableMatchesThePostgresPlugin(t *testing.T) {
	p := projectionFor("postgres")

	assert.Equal(t, "PostgreSQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("postgres", "prod", "public", "orders"),
		"the PostgreSQL plugin names a table by its bare object name")
}

func TestProjection_PostgresqlIsTheSameAsPostgres(t *testing.T) {
	// Amundsen extractors write both spellings.
	assert.Equal(t, projectionFor("postgres").Provider, projectionFor("postgresql").Provider)
}

func TestProjection_MysqlTableMatchesTheMysqlPlugin(t *testing.T) {
	p := projectionFor("mysql")

	assert.Equal(t, "MySQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("mysql", "prod", "shop", "orders"))
}

func TestProjection_MariaDBTableIsBare(t *testing.T) {
	p := projectionFor("mariadb")

	assert.Equal(t, "MariaDB", p.Provider)
	assert.Equal(t, "orders", p.TableName("mariadb", "prod", "shop", "orders"))
}

func TestProjection_BigQueryTableMatchesTheBigQueryPlugin(t *testing.T) {
	p := projectionFor("bigquery")

	assert.Equal(t, "BigQuery", p.Provider)
	assert.Equal(t, "orders", p.TableName("bigquery", "my-project", "analytics", "orders"),
		"the BigQuery plugin names a table by its bare table id")
}

func TestProjection_HiveTableIsSchemaQualified(t *testing.T) {
	p := projectionFor("hive")

	assert.Equal(t, "Hive", p.Provider)
	assert.Equal(t, "sales.orders", p.TableName("hive", "gold", "sales", "orders"))
}

func TestProjection_HiveLeavesTheClusterOutOfTheName(t *testing.T) {
	// Amundsen's Hive extractor defaults the cluster to an environment
	// label such as "gold", which is not a level Hive itself addresses
	// by, so it stays in metadata rather than in the identity.
	p := projectionFor("hive")

	assert.Equal(t,
		p.TableName("hive", "gold", "sales", "orders"),
		p.TableName("hive", "silver", "sales", "orders"))
}

func TestProjection_SnowflakeFoldsTheClusterIntoTheDatabaseSlot(t *testing.T) {
	// Amundsen's Snowflake extractor puts the Snowflake database in the
	// cluster, and a Snowflake table's Marmot name is
	// database.schema.table, so the cluster fills the leading slot.
	p := projectionFor("snowflake")

	assert.Equal(t, "Snowflake", p.Provider)
	assert.Equal(t, "analytics.public.orders", p.TableName("snowflake", "analytics", "public", "orders"))
}

func TestProjection_SnowflakeDatabasesDoNotCollide(t *testing.T) {
	p := projectionFor("snowflake")

	assert.NotEqual(t,
		p.TableName("snowflake", "prod", "public", "orders"),
		p.TableName("snowflake", "staging", "public", "orders"),
		"two databases holding the same table name must stay apart")
}

func TestProjection_RedshiftKeepsEveryLevel(t *testing.T) {
	p := projectionFor("redshift")

	assert.Equal(t, "Redshift", p.Provider)
	assert.Equal(t, "warehouse.public.orders", p.TableName("redshift", "warehouse", "public", "orders"))
}

func TestProjection_PrestoIsCatalogQualified(t *testing.T) {
	p := projectionFor("presto")

	assert.Equal(t, "Presto", p.Provider)
	assert.Equal(t, "hive.sales.orders", p.TableName("presto", "hive", "sales", "orders"))
}

func TestProjection_TrinoIsCatalogQualified(t *testing.T) {
	p := projectionFor("trino")

	assert.Equal(t, "Trino", p.Provider)
	assert.Equal(t, "hive.sales.orders", p.TableName("trino", "hive", "sales", "orders"))
}

func TestProjection_DeltaLakeKeepsTheSpaceInItsProvider(t *testing.T) {
	// The Delta Lake plugin's provider has a space in it, and the two
	// only merge when this one spells it the same way.
	p := projectionFor("delta")

	assert.Equal(t, "Delta Lake", p.Provider)
	assert.Equal(t, "orders", p.TableName("delta", "prod", "sales", "orders"))
}

func TestProjection_DruidIsSchemaQualified(t *testing.T) {
	p := projectionFor("druid")

	assert.Equal(t, "Druid", p.Provider)
	assert.Equal(t, "druid.orders", p.TableName("druid", "prod", "druid", "orders"))
}

func TestProjection_ElasticsearchIndexIsBare(t *testing.T) {
	p := projectionFor("elasticsearch")

	assert.Equal(t, "Elasticsearch", p.Provider)
	assert.Equal(t, "orders", p.TableName("elasticsearch", "prod", "default", "orders"))
}

func TestProjection_AthenaIsCataloguedAsGlue(t *testing.T) {
	// An Athena table is a Glue Data Catalog table. plugins/glue sets
	// Name to the bare table, so its assets land on
	// mrn://table/glue/<table>.
	p := projectionFor("athena")

	assert.Equal(t, "Glue", p.Provider)
	assert.Equal(t, mrn.New("Table", "Glue", "customer_events"),
		mrn.New("Table", p.Provider, p.TableName("athena", "awsdatacatalog", "analytics", "customer_events")))
}

func TestProjection_ClickHouseTableIsBare(t *testing.T) {
	p := projectionFor("clickhouse")

	assert.Equal(t, "ClickHouse", p.Provider)
	assert.Equal(t, "orders", p.TableName("clickhouse", "prod", "shop", "orders"))
}

func TestProjection_CassandraIsKeyspaceQualified(t *testing.T) {
	p := projectionFor("cassandra")

	assert.Equal(t, "Cassandra", p.Provider)
	assert.Equal(t, "shop.orders", p.TableName("cassandra", "prod", "shop", "orders"))
}

func TestProjection_SqlServerKeepsItsSpacedProvider(t *testing.T) {
	assert.Equal(t, "SQL Server", projectionFor("mssql").Provider)
}

func TestProjection_LookupIgnoresCaseAndSurroundingSpace(t *testing.T) {
	assert.Equal(t, "PostgreSQL", projectionFor("  Postgres ").Provider)
}

func TestProjection_UnknownTechnologyKeepsItsNameUpperCased(t *testing.T) {
	p := projectionFor("questdb")

	assert.Equal(t, "Questdb", p.Provider)
}

func TestProjection_UnknownTechnologyIsSchemaQualified(t *testing.T) {
	p := projectionFor("questdb")

	assert.Equal(t, "public.orders", p.TableName("questdb", "prod", "public", "orders"),
		"an unknown engine keeps every level Amundsen knows it has")
}

func TestProjection_UnknownTechnologyDropsAnEmptySchema(t *testing.T) {
	p := projectionFor("questdb")

	assert.Equal(t, "orders", p.TableName("questdb", "prod", "", "orders"))
}

// The OpenMetadata plugin projects the same technologies onto the same
// identities. It is a separate Go module with unexported tables, so the
// values it produces are restated here; they are copied from
// plugins/openmetadata/openmetadata/projection.go and this test fails
// when the two tables drift apart.

func TestProjection_AgreesWithOpenMetadataOnPostgres(t *testing.T) {
	// openmetadata: "Postgres": {Provider: "PostgreSQL", TableName: nameOnly}
	p := projectionFor("postgres")

	assert.Equal(t, "PostgreSQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("postgres", "prod", "public", "orders"))
}

func TestProjection_AgreesWithOpenMetadataOnMysql(t *testing.T) {
	// openmetadata: "Mysql": {Provider: "MySQL", TableName: nameOnly, ...}
	p := projectionFor("mysql")

	assert.Equal(t, "MySQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("mysql", "prod", "shop", "orders"))
}

func TestProjection_AgreesWithOpenMetadataOnSnowflake(t *testing.T) {
	// openmetadata: "Snowflake": {Provider: "Snowflake", TableName: fullyQualified}
	// fullyQualified joins database.schema.table, which is what the
	// Amundsen cluster fills the leading slot of.
	p := projectionFor("snowflake")

	assert.Equal(t, "Snowflake", p.Provider)
	assert.Equal(t, "analytics.public.orders", p.TableName("snowflake", "analytics", "public", "orders"))
}

func TestProjection_AgreesWithOpenMetadataOnHive(t *testing.T) {
	// openmetadata: "Hive": {Provider: "Hive", TableName: fullyQualified}.
	// fullyQualified drops an empty level, so it produces schema.table
	// whenever OpenMetadata has no database level, which is the shape
	// used here: Amundsen's Hive cluster is an environment label, not a
	// Hive catalog, so it never enters the name.
	p := projectionFor("hive")

	assert.Equal(t, "Hive", p.Provider)
	assert.Equal(t, "sales.orders", p.TableName("hive", "gold", "sales", "orders"))
}

func TestDashboardProvider_SupersetIsSuperset(t *testing.T) {
	assert.Equal(t, "Superset", dashboardProviderFor("superset"))
}

func TestDashboardProvider_ModeIsMode(t *testing.T) {
	assert.Equal(t, "Mode", dashboardProviderFor("mode"))
}

func TestDashboardProvider_RedashIsRedash(t *testing.T) {
	assert.Equal(t, "Redash", dashboardProviderFor("redash"))
}

func TestDashboardProvider_TableauIsTableau(t *testing.T) {
	assert.Equal(t, "Tableau", dashboardProviderFor("tableau"))
}

func TestDashboardProvider_LookerIsLooker(t *testing.T) {
	assert.Equal(t, "Looker", dashboardProviderFor("looker"))
}

func TestDashboardProvider_UnknownProductKeepsItsNameUpperCased(t *testing.T) {
	assert.Equal(t, "Sisense", dashboardProviderFor("sisense"))
}

func TestDashboardProvider_EmptyProductStaysEmpty(t *testing.T) {
	assert.Equal(t, "", dashboardProviderFor(""))
}
