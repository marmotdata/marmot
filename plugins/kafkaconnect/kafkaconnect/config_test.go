package kafkaconnect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitiseConfig_MasksEveryCredentialKey(t *testing.T) {
	config := map[string]string{
		"connection.password":                         "p",
		"aws.secret.access.key":                       "s",
		"confluent.topic.sasl.jaas.config":            "j",
		"gcs.credentials.json":                        "c",
		"api.token":                                   "t",
		"aws.access.key.id":                           "k",
		"snowflake.private.key":                       "pk",
		"database.history.producer.security.protocol": "SASL_SSL",
	}

	out := sanitiseConfig(config)

	for _, key := range []string{
		"connection.password", "aws.secret.access.key", "confluent.topic.sasl.jaas.config",
		"gcs.credentials.json", "api.token", "aws.access.key.id", "snowflake.private.key",
	} {
		assert.Equal(t, "****", out[key], key)
	}
	assert.Equal(t, "SASL_SSL", out["database.history.producer.security.protocol"])
}

func TestSanitiseConfig_MatchesKeysCaseInsensitively(t *testing.T) {
	out := sanitiseConfig(map[string]string{"Connection.Password": "p", "TOKEN": "t"})

	assert.Equal(t, "****", out["Connection.Password"])
	assert.Equal(t, "****", out["TOKEN"])
}

func TestSanitiseConfig_LeavesOrdinaryValuesAlone(t *testing.T) {
	out := sanitiseConfig(map[string]string{
		"connector.class": "io.confluent.connect.jdbc.JdbcSinkConnector",
		"connection.url":  "jdbc:postgresql://db:5432/shop",
		"connection.user": "marmot",
		"tasks.max":       "1",
	})

	assert.Equal(t, "io.confluent.connect.jdbc.JdbcSinkConnector", out["connector.class"])
	assert.Equal(t, "jdbc:postgresql://db:5432/shop", out["connection.url"])
	assert.Equal(t, "marmot", out["connection.user"])
	assert.Equal(t, "1", out["tasks.max"])
}

func TestConnectorConfig_GetReturnsTheFirstSetKey(t *testing.T) {
	cfg := connectorConfig{"database.server.name": "old", "topic.prefix": "  "}

	assert.Equal(t, "old", cfg.get("topic.prefix", "database.server.name"))
	assert.Empty(t, cfg.get("missing"))
}

func TestConnectorConfig_ListSplitsAndTrims(t *testing.T) {
	cfg := connectorConfig{"topics": " orders, payments ,,refunds "}

	assert.Equal(t, []string{"orders", "payments", "refunds"}, cfg.list("topics"))
	assert.Nil(t, cfg.list("missing"))
}

func TestPairs_ParsesTopicToTableMaps(t *testing.T) {
	assert.Equal(t, map[string]string{"orders": "ORDERS", "payments": "PAY"},
		pairs("orders:ORDERS, payments:PAY, broken"))
}

func TestLiteralIdentifier_AcceptsPlainAndEscapedDots(t *testing.T) {
	id, ok := literalIdentifier(`public\.orders`)
	require.True(t, ok)
	assert.Equal(t, "public.orders", id)

	id, ok = literalIdentifier("public.orders")
	require.True(t, ok)
	assert.Equal(t, "public.orders", id)
}

func TestLiteralIdentifier_RejectsPatterns(t *testing.T) {
	for _, entry := range []string{"public.*", "public.(orders|items)", "public.order[s]", "^public.orders$", ""} {
		_, ok := literalIdentifier(entry)
		assert.False(t, ok, entry)
	}
}

func TestParseJDBCURL_Postgres(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:postgresql://db:5432/shop?sslmode=require")
	require.True(t, ok)

	assert.Equal(t, "postgresql", conn.scheme)
	assert.Equal(t, "shop", conn.database)
	assert.Equal(t, "require", conn.params["sslmode"])
}

func TestParseJDBCURL_SQLServerSemicolonParameters(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:sqlserver://db:1433;databaseName=shop;encrypt=true")
	require.True(t, ok)

	assert.Equal(t, "sqlserver", conn.scheme)
	assert.Equal(t, "shop", conn.database)
	assert.Equal(t, "true", conn.params["encrypt"])
}

func TestParseJDBCURL_SQLServerDatabaseAlias(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:sqlserver://db;database=shop")
	require.True(t, ok)

	assert.Equal(t, "shop", conn.database)
}

func TestParseJDBCURL_OracleThin(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:oracle:thin:@//db:1521/ORCLPDB1")
	require.True(t, ok)

	assert.Equal(t, "oracle", conn.scheme)
	assert.Empty(t, conn.database, "an Oracle service name is not a schema")
}

func TestParseJDBCURL_SnowflakeDatabaseFromParameters(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:snowflake://acme.snowflakecomputing.com/?db=SHOP&schema=SALES")
	require.True(t, ok)

	assert.Equal(t, "snowflake", conn.scheme)
	assert.Equal(t, "SHOP", conn.database)
	assert.Equal(t, "SALES", conn.params["schema"])
}

func TestParseJDBCURL_Redshift(t *testing.T) {
	conn, ok := parseJDBCURL("jdbc:redshift://cluster.abc.eu-west-1.redshift.amazonaws.com:5439/dev")
	require.True(t, ok)

	assert.Equal(t, "redshift", conn.scheme)
	assert.Equal(t, "dev", conn.database)
}

func TestParseJDBCURL_RejectsNonJDBC(t *testing.T) {
	_, ok := parseJDBCURL("postgresql://db:5432/shop")
	assert.False(t, ok)
}

func TestDedupe_SortsAndDropsEmptyAndRepeats(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, dedupe([]string{"b", "", "a", "b"}))
	assert.Nil(t, dedupe(nil))
}

func TestIsInternalTopic_DebeziumHousekeepingTopics(t *testing.T) {
	cfg := connectorConfig{
		"topic.prefix":                        "shop",
		"schema.history.internal.kafka.topic": "shop-history",
	}

	assert.True(t, isInternalTopic(cfg, "shop"))
	assert.True(t, isInternalTopic(cfg, "shop.transaction"))
	assert.True(t, isInternalTopic(cfg, "__debezium-heartbeat.shop"))
	assert.True(t, isInternalTopic(cfg, "shop-history"))
	assert.False(t, isInternalTopic(cfg, "shop.public.orders"))
}

func TestIsInternalTopic_SinkDeadLetterQueue(t *testing.T) {
	cfg := connectorConfig{"errors.deadletterqueue.topic.name": "dlq-orders"}

	assert.True(t, isInternalTopic(cfg, "dlq-orders"))
	assert.False(t, isInternalTopic(cfg, "orders"))
}

func TestTrimTrace_CapsAtFiveHundredCharacters(t *testing.T) {
	long := make([]byte, 1200)
	for i := range long {
		long[i] = 'x'
	}

	assert.Len(t, trimTrace(string(long)), maxTraceLength)
	assert.Equal(t, "short", trimTrace("  short\n"))
}

func TestValidTopicName_AcceptsKafkaTopicCharacters(t *testing.T) {
	assert.True(t, validTopicName("orders-events"))
	assert.True(t, validTopicName("shop.public.orders"))
	assert.True(t, validTopicName("__consumer_offsets"))
}

func TestValidTopicName_RejectsPatternsAndEmpty(t *testing.T) {
	assert.False(t, validTopicName(".*"))
	assert.False(t, validTopicName("orders|payments"))
	assert.False(t, validTopicName(""))
	assert.False(t, validTopicName("."))
}
