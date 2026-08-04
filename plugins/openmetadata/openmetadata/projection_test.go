package openmetadata

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The MRN a table gets is the whole contract of this plugin: it decides
// whether an asset imported from OpenMetadata is the same asset the
// technology's own Marmot plugin discovers, or a duplicate of it.

func TestProjection_PostgresTableMatchesThePostgresPlugin(t *testing.T) {
	p := projectionFor("Postgres")

	assert.Equal(t, "PostgreSQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("shop", "public", "orders"),
		"the PostgreSQL plugin addresses a table by its bare name")
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "orders"),
		mrn.New("Table", p.Provider, p.TableName("shop", "public", "orders")))
}

func TestProjection_MysqlTableMatchesTheMysqlPlugin(t *testing.T) {
	p := projectionFor("Mysql")

	assert.Equal(t, "MySQL", p.Provider)
	assert.Equal(t, "orders", p.TableName("default", "shop", "orders"))
}

func TestProjection_BigQueryTableMatchesTheBigQueryPlugin(t *testing.T) {
	p := projectionFor("BigQuery")

	assert.Equal(t, "BigQuery", p.Provider)
	assert.Equal(t, "orders", p.TableName("my-project", "analytics", "orders"),
		"the BigQuery plugin addresses a table by its bare table id")
}

func TestProjection_BigQueryGroupsTablesUnderTheDataset(t *testing.T) {
	p := projectionFor("BigQuery")

	assert.Equal(t, groupSchema, p.TableGroup, "an OpenMetadata schema is a BigQuery dataset")
	assert.Equal(t, "Dataset", p.TableGroupType)
}

func TestProjection_PostgresGroupsTablesUnderTheDatabase(t *testing.T) {
	p := projectionFor("Postgres")

	assert.Equal(t, groupDatabase, p.TableGroup)
	assert.Equal(t, "Database", p.TableGroupType)
}

func TestProjection_MysqlGroupsTablesUnderTheSchema(t *testing.T) {
	// OpenMetadata gives MySQL a placeholder database level, so the
	// schema is the level that means anything.
	assert.Equal(t, groupSchema, projectionFor("Mysql").TableGroup)
}

func TestProjection_SnowflakeKeepsEveryLevel(t *testing.T) {
	p := projectionFor("Snowflake")

	assert.Equal(t, "Snowflake", p.Provider)
	assert.Equal(t, "sales.public.orders", p.TableName("sales", "public", "orders"),
		"Snowflake has real databases and schemas, so both stay in the name")
}

func TestProjection_SnowflakeDatabasesDoNotCollide(t *testing.T) {
	p := projectionFor("Snowflake")

	assert.NotEqual(t,
		p.TableName("prod", "public", "orders"),
		p.TableName("staging", "public", "orders"),
		"two databases holding the same table name must stay apart")
}

func TestProjection_GlueTableMatchesTheGluePlugin(t *testing.T) {
	p := projectionFor("Glue")

	assert.Equal(t, "Glue", p.Provider)
	assert.Equal(t, "orders", p.TableName("default", "analytics", "orders"))
}

func TestProjection_ElasticsearchIndicesAreTables(t *testing.T) {
	p := projectionFor("ElasticSearch")

	assert.Equal(t, "Elasticsearch", p.Provider)
	assert.Equal(t, "Table", p.IndexType,
		"the Elasticsearch plugin catalogues an index as a Table")
}

func TestProjection_OtherSearchEnginesUseTheIndexType(t *testing.T) {
	assert.Equal(t, "Index", projectionFor("Solr").IndexType)
}

func TestProjection_ObjectStoresCallTopLevelContainersBuckets(t *testing.T) {
	assert.Equal(t, "Bucket", projectionFor("S3").ContainerType)
	assert.Equal(t, "Bucket", projectionFor("GCS").ContainerType)
	assert.Equal(t, "Container", projectionFor("ADLS").ContainerType)
}

