package spline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Spline server. Start one with:
//
//	docker network create marmot-test-spline
//	docker run -d --name marmot-test-spline-db --network marmot-test-spline \
//	  -p 18529:8529 -e ARANGO_NO_AUTH=1 arangodb:3.11
//	docker run --rm --network marmot-test-spline absaoss/spline-admin:latest \
//	  db-init arangodb://marmot-test-spline-db:8529/spline -f --non-interactive
//	docker run -d --name marmot-test-spline --network marmot-test-spline -p 18080:8080 \
//	  -e spline.database.connectionUrl=arangodb://marmot-test-spline-db:8529/spline \
//	  absaoss/spline-rest-server:latest
//
// ArangoDB 3.12 does not work: it dropped the VelocyStream protocol the Spline
// admin tool connects with.
//
// Then run: MARMOT_TEST_SPLINE_URL=http://localhost:18080 go test ./spline/ -run E2E
//
// The seed uses timestamps anchored to the start of the current UTC day, and
// Spline derives an execution event's id from its plan and timestamp, so
// re-running the suite the same day re-seeds the same runs instead of adding
// more. The run counts asserted below assume a freshly initialised database.

// producerContentType is the media type the Spline producer API requires.
const producerContentType = "application/vnd.absa.spline.producer.v1.2+json"

const (
	e2ePlanDaily  = "aaaaaaaa-1111-1111-1111-111111111111"
	e2ePlanEvents = "aaaaaaaa-2222-2222-2222-222222222222"
	e2ePlanNight  = "aaaaaaaa-3333-3333-3333-333333333333"
)

func splineURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("MARMOT_TEST_SPLINE_URL")
	if url == "" {
		t.Skip("MARMOT_TEST_SPLINE_URL is not set, skipping Spline e2e tests")
	}
	return url
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func postProducer(t *testing.T, baseURL, path string, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/producer"+path, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", producerContentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	response, _ := io.ReadAll(resp.Body)
	require.Less(t, resp.StatusCode, 300, "producer rejected %s: %s", path, string(response))
}

// seedSpline pushes three execution plans and four runs through the producer
// API. Running real Spark would need a cluster and the Spline agent, and the
// producer API is the interface the agent itself uses.
func seedSpline(t *testing.T, baseURL string) {
	t.Helper()

	plan := func(id, name, output string, inputs []string, appendMode bool) map[string]any {
		reads := make([]map[string]any, 0, len(inputs))
		for i, input := range inputs {
			reads = append(reads, map[string]any{
				"id":           fmt.Sprintf("op-read-%d", i),
				"name":         "LogicalRelation",
				"inputSources": []string{input},
				"output":       []string{"attr-order-id", "attr-total"},
			})
		}
		return map[string]any{
			"id":   id,
			"name": name,
			"operations": map[string]any{
				"write": map[string]any{
					"id": "op-write", "name": "SaveIntoDataSourceCommand",
					"childIds": []string{"op-join"}, "outputSource": output, "append": appendMode,
				},
				"reads": reads,
				"other": []map[string]any{{
					"id": "op-join", "name": "Join", "childIds": []string{"op-read-0"},
					"output": []string{"attr-order-id", "attr-total", "attr-revenue"},
				}},
			},
			"attributes": []map[string]any{
				{"id": "attr-order-id", "name": "order_id", "dataType": "dt-long"},
				{"id": "attr-total", "name": "total", "dataType": "dt-double"},
				{"id": "attr-revenue", "name": "revenue", "dataType": "dt-double",
					"childRefs": []map[string]any{{"__attrId": "attr-total"}, {"__attrId": "attr-order-id"}}},
			},
			"systemInfo": map[string]any{"name": "spark", "version": "3.5.0"},
			"agentInfo":  map[string]any{"name": "spline", "version": "1.0.0"},
			"extraInfo":  map[string]any{"appName": name},
		}
	}

	postProducer(t, baseURL, "/execution-plans", plan(e2ePlanDaily, "daily-etl",
		"jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary",
		[]string{
			"jdbc:postgresql://pg.internal:5432/warehouse:public.orders",
			"jdbc:postgresql://pg.internal:5432/warehouse:public.customers",
		}, false))

	postProducer(t, baseURL, "/execution-plans", plan(e2ePlanEvents, "events-loader",
		"hive://sales/orders", []string{"s3://marmot-lake/raw/events"}, true))

	postProducer(t, baseURL, "/execution-plans", plan(e2ePlanNight, "nightly-report",
		"s3a://marmot-lake/reports/nightly",
		[]string{"jdbc:postgresql://pg.internal:5432/warehouse:public.orders"}, false))

	// Deterministic timestamps keep re-running the suite idempotent.
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(6 * time.Hour).UnixMilli()
	postProducer(t, baseURL, "/execution-events", []map[string]any{
		// Two runs of the same application, so grouping and run history are
		// both exercised.
		{"planId": e2ePlanDaily, "timestamp": day - 3600000, "durationNs": 12000000000,
			"extra": map[string]any{"appName": "daily-etl", "appId": "application_1699000000000_0001"}},
		{"planId": e2ePlanDaily, "timestamp": day - 600000, "durationNs": 9500000000,
			"extra": map[string]any{"appName": "daily-etl", "appId": "application_1699000000000_0002"}},
		{"planId": e2ePlanEvents, "timestamp": day - 1800000, "durationNs": 4200000000,
			"extra": map[string]any{"appName": "events-loader", "appId": "application_1699000000000_0003"}},
		{"planId": e2ePlanNight, "timestamp": day - 300000,
			"error": map[string]any{"message": "Job aborted due to stage failure"},
			"extra": map[string]any{"appName": "nightly-report", "appId": "application_1699000000000_0004"}},
	})
}

