package pubsub

import (
	"context"
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discoverFake(t *testing.T, s *Source, fake *fakeClient) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := s.discover(context.Background(), fake)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestDiscover_CreatesATopicAssetPerTopic(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	byName := assetsByName(result.Assets)
	require.Contains(t, byName, "orders")
	require.Contains(t, byName, "orders-dlq")

	assert.Equal(t, "Topic", byName["orders"].Type)
	assert.Equal(t, []string{"GooglePubSub"}, byName["orders"].Providers)
}

func TestDiscover_CreatesASubscriptionAssetPerSubscription(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	byName := assetsByName(result.Assets)
	for _, id := range []string{"orders-sub", "orders-push", "orders-bq", "orders-gcs"} {
		require.Contains(t, byName, id)
		assert.Equal(t, "Subscription", byName[id].Type)
	}
}

func TestDiscover_TopicMetadataCarriesTheProjectAndFullName(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	metadata := assetsByName(result.Assets)["orders"].Metadata
	assert.Equal(t, "test-project", metadata["project_id"])
	assert.Equal(t, "projects/test-project/topics/orders", metadata["topic_name"])
}

func TestDiscover_TopicMetadataCarriesLabelsAndStorageSettings(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	metadata := assetsByName(result.Assets)["orders"].Metadata
	assert.Equal(t, map[string]string{"team": "commerce"}, metadata["labels"])
	assert.Equal(t, []string{"europe-west1"}, metadata["allowed_persistence_regions"])
	assert.Equal(t, "projects/test-project/locations/eu/keyRings/r/cryptoKeys/k", metadata["kms_key_name"])
	assert.Equal(t, "24h0m0s", metadata["retention"])
}

func TestDiscover_TopicMetadataCountsAndListsItsSubscriptions(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	metadata := assetsByName(result.Assets)["orders"].Metadata
	assert.Equal(t, 4, metadata["subscription_count"])
	assert.Equal(t, []string{"orders-sub", "orders-push", "orders-bq", "orders-gcs"}, metadata["subscriptions"])
}

func TestDiscover_TopicWithoutSubscriptionsStillReportsACount(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	metadata := assetsByName(result.Assets)["orders-dlq"].Metadata
	assert.Equal(t, 0, metadata["subscription_count"])
	assert.NotContains(t, metadata, "subscriptions", "an empty subscription list is left out")
}

func TestDiscover_TopicLinksToTheCloudConsole(t *testing.T) {
	fake := newFakeProject()
	asset := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders"]

	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "Open in Google Cloud Console", asset.ExternalLinks[0].Name)
	assert.Equal(t,
		"https://console.cloud.google.com/cloudpubsub/topic/detail/orders?project=test-project",
		asset.ExternalLinks[0].URL)
}

func TestDiscover_EmulatorTopicsHaveNoConsoleLink(t *testing.T) {
	fake := newFakeProject()
	s := newTestSource(fake)
	s.config.EmulatorHost = "localhost:8085"

	asset := assetsByName(discoverFake(t, s, fake).Assets)["orders"]

	assert.Empty(t, asset.ExternalLinks)
	assert.NotContains(t, asset.Metadata, "url")
}

func TestDiscover_SubscriptionLinksToTheCloudConsole(t *testing.T) {
	fake := newFakeProject()
	asset := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-sub"]

	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t,
		"https://console.cloud.google.com/cloudpubsub/subscription/detail/orders-sub?project=test-project",
		asset.ExternalLinks[0].URL)
}

func TestDiscover_PullSubscriptionMetadata(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-sub"].Metadata

	assert.Equal(t, "pull", metadata["delivery_type"])
	assert.Equal(t, "orders", metadata["topic"])
	assert.Equal(t, 30, metadata["ack_deadline_seconds"])
	assert.Equal(t, "168h0m0s", metadata["message_retention"])
	assert.Equal(t, `attributes.region = "eu"`, metadata["filter"])
	assert.Equal(t, "ACTIVE", metadata["state"])
}

