package pinot_test

import (
	"encoding/json"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Pinot cluster. They expect
// the batch QuickStart (apachepinot/pinot QuickStart -type batch), which
// ships the baseballStats and dimBaseballTeams tables, and optionally a
// REALTIME table named orders_events consuming the Kafka topic
// orders-events. Set MARMOT_TEST_PINOT_CONTROLLER_URL to run them, for
// example http://localhost:19000.

const controllerURLEnv = "MARMOT_TEST_PINOT_CONTROLLER_URL"

func controllerURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv(controllerURLEnv)
	if url == "" {
		t.Skipf("%s is not set; skipping e2e tests against a real Pinot cluster", controllerURLEnv)
	}
	return url
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func findE2EAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func e2eStats(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func e2eColumns(t *testing.T, a *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column list on %s", *a.Name)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	controllerURL(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "pinot", meta.ID)
	assert.Equal(t, "Pinot", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Serve marks the plugin as a DataFetcher")
}

func TestE2E_ValidateMissingControllerURLFails(t *testing.T) {
	controllerURL(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_DiscoverOverTheWire(t *testing.T) {
	url := controllerURL(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"controller_url": url})
	require.NoError(t, err)
	require.NotNil(t, result)

	stats := findE2EAsset(result, "Table", "baseballStats")
	require.NotNil(t, stats, "the batch QuickStart ships baseballStats")
	assert.Equal(t, []string{"Pinot"}, stats.Providers)
	assert.Equal(t, "mrn://table/pinot/baseballstats", *stats.MRN)
	assert.Equal(t, "batch", stats.Metadata["ingestion_type"])
	assert.Equal(t, "DefaultTenant", stats.Metadata["broker_tenant"])
	assert.Equal(t, url+"/#/tenants/table/baseballStats_OFFLINE", stats.Metadata["url"])
	assert.NotEmpty(t, stats.Metadata["pinot_version"])

	tableTypes, ok := stats.Metadata["table_types"].([]interface{})
	require.True(t, ok, "table_types survives the wire as a list")
	assert.Equal(t, []interface{}{"OFFLINE"}, tableTypes)

	segmentCount, ok := stats.Metadata["segment_count"].(float64)
	require.True(t, ok, "segment_count survives the wire as a number")
	assert.GreaterOrEqual(t, segmentCount, float64(1))

	columns := e2eColumns(t, stats)
	assert.Equal(t, "dimension", columns["playerID"]["field_type"])
	assert.Equal(t, "STRING", columns["playerID"]["data_type"])
	assert.Equal(t, "metric", columns["numberOfGames"]["field_type"])
	assert.Equal(t, "INT", columns["numberOfGames"]["data_type"])
	assert.Equal(t, true, columns["playerID"]["is_nullable"])

	metrics := e2eStats(result, *stats.MRN)
	assert.Greater(t, metrics["asset.row_count"], float64(0))
	assert.Greater(t, metrics["asset.size_bytes"], float64(0))
	assert.Greater(t, metrics["asset.column_count"], float64(0))
}

func TestE2E_DiscoverMarksDimensionTablesAndPrimaryKeys(t *testing.T) {
	url := controllerURL(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"controller_url": url})
	require.NoError(t, err)

	teams := findE2EAsset(result, "Table", "dimBaseballTeams")
	require.NotNil(t, teams, "the batch QuickStart ships dimBaseballTeams")
	assert.Equal(t, true, teams.Metadata["is_dim_table"])
	assert.Equal(t, []interface{}{"teamID"}, teams.Metadata["primary_key_columns"])

	columns := e2eColumns(t, teams)
	assert.Equal(t, true, columns["teamID"]["is_primary_key"])
}

func TestE2E_DiscoverLinksTheRealtimeTableToItsTopic(t *testing.T) {
	url := controllerURL(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"controller_url": url})
	require.NoError(t, err)

	orders := findE2EAsset(result, "Table", "orders_events")
	if orders == nil {
		t.Skip("no orders_events REALTIME table in this cluster; create one with a Kafka stream config to run this test")
	}

	assert.Equal(t, "stream", orders.Metadata["ingestion_type"])
	assert.Equal(t, "kafka", orders.Metadata["stream_type"])
	assert.Equal(t, "orders-events", orders.Metadata["stream_topic"])
	assert.Equal(t, "30 DAYS", orders.Metadata["retention"])

	columns := e2eColumns(t, orders)
	assert.Equal(t, "INT[]", columns["item_ids"]["data_type"])
	assert.Equal(t, "datetime", columns["event_ts"]["field_type"])
	assert.Equal(t, true, columns["order_id"]["is_primary_key"])

	topic := findE2EAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	assert.Equal(t, []string{"Kafka"}, topic.Providers)
	assert.Equal(t, "mrn://topic/kafka/orders-events", *topic.MRN)

	var found bool
	for _, e := range result.Lineage {
		if e.Type == "FEEDS" && e.Source == "mrn://topic/kafka/orders-events" && e.Target == "mrn://table/pinot/orders_events" {
			found = true
		}
	}
	assert.True(t, found, "expected orders-events FEEDS orders_events edge")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	url := controllerURL(t)
	bin := buildBinary(t)

	name := "dimBaseballTeams"
	columns, rows, err := bin.FetchSampleData(t.Context(),
		pluginsdk.RawConfig{"controller_url": url},
		&pluginsdk.Asset{Name: &name, Metadata: map[string]interface{}{"table_name": name}})
	require.NoError(t, err)

	assert.Equal(t, []string{"teamID", "teamName"}, columns)
	require.NotEmpty(t, rows)
	assert.Len(t, rows[0], 2)
}
