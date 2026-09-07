package kinesis_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a Kinesis endpoint (moto or
// LocalStack) named by MARMOT_TEST_KINESIS_ENDPOINT. The endpoint is
// seeded on every run; seeding is idempotent so a reused container works.

const (
	e2eRegion = "us-east-1"
	e2eKey    = "test"
)

func endpoint(t *testing.T) string {
	t.Helper()

	endpoint := os.Getenv("MARMOT_TEST_KINESIS_ENDPOINT")
	if endpoint == "" {
		t.Skip("MARMOT_TEST_KINESIS_ENDPOINT not set (for example http://localhost:15555), skipping Kinesis e2e tests")
	}
	return endpoint
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func e2eConfig(endpoint string, extra pluginsdk.RawConfig) pluginsdk.RawConfig {
	raw := pluginsdk.RawConfig{
		"credentials": map[string]interface{}{
			"region":   e2eRegion,
			"id":       e2eKey,
			"secret":   e2eKey,
			"endpoint": endpoint,
		},
	}
	for k, v := range extra {
		raw[k] = v
	}
	return raw
}

func newClient(t *testing.T, endpoint string) *kinesis.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion(e2eRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(e2eKey, e2eKey, "")),
	)
	require.NoError(t, err)
	cfg.BaseEndpoint = aws.String(endpoint)

	return kinesis.NewFromConfig(cfg)
}

// seed creates the fixture streams: orders (2 provisioned shards, 48h
// retention, tagged, one consumer, a few JSON records) and clicks
// (on-demand). Every step tolerates the object already existing.
func seed(t *testing.T, endpoint string) {
	t.Helper()
	ctx := t.Context()
	client := newClient(t, endpoint)

	createStream(t, client, &kinesis.CreateStreamInput{
		StreamName:        aws.String("orders"),
		ShardCount:        aws.Int32(2),
		StreamModeDetails: &types.StreamModeDetails{StreamMode: types.StreamModeProvisioned},
	})
	createStream(t, client, &kinesis.CreateStreamInput{
		StreamName:        aws.String("clicks"),
		StreamModeDetails: &types.StreamModeDetails{StreamMode: types.StreamModeOnDemand},
	})

	orders := waitActive(t, client, "orders")
	waitActive(t, client, "clicks")

	if aws.ToInt32(orders.RetentionPeriodHours) < 48 {
		_, err := client.IncreaseStreamRetentionPeriod(ctx, &kinesis.IncreaseStreamRetentionPeriodInput{
			StreamName:           aws.String("orders"),
			RetentionPeriodHours: aws.Int32(48),
		})
		require.NoError(t, err)
	}

	_, err := client.AddTagsToStream(ctx, &kinesis.AddTagsToStreamInput{
		StreamName: aws.String("orders"),
		Tags:       map[string]string{"team": "data", "env": "test"},
	})
	require.NoError(t, err)

	// AWS refuses a second consumer with the same name; moto registers a
	// duplicate instead, so look before registering.
	if !hasConsumer(t, client, orders.StreamARN, "orders-consumer") {
		_, err = client.RegisterStreamConsumer(ctx, &kinesis.RegisterStreamConsumerInput{
			StreamARN:    orders.StreamARN,
			ConsumerName: aws.String("orders-consumer"),
		})
		if err != nil && !isResourceInUse(err) {
			require.NoError(t, err)
		}
	}

	// Server-side encryption is optional in the fixture: moto supports it
	// today, but the assertions only require the field to be reported.
	if orders.EncryptionType != types.EncryptionTypeKms {
		_, err = client.StartStreamEncryption(ctx, &kinesis.StartStreamEncryptionInput{
			StreamName:     aws.String("orders"),
			EncryptionType: types.EncryptionTypeKms,
			KeyId:          aws.String("alias/aws/kinesis"),
		})
		if err != nil {
			t.Logf("StartStreamEncryption not supported by this endpoint, continuing: %v", err)
		}
	}

	var entries []types.PutRecordsRequestEntry
	for i := 1; i <= 3; i++ {
		entries = append(entries, types.PutRecordsRequestEntry{
			PartitionKey: aws.String(fmt.Sprintf("order-%d", i)),
			Data:         []byte(fmt.Sprintf(`{"order_id": %d, "amount": %d.5, "currency": "EUR"}`, i, i*10)),
		})
	}
	out, err := client.PutRecords(ctx, &kinesis.PutRecordsInput{StreamName: aws.String("orders"), Records: entries})
	require.NoError(t, err)
	require.Zero(t, aws.ToInt32(out.FailedRecordCount))
}

func createStream(t *testing.T, client *kinesis.Client, in *kinesis.CreateStreamInput) {
	t.Helper()

	_, err := client.CreateStream(t.Context(), in)
	if err != nil && !isResourceInUse(err) {
		require.NoError(t, err)
	}
}

func hasConsumer(t *testing.T, client *kinesis.Client, streamARN *string, name string) bool {
	t.Helper()

	out, err := client.ListStreamConsumers(t.Context(), &kinesis.ListStreamConsumersInput{StreamARN: streamARN})
	require.NoError(t, err)
	for _, c := range out.Consumers {
		if aws.ToString(c.ConsumerName) == name {
			return true
		}
	}
	return false
}

func isResourceInUse(err error) bool {
	var inUse *types.ResourceInUseException
	return errors.As(err, &inUse)
}

