package nifi

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// single runs a discovery over a root group holding one processor and
// returns the result with the processor's task MRN.
func single(t *testing.T, f *fakeNiFi, p *fakeProcessor) (*pluginsdk.DiscoveryResult, string) {
	t.Helper()

	result := discover(t, f.withRoot(newGroup("pg-root", "NiFi Flow").add(p)), nil)
	return result, assetMRN("Task", "NiFi Flow/"+p.name)
}

func TestDataLineage_PutS3ObjectProducesTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Land", "org.apache.nifi.processors.aws.s3.PutS3Object").with("Bucket", "marmot-landing"))

	assert.True(t, hasEdge(result, task, "mrn://bucket/s3/marmot-landing", "PRODUCES"))
}

func TestDataLineage_FetchS3ObjectIsFedByTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Fetch", "org.apache.nifi.processors.aws.s3.FetchS3Object").with("Bucket", "marmot-landing"))

	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-landing", task, "FEEDS"))
}

func TestDataLineage_ListS3IsFedByTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "List", "org.apache.nifi.processors.aws.s3.ListS3").with("Bucket", "marmot-landing"))

	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-landing", task, "FEEDS"))
}

func TestDataLineage_PutGCSObjectProducesTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Put", "org.apache.nifi.processors.gcp.storage.PutGCSObject").with("Bucket", "marmot-archive"))

	assert.True(t, hasEdge(result, task, "mrn://bucket/gcs/marmot-archive", "PRODUCES"))
}

func TestDataLineage_FetchGCSObjectIsFedByTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Fetch", "org.apache.nifi.processors.gcp.storage.FetchGCSObject").with("Bucket", "marmot-archive"))

	assert.True(t, hasEdge(result, "mrn://bucket/gcs/marmot-archive", task, "FEEDS"))
}

func TestDataLineage_ListGCSBucketIsFedByTheBucket(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "List", "org.apache.nifi.processors.gcp.storage.ListGCSBucket").with("Bucket", "marmot-archive"))

	assert.True(t, hasEdge(result, "mrn://bucket/gcs/marmot-archive", task, "FEEDS"))
}

func TestDataLineage_PutAzureBlobStorageProducesTheContainer(t *testing.T) {
	// NiFi 2 ships the Azure processors with a _v12 suffix.
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Put", "org.apache.nifi.processors.azure.storage.PutAzureBlobStorage_v12").with("Container Name", "landing"))

	assert.True(t, hasEdge(result, task, "mrn://container/azureblob/landing", "PRODUCES"))
}

func TestDataLineage_FetchAzureBlobStorageIsFedByTheContainer(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Fetch", "org.apache.nifi.processors.azure.storage.FetchAzureBlobStorage_v12").with("Container Name", "landing"))

	assert.True(t, hasEdge(result, "mrn://container/azureblob/landing", task, "FEEDS"))
}

func TestDataLineage_PublishKafkaProducesTheTopic(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Publish", "org.apache.nifi.kafka.processors.PublishKafka").with("Topic Name", "orders-events"))

	assert.True(t, hasEdge(result, task, "mrn://topic/kafka/orders-events", "PRODUCES"))
}

func TestDataLineage_PublishKafkaCreatesTheTopicAsAKafkaAsset(t *testing.T) {
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Publish", "org.apache.nifi.kafka.processors.PublishKafka").with("Topic Name", "orders-events"))

	topic := findAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	assert.Equal(t, []string{"Kafka"}, topic.Providers, "the topic is the Kafka plugin's asset, NiFi is just a source of it")
	assert.Equal(t, "mrn://topic/kafka/orders-events", *topic.MRN)
	require.Len(t, topic.Sources, 1)
	assert.Equal(t, "NiFi", topic.Sources[0].Name)
	assert.Equal(t, "orders-events", topic.Metadata["topic_name"])
	assert.Equal(t, []string{"NiFi Flow/Publish"}, topic.Metadata["producers"])
	assert.Nil(t, topic.Metadata["consumers"])
}

func TestDataLineage_NiFi1KafkaProcessorsCarryAClientVersionSuffix(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Publish", "org.apache.nifi.processors.kafka.pubsub.PublishKafkaRecord_2_6").with("Topic Name", "orders-events"))

	assert.True(t, hasEdge(result, task, "mrn://topic/kafka/orders-events", "PRODUCES"))
}

func TestDataLineage_ConsumeKafkaIsFedByEveryTopicInTheList(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Consume", "org.apache.nifi.kafka.processors.ConsumeKafka").with("Topics", "orders-events, orders-dlq").with("Topic Format", "names"))

	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-events", task, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-dlq", task, "FEEDS"))

	dlq := findAsset(result, "Topic", "orders-dlq")
	require.NotNil(t, dlq)
	assert.Equal(t, []string{"NiFi Flow/Consume"}, dlq.Metadata["consumers"])
}

