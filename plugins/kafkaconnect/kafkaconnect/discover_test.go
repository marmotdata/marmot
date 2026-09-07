package kafkaconnect

import (
	"net/http"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func twoFileConnectors() *fakeConnect {
	return newFakeConnect().
		with(fileSourceFixture).
		with(fileSinkFixture).
		withTopics("orders-file-source", "orders-events").
		withTopics("orders-file-sink", "orders-events")
}

func TestDiscover_CreatesAPipelinePerConnector(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	source := findAsset(result, "Pipeline", "orders-file-source")
	require.NotNil(t, source)
	assert.Equal(t, []string{"Kafka Connect"}, source.Providers)
	assert.Equal(t, "mrn://pipeline/kafka connect/orders-file-source", *source.MRN)

	sink := findAsset(result, "Pipeline", "orders-file-sink")
	require.NotNil(t, sink)
	assert.Equal(t, "sink", sink.Metadata["connector_type"])
}

func TestDiscover_CarriesConnectorMetadata(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	source := findAsset(result, "Pipeline", "orders-file-source")
	require.NotNil(t, source)

	assert.Equal(t, "org.apache.kafka.connect.file.FileStreamSourceConnector", source.Metadata["connector_class"])
	assert.Equal(t, "source", source.Metadata["connector_type"])
	assert.Equal(t, "RUNNING", source.Metadata["state"])
	assert.Equal(t, "marmot-test-kafkaconnect-connect:8083", source.Metadata["worker_id"])
	assert.Equal(t, 1, source.Metadata["task_count"])
	assert.Equal(t, map[string]any{"0": "RUNNING"}, source.Metadata["task_states"])
	assert.Equal(t, []string{"orders-events"}, source.Metadata["topics"])
	assert.Equal(t, "7.9.0-ccs", source.Metadata["plugin_version"])
	assert.Equal(t, "7.9.0-ccs", source.Metadata["connect_version"])
	assert.Equal(t, "5L6g3nShT-eMCtK--X86sw", source.Metadata["kafka_cluster_id"])
	assert.True(t, strings.HasSuffix(source.Metadata["url"].(string), "/connectors/orders-file-source"))
	assert.NotContains(t, source.Metadata, "error")
	assert.Nil(t, source.Description)
}

func TestDiscover_StoresConfigWithSecretsMasked(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("pg-sink", "sink", map[string]string{
		"connector.class":     "io.confluent.connect.jdbc.JdbcSinkConnector",
		"connection.url":      "jdbc:postgresql://db:5432/shop",
		"connection.user":     "marmot",
		"connection.password": "hunter2",
		"topics":              "orders",
	}), nil)

	pipeline := findAsset(result, "Pipeline", "pg-sink")
	require.NotNil(t, pipeline)

	config, ok := pipeline.Metadata["config"].(map[string]any)
	require.True(t, ok, "expected the sanitised config map")
	assert.Equal(t, "****", config["connection.password"])
	assert.Equal(t, "marmot", config["connection.user"])
	assert.Equal(t, "jdbc:postgresql://db:5432/shop", config["connection.url"])
}

func TestDiscover_OmitsConfigWhenDisabled(t *testing.T) {
	result := discover(t, twoFileConnectors(), pluginsdk.RawConfig{"include_config": false})

	pipeline := findAsset(result, "Pipeline", "orders-file-source")
	require.NotNil(t, pipeline)
	assert.NotContains(t, pipeline.Metadata, "config")
}

func TestDiscover_UsesTheConfigDescriptionAsTheAssetDescription(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("described", "sink", map[string]string{
		"connector.class": "org.apache.kafka.connect.file.FileStreamSinkConnector",
		"topics":          "orders",
		"description":     "Writes orders to a file",
	}), nil)

	pipeline := findAsset(result, "Pipeline", "described")
	require.NotNil(t, pipeline)
	require.NotNil(t, pipeline.Description)
	assert.Equal(t, "Writes orders to a file", *pipeline.Description)
	assert.Equal(t, "Writes orders to a file", pipeline.Metadata["description"])
}

