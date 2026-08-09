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
	assert.Equal(t, "shop.public.orders", p.TableName("shop", "public", "orders"),
		"the PostgreSQL plugin identifies a table by schema.table, because one database holds many schemas")
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "shop.public.orders"),
		mrn.New("Table", p.Provider, p.TableName("shop", "public", "orders")))
}

func TestProjection_MysqlTableMatchesTheMysqlPlugin(t *testing.T) {
	p := projectionFor("Mysql")

	assert.Equal(t, "MySQL", p.Provider)
	assert.Equal(t, "shop.orders", p.TableName("default", "shop", "orders"),
		"the MySQL plugin identifies a table by database.table")
}

func TestProjection_BigQueryTableMatchesTheBigQueryPlugin(t *testing.T) {
	p := projectionFor("BigQuery")

	assert.Equal(t, "BigQuery", p.Provider)
	assert.Equal(t, "analytics.orders", p.TableName("my-project", "analytics", "orders"),
		"the BigQuery plugin identifies a table by dataset.table")
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
	assert.Equal(t, "analytics.orders", p.TableName("catalog", "analytics", "orders"),
		"the Glue plugin builds its MRN from database.table")
}

func TestProjection_AthenaIsCataloguedAsGlue(t *testing.T) {
	// An Athena table is a Glue Data Catalog table, and plugins/glue
	// builds mrn.New("Table", "Glue", database+"."+table) for it.
	p := projectionFor("Athena")

	assert.Equal(t, "Glue", p.Provider)
	assert.Equal(t, mrn.New("Table", "Glue", "analytics.customer_events"),
		mrn.New("Table", p.Provider, p.TableName("awsdatacatalog", "analytics", "customer_events")))
}

func TestProjection_AthenaGroupsTablesUnderTheGlueDatabase(t *testing.T) {
	// plugins/glue emits a Database asset named by the bare database
	// name, so the container has to be the OpenMetadata schema level.
	p := projectionFor("Athena")

	assert.Equal(t, groupSchema, p.TableGroup)
	assert.Equal(t, "Database", p.TableGroupType)
}

func TestProjection_CassandraTablesLiveInAKeyspace(t *testing.T) {
	// Cassandra has no databases. Naming its container Database would
	// invent a level, and a future Cassandra plugin would then have to
	// emit an asset type its own users never say.
	assert.Equal(t, "Keyspace", projectionFor("Cassandra").TableGroupType)
}

func TestProjection_BigtableTablesLiveInAnInstance(t *testing.T) {
	// Bigtable groups tables by instance. OpenMetadata puts the GCP project
	// at the database level, but the project stays out of the name: a
	// container asset is always named by its own bare level, so keeping the
	// project in the table name would leave the Instance asset named by the
	// instance alone and no longer a prefix of its tables. plugins/bigquery
	// settles the question for GCP by taking one required project_id and
	// leaving the project out of the MRN entirely.
	p := projectionFor("BigTable")

	assert.Equal(t, "Instance", p.TableGroupType)
	assert.Equal(t, "prod.events", p.TableName("my-project", "prod", "events"),
		"two instances can hold the same table name, so the instance is part of the identity")
}

// addTableGroup names a container asset by its own bare level and has no
// way to qualify it, so a table grouped under the schema level must not
// carry the level above it. Doing so leaves the container named by the
// schema alone while its tables carry the database too: the container stops
// being a prefix of its tables, and two containers of the same name in
// different databases merge into one. That is the defect the fully
// qualified Bigtable entry had.
//
// This does not require the table name to be exactly container.table.
// MariaDB and SingleStore deliberately name tables bare to agree with the
// Trino connector map, which is the authority for a technology with no
// Marmot plugin of its own.
func TestProjection_GroupedTablesNeverCarryTheLevelAboveTheirContainer(t *testing.T) {
	for serviceType, p := range projections {
		if p.TableGroup != groupSchema || p.TableName == nil {
			continue
		}

		got := p.TableName("theDatabase", "theContainer", "orders")
		assert.NotContains(t, got, "theDatabase",
			"%s groups tables under the schema level, so the database must not appear in the table name %q: "+
				"the container asset is named by the bare schema and would no longer match",
			serviceType, got)
	}
}

