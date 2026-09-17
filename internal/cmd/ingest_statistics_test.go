package cmd

import (
	"encoding/json"
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Row counts and sizes are measured by the plugin, against the source, during
// discovery. Nothing else can measure them, so whatever the CLI leaves out of
// the batch is lost.

func TestStatisticRequests_CarriesEveryMetric(t *testing.T) {
	requests := statisticRequests([]plugin.Statistic{
		{AssetMRN: "mrn://table/sqlite/orders", MetricName: "asset.row_count", Value: 3},
		{AssetMRN: "mrn://table/sqlite/orders", MetricName: "asset.column_count", Value: 5},
	})

	require.Len(t, requests, 2)
	assert.Equal(t, "mrn://table/sqlite/orders", requests[0].AssetMRN)
	assert.Equal(t, "asset.row_count", requests[0].MetricName)
	assert.Equal(t, float64(3), requests[0].Value)
	assert.Equal(t, "asset.column_count", requests[1].MetricName)
}

func TestStatisticRequests_KeepsAZeroValue(t *testing.T) {
	// An empty table has a row count of zero. That is a measurement, not a
	// missing one, so it has to reach the server like any other.
	requests := statisticRequests([]plugin.Statistic{
		{AssetMRN: "mrn://table/sqlite/empty", MetricName: "asset.row_count", Value: 0},
	})

	require.Len(t, requests, 1)
	assert.Equal(t, float64(0), requests[0].Value)
}

func TestStatisticRequests_MeasuredNothingIsAnEmptyList(t *testing.T) {
	// The field has no omitempty, so a nil slice would put a null on the
	// wire where the server has always seen a list.
	assert.Equal(t, []CreateStatisticRequest{}, statisticRequests(nil))
}

func TestBatchCreateRequest_SendsTheMetricsThePluginMeasured(t *testing.T) {
	encoded, err := json.Marshal(BatchCreateRequest{
		PipelineName: "demo",
		Statistics: statisticRequests([]plugin.Statistic{
			{AssetMRN: "mrn://table/sqlite/orders", MetricName: "asset.row_count", Value: 3},
		}),
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))

	statistics, ok := out["statistics"].([]interface{})
	require.True(t, ok, "the batch has to carry the metrics the plugin measured")
	require.Len(t, statistics, 1)

	statistic := statistics[0].(map[string]interface{})
	assert.Equal(t, "mrn://table/sqlite/orders", statistic["asset_mrn"])
	assert.Equal(t, "asset.row_count", statistic["metric_name"])
	assert.Equal(t, float64(3), statistic["value"])
}
