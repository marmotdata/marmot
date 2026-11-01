package pubsub_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	gpubsub "cloud.google.com/go/pubsub"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses: plugintest.Build compiles the main package
// and every call spawns the process, runs one RPC and kills it again.
//
// They need a Pub/Sub emulator:
//
//	docker run -d --name marmot-test-pubsub -p 18085:8085 \
//	  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
//	  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085
//	MARMOT_TEST_PUBSUB_EMULATOR=localhost:18085 go test ./...

const (
	e2eProjectID  = "marmot-e2e"
	ordersSchema  = `{"type":"record","name":"Order","fields":[{"name":"order_id","type":"string","doc":"Unique order identifier"},{"name":"amount","type":["null","double"]}]}`
	bigQueryTable = "marmot-e2e.analytics.orders_raw"
	gcsBucket     = "order-archive"

	kinesisStreamARN = "arn:aws:kinesis:us-east-1:111122223333:stream/orders-stream"
)

func emulatorHost(t *testing.T) string {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_PUBSUB_EMULATOR")
	if host == "" {
		t.Skip("set MARMOT_TEST_PUBSUB_EMULATOR to a Pub/Sub emulator address, for example localhost:18085")
	}
	return host
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()
	return pluginsdk.RawConfig{
		"project_id":    e2eProjectID,
		"emulator_host": emulatorHost(t),
	}
}

// seedEmulator creates the fixtures the assertions below expect. The emulator
// keeps state for the life of the container, so everything is deleted first
// and the test can be re-run against the same container.
func seedEmulator(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	host := emulatorHost(t)
	opts := []option.ClientOption{
		option.WithEndpoint(host),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	client, err := gpubsub.NewClient(ctx, e2eProjectID, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	schemaClient, err := gpubsub.NewSchemaClient(ctx, e2eProjectID, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { schemaClient.Close() })

	// Delete in dependency order: subscriptions reference topics, topics
	// reference schemas.
	for _, id := range []string{"orders-sub", "orders-push", "orders-bq", "orders-gcs"} {
		_ = client.Subscription(id).Delete(ctx)
	}
	for _, id := range []string{"orders", "orders-dlq", "orders-typed", "orders-from-kinesis"} {
		_ = client.Topic(id).Delete(ctx)
	}
	_ = schemaClient.DeleteSchema(ctx, "order")

	_, err = schemaClient.CreateSchema(ctx, "order", gpubsub.SchemaConfig{
		Type:       gpubsub.SchemaAvro,
		Definition: ordersSchema,
	})
	require.NoError(t, err)

	orders, err := client.CreateTopicWithConfig(ctx, "orders", &gpubsub.TopicConfig{
		Labels: map[string]string{"team": "commerce"},
	})
	require.NoError(t, err)

	dlq, err := client.CreateTopic(ctx, "orders-dlq")
	require.NoError(t, err)

	_, err = client.CreateTopicWithConfig(ctx, "orders-typed", &gpubsub.TopicConfig{
		SchemaSettings: &gpubsub.SchemaSettings{
			Schema:   "projects/" + e2eProjectID + "/schemas/order",
			Encoding: gpubsub.EncodingJSON,
		},
	})
	require.NoError(t, err)

	_, err = client.CreateTopicWithConfig(ctx, "orders-from-kinesis", &gpubsub.TopicConfig{
		IngestionDataSourceSettings: &gpubsub.IngestionDataSourceSettings{
			Source: &gpubsub.IngestionDataSourceAWSKinesis{
				StreamARN:         kinesisStreamARN,
				ConsumerARN:       kinesisStreamARN + "/consumer/marmot:1",
				AWSRoleARN:        "arn:aws:iam::111122223333:role/pubsub-ingestion",
				GCPServiceAccount: "ingestion@" + e2eProjectID + ".iam.gserviceaccount.com",
			},
		},
	})
	require.NoError(t, err)

	_, err = client.CreateSubscription(ctx, "orders-sub", gpubsub.SubscriptionConfig{
		Topic:             orders,
		AckDeadline:       30 * time.Second,
		RetentionDuration: 7 * 24 * time.Hour,
		Labels:            map[string]string{"team": "commerce"},
		DeadLetterPolicy: &gpubsub.DeadLetterPolicy{
			DeadLetterTopic:     dlq.String(),
			MaxDeliveryAttempts: 5,
		},
	})
	require.NoError(t, err)

	_, err = client.CreateSubscription(ctx, "orders-push", gpubsub.SubscriptionConfig{
		Topic:       orders,
		AckDeadline: 10 * time.Second,
		PushConfig:  gpubsub.PushConfig{Endpoint: "https://example.com/hooks/orders"},
	})
	require.NoError(t, err)

	_, err = client.CreateSubscription(ctx, "orders-bq", gpubsub.SubscriptionConfig{
		Topic:          orders,
		AckDeadline:    10 * time.Second,
		BigQueryConfig: gpubsub.BigQueryConfig{Table: bigQueryTable, WriteMetadata: true},
	})
	require.NoError(t, err)

	_, err = client.CreateSubscription(ctx, "orders-gcs", gpubsub.SubscriptionConfig{
		Topic:       orders,
		AckDeadline: 10 * time.Second,
		CloudStorageConfig: gpubsub.CloudStorageConfig{
			Bucket:         gcsBucket,
			FilenamePrefix: "orders-",
			FilenameSuffix: ".json",
		},
	})
	require.NoError(t, err)

	publisher := client.Topic("orders")
	defer publisher.Stop()
	for _, body := range []string{`{"order_id":"a1"}`, `{"order_id":"a2"}`} {
		_, err := publisher.Publish(ctx, &gpubsub.Message{
			Data:       []byte(body),
			Attributes: map[string]string{"region": "eu"},
		}).Get(ctx)
		require.NoError(t, err)
	}
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	seedEmulator(t)

	result, err := buildBinary(t).Discover(t.Context(), e2eConfig(t))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func e2eAssets(t *testing.T, result *pluginsdk.DiscoveryResult) map[string]pluginsdk.Asset {
	t.Helper()

	byName := make(map[string]pluginsdk.Asset, len(result.Assets))
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		byName[*a.Name] = a
	}
	return byName
}

func e2eHasEdge(edges []pluginsdk.LineageEdge, source, target, edgeType string) bool {
	for _, e := range edges {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "pubsub", meta.ID)
	assert.Equal(t, "Google Pub/Sub", meta.Name)
	assert.Equal(t, "messaging", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview)
}

func TestE2E_ValidateMissingProjectIDFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAnEmulatorConfig(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), e2eConfig(t))
	require.NoError(t, err)
}

