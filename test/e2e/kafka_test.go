package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKafkaTopicIngestion tests the ingestion of Kafka topics using various configurations
func TestKafkaTopicIngestion(t *testing.T) {
	ctx := context.Background()

	err := env.EnsureRedpandaStarted(ctx, false)
	require.NoError(t, err)

	// Create test topics
	testTopics := []struct {
		name           string
		partitions     int
		replicas       int
		retentionMs    int64
		cleanupPolicy  string
		tags           map[string]string
		maxMessageSize int
	}{
		{
			name:          "test-topic-basic",
			partitions:    3,
			replicas:      1,
			retentionMs:   86400000, // 1 day
			cleanupPolicy: "delete",
			tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
		{
			name:           "test-topic-custom",
			partitions:     5,
			replicas:       1,
			retentionMs:    604800000, // 1 week
			cleanupPolicy:  "compact",
			maxMessageSize: 2097152, // 2MB
			tags: map[string]string{
				"Environment": "staging",
				"Team":        "orders",
				"CostCenter":  "cc-123",
			},
		},
	}
	for _, topic := range testTopics {
		configs := make(map[string]*string)

		retentionStr := fmt.Sprintf("%d", topic.retentionMs)
		configs["retention.ms"] = &retentionStr

		configs["cleanup.policy"] = &topic.cleanupPolicy

		if topic.maxMessageSize > 0 {
			maxMsgStr := fmt.Sprintf("%d", topic.maxMessageSize)
			configs["max.message.bytes"] = &maxMsgStr
		}

		_, err := env.KafkaAdminClient.CreateTopic(ctx, int32(topic.partitions), int16(topic.replicas), configs, topic.name)
		require.NoError(t, err, "Failed to create topic: %s", topic.name)
	}
	time.Sleep(2 * time.Second)

	configContent := fmt.Sprintf(`
runs:
  - kafka:
      bootstrap_servers: "redpanda-test:%s"
      client_id: "marmot-kafka-test"
      client_timeout_seconds: 30
      include_partition_info: true
      include_topic_config: true
      tags:
        - kafka
        - test
        - env-prod
        - team-platform
      topic_filter:
        include:
          - "^test-topic-.*"
        exclude:
          - "^_.*"
`, env.RedpandaPort)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	// Fetch all assets
	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	topic1 := utils.FindAssetByName(resp.Payload.Assets, "test-topic-basic")
	require.NotNil(t, topic1, "asset test-topic-basic not found")
	assert.Equal(t, "Topic", topic1.Type)
	assert.Contains(t, topic1.Providers, "Kafka")
	assert.Contains(t, topic1.Tags, "kafka")
	assert.Contains(t, topic1.Tags, "test")
	assert.Contains(t, topic1.Tags, "env-prod")
	assert.Contains(t, topic1.Tags, "team-platform")

	metadata1 := topic1.Metadata.(map[string]interface{})
	t.Logf("Topic metadata: %+v", metadata1)

	partitionCount1, ok := metadata1["partition_count"]
	require.True(t, ok, "partition_count not found in metadata")
	assert.Equal(t, "3", partitionCount1.(json.Number).String())

	replicationFactor1, ok := metadata1["replication_factor"]
	require.True(t, ok, "replication_factor not found in metadata")
	assert.Equal(t, "1", replicationFactor1.(json.Number).String())

	retentionMs1, ok := metadata1["retention_ms"]
	require.True(t, ok, "retention.ms not found in metadata")
	assert.Equal(t, "86400000", retentionMs1)

	cleanupPolicy1, ok := metadata1["cleanup_policy"]
	require.True(t, ok, "cleanup.policy not found in metadata")
	assert.Equal(t, "delete", cleanupPolicy1)

	// Create updated config file for second topic checks
	configContentCustom := fmt.Sprintf(`
runs:
  - kafka:
      bootstrap_servers: "redpanda-test:%s"
      client_id: "marmot-kafka-test"
      client_timeout_seconds: 30
      include_partition_info: true
      include_topic_config: true
      tags:
        - kafka
        - test
        - env-staging
        - team-orders
      topic_filter:
        include:
          - "^test-topic-custom$"
        exclude:
          - "^_.*"
`, env.RedpandaPort)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContentCustom,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	params = assets.NewGetAssetsListParams()
	resp, err = env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	topic2 := utils.FindAssetByName(resp.Payload.Assets, "test-topic-custom")
	require.NotNil(t, topic2, "asset test-topic-custom not found")

	assert.Equal(t, "Topic", topic2.Type)
	assert.Contains(t, topic2.Providers, "Kafka")
	assert.Contains(t, topic2.Tags, "kafka")
	assert.Contains(t, topic2.Tags, "test")
	assert.Contains(t, topic2.Tags, "env-staging")
	assert.Contains(t, topic2.Tags, "team-orders")

	metadata2 := topic2.Metadata.(map[string]interface{})
	partitionCount2, ok := metadata2["partition_count"]
	require.True(t, ok, "partition_count not found in metadata")
	assert.Equal(t, "5", partitionCount2.(json.Number).String())

	replicationFactor2, ok := metadata2["replication_factor"]
	require.True(t, ok, "replication_factor not found in metadata")
	assert.Equal(t, "1", replicationFactor2.(json.Number).String())

	retentionMs2, ok := metadata2["retention_ms"]
	require.True(t, ok, "retention_ms not found in metadata")
	assert.Equal(t, "604800000", retentionMs2)

	cleanupPolicy2, ok := metadata2["cleanup_policy"]
	require.True(t, ok, "cleanup_policy not found in metadata")
	assert.Equal(t, "compact", cleanupPolicy2)

	maxMessageBytes, ok := metadata2["max_message_bytes"]
	require.True(t, ok, "max_message_bytes not found in metadata")
	assert.Equal(t, "2097152", maxMessageBytes)

	testAuthConfig(t)

	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(topic1.ID))
	assert.NoError(t, err, "failed to delete asset", topic1.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(topic2.ID))
	assert.NoError(t, err, "failed to delete asset", topic2.ID)
}

