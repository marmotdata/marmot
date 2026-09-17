package pubsub

import (
	"context"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesTheMessagingPlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "pubsub", meta.ID)
	assert.Equal(t, "Google Pub/Sub", meta.Name)
	assert.Equal(t, "messaging", meta.Category)
	assert.Equal(t, "googlepubsub", meta.Icon)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "company-streaming"})
	require.NoError(t, err)
}

func TestValidate_MissingProjectIDFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id")
}

func TestValidate_DefaultsDiscoveryFlagsToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "company-streaming"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeSubscriptions)
	assert.True(t, s.config.IncludeSchemas)
	assert.True(t, s.config.IncludeDeadLetterTopics)
}

func TestValidate_DefaultsSampleMessagesToFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "company-streaming"})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeSampleMessages)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":            "company-streaming",
		"include_subscriptions": false,
		"include_schemas":       false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeSubscriptions)
	assert.False(t, s.config.IncludeSchemas)
	assert.True(t, s.config.IncludeDeadLetterTopics, "untouched flags still default to true")
}

func TestValidate_StripsASchemeFromTheEmulatorHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":    "test-project",
		"emulator_host": "http://localhost:8085/",
	})
	require.NoError(t, err)

	assert.Equal(t, "localhost:8085", s.config.EmulatorHost)
}

func TestValidate_RejectsTwoCredentialSources(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":       "company-streaming",
		"credentials_file": "/etc/marmot/key.json",
		"credentials_json": `{"type":"service_account"}`,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of credentials_file or credentials_json")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id": "company-streaming",
		"filter": map[string]interface{}{
			"include": []interface{}{"^orders.*"},
			"exclude": []interface{}{".*-dlq$"},
		},
	})
	require.NoError(t, err)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes the
// flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{ProjectID: "company-streaming"}

	assert.False(t, config.IncludeSubscriptions)
	assert.False(t, config.IncludeSchemas)
	assert.False(t, config.IncludeDeadLetterTopics)
}

func TestDeliveryType_DefaultsToPull(t *testing.T) {
	assert.Equal(t, "pull", deliveryType(subscriptionInfo{ID: "orders-sub"}))
}

func TestDeliveryType_PushWhenAnEndpointIsSet(t *testing.T) {
	assert.Equal(t, "push", deliveryType(subscriptionInfo{PushEndpoint: "https://example.com/hooks"}))
}

func TestDeliveryType_BigQueryWhenATableIsSet(t *testing.T) {
	assert.Equal(t, "bigquery", deliveryType(subscriptionInfo{BigQueryTable: "p.d.t"}))
}

func TestDeliveryType_CloudStorageWhenABucketIsSet(t *testing.T) {
	assert.Equal(t, "cloud_storage", deliveryType(subscriptionInfo{CloudStorageBucket: "archive"}))
}

func TestBigQueryTableID_TakesTheTableFromADottedTarget(t *testing.T) {
	assert.Equal(t, "orders_raw", bigQueryTableID("test-project.analytics.orders_raw"))
}

// The BigQuery API also writes the project separator as a colon.
func TestBigQueryTableID_TakesTheTableFromAColonSeparatedTarget(t *testing.T) {
	assert.Equal(t, "orders_raw", bigQueryTableID("test-project:analytics.orders_raw"))
}

func TestBigQueryTableID_IsEmptyForAnEmptyTarget(t *testing.T) {
	assert.Equal(t, "", bigQueryTableID(""))
}

func TestKinesisStreamName_TakesTheNameFromAnARN(t *testing.T) {
	assert.Equal(t, "orders-stream", kinesisStreamName("arn:aws:kinesis:us-east-1:111122223333:stream/orders-stream"))
}

func TestKinesisStreamName_IsEmptyForSomethingThatIsNotAnARN(t *testing.T) {
	assert.Equal(t, "", kinesisStreamName("orders-stream"))
}