func TestE2E_DiscoversTopicsOverTheWire(t *testing.T) {
	byName := e2eAssets(t, discoverE2E(t))

	for _, id := range []string{"orders", "orders-dlq", "orders-typed", "orders-from-kinesis"} {
		require.Contains(t, byName, id)
		assert.Equal(t, "Topic", byName[id].Type)
		assert.Equal(t, []string{"GooglePubSub"}, byName[id].Providers)
	}
}

func TestE2E_TopicCarriesItsLabelsAndSubscriptionCount(t *testing.T) {
	metadata := e2eAssets(t, discoverE2E(t))["orders"].Metadata

	assert.Equal(t, "projects/marmot-e2e/topics/orders", metadata["topic_name"])
	assert.Equal(t, map[string]any{"team": "commerce"}, metadata["labels"])
	assert.EqualValues(t, 4, metadata["subscription_count"])
	assert.ElementsMatch(t,
		[]any{"orders-sub", "orders-push", "orders-bq", "orders-gcs"},
		metadata["subscriptions"])
}

func TestE2E_DiscoversSubscriptionsWithTheirDeliveryTypes(t *testing.T) {
	byName := e2eAssets(t, discoverE2E(t))

	require.Contains(t, byName, "orders-sub")
	assert.Equal(t, "Subscription", byName["orders-sub"].Type)
	assert.Equal(t, "pull", byName["orders-sub"].Metadata["delivery_type"])
	assert.Equal(t, "push", byName["orders-push"].Metadata["delivery_type"])
	assert.Equal(t, "bigquery", byName["orders-bq"].Metadata["delivery_type"])
	assert.Equal(t, "cloud_storage", byName["orders-gcs"].Metadata["delivery_type"])
}

func TestE2E_PullSubscriptionCarriesItsSettings(t *testing.T) {
	metadata := e2eAssets(t, discoverE2E(t))["orders-sub"].Metadata

	assert.Equal(t, "orders", metadata["topic"])
	assert.EqualValues(t, 30, metadata["ack_deadline_seconds"])
	assert.Equal(t, "168h0m0s", metadata["message_retention"])
	assert.Equal(t, map[string]any{"team": "commerce"}, metadata["labels"])
	assert.Equal(t, "orders-dlq", metadata["dead_letter_topic"])
	assert.EqualValues(t, 5, metadata["max_delivery_attempts"])
}

func TestE2E_ExportSubscriptionsCarryTheirDestinations(t *testing.T) {
	byName := e2eAssets(t, discoverE2E(t))

	assert.Equal(t, bigQueryTable, byName["orders-bq"].Metadata["bigquery_table"])
	assert.Equal(t, gcsBucket, byName["orders-gcs"].Metadata["cloud_storage_bucket"])
	assert.Equal(t, "https://example.com/hooks/orders", byName["orders-push"].Metadata["push_endpoint"])
}