func waitActive(t *testing.T, client *kinesis.Client, name string) *types.StreamDescriptionSummary {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	for {
		out, err := client.DescribeStreamSummary(t.Context(), &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(name)})
		require.NoError(t, err)
		if out.StreamDescriptionSummary.StreamStatus == types.StreamStatusActive {
			return out.StreamDescriptionSummary
		}
		require.False(t, time.Now().After(deadline), "stream %s did not become ACTIVE", name)
		time.Sleep(500 * time.Millisecond)
	}
}

func discover(t *testing.T, bin plugintest.Binary, raw pluginsdk.RawConfig) map[string]pluginsdk.Asset {
	t.Helper()

	result, err := bin.Discover(t.Context(), raw)
	require.NoError(t, err)
	require.NotNil(t, result)

	byName := make(map[string]pluginsdk.Asset)
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		byName[*a.Name] = a
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	endpoint(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "kinesis", meta.ID)
	assert.Equal(t, "Kinesis", meta.Name)
	assert.Equal(t, "messaging", meta.Category)
	assert.Equal(t, []string{"Assets"}, meta.Features)
	assert.True(t, meta.SupportsDataPreview, "Serve flags the DataFetcher implementation")
}

func TestE2E_ValidateRejectsABadEndpoint(t *testing.T) {
	// The config has no required field: an empty config means the default
	// credential chain. The one thing Validate can refuse is a malformed
	// endpoint.
	endpoint(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{
		"credentials": map[string]interface{}{"endpoint": "not a url"},
	})
	require.Error(t, err)
}

func TestE2E_DiscoverStreamsOverTheWire(t *testing.T) {
	ep := endpoint(t)
	seed(t, ep)
	bin := buildBinary(t)

	assets := discover(t, bin, e2eConfig(ep, pluginsdk.RawConfig{"tags": []interface{}{"kinesis"}}))

	orders, ok := assets["orders"]
	require.True(t, ok, "expected the orders stream, got %v", keys(assets))
	assert.Equal(t, "Stream", orders.Type)
	assert.Equal(t, []string{"Kinesis"}, orders.Providers)
	assert.Equal(t, "mrn://stream/kinesis/orders", *orders.MRN)
	assert.Equal(t, []string{"kinesis"}, orders.Tags)
	assert.Nil(t, orders.Description, "no description tag was set")

	m := orders.Metadata
	assert.Equal(t, "ACTIVE", m["status"])
	assert.Equal(t, "PROVISIONED", m["stream_mode"])
	assert.Equal(t, float64(48), m["retention_hours"])
	assert.Equal(t, float64(2), m["shard_count"])
	assert.Equal(t, float64(2), m["open_shard_count"])
	assert.Equal(t, "data", m["tag_team"])
	assert.Equal(t, "test", m["tag_env"])
	assert.Equal(t, []interface{}{"orders-consumer"}, m["consumers"])
	assert.Equal(t, float64(1), m["consumer_count"])
	assert.Equal(t, e2eRegion, m["region"])
	assert.Equal(t, "https://us-east-1.console.aws.amazon.com/kinesis/home?region=us-east-1#/streams/details/orders/monitoring", m["url"])
	assert.True(t, strings.HasSuffix(m["arn"].(string), ":stream/orders"), "arn: %v", m["arn"])
	assert.NotEmpty(t, m["created_at"])
	assert.Contains(t, m, "encryption_type")
	if m["encryption_type"] == "KMS" {
		assert.Equal(t, "alias/aws/kinesis", m["kms_key_id"])
	}

	require.Len(t, orders.ExternalLinks, 1)
	assert.Equal(t, "Open in AWS Console", orders.ExternalLinks[0].Name)

	clicks, ok := assets["clicks"]
	require.True(t, ok, "expected the clicks stream, got %v", keys(assets))
	assert.Equal(t, "ON_DEMAND", clicks.Metadata["stream_mode"])
	assert.Equal(t, float64(24), clicks.Metadata["retention_hours"])
	assert.Greater(t, clicks.Metadata["shard_count"], float64(0))
	assert.NotContains(t, clicks.Metadata, "consumers")
	assert.NotContains(t, clicks.Metadata, "tag_team")
}

func TestE2E_DiscoverWithoutShardsAndConsumers(t *testing.T) {
	ep := endpoint(t)
	seed(t, ep)
	bin := buildBinary(t)

	assets := discover(t, bin, e2eConfig(ep, pluginsdk.RawConfig{
		"include_shards":    false,
		"include_consumers": false,
	}))

	orders, ok := assets["orders"]
	require.True(t, ok)
	assert.NotContains(t, orders.Metadata, "shard_count")
	assert.NotContains(t, orders.Metadata, "consumers")
	assert.Equal(t, float64(2), orders.Metadata["open_shard_count"], "the summary still reports open shards")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	ep := endpoint(t)
	seed(t, ep)
	bin := buildBinary(t)

	name := "orders"
	columns, rows, err := bin.FetchSampleData(t.Context(), e2eConfig(ep, nil), &pluginsdk.Asset{
		Name: &name,
		Type: "Stream",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"sequence_number", "partition_key", "arrival_time", "data"}, columns)
	require.NotEmpty(t, rows, "the first open shard holds seeded records")
	assert.LessOrEqual(t, len(rows), 20)

	for _, row := range rows {
		require.Len(t, row, 4)
		assert.NotEmpty(t, row[0], "sequence number")
		assert.True(t, strings.HasPrefix(row[1].(string), "order-"), "partition key: %v", row[1])
		assert.Contains(t, row[3], `"order_id":`, "the JSON payload comes back compacted")
	}
}

func keys(assets map[string]pluginsdk.Asset) []string {
	var names []string
	for name := range assets {
		names = append(names, name)
	}
	return names
}