func TestDiscover_PushSubscriptionRecordsItsEndpoint(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-push"].Metadata

	assert.Equal(t, "push", metadata["delivery_type"])
	assert.Equal(t, "https://example.com/hooks/orders", metadata["push_endpoint"])
}

func TestDiscover_BigQuerySubscriptionRecordsItsTable(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-bq"].Metadata

	assert.Equal(t, "bigquery", metadata["delivery_type"])
	assert.Equal(t, "test-project.analytics.orders_raw", metadata["bigquery_table"])
}

func TestDiscover_BigQuerySubscriptionRecordsItsExportState(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-bq"].Metadata

	assert.Equal(t, "ACTIVE", metadata["bigquery_state"])
	assert.Equal(t, false, metadata["bigquery_use_topic_schema"])
}

func TestDiscover_CloudStorageSubscriptionRecordsItsObjectNaming(t *testing.T) {
	fake := newFakeProject()
	fake.subscriptions["orders"][3].CloudStorageFilenamePrefix = "orders-"
	fake.subscriptions["orders"][3].CloudStorageFilenameSuffix = ".json"

	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-gcs"].Metadata

	assert.Equal(t, "orders-", metadata["cloud_storage_filename_prefix"])
	assert.Equal(t, ".json", metadata["cloud_storage_filename_suffix"])
	assert.Equal(t, "ACTIVE", metadata["cloud_storage_state"])
}

func TestDiscover_CloudStorageSubscriptionRecordsItsBucket(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-gcs"].Metadata

	assert.Equal(t, "cloud_storage", metadata["delivery_type"])
	assert.Equal(t, "order-archive", metadata["cloud_storage_bucket"])
}

func TestDiscover_SubscriptionRecordsItsDeadLetterPolicy(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders-sub"].Metadata

	assert.Equal(t, "orders-dlq", metadata["dead_letter_topic"])
	assert.Equal(t, 5, metadata["max_delivery_attempts"])
}

func TestDiscover_DeadLetterPolicyIsSkippedWhenDisabled(t *testing.T) {
	fake := newFakeProject()
	s := newTestSource(fake)
	s.config.IncludeDeadLetterTopics = false

	result := discoverFake(t, s, fake)
	metadata := assetsByName(result.Assets)["orders-sub"].Metadata

	assert.NotContains(t, metadata, "dead_letter_topic")
	assert.False(t, hasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-sub",
		"mrn://topic/googlepubsub/orders-dlq",
		"FEEDS"))
}

func TestDiscover_SubscriptionsAreSkippedWhenDisabled(t *testing.T) {
	fake := newFakeProject()
	s := newTestSource(fake)
	s.config.IncludeSubscriptions = false

	result := discoverFake(t, s, fake)

	byName := assetsByName(result.Assets)
	assert.NotContains(t, byName, "orders-sub")
	assert.Len(t, result.Assets, 2, "only the two topics remain")
	assert.Empty(t, result.Lineage)
	// A count of zero would read as "this topic has no subscribers".
	assert.NotContains(t, byName["orders"].Metadata, "subscription_count")
}

func TestDiscover_TopicFeedsItsSubscriptions(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	assert.True(t, hasEdge(result.Lineage,
		"mrn://topic/googlepubsub/orders",
		"mrn://subscription/googlepubsub/orders-sub",
		"FEEDS"))
}

func TestDiscover_BigQuerySubscriptionProducesTheBigQueryTable(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	assert.True(t, hasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-bq",
		"mrn://table/bigquery/orders_raw",
		"PRODUCES"))
}

func TestDiscover_CloudStorageSubscriptionProducesTheBucket(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	assert.True(t, hasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-gcs",
		"mrn://bucket/gcs/order-archive",
		"PRODUCES"))
}

func TestDiscover_SubscriptionFeedsItsDeadLetterTopic(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	assert.True(t, hasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-sub",
		"mrn://topic/googlepubsub/orders-dlq",
		"FEEDS"))
}