func TestDataLineage_ConsumeKafkaByPatternNamesNoTopic(t *testing.T) {
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Consume", "org.apache.nifi.kafka.processors.ConsumeKafka").with("Topics", "orders-.*").with("Topic Format", "pattern"))

	assert.Empty(t, assetNames(result, "Topic"))
	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_OneTopicIsCreatedOnceHoweverManyProcessorsUseIt(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(newGroup("pg-root", "NiFi Flow").
		add(processor("p1", "Publish", "org.apache.nifi.kafka.processors.PublishKafka").with("Topic Name", "orders-events")).
		add(processor("p2", "Consume", "org.apache.nifi.kafka.processors.ConsumeKafka").with("Topics", "orders-events"))), nil)

	assert.Equal(t, []string{"orders-events"}, assetNames(result, "Topic"))
	topic := findAsset(result, "Topic", "orders-events")
	assert.Equal(t, []string{"NiFi Flow/Publish"}, topic.Metadata["producers"])
	assert.Equal(t, []string{"NiFi Flow/Consume"}, topic.Metadata["consumers"])
}

func postgresPool(f *fakeNiFi) *fakeNiFi {
	return f.withService("cs-pool", "Shop Postgres", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL":    "jdbc:postgresql://db.internal:5432/shop",
		"Database Driver Class Name": "org.postgresql.Driver",
		"Database User":              "shop",
	}, "Password")
}

func TestDataLineage_PutDatabaseRecordProducesThePostgreSQLTable(t *testing.T) {
	result, task := single(t, postgresPool(newFakeNiFi()),
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/postgresql/orders", "PRODUCES"))
}

func TestDataLineage_PostgreSQLTableIsNamedBareEvenWhenSchemaQualified(t *testing.T) {
	// The PostgreSQL plugin names a table by its bare name.
	result, task := single(t, postgresPool(newFakeNiFi()),
		processor("p1", "Read", "org.apache.nifi.processors.standard.QueryDatabaseTable").with("Table Name", "public.customers").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", task, "FEEDS"))
}

func TestDataLineage_QueryDatabaseTableIsFedByTheTable(t *testing.T) {
	result, task := single(t, postgresPool(newFakeNiFi()),
		processor("p1", "Read", "org.apache.nifi.processors.standard.QueryDatabaseTable").with("Table Name", "customers").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", task, "FEEDS"))
}

func TestDataLineage_QueryDatabaseTableRecordIsFedByTheTable(t *testing.T) {
	result, task := single(t, postgresPool(newFakeNiFi()),
		processor("p1", "Read", "org.apache.nifi.processors.standard.QueryDatabaseTableRecord").with("Table Name", "customers").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", task, "FEEDS"))
}

func TestDataLineage_GenerateTableFetchIsFedByTheTable(t *testing.T) {
	result, task := single(t, postgresPool(newFakeNiFi()),
		processor("p1", "Read", "org.apache.nifi.processors.standard.GenerateTableFetch").with("Table Name", "customers").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", task, "FEEDS"))
}

func TestDataLineage_MySQLPoolNamesAMySQLTable(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Shop MySQL", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:mysql://db.internal:3306/shop",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/mysql/orders", "PRODUCES"))
}

func TestDataLineage_MariaDBPoolNamesAMariaDBTable(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Shop MariaDB", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:mariadb://db.internal:3306/shop",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/mariadb/orders", "PRODUCES"))
}

func TestDataLineage_SQLServerTableIsQualifiedWithDatabaseAndSchema(t *testing.T) {
	// The SQL Server plugin names tables database.schema.table. The
	// database comes from the JDBC URL and the schema defaults to dbo.
	f := newFakeNiFi().withService("cs-pool", "Shop MSSQL", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:sqlserver://db.internal:1433;databaseName=shop;encrypt=true",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/sql-server/shop.dbo.orders", "PRODUCES"))
}

func TestDataLineage_SQLServerPrefersTheProcessorsDatabaseAndSchemaProperties(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Shop MSSQL", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:sqlserver://db.internal:1433;databaseName=master",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").
			with("Table Name", "orders").with("Schema Name", "sales").with("Database Name", "shop").
			with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/sql-server/shop.sales.orders", "PRODUCES"))
}

func TestDataLineage_SQLServerWithoutADatabaseIsSkipped(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Shop MSSQL", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:sqlserver://db.internal:1433",
	})
	result, _ := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge: a half-qualified name would never match the SQL Server plugin's asset")
}

func TestDataLineage_OracleTableIsQualifiedWithTheSchema(t *testing.T) {
	// The Oracle plugin names tables schema.table, and an Oracle session's
	// default schema is its user in upper case.
	f := newFakeNiFi().withService("cs-pool", "Shop Oracle", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:oracle:thin:@db.internal:1521/ORCL",
		"Database User":           "shop",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "ORDERS").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/oracle/shop.orders", "PRODUCES"))
}

func TestDataLineage_OraclePrefersTheProcessorsSchemaProperty(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Shop Oracle", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:oracle:thin:@db.internal:1521/ORCL",
		"Database User":           "shop",
	})
	result, task := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "ORDERS").with("Schema Name", "SALES").with("Database Connection Pooling Service", "cs-pool"))

	assert.True(t, hasEdge(result, task, "mrn://table/oracle/sales.orders", "PRODUCES"))
}