func TestProjection_ElasticsearchIndicesAreTables(t *testing.T) {
	p := projectionFor("ElasticSearch")

	assert.Equal(t, "Elasticsearch", p.Provider)
	assert.Equal(t, "Table", p.IndexType,
		"the Elasticsearch plugin catalogues an index as a Table")
}

func TestProjection_OpenSearchIndicesAreTablesToo(t *testing.T) {
	assert.Equal(t, "Table", projectionFor("OpenSearch").IndexType,
		"the OpenSearch plugin catalogues an index as a Table")
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
	assert.Equal(t, "Kafka", projectionFor("KafkaConnect").Provider)
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

// Every technology Marmot has a native plugin for must be projected onto
// exactly the MRN that plugin declares, or an OpenMetadata import files a
// second copy of a table the plugin already owns and the handover to the
// real plugin duplicates instead of merging.
//
// The "declares" is load-bearing: a plugin's Name is a display field and is
// usually the object's bare name, while its identity is the value it passes
// to mrn.New. Ingest carries a declared MRN through untouched, so the
// mrn.New call is what these have to be matched against. An earlier version
// of this test asserted bare names, which was only true while the ingest
// API dropped the MRN and rebuilt it from Name.
//
// Each case cites the plugin call it copies. When a plugin and this table
// disagree, the plugin wins and this table changes.
func TestProjection_MatchesWhatNativePluginsDeclare(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		// the OpenMetadata FQN levels below the service
		database, schema, object string
		wantProvider             string
		wantName                 string
		authority                string
	}{
		{
			name: "postgres", serviceType: "Postgres",
			database: "shop", schema: "public", object: "orders",
			wantProvider: "PostgreSQL", wantName: "shop.public.orders",
			authority: "plugins/postgresql/postgresql/source.go assetMRN: database.schema.table",
		},
		{
			name: "mysql", serviceType: "Mysql",
			database: "default", schema: "shop", object: "orders",
			wantProvider: "MySQL", wantName: "shop.orders",
			authority: "plugins/mysql/mysql/source.go assetMRN: database.table",
		},
		{
			name: "bigquery", serviceType: "BigQuery",
			database: "my-gcp-project", schema: "analytics", object: "orders",
			wantProvider: "BigQuery", wantName: "analytics.orders",
			authority: "plugins/bigquery/bigquery/source.go assetMRN: dataset.table, project deliberately excluded",
		},
		{
			name: "mongodb", serviceType: "MongoDB",
			database: "default", schema: "shop", object: "orders",
			wantProvider: "MongoDB", wantName: "shop.orders",
			authority: "plugins/mongodb/mongodb/collection.go assetMRN: database.collection",
		},
		{
			name: "clickhouse", serviceType: "Clickhouse",
			database: "default", schema: "shop", object: "orders",
			wantProvider: "ClickHouse", wantName: "shop.orders",
			authority: "plugins/clickhouse/clickhouse/source.go: database.table",
		},
		{
			name: "glue", serviceType: "Glue",
			database: "default", schema: "analytics", object: "orders",
			wantProvider: "Glue", wantName: "analytics.orders",
			authority: "plugins/glue/glue/source.go: database.table",
		},
		{
			// Athena keeps no catalog of its own: its tables are Glue Data
			// Catalog tables, so it is projected onto the Glue plugin.
			name: "athena is glue", serviceType: "Athena",
			database: "seed_athena_db", schema: "analytics", object: "orders",
			wantProvider: "Glue", wantName: "analytics.orders",
			authority: "plugins/glue/glue/source.go: database.table",
		},
		{
			name: "iceberg", serviceType: "Iceberg",
			database: "default", schema: "ns", object: "orders",
			wantProvider: "Iceberg", wantName: "ns.orders",
			authority: "plugins/iceberg/iceberg/table.go: namespace.table",
		},
		{
			// Delta Lake really is bare: its plugin names a table by the
			// last path segment of its location.
			name: "delta lake", serviceType: "DeltaLake",
			database: "default", schema: "ns", object: "orders",
			wantProvider: "Delta Lake", wantName: "orders",
			authority: "plugins/deltalake/deltalake/table.go: filepath.Base(location)",
		},
		{
			name: "dynamodb", serviceType: "DynamoDB",
			database: "default", schema: "default", object: "orders",
			wantProvider: "DynamoDB", wantName: "orders",
			authority: "plugins/dynamodb/dynamodb/source.go: bare table name",
		},
		{
			// Timescale hypertables live in an ordinary Postgres server
			// that plugins/postgresql already reads.
			name: "timescale is postgres", serviceType: "Timescale",
			database: "shop", schema: "public", object: "metrics",
			wantProvider: "PostgreSQL", wantName: "shop.public.metrics",
			authority: "plugins/postgresql/postgresql/source.go assetMRN: database.schema.table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := projectionFor(tt.serviceType)
			assert.Equal(t, tt.wantProvider, p.Provider, tt.authority)
			assert.Equal(t, tt.wantName, p.TableName(tt.database, tt.schema, tt.object), tt.authority)
		})
	}
}

