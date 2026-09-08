package kafkaconnect_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Kafka Connect worker.
//
// They expect the worker named by MARMOT_TEST_KAFKACONNECT_URL to run two
// FileStream connectors: orders-file-source writing /tmp/orders.txt to
// the orders-events topic and orders-file-sink reading it back, both
// RUNNING with one task each.

const (
	sourceConnector = "orders-file-source"
	sinkConnector   = "orders-file-sink"
	topic           = "orders-events"
)

func connectURL(t *testing.T) string {
	t.Helper()
	host := os.Getenv("MARMOT_TEST_KAFKACONNECT_URL")
	if host == "" {
		t.Skip("MARMOT_TEST_KAFKACONNECT_URL not set; skipping Kafka Connect e2e tests")
	}
	return host
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	host := connectURL(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"host": host})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	connectURL(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "kafkaconnect", meta.ID)
	assert.Equal(t, "Kafka Connect", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	connectURL(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheWorkerURL(t *testing.T) {
	host := connectURL(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": host})
	require.NoError(t, err)
}

func TestE2E_DiscoversBothConnectorsAsPipelines(t *testing.T) {
	result := discoverE2E(t)

	source := findAsset(result, "Pipeline", sourceConnector)
	require.NotNil(t, source)
	assert.Equal(t, []string{"Kafka Connect"}, source.Providers)
	assert.Equal(t, "mrn://pipeline/kafka-connect/orders-file-source", *source.MRN)
	assert.Equal(t, "source", source.Metadata["connector_type"])
	assert.Equal(t, "RUNNING", source.Metadata["state"])
	assert.Equal(t, "org.apache.kafka.connect.file.FileStreamSourceConnector", source.Metadata["connector_class"])
	assert.NotEmpty(t, source.Metadata["plugin_version"])
	assert.NotEmpty(t, source.Metadata["connect_version"])
	assert.NotEmpty(t, source.Metadata["kafka_cluster_id"])

	sink := findAsset(result, "Pipeline", sinkConnector)
	require.NotNil(t, sink)
	assert.Equal(t, "sink", sink.Metadata["connector_type"])
	assert.Equal(t, "RUNNING", sink.Metadata["state"])
}

func TestE2E_DiscoversTasksUnderTheirPipelines(t *testing.T) {
	result := discoverE2E(t)

	for _, connector := range []string{sourceConnector, sinkConnector} {
		task := findAsset(result, "Task", connector+".task-0")
		require.NotNil(t, task, connector)
		assert.Equal(t, "RUNNING", task.Metadata["state"])
		assert.Equal(t, connector, task.Metadata["connector"])
		assert.True(t, hasEdge(result,
			"mrn://pipeline/kafka-connect/"+connector,
			"mrn://task/kafka-connect/"+connector+".task-0",
			"CONTAINS"), connector)
	}
}

func TestE2E_DiscoversTheSharedTopic(t *testing.T) {
	result := discoverE2E(t)

	asset := findAsset(result, "Topic", topic)
	require.NotNil(t, asset)
	assert.Equal(t, []string{"Kafka"}, asset.Providers)
	assert.Equal(t, "mrn://topic/kafka/orders-events", *asset.MRN)
	assert.Contains(t, asset.Metadata["producers"], sourceConnector)
	assert.Contains(t, asset.Metadata["consumers"], sinkConnector)
}

func TestE2E_LinksSourceToTopicAndTopicToSink(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, hasEdge(result,
		"mrn://pipeline/kafka-connect/orders-file-source",
		"mrn://topic/kafka/orders-events",
		"PRODUCES"))
	assert.True(t, hasEdge(result,
		"mrn://topic/kafka/orders-events",
		"mrn://pipeline/kafka-connect/orders-file-sink",
		"FEEDS"))
}

func TestE2E_StoresTheConnectorConfig(t *testing.T) {
	result := discoverE2E(t)

	source := findAsset(result, "Pipeline", sourceConnector)
	require.NotNil(t, source)

	config, ok := source.Metadata["config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/tmp/orders.txt", config["file"])
	assert.Equal(t, topic, config["topic"])
}
