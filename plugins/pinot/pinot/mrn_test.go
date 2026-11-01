package pinot

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pinot has no schema layer, so a table's MRN is built from its bare
// logical name. mrn.New lowercases the whole string; the asset's Name keeps
// the case Pinot uses.

func TestTableMRN_IsTheBareLogicalTableName(t *testing.T) {
	assert.Equal(t, "mrn://table/pinot/baseballstats", assetMRN("Table", "baseballStats"))
}

func TestTableMRN_DropsTheTableTypeSuffix(t *testing.T) {
	// Identity is the logical table, never baseballStats_OFFLINE, so a
	// hybrid table with both sides is still one asset.
	assert.NotContains(t, assetMRN("Table", "baseballStats"), "offline")
	assert.NotContains(t, assetMRN("Table", "baseballStats"), "realtime")
}

func TestTopicMRN_MatchesWhatTheKafkaPluginProduces(t *testing.T) {
	// The FEEDS edge points at the topic the Kafka plugin owns, so the
	// identity has to be exactly the one plugins/kafka builds.
	assert.Equal(t, "mrn://topic/kafka/orders-events", mrn.New("Topic", "Kafka", "orders-events"))
}

func TestStreamMRN_MatchesWhatTheKinesisPluginProduces(t *testing.T) {
	assert.Equal(t, "mrn://stream/kinesis/orders-stream", mrn.New("Stream", "Kinesis", "orders-stream"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Table", "baseballStats")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name); the
	// MRN the plugin sets must be exactly that or the asset is filed twice.
	result := discover(t, newFakeController().withBaseballStats(), nil)

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)
	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "baseballStats", *a.Name, "the name people read keeps Pinot's casing")
}

func TestTopicAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	topic := findAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	require.NotNil(t, topic.MRN)
	assert.Equal(t, mrn.New(topic.Type, topic.Providers[0], *topic.Name), *topic.MRN)
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

func TestTableMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// plugins/openmetadata projects a PinotDB table to provider "Pinot"
	// with its bare name and no container, and plugins/trino does the same
	// for the pinot connector. This plugin has to land on the same MRN or
	// the day it takes over, that asset is stranded and a second appears.
	assert.Equal(t, "mrn://table/pinot/baseballstats", mrn.New("Table", "Pinot", "baseballStats"))
	assert.Equal(t, assetMRN("Table", "baseballStats"), mrn.New("Table", "Pinot", "baseballStats"))
}