func TestDiscover_CreatesATaskPerTaskAndContainsEdges(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	task := findAsset(result, "Task", "orders-file-source.task-0")
	require.NotNil(t, task)
	assert.Equal(t, []string{"Kafka Connect"}, task.Providers)
	assert.Equal(t, "mrn://task/kafka connect/orders-file-source.task-0", *task.MRN)
	assert.Equal(t, 0, task.Metadata["task_id"])
	assert.Equal(t, "RUNNING", task.Metadata["state"])
	assert.Equal(t, "marmot-test-kafkaconnect-connect:8083", task.Metadata["worker_id"])
	assert.Equal(t, "orders-file-source", task.Metadata["connector"])

	assert.True(t, hasEdge(result,
		"mrn://pipeline/kafka connect/orders-file-source",
		"mrn://task/kafka connect/orders-file-source.task-0",
		"CONTAINS"))
}

func TestDiscover_OmitsTasksWhenDisabled(t *testing.T) {
	result := discover(t, twoFileConnectors(), pluginsdk.RawConfig{"include_tasks": false})

	assert.Nil(t, findAsset(result, "Task", "orders-file-source.task-0"))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "CONTAINS", edge.Type)
	}
}

func TestDiscover_SurfacesAFailedTaskTrace(t *testing.T) {
	result := discover(t, newFakeConnect().with(brokenSinkFixture), nil)

	pipeline := findAsset(result, "Pipeline", "broken-sink")
	require.NotNil(t, pipeline)
	// The connector itself is still RUNNING; the failure lives on the task.
	assert.Equal(t, "RUNNING", pipeline.Metadata["state"])
	assert.Equal(t, map[string]any{"0": "FAILED"}, pipeline.Metadata["task_states"])

	trace, ok := pipeline.Metadata["error"].(string)
	require.True(t, ok, "expected the task trace on the pipeline")
	assert.True(t, strings.HasPrefix(trace, "org.apache.kafka.connect.errors.ConnectException: Couldn't find or create file"))
	assert.LessOrEqual(t, len(trace), maxTraceLength)

	task := findAsset(result, "Task", "broken-sink.task-0")
	require.NotNil(t, task)
	assert.Equal(t, "FAILED", task.Metadata["state"])
	assert.Equal(t, trace, task.Metadata["error"])
}

func TestDiscover_ReportsAPausedConnector(t *testing.T) {
	result := discover(t, newFakeConnect().with(pausedSinkFixture), nil)

	pipeline := findAsset(result, "Pipeline", "orders-file-sink")
	require.NotNil(t, pipeline)
	assert.Equal(t, "PAUSED", pipeline.Metadata["state"])
	assert.Equal(t, map[string]any{"0": "PAUSED"}, pipeline.Metadata["task_states"])
}

func TestDiscover_EmitsOneTopicSharedByProducerAndConsumer(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	var topics []pluginsdk.Asset
	for _, a := range result.Assets {
		if a.Type == "Topic" {
			topics = append(topics, a)
		}
	}
	require.Len(t, topics, 1)

	topic := topics[0]
	assert.Equal(t, "orders-events", *topic.Name)
	assert.Equal(t, []string{"Kafka"}, topic.Providers, "the identity the Kafka plugin uses")
	assert.Equal(t, "mrn://topic/kafka/orders-events", *topic.MRN)
	assert.Equal(t, []string{"orders-file-source"}, topic.Metadata["producers"])
	assert.Equal(t, []string{"orders-file-sink"}, topic.Metadata["consumers"])
	require.Len(t, topic.Sources, 1)
	assert.Equal(t, "Kafka Connect", topic.Sources[0].Name)
}

func TestDiscover_OmitsTopicAssetsWhenDisabled(t *testing.T) {
	result := discover(t, twoFileConnectors(), pluginsdk.RawConfig{"include_topics": false})

	assert.Nil(t, findAsset(result, "Topic", "orders-events"))
	// The pipeline still records which topics it uses.
	pipeline := findAsset(result, "Pipeline", "orders-file-source")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"orders-events"}, pipeline.Metadata["topics"])
}

func TestDiscover_LinksSourceToTopicAndTopicToSink(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	assert.True(t, hasEdge(result,
		"mrn://pipeline/kafka connect/orders-file-source",
		"mrn://topic/kafka/orders-events",
		"PRODUCES"))
	assert.True(t, hasEdge(result,
		"mrn://topic/kafka/orders-events",
		"mrn://pipeline/kafka connect/orders-file-sink",
		"FEEDS"))
}