func TestKinesisStreamName_IsEmptyForAnARNWithoutAStream(t *testing.T) {
	assert.Equal(t, "", kinesisStreamName("arn:aws:sqs:us-east-1:111122223333:orders-queue"))
}

func TestLastSegment_TakesTheIDFromAResourceName(t *testing.T) {
	assert.Equal(t, "orders", lastSegment("projects/test-project/topics/orders"))
}

func TestLastSegment_LeavesABareIDAlone(t *testing.T) {
	assert.Equal(t, "orders", lastSegment("orders"))
}

func TestFetchSampleData_FailsWhenSamplingIsDisabled(t *testing.T) {
	s := &Source{newClient: func(context.Context, *Config) (client, error) { return newFakeProject(), nil }}

	name := "orders-sub"
	_, _, err := s.FetchSampleData(context.Background(),
		pluginsdk.RawConfig{"project_id": "test-project"},
		&pluginsdk.Asset{Name: &name, Type: "Subscription"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "include_sample_messages")
}

func TestFetchSampleData_ReadsMessagesFromASubscriptionAsset(t *testing.T) {
	fake := newFakeProject()
	fake.messages = []sampleMessage{{
		ID:          "1",
		Data:        []byte(`{"order_id":"a1"}`),
		Attributes:  map[string]string{"region": "eu"},
		PublishTime: time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC),
		OrderingKey: "a1",
	}}
	s := &Source{newClient: func(context.Context, *Config) (client, error) { return fake, nil }}

	name := "orders-sub"
	columns, rows, err := s.FetchSampleData(context.Background(),
		pluginsdk.RawConfig{"project_id": "test-project", "include_sample_messages": true},
		&pluginsdk.Asset{Name: &name, Type: "Subscription"})
	require.NoError(t, err)

	assert.Equal(t, []string{"message_id", "publish_time", "ordering_key", "attributes", "data"}, columns)
	require.Len(t, rows, 1)
	assert.Equal(t, "1", rows[0][0])
	assert.Equal(t, "2026-09-08T10:00:00Z", rows[0][1])
	assert.Equal(t, "a1", rows[0][2])
	assert.Equal(t, map[string]string{"region": "eu"}, rows[0][3])
	assert.Equal(t, `{"order_id":"a1"}`, rows[0][4])
}

// A topic cannot be read from directly, so the preview borrows one of its
// pull subscriptions.
func TestFetchSampleData_BorrowsAPullSubscriptionForATopicAsset(t *testing.T) {
	fake := newFakeProject()
	fake.messages = []sampleMessage{{ID: "1", Data: []byte("hello")}}
	s := &Source{newClient: func(context.Context, *Config) (client, error) { return fake, nil }}

	name := "orders"
	_, rows, err := s.FetchSampleData(context.Background(),
		pluginsdk.RawConfig{"project_id": "test-project", "include_sample_messages": true},
		&pluginsdk.Asset{Name: &name, Type: "Topic"})
	require.NoError(t, err)

	require.Len(t, rows, 1)
	assert.Equal(t, "hello", rows[0][4])
}

func TestFetchSampleData_FailsForATopicWithoutAPullSubscription(t *testing.T) {
	fake := newFakeProject()
	s := &Source{newClient: func(context.Context, *Config) (client, error) { return fake, nil }}

	name := "orders-dlq"
	_, _, err := s.FetchSampleData(context.Background(),
		pluginsdk.RawConfig{"project_id": "test-project", "include_sample_messages": true},
		&pluginsdk.Asset{Name: &name, Type: "Topic"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pull subscription")
}

func TestFetchSampleData_RejectsAnAssetKindItCannotRead(t *testing.T) {
	fake := newFakeProject()
	s := &Source{newClient: func(context.Context, *Config) (client, error) { return fake, nil }}

	name := "orders_raw"
	_, _, err := s.FetchSampleData(context.Background(),
		pluginsdk.RawConfig{"project_id": "test-project", "include_sample_messages": true},
		&pluginsdk.Asset{Name: &name, Type: "Table"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Topic and Subscription")
}