func TestDataLineage_ADatabaseMarmotDoesNotCatalogueIsSkipped(t *testing.T) {
	f := newFakeNiFi().withService("cs-pool", "Embedded H2", "org.apache.nifi.dbcp.DBCPConnectionPool", map[string]string{
		"Database Connection URL": "jdbc:h2:mem:test",
	})
	result, _ := single(t, f,
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_AnUnreadableConnectionPoolIsSkipped(t *testing.T) {
	// No service registered, so the lookup answers 404. The task is still
	// catalogued, just without table lineage.
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Write", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-missing"))

	assert.NotNil(t, findAsset(result, "Task", "NiFi Flow/Write"))
	assert.True(t, hasEdge(result, assetMRN("Pipeline", "NiFi Flow"), task, "CONTAINS"))
	assert.Len(t, result.Lineage, 1)
}

func TestDataLineage_AConnectionPoolIsReadOncePerRun(t *testing.T) {
	f := postgresPool(newFakeNiFi())
	discover(t, f.withRoot(newGroup("pg-root", "NiFi Flow").
		add(processor("p1", "Write Orders", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "orders").with("Database Connection Pooling Service", "cs-pool")).
		add(processor("p2", "Write Items", "org.apache.nifi.processors.standard.PutDatabaseRecord").with("Table Name", "items").with("Database Connection Pooling Service", "cs-pool"))), nil)

	reads := 0
	for _, path := range f.paths() {
		if path == "GET /controller-services/cs-pool" {
			reads++
		}
	}
	assert.Equal(t, 1, reads)
}

func TestDataLineage_PutElasticsearchRecordProducesTheIndex(t *testing.T) {
	// The Elasticsearch plugin catalogues an index as a Table.
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Index", "org.apache.nifi.processors.elasticsearch.PutElasticsearchRecord").with("Index", "orders-2026"))

	assert.True(t, hasEdge(result, task, "mrn://table/elasticsearch/orders-2026", "PRODUCES"))
}

func TestDataLineage_PutElasticsearchJsonProducesTheIndex(t *testing.T) {
	result, task := single(t, newFakeNiFi(),
		processor("p1", "Index", "org.apache.nifi.processors.elasticsearch.PutElasticsearchJson").with("Index", "orders-2026"))

	assert.True(t, hasEdge(result, task, "mrn://table/elasticsearch/orders-2026", "PRODUCES"))
}