func TestDiscover_OmitsLineageWhenDisabled(t *testing.T) {
	result := discover(t, twoFileConnectors(), pluginsdk.RawConfig{"discover_lineage": false})

	for _, edge := range result.Lineage {
		assert.Equal(t, "CONTAINS", edge.Type, "only the pipeline to task edges remain")
	}
	assert.NotNil(t, findAsset(result, "Topic", "orders-events"))
}

func TestDiscover_PrefersTheWorkersActiveTopicsOverConfig(t *testing.T) {
	// The sink is configured for two topics but has only read from one.
	result := discover(t, newFakeConnect().
		withConnector("file-sink", "sink", map[string]string{
			"connector.class": "org.apache.kafka.connect.file.FileStreamSinkConnector",
			"topics":          "orders,payments",
		}).
		withTopics("file-sink", "orders"), nil)

	pipeline := findAsset(result, "Pipeline", "file-sink")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"orders"}, pipeline.Metadata["topics"])
}

func TestDiscover_FallsBackToConfigWhenTheWorkerHasNoActiveTopics(t *testing.T) {
	// A connector that has not moved a record yet reports no topics.
	result := discover(t, newFakeConnect().
		withConnector("file-sink", "sink", map[string]string{
			"connector.class": "org.apache.kafka.connect.file.FileStreamSinkConnector",
			"topics":          "orders, payments",
		}), nil)

	pipeline := findAsset(result, "Pipeline", "file-sink")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"orders", "payments"}, pipeline.Metadata["topics"])
}

func TestDiscover_StopsAskingForTopicsWhenTheEndpointIsMissing(t *testing.T) {
	f := twoFileConnectors()
	f.topicsStatus = http.StatusNotFound
	f.topicsBody = `{"error_code":404,"message":"HTTP 404 Not Found"}`

	result := discover(t, f, nil)

	assert.Equal(t, 1, f.topicsCalls, "one 404 is enough to stop asking")
	// Topics still come from the config.
	assert.NotNil(t, findAsset(result, "Topic", "orders-events"))
}

func TestDiscover_StopsAskingForTopicsWhenTrackingIsDisabled(t *testing.T) {
	f := twoFileConnectors()
	f.topicsStatus = http.StatusForbidden
	f.topicsBody = `{"error_code":403,"message":"Topic tracking is disabled."}`

	discover(t, f, nil)

	assert.Equal(t, 1, f.topicsCalls)
}

func TestDiscover_KeepsAskingForTopicsAfterAnUnrelatedError(t *testing.T) {
	f := twoFileConnectors()
	f.topicsStatus = http.StatusInternalServerError
	f.topicsBody = `{"error_code":500,"message":"boom"}`

	result := discover(t, f, nil)

	assert.Equal(t, 2, f.topicsCalls, "a passing failure does not switch the endpoint off")
	assert.NotNil(t, findAsset(result, "Topic", "orders-events"))
}

func TestDiscover_FallsBackWhenTheWorkerRejectsTwoExpands(t *testing.T) {
	f := twoFileConnectors()
	f.rejectDoubleExpand = true

	result := discover(t, f, nil)

	assert.Equal(t, 1, countRequests(f, "/connectors?expand=status&expand=info"))
	assert.Equal(t, 1, countRequests(f, "/connectors?expand=status"), "retried with the status expand alone")
	assert.Equal(t, 1, countRequests(f, "/connectors/orders-file-source"), "config fetched per connector")
	assert.Equal(t, "RUNNING", findAsset(result, "Pipeline", "orders-file-source").Metadata["state"])
	assert.Equal(t, []string{"orders-events"}, findAsset(result, "Pipeline", "orders-file-sink").Metadata["topics"])
}

func TestDiscover_HandlesAWorkerWithoutExpand(t *testing.T) {
	f := twoFileConnectors()
	f.noExpand = true

	result := discover(t, f, nil)

	assert.Equal(t, 1, countRequests(f, "/connectors/orders-file-source/status"))
	assert.NotNil(t, findAsset(result, "Pipeline", "orders-file-source"))
	assert.NotNil(t, findAsset(result, "Task", "orders-file-sink.task-0"))
}

