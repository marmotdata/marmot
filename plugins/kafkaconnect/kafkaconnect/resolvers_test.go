package kafkaconnect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each resolver is exercised with the config a real deployment of that
// connector carries, trimmed to the keys the resolver reads.

func table(provider, name string) dataset {
	return dataset{Type: "Table", Provider: provider, Name: name}
}

func mustResolve(t *testing.T, class string) resolver {
	t.Helper()
	r, ok := resolverFor(class)
	require.True(t, ok, "no resolver registered for %s", class)
	return r
}

func TestResolverFor_MatchesTheFullyQualifiedClass(t *testing.T) {
	_, ok := resolverFor("io.debezium.connector.postgresql.PostgresConnector")
	assert.True(t, ok)
}

func TestResolverFor_MatchesTheSimpleClassName(t *testing.T) {
	_, ok := resolverFor("PostgresConnector")
	assert.True(t, ok)
}

func TestResolverFor_MatchesAConfluentCloudName(t *testing.T) {
	_, ok := resolverFor("PostgresCdcSourceV2")
	assert.True(t, ok)
}

func TestResolverFor_UnknownClassHasNoResolver(t *testing.T) {
	_, ok := resolverFor("com.example.CustomConnector")
	assert.False(t, ok)
}

func TestResolverFor_FileStreamConnectorsHaveNoResolver(t *testing.T) {
	_, ok := resolverFor("org.apache.kafka.connect.file.FileStreamSourceConnector")
	assert.False(t, ok)
	_, ok = resolverFor("org.apache.kafka.connect.file.FileStreamSinkConnector")
	assert.False(t, ok)
}

// Debezium PostgreSQL

func TestDebeziumPostgres_NamesTablesBare(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.postgresql.PostgresConnector")
	cfg := connectorConfig{
		"database.dbname":    "shop",
		"topic.prefix":       "shop",
		"table.include.list": "public.orders,inventory.items",
	}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders"), table("PostgreSQL", "items")}, r.sources(cfg))
}

func TestDebeziumPostgres_NamesTopicsPrefixSchemaTable(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.postgresql.PostgresConnector")
	cfg := connectorConfig{"topic.prefix": "shop", "table.include.list": "public.orders,inventory.items"}

	assert.Equal(t, []string{"shop.public.orders", "shop.inventory.items"}, r.topics(cfg))
}

func TestDebeziumPostgres_AcceptsDebezium1Keys(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.postgresql.PostgresConnector")
	cfg := connectorConfig{"database.server.name": "shop", "table.whitelist": "public.orders"}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders")}, r.sources(cfg))
	assert.Equal(t, []string{"shop.public.orders"}, r.topics(cfg))
}

func TestDebeziumPostgres_UnescapesDotsInIncludeEntries(t *testing.T) {
	r := mustResolve(t, "PostgresCdcSource")
	cfg := connectorConfig{"topic.prefix": "shop", "table.include.list": `public\.orders`}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders")}, r.sources(cfg))
	assert.Equal(t, []string{"shop.public.orders"}, r.topics(cfg))
}

func TestDebeziumPostgres_SkipsPatternEntries(t *testing.T) {
	r := mustResolve(t, "PostgresCdcSourceV2")
	cfg := connectorConfig{"topic.prefix": "shop", "table.include.list": "public.*,public.orders"}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders")}, r.sources(cfg))
	assert.Equal(t, []string{"shop.public.orders"}, r.topics(cfg))
}

func TestDebeziumPostgres_NoPrefixMeansNoTopics(t *testing.T) {
	r := mustResolve(t, "PostgresSourceConnector")
	cfg := connectorConfig{"table.include.list": "public.orders"}

	assert.Empty(t, r.topics(cfg))
	assert.Equal(t, []dataset{table("PostgreSQL", "orders")}, r.sources(cfg))
}

// Debezium MySQL