// seedOnce keeps the seed to a single pass per test process; every test below
// discovers over the same data.
var seedOnce sync.Once

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	baseURL := splineURL(t)
	seedOnce.Do(func() { seedSpline(t, baseURL) })

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"host":    baseURL,
		"ui_host": baseURL,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func e2eAssetsByName(result *pluginsdk.DiscoveryResult) map[string]pluginsdk.Asset {
	byName := make(map[string]pluginsdk.Asset, len(result.Assets))
	for _, asset := range result.Assets {
		byName[*asset.Name] = asset
	}
	return byName
}

func e2eHasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "spline", meta.ID)
	assert.Equal(t, "Spline", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAConsumerURL(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": "http://spline:8080/consumer"})
	require.NoError(t, err)
}

func TestE2E_DiscoverGroupsRunsByApplication(t *testing.T) {
	result := discoverE2E(t)

	byName := e2eAssetsByName(result)
	assert.Contains(t, byName, "daily-etl")
	assert.Contains(t, byName, "events-loader")
	assert.Contains(t, byName, "nightly-report")

	for _, asset := range result.Assets {
		assert.Equal(t, "Pipeline", asset.Type)
		assert.Equal(t, []string{"Spline"}, asset.Providers)
	}
}

func TestE2E_DiscoverCountsTheTwoRunsOfOneApplication(t *testing.T) {
	result := discoverE2E(t)

	// The count arrives over the wire as a JSON number.
	assert.EqualValues(t, 2, e2eAssetsByName(result)["daily-etl"].Metadata["execution_count"])
}

func TestE2E_DiscoverReadsTheFrameworkAndAgent(t *testing.T) {
	result := discoverE2E(t)

	metadata := e2eAssetsByName(result)["daily-etl"].Metadata
	assert.Equal(t, "spark 3.5.0", metadata["framework"])
	assert.Equal(t, "spark", metadata["system_name"])
	assert.Equal(t, "spline", metadata["agent_name"])
	assert.EqualValues(t, 9500, metadata["last_duration_ms"])
}

func TestE2E_DiscoverRecordsTheFailure(t *testing.T) {
	result := discoverE2E(t)

	assert.Equal(t, "Job aborted due to stage failure",
		e2eAssetsByName(result)["nightly-report"].Metadata["last_error"])
}

func TestE2E_DiscoverLinksPostgresTablesToThePipeline(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, e2eHasEdge(result, "mrn://table/postgresql/orders", "mrn://pipeline/spline/daily-etl", "FEEDS"))
	assert.True(t, e2eHasEdge(result, "mrn://table/postgresql/customers", "mrn://pipeline/spline/daily-etl", "FEEDS"))
	assert.True(t, e2eHasEdge(result, "mrn://pipeline/spline/daily-etl", "mrn://table/postgresql/order_summary", "PRODUCES"))
}

func TestE2E_DiscoverLinksAnS3BucketAndAHiveTable(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, e2eHasEdge(result, "mrn://bucket/s3/marmot-lake", "mrn://pipeline/spline/events-loader", "FEEDS"))
	assert.True(t, e2eHasEdge(result, "mrn://pipeline/spline/events-loader", "mrn://table/hive/sales.orders", "PRODUCES"))
	assert.True(t, e2eHasEdge(result, "mrn://pipeline/spline/nightly-report", "mrn://bucket/s3/marmot-lake", "PRODUCES"))
}

func TestE2E_DiscoverReturnsRunHistoryIncludingTheFailure(t *testing.T) {
	result := discoverE2E(t)

	byMRN := map[string]pluginsdk.AssetRunHistory{}
	for _, history := range result.RunHistory {
		byMRN[history.AssetMRN] = history
	}

	daily := byMRN["mrn://pipeline/spline/daily-etl"]
	require.Len(t, daily.Runs, 2)
	assert.Equal(t, "spline", daily.Runs[0].JobNamespace)
	assert.Equal(t, "daily-etl", daily.Runs[0].JobName)
	assert.Equal(t, "COMPLETE", daily.Runs[0].EventType)
	assert.EqualValues(t, 9500, daily.Runs[0].RunFacets["duration_ms"])

	night := byMRN["mrn://pipeline/spline/nightly-report"]
	require.Len(t, night.Runs, 1)
	assert.Equal(t, "FAIL", night.Runs[0].EventType)
}

func TestE2E_DiscoverReturnsColumnLineageOnTheOutput(t *testing.T) {
	result := discoverE2E(t)

	encoded, ok := e2eAssetsByName(result)["daily-etl"].Metadata["column_lineage"].(string)
	require.True(t, ok, "expected column_lineage on the pipeline")

	var lineage map[string][]struct {
		ToColumn    string   `json:"to_column"`
		FromColumns []string `json:"from_columns"`
	}
	require.NoError(t, json.Unmarshal([]byte(encoded), &lineage))

	entries := lineage["mrn://table/postgresql/order_summary"]
	require.NotEmpty(t, entries)
	assert.Equal(t, "revenue", entries[0].ToColumn)
	assert.Equal(t, []string{"order_id", "total"}, entries[0].FromColumns)
}

func TestE2E_DiscoverAddsTheSplineUILink(t *testing.T) {
	result := discoverE2E(t)

	asset := e2eAssetsByName(result)["daily-etl"]
	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "Open in Spline", asset.ExternalLinks[0].Name)
	assert.Contains(t, asset.ExternalLinks[0].URL, "/app/events/overview/")
}