func TestE2E_TopicFeedsItsSubscriptions(t *testing.T) {
	result := discoverE2E(t)

	for _, id := range []string{"orders-sub", "orders-push", "orders-bq", "orders-gcs"} {
		assert.True(t, e2eHasEdge(result.Lineage,
			"mrn://topic/googlepubsub/orders",
			"mrn://subscription/googlepubsub/"+id,
			"FEEDS"), "expected orders to feed %s", id)
	}
}

func TestE2E_SubscriptionProducesItsBigQueryTable(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, e2eHasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-bq",
		"mrn://table/bigquery/orders_raw",
		"PRODUCES"))
}

func TestE2E_SubscriptionProducesItsCloudStorageBucket(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, e2eHasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-gcs",
		"mrn://bucket/gcs/order-archive",
		"PRODUCES"))
}

func TestE2E_SubscriptionFeedsItsDeadLetterTopic(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, e2eHasEdge(result.Lineage,
		"mrn://subscription/googlepubsub/orders-sub",
		"mrn://topic/googlepubsub/orders-dlq",
		"FEEDS"))
}

// A topic that ingests from Kinesis is fed by the stream, which belongs to
// the Kinesis plugin. Only the edge is emitted, so its target has to match
// that plugin's naming exactly.
func TestE2E_KinesisIngestionStreamFeedsTheTopic(t *testing.T) {
	result := discoverE2E(t)

	assert.Equal(t, "aws_kinesis", e2eAssets(t, result)["orders-from-kinesis"].Metadata["ingestion_source"])
	assert.True(t, e2eHasEdge(result.Lineage,
		"mrn://stream/kinesis/orders-stream",
		"mrn://topic/googlepubsub/orders-from-kinesis",
		"FEEDS"))
}

func TestE2E_TopicSchemaBecomesColumns(t *testing.T) {
	asset := e2eAssets(t, discoverE2E(t))["orders-typed"]

	assert.Equal(t, "order", asset.Metadata["schema"])
	assert.Equal(t, "AVRO", asset.Metadata["schema_type"])
	assert.Equal(t, "JSON", asset.Metadata["schema_encoding"])
	assert.Equal(t, ordersSchema, asset.Schema["schema"])

	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(asset.Schema["columns"]), &columns))
	require.Len(t, columns, 2)
	assert.Equal(t, "order_id", columns[0].Name)
	assert.Equal(t, "string", columns[0].DataType)
	assert.Equal(t, "Unique order identifier", columns[0].Description)
	assert.Equal(t, "amount", columns[1].Name)
	assert.True(t, columns[1].Nullable)
}

// The emulator has no Google Cloud console behind it, so a link there would
// be a dead end.
func TestE2E_EmulatorAssetsHaveNoConsoleLink(t *testing.T) {
	asset := e2eAssets(t, discoverE2E(t))["orders"]

	assert.Empty(t, asset.ExternalLinks)
}

func TestE2E_DiscoverWithSubscriptionsDisabledReturnsTopicsOnly(t *testing.T) {
	seedEmulator(t)

	config := e2eConfig(t)
	config["include_subscriptions"] = false

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	for _, a := range result.Assets {
		assert.Equal(t, "Topic", a.Type)
	}
	// Only subscription edges go away; a topic's ingestion source is not a
	// subscription and still shows up.
	for _, e := range result.Lineage {
		assert.NotContains(t, e.Target, "mrn://subscription/")
		assert.NotContains(t, e.Source, "mrn://subscription/")
	}
}

func TestE2E_FetchSampleDataReadsPublishedMessages(t *testing.T) {
	seedEmulator(t)

	config := e2eConfig(t)
	config["include_sample_messages"] = true

	name := "orders-sub"
	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), config,
		&pluginsdk.Asset{Name: &name, Type: "Subscription"})
	require.NoError(t, err)

	assert.Equal(t, []string{"message_id", "publish_time", "ordering_key", "attributes", "data"}, columns)
	require.NotEmpty(t, rows, "expected the two published messages to come back")

	bodies := make([]string, 0, len(rows))
	for _, row := range rows {
		bodies = append(bodies, row[4].(string))
	}
	assert.Contains(t, bodies, `{"order_id":"a1"}`)
}

func TestE2E_FetchSampleDataFailsWhenSamplingIsDisabled(t *testing.T) {
	name := "orders-sub"
	_, _, err := buildBinary(t).FetchSampleData(t.Context(), e2eConfig(t),
		&pluginsdk.Asset{Name: &name, Type: "Subscription"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "include_sample_messages")
}