func TestDebeziumMySQL_NamesTablesBareAndTopicsPrefixDbTable(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.mysql.MySqlConnector")
	cfg := connectorConfig{"topic.prefix": "mysql1", "table.include.list": "shop.orders,shop.customers"}

	assert.Equal(t, []dataset{table("MySQL", "orders"), table("MySQL", "customers")}, r.sources(cfg))
	assert.Equal(t, []string{"mysql1.shop.orders", "mysql1.shop.customers"}, r.topics(cfg))
}

func TestDebeziumMySQL_ConfluentCloudAliases(t *testing.T) {
	for _, class := range []string{"MySqlCdcSource", "MySqlCdcSourceV2"} {
		r := mustResolve(t, class)
		assert.Equal(t, []dataset{table("MySQL", "orders")},
			r.sources(connectorConfig{"table.include.list": "shop.orders"}), class)
	}
}

// Debezium SQL Server

func TestDebeziumSQLServer_NamesTablesDatabaseSchemaTable(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.sqlserver.SqlServerConnector")
	cfg := connectorConfig{
		"topic.prefix":       "mssql1",
		"database.names":     "shop,archive",
		"table.include.list": "dbo.orders,sales.invoices",
	}

	assert.Equal(t, []dataset{table("SQL Server", "shop.dbo.orders"), table("SQL Server", "shop.sales.invoices")}, r.sources(cfg))
}

func TestDebeziumSQLServer_NamesTopicsPrefixDbSchemaTable(t *testing.T) {
	r := mustResolve(t, "SqlServerCdcSource")
	cfg := connectorConfig{"topic.prefix": "mssql1", "database.names": "shop", "table.include.list": "dbo.orders"}

	assert.Equal(t, []string{"mssql1.shop.dbo.orders"}, r.topics(cfg))
}

func TestDebeziumSQLServer_FallsBackToDebezium1DatabaseKey(t *testing.T) {
	r := mustResolve(t, "SqlServerCdcSourceV2")
	cfg := connectorConfig{"database.server.name": "mssql1", "database.dbname": "shop", "table.include.list": "dbo.orders"}

	assert.Equal(t, []dataset{table("SQL Server", "shop.dbo.orders")}, r.sources(cfg))
	assert.Equal(t, []string{"mssql1.shop.dbo.orders"}, r.topics(cfg))
}

func TestDebeziumSQLServer_NoDatabaseMeansNoDataset(t *testing.T) {
	r := mustResolve(t, "SqlServerConnector")
	cfg := connectorConfig{"topic.prefix": "mssql1", "table.include.list": "dbo.orders"}

	assert.Empty(t, r.sources(cfg))
	assert.Empty(t, r.topics(cfg))
}

// Debezium MongoDB

func TestDebeziumMongoDB_NamesCollectionsBare(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.mongodb.MongoDbConnector")
	cfg := connectorConfig{"topic.prefix": "mongo1", "collection.include.list": "shop.orders,shop.carts"}

	assert.Equal(t, []dataset{
		{Type: "Collection", Provider: "MongoDB", Name: "orders"},
		{Type: "Collection", Provider: "MongoDB", Name: "carts"},
	}, r.sources(cfg))
	assert.Equal(t, []string{"mongo1.shop.orders", "mongo1.shop.carts"}, r.topics(cfg))
}

func TestDebeziumMongoDB_ConfluentCloudAlias(t *testing.T) {
	r := mustResolve(t, "MongoDbCdcSource")
	assert.Equal(t, []dataset{{Type: "Collection", Provider: "MongoDB", Name: "orders"}},
		r.sources(connectorConfig{"collection.include.list": "shop.orders"}))
}

// Debezium Oracle

func TestDebeziumOracle_NamesTablesSchemaTable(t *testing.T) {
	r := mustResolve(t, "io.debezium.connector.oracle.OracleConnector")
	cfg := connectorConfig{"topic.prefix": "ora1", "table.include.list": "SHOP.ORDERS,SHOP.CUSTOMERS"}

	assert.Equal(t, []dataset{table("Oracle", "SHOP.ORDERS"), table("Oracle", "SHOP.CUSTOMERS")}, r.sources(cfg))
	assert.Equal(t, []string{"ora1.SHOP.ORDERS", "ora1.SHOP.CUSTOMERS"}, r.topics(cfg))
}