func TestProjection_RedpandaIsCataloguedAsKafka(t *testing.T) {
	// The Redpanda plugin reports Kafka as the provider, so topics from
	// either route land on the same asset.
	assert.Equal(t, "Kafka", projectionFor("Redpanda").Provider)
	assert.Equal(t, "Kafka", projectionFor("Kafka").Provider)
}

func TestProjection_UnknownServiceKeepsItsOwnName(t *testing.T) {
	p := projectionFor("SomeNewWarehouse")

	assert.Equal(t, "SomeNewWarehouse", p.Provider,
		"an unknown technology is still catalogued, under its own name")
	assert.Equal(t, "db.schema.orders", p.TableName("db", "schema", "orders"))
}

func TestProjection_EveryEntryHasADefaultedRule(t *testing.T) {
	for serviceType := range projections {
		p := projectionFor(serviceType)

		require.NotEmpty(t, p.Provider, serviceType)
		require.NotNil(t, p.TableName, serviceType)
		require.NotEmpty(t, p.TableGroupType, serviceType)
		require.NotEmpty(t, p.ContainerType, serviceType)
		require.NotEmpty(t, p.IndexType, serviceType)
	}
}

func TestAssetTypeFor_ReusesExistingMarmotTypes(t *testing.T) {
	p := projectionFor("Postgres")

	assert.Equal(t, "Table", p.assetTypeFor("Regular"))
	assert.Equal(t, "Table", p.assetTypeFor("Partitioned"))
	assert.Equal(t, "View", p.assetTypeFor("View"))
	assert.Equal(t, "View", p.assetTypeFor("SecureView"))
	assert.Equal(t, "View", p.assetTypeFor("MaterializedView"),
		"Marmot's SQL plugins call a materialized view a View")
}

func TestAssetTypeFor_MongoCollectionsMatchTheMongoPlugin(t *testing.T) {
	p := projectionFor("MongoDB")

	assert.Equal(t, "Collection", p.assetTypeFor("Regular"))
	assert.Equal(t, "View", p.assetTypeFor("View"))
}

func TestAssetTypeFor_BigQueryExternalTablesMatchTheBigQueryPlugin(t *testing.T) {
	assert.Equal(t, "ExternalTable", projectionFor("BigQuery").assetTypeFor("External"))
}

// The native ClickHouse, Glue and Iceberg plugins set a qualified MRN but
// a bare Name, and the server rebuilds the MRN from the Name, so their
// assets are effectively addressed by the bare name.
func TestProjection_MatchesTheNameNativePluginsActuallySet(t *testing.T) {
	assert.Equal(t, "orders", projectionFor("Clickhouse").TableName("default", "shop", "orders"))
	assert.Equal(t, "orders", projectionFor("Glue").TableName("default", "analytics", "orders"))
	assert.Equal(t, "orders", projectionFor("Iceberg").TableName("default", "ns", "orders"))
}

func TestSplitFQN_KeepsQuotedNamesWhole(t *testing.T) {
	assert.Equal(t, []string{"service", "my.db", "shopify", "orders"},
		splitFQN(`service."my.db".shopify.orders`))
}

func TestSplitFQN_SplitsPlainNames(t *testing.T) {
	assert.Equal(t, []string{"service", "db", "schema", "orders"},
		splitFQN("service.db.schema.orders"))
}

func TestFQNBelowService_DropsTheServiceComponent(t *testing.T) {
	assert.Equal(t, []string{"db", "schema", "orders"},
		fqnBelowService("service.db.schema.orders"))
	assert.Nil(t, fqnBelowService("service"))
}

func TestAPIBaseURL_AcceptsTheFormsPeopleActuallyPaste(t *testing.T) {
	assert.Equal(t, "https://om.example.com/api", apiBaseURL("https://om.example.com"))
	assert.Equal(t, "https://om.example.com/api", apiBaseURL("https://om.example.com/"))
	assert.Equal(t, "https://om.example.com/api", apiBaseURL("https://om.example.com/api"))
	assert.Equal(t, "https://om.example.com/api", apiBaseURL("  https://om.example.com  "))
}