// Deliberate divergence from the native-plugin rule, already decided: a
// Trino catalog binds to one database, so plugins/trino can name these
// schema.table, but OpenMetadata reads whole accounts and must keep the
// database to stop two databases colliding.
func TestProjection_WarehousesWithoutAPluginKeepEveryLevel(t *testing.T) {
	for _, serviceType := range []string{"Snowflake", "Redshift", "Mssql", "Databricks", "Hive", "Trino", "Presto"} {
		t.Run(serviceType, func(t *testing.T) {
			assert.Equal(t, "db.schema.orders",
				projectionFor(serviceType).TableName("db", "schema", "orders"))
		})
	}
}

// mrn.New lowercases all three components but only sanitizes spaces in the
// name, never in the service. A provider with a space would put that space
// into the MRN and into every URL built from it, so every provider this
// table can emit has to survive mrnService unchanged in the MRN.
func TestProjection_NoProviderLeavesASpaceInTheMRN(t *testing.T) {
	for serviceType := range projections {
		p := projectionFor(serviceType)
		assert.NotContains(t, mrnService(p.Provider), " ",
			"provider %q for service type %q would put a space in the MRN", p.Provider, serviceType)
	}
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

func TestProjection_DruidFollowsTheTrinoPlugin(t *testing.T) {
	// plugins/trino ships an explicit druid entry naming schema.table.
	// A shipped plugin is the authority even where a tidier model exists.
	p := projectionFor("Druid")

	assert.Equal(t, "druid.wikipedia", p.TableName("default", "druid", "wikipedia"))
}

func TestProjection_MysqlCompatibleServersFollowTheMysqlPlugin(t *testing.T) {
	// plugins/trino ships explicit mariadb and singlestore entries, both
	// naming a table by the bare name, the same as Marmot's MySQL plugin.
	// A shipped plugin is the authority even though qualifying by database
	// would be the tidier model.
	for _, serviceType := range []string{"MariaDB", "SingleStore"} {
		p := projectionFor(serviceType)
		assert.Equal(t, "orders", p.TableName("default", "shop", "orders"), serviceType)
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
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "iot.public.metrics"),
		mrn.New("Table", p.Provider, p.TableName("iot", "public", "metrics")))
}