func TestDebeziumOracle_ConfluentCloudAlias(t *testing.T) {
	r := mustResolve(t, "OracleCdcSource")
	assert.Equal(t, []dataset{table("Oracle", "SHOP.ORDERS")},
		r.sources(connectorConfig{"table.include.list": "SHOP.ORDERS"}))
}

// Confluent JDBC source

func TestJDBCSource_PostgresTablesAreBare(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.jdbc.JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url":  "jdbc:postgresql://db:5432/shop?sslmode=require",
		"table.whitelist": "orders,public.customers",
		"topic.prefix":    "pg-",
	}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders"), table("PostgreSQL", "customers")}, r.sources(cfg))
	assert.Equal(t, []string{"pg-orders", "pg-customers"}, r.topics(cfg))
}

func TestJDBCSource_MySQLAndMariaDBTablesAreBare(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")

	mysql := connectorConfig{"connection.url": "jdbc:mysql://db:3306/shop", "table.whitelist": "orders"}
	assert.Equal(t, []dataset{table("MySQL", "orders")}, r.sources(mysql))

	mariadb := connectorConfig{"connection.url": "jdbc:mariadb://db:3306/shop", "table.whitelist": "orders"}
	assert.Equal(t, []dataset{table("MariaDB", "orders")}, r.sources(mariadb))
}

func TestJDBCSource_SQLServerTablesAreDatabaseSchemaTable(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url":  "jdbc:sqlserver://db:1433;databaseName=shop;encrypt=true;trustServerCertificate=true",
		"table.whitelist": "orders,sales.invoices",
	}

	assert.Equal(t, []dataset{table("SQL Server", "shop.dbo.orders"), table("SQL Server", "shop.sales.invoices")}, r.sources(cfg))
}

func TestJDBCSource_OracleTablesAreSchemaTable(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url":  "jdbc:oracle:thin:@//db:1521/ORCLPDB1",
		"connection.user": "shop",
		"table.whitelist": "ORDERS,HR.EMPLOYEES",
	}

	assert.Equal(t, []dataset{table("Oracle", "SHOP.ORDERS"), table("Oracle", "HR.EMPLOYEES")}, r.sources(cfg))
}

func TestJDBCSource_SnowflakeTablesAreDatabaseSchemaTable(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url":  "jdbc:snowflake://acme.snowflakecomputing.com/?db=SHOP&schema=SALES&warehouse=WH",
		"table.whitelist": "ORDERS,FINANCE.INVOICES",
	}

	assert.Equal(t, []dataset{table("Snowflake", "SHOP.SALES.ORDERS"), table("Snowflake", "SHOP.FINANCE.INVOICES")}, r.sources(cfg))
}

func TestJDBCSource_RedshiftTablesAreDatabaseSchemaTable(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url":  "jdbc:redshift://cluster.abc.eu-west-1.redshift.amazonaws.com:5439/dev",
		"table.whitelist": "orders,analytics.sessions",
	}

	assert.Equal(t, []dataset{table("Redshift", "dev.public.orders"), table("Redshift", "dev.analytics.sessions")}, r.sources(cfg))
}

func TestJDBCSource_QueryModeHasNoDatasetAndTopicIsThePrefix(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{
		"connection.url": "jdbc:postgresql://db:5432/shop",
		"query":          "SELECT * FROM orders WHERE total > 100",
		"topic.prefix":   "big-orders",
	}

	assert.Empty(t, r.sources(cfg))
	assert.Equal(t, []string{"big-orders"}, r.topics(cfg))
}

func TestJDBCSource_UnknownDriverHasNoDataset(t *testing.T) {
	r := mustResolve(t, "JdbcSourceConnector")
	cfg := connectorConfig{"connection.url": "jdbc:db2://db:50000/shop", "table.whitelist": "orders"}

	assert.Empty(t, r.sources(cfg))
}