func TestDiscover_FoldsConfluentCloudConfigs(t *testing.T) {
	// Confluent Cloud returns the config as a list of {config, value}
	// pairs. This shape follows its API reference; it was not observed
	// against a live Confluent Cloud cluster.
	fixture := `{"status":{"name":"cc-sink","connector":{"state":"RUNNING","worker_id":"lcc-abc123"},"tasks":[{"id":0,"state":"RUNNING","worker_id":"lcc-abc123"}],"type":"sink"},"info":{"name":"cc-sink","configs":[{"config":"connector.class","value":"S3_SINK"},{"config":"topics","value":"orders"},{"config":"aws.secret.access.key","value":"xyz"}],"tasks":[{"connector":"cc-sink","task":0}],"type":"sink"}}`

	result := discover(t, newFakeConnect().with(fixture), nil)

	pipeline := findAsset(result, "Pipeline", "cc-sink")
	require.NotNil(t, pipeline)
	assert.Equal(t, "S3_SINK", pipeline.Metadata["connector_class"])
	assert.Equal(t, []string{"orders"}, pipeline.Metadata["topics"])
	assert.Equal(t, "****", pipeline.Metadata["config"].(map[string]any)["aws.secret.access.key"])
}

func TestDiscover_SendsBasicAuth(t *testing.T) {
	f := twoFileConnectors()
	f.username, f.password = "marmot", "s3cret"

	result := discover(t, f, pluginsdk.RawConfig{"username": "marmot", "password": "s3cret"})

	assert.NotNil(t, findAsset(result, "Pipeline", "orders-file-source"))
}

func TestDiscover_WrongCredentialsFail(t *testing.T) {
	f := twoFileConnectors()
	f.username, f.password = "marmot", "s3cret"
	server := f.start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL, "username": "marmot", "password": "wrong",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")
}

func TestDiscover_UnreachableWorkerFails(t *testing.T) {
	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connecting to Kafka Connect")
}

func TestDiscover_SurvivesAMissingPluginList(t *testing.T) {
	f := twoFileConnectors()
	f.pluginsStatus = http.StatusInternalServerError

	result := discover(t, f, nil)

	pipeline := findAsset(result, "Pipeline", "orders-file-source")
	require.NotNil(t, pipeline)
	assert.NotContains(t, pipeline.Metadata, "plugin_version")
}

func TestDiscover_DebeziumSourceLinksTablesAndTopics(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("shop-cdc", "source", map[string]string{
		"connector.class":    "io.debezium.connector.postgresql.PostgresConnector",
		"database.hostname":  "db",
		"database.dbname":    "shop",
		"topic.prefix":       "shop",
		"table.include.list": "public.orders,public.customers",
	}), nil)

	pipeline := "mrn://pipeline/kafka connect/shop-cdc"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", pipeline, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", pipeline, "FEEDS"))
	assert.True(t, hasEdge(result, pipeline, "mrn://topic/kafka/shop.public.orders", "PRODUCES"))
	assert.True(t, hasEdge(result, pipeline, "mrn://topic/kafka/shop.public.customers", "PRODUCES"))

	assert.NotNil(t, findAsset(result, "Topic", "shop.public.orders"))
	assert.Nil(t, findAsset(result, "Table", "orders"), "tables are linked, never created")
}

func TestDiscover_DebeziumSourceAppliesRegexRouter(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("shop-cdc", "source", map[string]string{
		"connector.class":                     "io.debezium.connector.postgresql.PostgresConnector",
		"topic.prefix":                        "shop",
		"table.include.list":                  "public.orders",
		"transforms":                          "route",
		"transforms.route.type":               "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":              "([^.]+)\\.([^.]+)\\.([^.]+)",
		"transforms.route.replacement":        "$1.$3",
		"schema.history.internal.kafka.topic": "shop-history",
	}), nil)

	assert.True(t, hasEdge(result, "mrn://pipeline/kafka connect/shop-cdc", "mrn://topic/kafka/shop.orders", "PRODUCES"))
	assert.Nil(t, findAsset(result, "Topic", "shop.public.orders"))
}

func TestDiscover_ExcludesInternalTopicsReportedByTheWorker(t *testing.T) {
	result := discover(t, newFakeConnect().
		withConnector("shop-cdc", "source", map[string]string{
			"connector.class":                     "io.debezium.connector.postgresql.PostgresConnector",
			"topic.prefix":                        "shop",
			"table.include.list":                  "public.orders",
			"schema.history.internal.kafka.topic": "shop-history",
		}).
		withTopics("shop-cdc", "shop", "shop.transaction", "shop.public.orders", "__debezium-heartbeat.shop", "shop-history"), nil)

	pipeline := findAsset(result, "Pipeline", "shop-cdc")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"shop.public.orders"}, pipeline.Metadata["topics"])
}