func TestDataLineage_ExpressionLanguageValuesAreSkipped(t *testing.T) {
	// ${...} is resolved per flow file at run time, so it names no bucket.
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Land", "org.apache.nifi.processors.aws.s3.PutS3Object").with("Bucket", "${s3.bucket}"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_ParameterReferencesAreSkipped(t *testing.T) {
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Land", "org.apache.nifi.processors.aws.s3.PutS3Object").with("Bucket", "#{landing_bucket}"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_AnUnsetPropertyGivesNoEdge(t *testing.T) {
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Land", "org.apache.nifi.processors.aws.s3.PutS3Object"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_AProcessorWithoutARuleHasNoDataEdges(t *testing.T) {
	result, _ := single(t, newFakeNiFi(),
		processor("p1", "Log", "org.apache.nifi.processors.standard.LogAttribute").with("Log Level", "info"))

	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

func TestDataLineage_CanBeTurnedOff(t *testing.T) {
	f := newFakeNiFi().withRoot(newGroup("pg-root", "NiFi Flow").
		add(processor("p1", "Publish", "org.apache.nifi.kafka.processors.PublishKafka").with("Topic Name", "orders-events")))

	result := discover(t, f, pluginsdk.RawConfig{"discover_lineage": false})

	assert.Empty(t, assetNames(result, "Topic"))
	assert.Len(t, result.Lineage, 1, "only the CONTAINS edge")
}

// Rule table entries, one per processor type in the spec.

func TestRuleFor_KnowsEveryProcessorTypeInTheSpec(t *testing.T) {
	for _, typ := range []string{
		"PutS3Object", "FetchS3Object", "ListS3",
		"PutGCSObject", "FetchGCSObject", "ListGCSBucket",
		"PutAzureBlobStorage", "FetchAzureBlobStorage",
		"PublishKafka", "PublishKafkaRecord", "ConsumeKafka", "ConsumeKafkaRecord",
		"PutDatabaseRecord", "QueryDatabaseTable", "QueryDatabaseTableRecord", "GenerateTableFetch",
		"PutElasticsearchRecord", "PutElasticsearchJson",
	} {
		_, ok := ruleFor("org.apache.nifi.processors." + typ)
		assert.True(t, ok, typ)
	}
}

func TestRuleFor_StripsTheKafkaClientVersionSuffix(t *testing.T) {
	rule, ok := ruleFor("org.apache.nifi.processors.kafka.pubsub.ConsumeKafka_2_6")

	require.True(t, ok)
	assert.Equal(t, "Kafka", rule.Provider)
	assert.True(t, rule.List)
}

func TestRuleFor_StripsTheAzureVersionSuffix(t *testing.T) {
	rule, ok := ruleFor("org.apache.nifi.processors.azure.storage.FetchAzureBlobStorage_v12")

	require.True(t, ok)
	assert.Equal(t, "AzureBlob", rule.Provider)
	assert.Equal(t, readsFrom, rule.Direction)
}

func TestRuleFor_HasNoRuleForPutSQL(t *testing.T) {
	// PutSQL and ExecuteSQL only know their table from the SQL text.
	_, ok := ruleFor("org.apache.nifi.processors.standard.PutSQL")
	assert.False(t, ok)
	_, ok = ruleFor("org.apache.nifi.processors.standard.ExecuteSQL")
	assert.False(t, ok)
}

func TestJDBCProvider_MapsEachSubprotocol(t *testing.T) {
	assert.Equal(t, "PostgreSQL", jdbcProvider("jdbc:postgresql://db:5432/shop"))
	assert.Equal(t, "MySQL", jdbcProvider("jdbc:mysql://db:3306/shop"))
	assert.Equal(t, "MariaDB", jdbcProvider("jdbc:mariadb://db:3306/shop"))
	assert.Equal(t, "SQL Server", jdbcProvider("jdbc:sqlserver://db:1433;databaseName=shop"))
	assert.Equal(t, "Oracle", jdbcProvider("jdbc:oracle:thin:@db:1521/ORCL"))
}

func TestJDBCProvider_IsCaseInsensitive(t *testing.T) {
	assert.Equal(t, "PostgreSQL", jdbcProvider("JDBC:PostgreSQL://db:5432/shop"))
}

func TestJDBCProvider_IsEmptyForUnknownOrNonJDBC(t *testing.T) {
	assert.Empty(t, jdbcProvider("jdbc:h2:mem:test"))
	assert.Empty(t, jdbcProvider("postgresql://db:5432/shop"))
	assert.Empty(t, jdbcProvider(""))
}

func TestJDBCParameter_ReadsSemicolonSeparatedParameters(t *testing.T) {
	assert.Equal(t, "shop", jdbcParameter("jdbc:sqlserver://db:1433;databaseName=shop;encrypt=true", "databaseName"))
}

func TestJDBCParameter_ReadsQueryStringParameters(t *testing.T) {
	assert.Equal(t, "shop", jdbcParameter("jdbc:sqlserver://db:1433?user=sa&database=shop", "databaseName", "database"))
}

func TestJDBCParameter_IsEmptyWhenAbsent(t *testing.T) {
	assert.Empty(t, jdbcParameter("jdbc:sqlserver://db:1433", "databaseName"))
}

func TestQualify_PadsAShortNameWithTheFillers(t *testing.T) {
	name, ok := qualify("orders", 3, "dbo", "shop")

	require.True(t, ok)
	assert.Equal(t, "shop.dbo.orders", name)
}

func TestQualify_KeepsAnAlreadyQualifiedName(t *testing.T) {
	name, ok := qualify("shop.sales.orders", 3, "dbo", "shop")

	require.True(t, ok)
	assert.Equal(t, "shop.sales.orders", name)
}

func TestQualify_FailsWhenAFillerIsMissing(t *testing.T) {
	_, ok := qualify("orders", 3, "dbo", "")

	assert.False(t, ok)
}

func TestQualify_FailsWhenTheNameHasTooManyParts(t *testing.T) {
	_, ok := qualify("a.b.c.d", 3, "dbo", "shop")

	assert.False(t, ok)
}

func TestSplitList_TrimsAndDropsDuplicatesAndBlanks(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, splitList(" a, b ,,a "))
}