// Confluent JDBC sink

func TestJDBCSink_DefaultsTheTableToTheTopic(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.jdbc.JdbcSinkConnector")
	cfg := connectorConfig{"connection.url": "jdbc:postgresql://db:5432/warehouse"}

	assert.Equal(t, []dataset{table("PostgreSQL", "orders"), table("PostgreSQL", "payments")},
		r.targets(cfg, []string{"orders", "payments"}))
}

func TestJDBCSink_SubstitutesTheTopicIntoTableNameFormat(t *testing.T) {
	r := mustResolve(t, "JdbcSinkConnector")
	cfg := connectorConfig{"connection.url": "jdbc:mysql://db:3306/warehouse", "table.name.format": "kafka_${topic}"}

	assert.Equal(t, []dataset{table("MySQL", "kafka_orders")}, r.targets(cfg, []string{"orders"}))
}

func TestJDBCSink_SchemaQualifiedFormatOnSQLServer(t *testing.T) {
	r := mustResolve(t, "JdbcSinkConnector")
	cfg := connectorConfig{
		"connection.url":    "jdbc:sqlserver://db:1433;databaseName=warehouse",
		"table.name.format": "staging.${topic}",
	}

	assert.Equal(t, []dataset{table("SQL Server", "warehouse.staging.orders")}, r.targets(cfg, []string{"orders"}))
}

func TestJDBCSink_NoConnectionURLMeansNoTargets(t *testing.T) {
	r := mustResolve(t, "JdbcSinkConnector")
	assert.Empty(t, r.targets(connectorConfig{}, []string{"orders"}))
}

// Object storage sinks

func TestS3Sink_EveryTopicLandsInTheOneBucket(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.s3.S3SinkConnector")
	cfg := connectorConfig{"s3.bucket.name": "data-lake", "s3.region": "eu-west-1", "topics.dir": "raw"}

	assert.Equal(t, []dataset{{Type: "Bucket", Provider: "S3", Name: "data-lake"}}, r.targets(cfg, []string{"orders", "payments"}))
}

func TestS3Sink_NoBucketMeansNoTarget(t *testing.T) {
	r := mustResolve(t, "S3SinkConnector")
	assert.Empty(t, r.targets(connectorConfig{}, []string{"orders"}))
}

func TestGCSSink_TargetsTheBucket(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.gcs.GcsSinkConnector")
	cfg := connectorConfig{"gcs.bucket.name": "acme-lake"}

	assert.Equal(t, []dataset{{Type: "Bucket", Provider: "GCS", Name: "acme-lake"}}, r.targets(cfg, []string{"orders"}))
}

func TestAzureBlobSink_TargetsTheContainer(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.azure.blob.AzureBlobStorageSinkConnector")
	cfg := connectorConfig{"azblob.container.name": "raw", "azblob.account.name": "acme"}

	assert.Equal(t, []dataset{{Type: "Container", Provider: "AzureBlob", Name: "raw"}}, r.targets(cfg, []string{"orders"}))
}

// Snowflake sink

func TestSnowflakeSink_UsesTheTopicToTableMap(t *testing.T) {
	r := mustResolve(t, "com.snowflake.kafka.connector.SnowflakeSinkConnector")
	cfg := connectorConfig{
		"snowflake.database.name":   "SHOP",
		"snowflake.schema.name":     "RAW",
		"snowflake.topic2table.map": "orders:ORDERS_RAW,payments:PAYMENTS_RAW",
	}

	assert.Equal(t, []dataset{table("Snowflake", "SHOP.RAW.ORDERS_RAW"), table("Snowflake", "SHOP.RAW.PAYMENTS_RAW")},
		r.targets(cfg, []string{"orders", "payments"}))
}

func TestSnowflakeSink_DefaultsTheTableToTheTopic(t *testing.T) {
	r := mustResolve(t, "SnowflakeSinkConnector")
	cfg := connectorConfig{"snowflake.database.name": "SHOP", "snowflake.schema.name": "RAW"}

	assert.Equal(t, []dataset{table("Snowflake", "SHOP.RAW.orders")}, r.targets(cfg, []string{"orders"}))
}