func TestDiscover_SinkTargetsUseTheRoutedTopicName(t *testing.T) {
	result := discover(t, newFakeConnect().
		withConnector("pg-sink", "sink", map[string]string{
			"connector.class":              "io.confluent.connect.jdbc.JdbcSinkConnector",
			"connection.url":               "jdbc:postgresql://db:5432/warehouse",
			"topics":                       "shop.public.orders",
			"transforms":                   "route",
			"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
			"transforms.route.regex":       "shop\\.public\\.(.*)",
			"transforms.route.replacement": "$1",
		}), nil)

	pipeline := "mrn://pipeline/kafka connect/pg-sink"
	assert.True(t, hasEdge(result, "mrn://topic/kafka/shop.public.orders", pipeline, "FEEDS"),
		"the topic edge uses the real topic name")
	assert.True(t, hasEdge(result, pipeline, "mrn://table/postgresql/orders", "PRODUCES"),
		"the table edge uses the name after routing")
}

func TestDiscover_StorageSinkFansEveryTopicIntoOneBucket(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("lake", "sink", map[string]string{
		"connector.class": "io.confluent.connect.s3.S3SinkConnector",
		"s3.bucket.name":  "data-lake",
		"topics":          "orders,payments",
	}), nil)

	pipeline := "mrn://pipeline/kafka connect/lake"
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders", pipeline, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://topic/kafka/payments", pipeline, "FEEDS"))
	assert.True(t, hasEdge(result, pipeline, "mrn://bucket/s3/data-lake", "PRODUCES"))
	assert.Nil(t, findAsset(result, "Bucket", "data-lake"), "buckets are linked, never created")
}

func TestDiscover_UnknownClassLinksTopicsOnly(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("custom", "sink", map[string]string{
		"connector.class": "com.example.CustomSinkConnector",
		"topics":          "orders",
		"table":           "should-not-be-used",
	}), nil)

	pipeline := "mrn://pipeline/kafka connect/custom"
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders", pipeline, "FEEDS"))
	for _, edge := range result.Lineage {
		if edge.Source == pipeline {
			assert.Equal(t, "CONTAINS", edge.Type, "nothing but tasks hangs off an unknown connector")
		}
	}
}

func TestDiscover_OutboxRouterWithAStaticRouteNamesThatTopic(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("outbox", "source", map[string]string{
		"connector.class":        "io.debezium.connector.postgresql.PostgresConnector",
		"topic.prefix":           "shop",
		"table.include.list":     "public.outbox",
		"transforms":             "outbox",
		"transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
		"transforms.outbox.route.topic.replacement": "domain-events",
	}), nil)

	pipeline := findAsset(result, "Pipeline", "outbox")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"domain-events"}, pipeline.Metadata["topics"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/outbox", "mrn://pipeline/kafka connect/outbox", "FEEDS"))
}

func TestDiscover_OutboxRouterWithADynamicRouteDerivesNoTopics(t *testing.T) {
	result := discover(t, newFakeConnect().withConnector("outbox", "source", map[string]string{
		"connector.class":        "io.debezium.connector.postgresql.PostgresConnector",
		"topic.prefix":           "shop",
		"table.include.list":     "public.outbox",
		"transforms":             "outbox",
		"transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
	}), nil)

	pipeline := findAsset(result, "Pipeline", "outbox")
	require.NotNil(t, pipeline)
	assert.NotContains(t, pipeline.Metadata, "topics")
}

func TestDiscover_IgnoresTopicPatternsInSourceConfig(t *testing.T) {
	// MirrorMaker lists the topics it mirrors as patterns in the topics
	// key of a source connector; a pattern is not a topic.
	result := discover(t, newFakeConnect().withConnector("mirror", "source", map[string]string{
		"connector.class": "org.apache.kafka.connect.mirror.MirrorSourceConnector",
		"topics":          ".*,orders",
	}), nil)

	pipeline := findAsset(result, "Pipeline", "mirror")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"orders"}, pipeline.Metadata["topics"])
	assert.Nil(t, findAsset(result, "Topic", ".*"))
}
