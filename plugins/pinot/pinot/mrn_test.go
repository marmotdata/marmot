package pinot

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pinot has no schema layer, so a table's MRN is built from its bare logical name. The path is lowercased in the MRN; the asset's Name keeps the case Pinot uses.

// assetMRN builds the default-zone MRN of a Pinot asset.
func assetMRN(assetType, name string) string {
	return mrn.Asset(assetType, provider, name).String()
}

func TestTableMRN_IsTheBareLogicalTableName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/pinot/baseballstats", assetMRN("Table", "baseballStats"))
}

func TestTableMRN_DropsTheTableTypeSuffix(t *testing.T) {
	// Identity is the logical table, never baseballStats_OFFLINE, so a hybrid table with both sides is still one asset.
	assert.NotContains(t, assetMRN("Table", "baseballStats"), "offline")
	assert.NotContains(t, assetMRN("Table", "baseballStats"), "realtime")
}

func TestTableMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod-eu"

	assert.Equal(t, "mrn://asset/table/pinot@prod-eu/baseballstats", cfg.AssetMRN("Table", provider, "baseballStats"))
}

func TestTableMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := assetMRN("Table", "baseballStats")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestTopicMRN_HintsAtTheKafkaHost(t *testing.T) {
	// The topic belongs to the Kafka plugin, so the broker list only hints at which of its zones this is.
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	topic := findAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	assert.Equal(t, "mrn://asset/topic/kafka@host=localhost:9092/orders-events", *topic.MRN)
}

func TestStreamMRN_KinesisHasNoHostHint(t *testing.T) {
	assert.Equal(t, "mrn://asset/stream/kinesis/orders-stream", mrn.Asset("Stream", "Kinesis", "orders-stream").String())
}

func TestFirstBroker(t *testing.T) {
	assert.Equal(t, "kafka-1:9092", firstBroker("Kafka-1:9092,kafka-2:9092"))
	assert.Equal(t, "kafka:9092", firstBroker("kafka"))
	assert.Equal(t, "kafka:9093", firstBroker("SSL://kafka:9093"))
	assert.Equal(t, "", firstBroker(""))
}

func TestZoneBindings(t *testing.T) {
	cfg := &Config{ControllerURL: "http://Controller.Internal:9000", BrokerURL: "https://broker.internal"}
	cfg.Zone = "prod"
	s := &Source{config: cfg}

	assert.Equal(t, []pluginsdk.ZoneBinding{
		{Provider: "Pinot", Host: "controller.internal:9000", Zone: "prod"},
		{Provider: "Pinot", Host: "broker.internal:443", Zone: "prod"},
	}, s.zoneBindings())
}

func TestTableAsset_MRNAndName(t *testing.T) {
	result := discover(t, newFakeController().withBaseballStats(), nil)

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)
	assert.Equal(t, assetMRN("Table", "baseballStats"), *a.MRN)
	assert.Equal(t, "baseballStats", *a.Name, "the name people read keeps Pinot's casing")
	assert.Empty(t, a.PreviousMRNs)
}

func TestLineageEdges_PointAtMRNsCreatedInTheSameRun(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents().withBaseballStats(), nil)

	created := make(map[string]bool)
	for _, a := range result.Assets {
		created[*a.MRN] = true
	}
	for _, edge := range result.Lineage {
		assert.True(t, created[edge.Source], "edge source %s was never created", edge.Source)
		assert.True(t, created[edge.Target], "edge target %s was never created", edge.Target)
	}
}