func TestDiscover_KinesisIngestionStreamFeedsTheTopic(t *testing.T) {
	fake := newFakeProject()
	fake.topics[0].IngestionSource = "aws_kinesis"
	fake.topics[0].IngestionKinesisStreamARN = "arn:aws:kinesis:us-east-1:111122223333:stream/orders-stream"

	result := discoverFake(t, newTestSource(fake), fake)

	assert.Equal(t, "aws_kinesis", assetsByName(result.Assets)["orders"].Metadata["ingestion_source"])
	assert.True(t, hasEdge(result.Lineage,
		"mrn://stream/kinesis/orders-stream",
		"mrn://topic/googlepubsub/orders",
		"FEEDS"))
}

func TestDiscover_TopicSchemaMetadata(t *testing.T) {
	fake := newFakeProject()
	metadata := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders"].Metadata

	assert.Equal(t, "order", metadata["schema"])
	assert.Equal(t, "AVRO", metadata["schema_type"])
	assert.Equal(t, "JSON", metadata["schema_encoding"])
	assert.Equal(t, "a1b2c3d4", metadata["schema_revision"])
}

func TestDiscover_TopicKeepsTheRawSchemaDefinition(t *testing.T) {
	fake := newFakeProject()
	asset := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders"]

	assert.Equal(t, ordersAvroSchema, asset.Schema["schema"])
}

func TestDiscover_TopicAvroFieldsBecomeColumns(t *testing.T) {
	fake := newFakeProject()
	asset := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders"]

	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(asset.Schema["columns"]), &columns))

	require.Len(t, columns, 3)
	assert.Equal(t, "order_id", columns[0].Name)
	assert.Equal(t, "string", columns[0].DataType)
	assert.Equal(t, "Unique order identifier", columns[0].Description)
	assert.False(t, columns[0].Nullable)
}

func TestDiscover_ProtobufSchemaKeepsTheDefinitionWithoutColumns(t *testing.T) {
	fake := newFakeProject()
	fake.schemas[0].Type = "PROTOCOL_BUFFER"
	fake.schemas[0].Definition = `syntax = "proto3"; message Order { string order_id = 1; }`

	asset := assetsByName(discoverFake(t, newTestSource(fake), fake).Assets)["orders"]

	assert.Equal(t, `syntax = "proto3"; message Order { string order_id = 1; }`, asset.Schema["schema"])
	assert.NotContains(t, asset.Schema, "columns")
}

// The Pub/Sub emulator has no schema service, and a service account may lack
// the viewer role. Neither is a reason to lose the topics.
func TestDiscover_ContinuesWhenTheSchemaServiceIsUnavailable(t *testing.T) {
	fake := newFakeProject()
	fake.schemasErr = errNoSchemaService

	result := discoverFake(t, newTestSource(fake), fake)
	asset := assetsByName(result.Assets)["orders"]

	assert.Equal(t, "order", asset.Metadata["schema"], "the topic still names its schema")
	assert.NotContains(t, asset.Metadata, "schema_type")
	assert.NotContains(t, asset.Schema, "columns")
}

func TestDiscover_SchemasAreSkippedWhenDisabled(t *testing.T) {
	fake := newFakeProject()
	s := newTestSource(fake)
	s.config.IncludeSchemas = false

	asset := assetsByName(discoverFake(t, s, fake).Assets)["orders"]

	assert.NotContains(t, asset.Metadata, "schema_type")
	assert.Empty(t, asset.Schema)
}

func TestDiscover_EveryAssetRecordsPubSubAsItsSource(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)

	for _, a := range result.Assets {
		require.Len(t, a.Sources, 1, "asset %s", *a.Name)
		assert.Equal(t, "GooglePubSub", a.Sources[0].Name)
		assert.Equal(t, 1, a.Sources[0].Priority)
	}
}
