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

func TestProjectionFor_AddressesACustomServiceByItsType(t *testing.T) {
	// A custom service is user-defined, so there is no technology to name
	// it after. Naming it for its category instead would put two
	// unrelated custom services in one namespace.
	for _, serviceType := range []string{"CustomMessaging", "CustomDatabase", "CustomDashboard"} {
		p := projectionFor(serviceType)
		assert.Equal(t, serviceType, p.Provider, serviceType)
	}
}

// A projection key that is not a real OpenMetadata serviceType silently
// never applies: the service falls through to the default instead, and
// the mistake is invisible until someone compares the catalog with
// OpenMetadata by hand.

func TestProjection_KeysAreRealOpenMetadataServiceTypes(t *testing.T) {
	// These three were wrong: OpenMetadata calls them PinotDB and
	// DomoDatabase, and has no Pulsar service at all.
	for _, notAServiceType := range []string{"Pinot", "Domo", "Pulsar"} {
		_, found := projections[notAServiceType]
		assert.False(t, found, "%s is not an OpenMetadata serviceType", notAServiceType)
	}

	for _, serviceType := range []string{"PinotDB", "DomoDatabase"} {
		_, found := projections[serviceType]
		assert.True(t, found, "%s is an OpenMetadata serviceType", serviceType)
	}
}

func TestProjection_PinotTablesAreFlat(t *testing.T) {
	p := projectionFor("PinotDB")

	assert.Equal(t, "Pinot", p.Provider)
	assert.Equal(t, "events", p.TableName("default", "default", "events"),
		"Pinot tables live in one flat namespace")
}

func TestProjection_DomoSharesOneProviderAcrossItsServices(t *testing.T) {
	// Domo is one product; OpenMetadata splits it into three services.
	for _, serviceType := range []string{"DomoDatabase", "DomoDashboard", "DomoPipeline"} {
		assert.Equal(t, "Domo", projectionFor(serviceType).Provider, serviceType)
	}
}

func TestProjection_CouchbaseHoldsCollectionsNotTables(t *testing.T) {
	p := projectionFor("Couchbase")

	assert.Equal(t, "Collection", p.assetTypeFor("Regular"),
		"Couchbase stores documents in collections, the same as MongoDB")
	assert.Equal(t, "travel.inventory.airline",
		p.TableName("travel", "inventory", "airline"),
		"a bucket is a real level: two buckets can hold the same scope and collection")
}

func TestProjection_DruidDatasourcesAreFlat(t *testing.T) {
	p := projectionFor("Druid")

	assert.Equal(t, "wikipedia", p.TableName("druid", "druid", "wikipedia"),
		"OpenMetadata files every Druid datasource under a constant schema")
}

func TestProjection_MysqlCompatibleServersKeepTheDatabaseInTheName(t *testing.T) {
	// MariaDB and SingleStore have no native Marmot plugin, so they are
	// modelled properly rather than copying the MySQL plugin's bare name.
	for _, serviceType := range []string{"MariaDB", "SingleStore"} {
		p := projectionFor(serviceType)
		assert.Equal(t, "shop.orders", p.TableName("default", "shop", "orders"),
			"%s: one server holds many databases", serviceType)
	}
}

// Technologies with no native Marmot plugin yet. These tests pin the
// tuple a future native plugin has to emit to land on the same asset.

func TestProjection_TimescaleIsPostgres(t *testing.T) {
	// TimescaleDB is a Postgres extension, so Marmot's PostgreSQL plugin
	// already reads it. Giving it its own provider would catalogue one
	// hypertable twice.
	p := projectionFor("Timescale")

	assert.Equal(t, "PostgreSQL", p.Provider)
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "metrics"),
		mrn.New("Table", p.Provider, p.TableName("iot", "public", "metrics")))
}

func TestProjection_PubSubMatchesTheAsyncAPIPlugin(t *testing.T) {
	// plugins/asyncapi already emits (Topic, GooglePubSub, bare topic id).
	assert.Equal(t, "GooglePubSub", projectionFor("PubSub").Provider)
}

func TestProjection_KafkaConnectBelongsToKafka(t *testing.T) {
	// Kafka Connect ships inside the Kafka distribution, the same way
	// Glue's ETL jobs and Glue's catalog share one provider. Identity is
	// (Type, Provider, Name), so a Pipeline can never collide with a Topic.
	assert.Equal(t, "Kafka", projectionFor("KafkaConnect").Provider)
	assert.Equal(t, "Glue", projectionFor("GluePipeline").Provider)
}

func TestProjection_DataLakeObjectsAreAddressedByBucket(t *testing.T) {
	p := projectionFor("Datalake")

	assert.Equal(t, "raw/events.parquet",
		p.TableName("default", "raw", "events.parquet"),
		"a data lake reads files out of object storage, so bucket/key is the address")
	assert.Equal(t, groupNone, p.TableGroup,
		"the Bucket asset belongs to the S3 and GCS plugins, not to this one")
}

func TestProjection_EnginesWithoutADatabaseLevelDoNotInheritOne(t *testing.T) {
	// OpenMetadata fills the level an engine does not have with the
	// literal "default". Letting that reach a name puts "default" in the
	// asset's identity and collapses every service onto one container.
	for _, serviceType := range []string{"Oracle", "Hive", "Teradata", "Exasol"} {
		p := projectionFor(serviceType)
		assert.Equal(t, "hr.employees", p.TableName("default", "hr", "employees"), serviceType)
		assert.Equal(t, groupSchema, p.TableGroup, serviceType)
	}
}

func TestProjection_SQLiteMakesNoContainerAsset(t *testing.T) {
	// Every SQLite database calls its only schema "main", so grouping on
	// it would join unrelated files under one asset.
	p := projectionFor("SQLite")

	assert.Equal(t, groupNone, p.TableGroup)
	assert.Equal(t, "orders", p.TableName("default", "main", "orders"))
}

func TestProjection_ExternalTablesAreTypedAsSuch(t *testing.T) {
	// A Snowflake or Redshift external table holds no data of its own,
	// the same as the BigQuery external tables Marmot already types this
	// way.
	for _, serviceType := range []string{"Snowflake", "Redshift", "BigQuery"} {
		assert.Equal(t, "ExternalTable", projectionFor(serviceType).assetTypeFor("External"), serviceType)
	}
}

func TestProjection_DomoHoldsDatasets(t *testing.T) {
	assert.Equal(t, "Dataset", projectionFor("DomoDatabase").assetTypeFor("Regular"),
		"Domo has no tables; its object is a DataSet")
}