// testAuthConfig tests different authentication configurations
// TODO: test actual auth configs use SASL/certs etc.
func testAuthConfig(t *testing.T) {
	ctx := context.Background()

	authTopicName := "test-topic-auth"
	_, err := env.KafkaAdminClient.CreateTopic(ctx, 1, 1, nil, authTopicName)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	// Test with plaintext auth config
	configContent := fmt.Sprintf(`
runs:
  - kafka:
      bootstrap_servers: "redpanda-test:%s"
      client_id: "marmot-kafka-auth-test"
      client_timeout_seconds: 30
      include_partition_info: true
      include_topic_config: true
      authentication:
        type: "none"
      tags:
        - kafka
        - auth-test
      topic_filter:
        include:
          - "^test-topic-auth$"
`, env.RedpandaPort)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	authTopic := utils.FindAssetByName(resp.Payload.Assets, "test-topic-auth")
	require.NotNil(t, authTopic, "asset test-topic-auth not found")

	assert.Equal(t, "Topic", authTopic.Type)
	assert.Contains(t, authTopic.Providers, "Kafka")
	assert.Contains(t, authTopic.Tags, "kafka")
	assert.Contains(t, authTopic.Tags, "auth-test")

	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(authTopic.ID))
	assert.NoError(t, err, "failed to delete asset", authTopic.ID)
}