func TestSnowflakeSink_NoDatabaseMeansNoTargets(t *testing.T) {
	r := mustResolve(t, "SnowflakeSinkConnector")
	assert.Empty(t, r.targets(connectorConfig{"snowflake.schema.name": "RAW"}, []string{"orders"}))
}

// BigQuery sink

func TestBigQuerySink_DefaultsTheTableToTheTopic(t *testing.T) {
	r := mustResolve(t, "com.wepay.kafka.connect.bigquery.BigQuerySinkConnector")
	cfg := connectorConfig{"project": "acme", "defaultDataset": "raw"}

	assert.Equal(t, []dataset{table("BigQuery", "orders-events")}, r.targets(cfg, []string{"orders-events"}))
}

func TestBigQuerySink_SanitisesTheTopicWhenAsked(t *testing.T) {
	r := mustResolve(t, "BigQuerySinkConnector")
	cfg := connectorConfig{"sanitizeTopics": "true"}

	assert.Equal(t, []dataset{table("BigQuery", "orders_events_v2")}, r.targets(cfg, []string{"orders-events.v2"}))
}

func TestBigQuerySink_UsesTheTopicToTableMap(t *testing.T) {
	r := mustResolve(t, "BigQuerySinkConnector")
	cfg := connectorConfig{"topic2TableMap": "orders-events:orders", "sanitizeTopics": "true"}

	assert.Equal(t, []dataset{table("BigQuery", "orders")}, r.targets(cfg, []string{"orders-events"}))
}

// Elasticsearch sink

func TestElasticsearchSink_IndexIsTheTopicCataloguedAsATable(t *testing.T) {
	r := mustResolve(t, "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector")

	assert.Equal(t, []dataset{table("Elasticsearch", "orders")}, r.targets(connectorConfig{}, []string{"orders"}))
}

// MongoDB connectors

func TestMongoSink_TargetsTheConfiguredCollection(t *testing.T) {
	r := mustResolve(t, "com.mongodb.kafka.connect.MongoSinkConnector")
	cfg := connectorConfig{"database": "shop", "collection": "orders_raw"}

	assert.Equal(t, []dataset{{Type: "Collection", Provider: "MongoDB", Name: "orders_raw"}},
		r.targets(cfg, []string{"orders", "payments"}))
}

func TestMongoSink_DefaultsTheCollectionToTheTopic(t *testing.T) {
	r := mustResolve(t, "MongoSinkConnector")
	cfg := connectorConfig{"database": "shop"}

	assert.Equal(t, []dataset{
		{Type: "Collection", Provider: "MongoDB", Name: "orders"},
		{Type: "Collection", Provider: "MongoDB", Name: "payments"},
	}, r.targets(cfg, []string{"orders", "payments"}))
}

func TestMongoSource_ReadsTheCollectionAndNamesItsTopic(t *testing.T) {
	r := mustResolve(t, "com.mongodb.kafka.connect.MongoSourceConnector")
	cfg := connectorConfig{"database": "shop", "collection": "orders", "topic.prefix": "mongo"}

	assert.Equal(t, []dataset{{Type: "Collection", Provider: "MongoDB", Name: "orders"}}, r.sources(cfg))
	assert.Equal(t, []string{"mongo.shop.orders"}, r.topics(cfg))
}

func TestMongoSource_TopicWithoutPrefixIsDatabaseCollection(t *testing.T) {
	r := mustResolve(t, "MongoSourceConnector")
	cfg := connectorConfig{"database": "shop", "collection": "orders"}

	assert.Equal(t, []string{"shop.orders"}, r.topics(cfg))
}

func TestMongoSource_WholeDatabaseHasNoDatasetOrTopic(t *testing.T) {
	r := mustResolve(t, "MongoSourceConnector")
	cfg := connectorConfig{"database": "shop"}

	assert.Empty(t, r.sources(cfg))
	assert.Empty(t, r.topics(cfg))
}