func TestProjection_TimescaleNamesTablesLikeThePostgresEntry(t *testing.T) {
	// Both project onto the PostgreSQL provider, so a hypertable read as
	// Timescale and the same table read as Postgres have to produce one
	// name. Anything else duplicates the asset against its own provider.
	timescale, postgres := projectionFor("Timescale"), projectionFor("Postgres")

	assert.Equal(t,
		postgres.TableName("iot", "public", "metrics"),
		timescale.TableName("iot", "public", "metrics"))
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
	for _, serviceType := range []string{"Oracle", "Teradata", "Exasol"} {
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

// Marmot's own plugins are authoritative: an OpenMetadata import is a
// stand-in until the real plugin takes over, and the handover must not
// move any asset. plugins/trino carries an explicit table-naming entry
// per connector, so wherever both cover a technology they must agree.
//
// The exceptions below are the cases where a Trino catalog cannot see
// what this plugin can, so following it would lose information.
func TestProjection_AgreesWithTheTrinoPlugin(t *testing.T) {
	const (
		bare       = "orders"
		schemaName = "public.orders"
		fullName   = "shop.public.orders"
	)

	trino := map[string]string{
		// provider -> the name plugins/trino gives shop.public.orders
		"PostgreSQL":  fullName,
		"MySQL":       schemaName,
		"MariaDB":     bare,
		"SingleStore": bare,
		"MongoDB":     schemaName,
		"Pinot":       bare,
		"Oracle":      schemaName,
		"ClickHouse":  schemaName, // the ClickHouse plugin builds db.table, and it is the authority
		"Cassandra":   schemaName,
		"Druid":       schemaName,
		"Hive":        fullName,
	}

	byProvider := map[string]projection{}
	for _, serviceType := range []string{
		"Postgres", "Mysql", "MariaDB", "SingleStore", "MongoDB", "PinotDB",
		"Oracle", "Clickhouse", "Cassandra", "Druid", "Hive",
	} {
		p := projectionFor(serviceType)
		byProvider[p.Provider] = p
	}

	for provider, want := range trino {
		p, ok := byProvider[provider]
		require.True(t, ok, "no projection produces provider %s", provider)
		assert.Equal(t, want, p.TableName("shop", "public", "orders"), provider)
	}
}

func TestProjection_DivergesFromTrinoOnlyWhereTrinoCannotSeeTheDatabase(t *testing.T) {
	// A Trino catalog binds to one database, so Trino names Snowflake,
	// Redshift and SQL Server tables schema.table and the database name
	// carries nothing. This plugin reads whole accounts, where two
	// databases really can hold the same schema and table.
	for _, serviceType := range []string{"Snowflake", "Redshift", "Mssql"} {
		p := projectionFor(serviceType)
		assert.Equal(t, "sales.public.orders", p.TableName("sales", "public", "orders"), serviceType)
		assert.NotEqual(t,
			p.TableName("sales", "public", "orders"),
			p.TableName("hr", "public", "orders"),
			"%s: two databases must not collapse onto one asset", serviceType)
	}
}

// The projection follows the technology's own plugin, judged by the MRN
// that plugin declares. Marmot carries a declared MRN through ingest, so
// the MRN is the identity and a projection that disagrees produces a
// duplicate the day the native plugin is switched on.
func TestProjection_FollowsWhatEachPluginDeclares(t *testing.T) {
	declared := map[string]struct{ serviceType, name string }{
		// plugins/clickhouse: mrn.New(type, "ClickHouse", dbName+"."+name)
		"ClickHouse": {"Clickhouse", "analytics.events"},
		// plugins/iceberg: mrn.New("Table", "Iceberg", strings.Join(ident, "."))
		"Iceberg": {"Iceberg", "analytics.events"},
		// plugins/deltalake: mrn.New("Table", "DeltaLake", filepath.Base(path))
		// which really is just the directory name
		"Delta Lake": {"DeltaLake", "events"},
	}

	for provider, want := range declared {
		p := projectionFor(want.serviceType)
		assert.Equal(t, provider, p.Provider, want.serviceType)
		assert.Equal(t, want.name, p.TableName("catalog", "analytics", "events"),
			"%s must produce what its plugin declares", provider)
	}
}