// TestKafkaSchemaRegistry tests integration with Schema Registry
func TestKafkaSchemaRegistry(t *testing.T) {
	ctx := context.Background()

	err := env.EnsureRedpandaStarted(ctx, true)
	require.NoError(t, err)

	schemaTopicName := "test-topic-schema"
	_, err = env.KafkaAdminClient.CreateTopic(ctx, 1, 1, nil, schemaTopicName)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	schemaRegistryURL := fmt.Sprintf("http://localhost:%s", env.SchemaRegistryPort)
	schemaClient := NewSchemaRegistryClient(schemaRegistryURL)

	valueSchema := Schema{
		Schema: `{
			"type": "record",
			"name": "Payment",
			"namespace": "com.example",
			"fields": [
				{"name": "id", "type": "string"},
				{"name": "amount", "type": "double"},
				{"name": "currency", "type": "string"},
				{"name": "timestamp", "type": "long", "logicalType": "timestamp-millis"},
				{"name": "status", "type": {"type": "enum", "name": "PaymentStatus", "symbols": ["PENDING", "COMPLETED", "FAILED"]}}
			]
		}`,
		SchemaType: "AVRO",
	}
	valueSubject := fmt.Sprintf("%s-value", schemaTopicName)
	valueSchemaResp, err := schemaClient.RegisterSchema(valueSubject, valueSchema)
	require.NoError(t, err, "Failed to register value schema")
	t.Logf("Registered value schema with ID: %d", valueSchemaResp.ID)

	// Register key schema
	keySchema := Schema{
		Schema: `{
			"type": "record",
			"name": "PaymentKey",
			"namespace": "com.example",
			"fields": [
				{"name": "id", "type": "string"}
			]
		}`,
		SchemaType: "AVRO",
	}
	keySubject := fmt.Sprintf("%s-key", schemaTopicName)
	keySchemaResp, err := schemaClient.RegisterSchema(keySubject, keySchema)
	require.NoError(t, err, "Failed to register key schema")
	t.Logf("Registered key schema with ID: %d", keySchemaResp.ID)

	configContent := fmt.Sprintf(`
runs:
  - kafka:
      bootstrap_servers: "redpanda-test:%s"
      client_id: "marmot-kafka-schema-test"
      client_timeout_seconds: 30
      include_partition_info: true
      include_topic_config: true
      schema_registry:
        url: "http://redpanda-test:%s"
        enabled: true
      tags:
        - kafka
        - schema-test
      topic_filter:
        include:
          - "^test-topic-schema$"
`, env.RedpandaPort, env.SchemaRegistryPort)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	schemaTopic := utils.FindAssetByName(resp.Payload.Assets, schemaTopicName)
	require.NotNil(t, schemaTopic, "asset test-topic-schema not found")

	assert.Equal(t, "Topic", schemaTopic.Type)
	assert.Contains(t, schemaTopic.Providers, "Kafka")
	assert.Contains(t, schemaTopic.Tags, "kafka")
	assert.Contains(t, schemaTopic.Tags, "schema-test")

	metadata := schemaTopic.Metadata.(map[string]interface{})

	valueSchemaID, ok := metadata["value_schema_id"]
	require.True(t, ok, "value_schema_id not found in metadata")
	assert.Equal(t, fmt.Sprintf("%d", valueSchemaResp.ID), valueSchemaID.(json.Number).String(), "Value schema ID doesn't match")

	valueSchemaVersion, ok := metadata["value_schema_version"]
	require.True(t, ok, "value_schema_version not found in metadata")
	assert.Equal(t, fmt.Sprintf("%d", valueSchemaResp.Version), valueSchemaVersion.(json.Number).String(), "Value schema version doesn't match")

	valueSchemaType, ok := metadata["value_schema_type"]
	require.True(t, ok, "value_schema_type not found in metadata")
	assert.Equal(t, valueSchemaResp.SchemaType, valueSchemaType, "Value schema type doesn't match")

	valueSchemaContent, ok := metadata["value_schema"]
	require.True(t, ok, "value_schema not found in metadata")

	// Normalize both schemas for comparison
	var parsedValueSchema, parsedReturnedValueSchema interface{}
	err = json.Unmarshal([]byte(valueSchemaResp.Schema), &parsedValueSchema)
	require.NoError(t, err, "Error parsing value schema JSON")
	err = json.Unmarshal([]byte(valueSchemaContent.(string)), &parsedReturnedValueSchema)
	require.NoError(t, err, "Error parsing returned value schema JSON")
	assert.Equal(t, parsedValueSchema, parsedReturnedValueSchema, "Value schema content doesn't match")

	keySchemaID, ok := metadata["key_schema_id"]
	require.True(t, ok, "key_schema_id not found in metadata")
	assert.Equal(t, fmt.Sprintf("%d", keySchemaResp.ID), keySchemaID.(json.Number).String(), "Key schema ID doesn't match")

	keySchemaVersion, ok := metadata["key_schema_version"]
	require.True(t, ok, "key_schema_version not found in metadata")
	assert.Equal(t, fmt.Sprintf("%d", keySchemaResp.Version), keySchemaVersion.(json.Number).String(), "Key schema version doesn't match")

	keySchemaType, ok := metadata["key_schema_type"]
	require.True(t, ok, "key_schema_type not found in metadata")
	assert.Equal(t, keySchemaResp.SchemaType, keySchemaType, "Key schema type doesn't match")

	keySchemaContent, ok := metadata["key_schema"]
	require.True(t, ok, "key_schema not found in metadata")

	// Normalize both schemas for comparison
	var parsedKeySchema, parsedReturnedKeySchema interface{}
	err = json.Unmarshal([]byte(keySchemaResp.Schema), &parsedKeySchema)
	require.NoError(t, err, "Error parsing key schema JSON")
	err = json.Unmarshal([]byte(keySchemaContent.(string)), &parsedReturnedKeySchema)
	require.NoError(t, err, "Error parsing returned key schema JSON")
	assert.Equal(t, parsedKeySchema, parsedReturnedKeySchema, "Key schema content doesn't match")

	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(schemaTopic.ID))
	assert.NoError(t, err, "failed to delete asset", schemaTopic.ID)
}

// SchemaRegistryClient provides methods for interacting with Schema Registry
type SchemaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSchemaRegistryClient creates a new Schema Registry client
func NewSchemaRegistryClient(baseURL string) *SchemaRegistryClient {
	return &SchemaRegistryClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Schema represents a schema in Schema Registry
type Schema struct {
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType,omitempty"`
}

// SchemaResponse represents a response from the Schema Registry API
type SchemaResponse struct {
	ID         int    `json:"id"`
	Version    int    `json:"version"`
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType"`
}

// RegisterSchema registers a schema with Schema Registry
func (c *SchemaRegistryClient) RegisterSchema(subject string, schema Schema) (*SchemaResponse, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)

	body, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshaling schema: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return c.GetSchemaBySubject(subject)
}

// GetSchemaBySubject gets the latest schema for a subject
func (c *SchemaRegistryClient) GetSchemaBySubject(subject string) (*SchemaResponse, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions/latest", c.baseURL, subject)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var schemaResp SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &schemaResp, nil
}
