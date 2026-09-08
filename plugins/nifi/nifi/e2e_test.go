package nifi_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real NiFi. They expect the
// flow the Docker recipe in the plugin spec seeds: an Ingest group with
// Generate Orders (running), Stamp Attributes, Land in S3 (bucket
// marmot-landing) and Publish Orders (topic orders-events) feeding a
// Deliver group through ports, where Log Orders, Consume Orders
// (orders-events, orders-dlq), Read Customers (PostgreSQL pool) and
// Notify Webhook (sensitive Request Password) live.

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_NIFI_URL")
	if host == "" {
		t.Skip("MARMOT_TEST_NIFI_URL not set, skipping NiFi e2e tests")
	}

	return pluginsdk.RawConfig{
		"host":       host,
		"username":   os.Getenv("MARMOT_TEST_NIFI_USERNAME"),
		"password":   os.Getenv("MARMOT_TEST_NIFI_PASSWORD"),
		"verify_ssl": false,
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func pipelineMRN(name string) string { return mrn.New("Pipeline", "NiFi", name) }
func taskMRN(name string) string     { return mrn.New("Task", "NiFi", name) }

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "nifi", meta.ID)
	assert.Equal(t, "NiFi", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"username": "marmot", "password": "secret"})
	require.Error(t, err)
}

func TestE2E_WrongPasswordFails(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	config["password"] = "not-the-password"
	_, err := bin.Discover(t.Context(), config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in")
}

func TestE2E_DiscoverOverTheWire(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Pipelines: the root group and the two seeded children.
	root := findAsset(result, "Pipeline", "NiFi Flow")
	require.NotNil(t, root)
	assert.Equal(t, pipelineMRN("NiFi Flow"), *root.MRN)
	assert.Equal(t, []string{"NiFi"}, root.Providers)
	assert.NotEmpty(t, root.Metadata["nifi_version"])

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	require.NotNil(t, ingest)
	assert.Equal(t, "mrn://pipeline/nifi/nifi-flow-ingest", *ingest.MRN)
	require.NotNil(t, ingest.Description)
	assert.Equal(t, "Lands raw orders in S3 and Kafka", *ingest.Description)
	// Counts arrive as float64 after the JSON hop over the wire.
	assert.EqualValues(t, 4, ingest.Metadata["processor_count"])
	assert.EqualValues(t, 1, ingest.Metadata["output_port_count"])
	assert.EqualValues(t, 1, ingest.Metadata["running_count"])
	require.Len(t, ingest.ExternalLinks, 1)
	assert.Equal(t, "Open in NiFi", ingest.ExternalLinks[0].Name)

	deliver := findAsset(result, "Pipeline", "NiFi Flow/Deliver")
	require.NotNil(t, deliver)

	// Tasks with their types and states.
	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	require.NotNil(t, gen)
	assert.Equal(t, "mrn://task/nifi/nifi-flow-ingest-generate-orders", *gen.MRN)
	assert.Equal(t, "GenerateFlowFile", gen.Metadata["type"])
	assert.Equal(t, "org.apache.nifi.processors.standard.GenerateFlowFile", gen.Metadata["type_full"])
	assert.Equal(t, "RUNNING", gen.Metadata["state"])
	assert.Equal(t, "1 hour", gen.Metadata["scheduling_period"])

	s3 := findAsset(result, "Task", "NiFi Flow/Ingest/Land in S3")
	require.NotNil(t, s3)
	assert.Equal(t, "PutS3Object", s3.Metadata["type"])
	assert.Equal(t, "INVALID", s3.Metadata["state"], "no credentials configured, so NiFi reports it invalid")

	stamp := findAsset(result, "Task", "NiFi Flow/Ingest/Stamp Attributes")
	require.NotNil(t, stamp)
	assert.Equal(t, "STOPPED", stamp.Metadata["state"])

	hook := findAsset(result, "Task", "NiFi Flow/Deliver/Notify Webhook")
	require.NotNil(t, hook)
	properties, ok := hook.Metadata["properties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "****", properties["Request Password"])
	assert.Equal(t, "https://hooks.example.com/orders", properties["HTTP URL"])
	require.NotNil(t, hook.Description)
	assert.Equal(t, "Posts each order to the fulfilment webhook", *hook.Description)

	// Structure edges.
	assert.True(t, hasEdge(result, *root.MRN, *ingest.MRN, "CONTAINS"))
	assert.True(t, hasEdge(result, *root.MRN, *deliver.MRN, "CONTAINS"))
	assert.True(t, hasEdge(result, *ingest.MRN, *gen.MRN, "CONTAINS"))
	assert.True(t, hasEdge(result, *gen.MRN, *stamp.MRN, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, *stamp.MRN, *s3.MRN, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, *ingest.MRN, *deliver.MRN, "DEPENDS_ON"), "the output port to input port connection links the groups")
	assert.True(t, hasEdge(result, taskMRN("NiFi Flow/Deliver/Log Orders"), *hook.MRN, "DEPENDS_ON"))

	// Ports are off by default.
	assert.Nil(t, findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver"))

	// Data lineage.
	topic := findAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	assert.Equal(t, []string{"Kafka"}, topic.Providers)
	assert.Equal(t, "mrn://topic/kafka/orders-events", *topic.MRN)
	require.Len(t, topic.Sources, 1)
	assert.Equal(t, "NiFi", topic.Sources[0].Name)
	assert.True(t, hasEdge(result, taskMRN("NiFi Flow/Ingest/Publish Orders"), *topic.MRN, "PRODUCES"))
	assert.True(t, hasEdge(result, *topic.MRN, taskMRN("NiFi Flow/Deliver/Consume Orders"), "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-dlq", taskMRN("NiFi Flow/Deliver/Consume Orders"), "FEEDS"))
	assert.True(t, hasEdge(result, *s3.MRN, "mrn://bucket/s3/marmot-landing", "PRODUCES"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", taskMRN("NiFi Flow/Deliver/Read Customers"), "FEEDS"))

	assert.Empty(t, result.Statistics)
	assert.Empty(t, result.RunHistory)

	for _, a := range result.Assets {
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %s", *a.Name)
	}
}

func TestE2E_DiscoverWithPorts(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	config["include_ports"] = true
	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	out := findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver")
	require.NotNil(t, out)
	assert.Equal(t, "OUTPUT_PORT", out.Metadata["port_type"])

	in := findAsset(result, "Task", "NiFi Flow/Deliver/from-ingest")
	require.NotNil(t, in)
	assert.Equal(t, "INPUT_PORT", in.Metadata["port_type"])

	assert.True(t, hasEdge(result, taskMRN("NiFi Flow/Ingest/Stamp Attributes"), *out.MRN, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, *out.MRN, *in.MRN, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, *in.MRN, taskMRN("NiFi Flow/Deliver/Log Orders"), "DEPENDS_ON"))
}

func TestE2E_DiscoverFromAChildGroup(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	full, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	ingest := findAsset(full, "Pipeline", "NiFi Flow/Ingest")
	require.NotNil(t, ingest)

	config["root_process_group"] = ingest.Metadata["id"]
	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "Pipeline", "NiFi Flow/Ingest"), "named by breadcrumb, so it matches the full run")
	assert.Nil(t, findAsset(result, "Pipeline", "NiFi Flow"))
	assert.Nil(t, findAsset(result, "Pipeline", "NiFi Flow/Deliver"))
	assert.NotNil(t, findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders"))
}
