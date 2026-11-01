package pinot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// The fixtures below are responses captured from apachepinot/pinot:latest
// (1.6.0-SNAPSHOT) running the batch QuickStart, trimmed to the fields
// that matter. Field names and value types are as the controller sent them.

const versionJSON = `{"pinot-distribution":"1.6.0-SNAPSHOT-1f17ab0ede12e2703e4a037837d4c641c4756d11","pinot-csv":"1.6.0-SNAPSHOT-1f17ab0ede12e2703e4a037837d4c641c4756d11"}`

const baseballStatsConfigJSON = `{"OFFLINE":{"tableName":"baseballStats_OFFLINE","tableType":"OFFLINE","segmentsConfig":{"replication":"1","minimizeDataMovement":false,"segmentPushType":"APPEND"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"rangeIndexVersion":2,"loadMode":"HEAP","nullHandlingEnabled":false,"invertedIndexColumns":["playerID","teamID"]},"metadata":{"customConfigs":{}},"ingestionConfig":{"continueOnError":false,"batchIngestionConfig":{"batchConfigMaps":[{"inputDirURI":"examples/minions/batch/baseballStats/rawdata","inputFormat":"csv","overwriteOutput":"true"}],"segmentIngestionType":"APPEND","segmentIngestionFrequency":"DAILY","consistentDataPush":false}},"isDimTable":false,"isMaterializedView":false}}`

const baseballStatsSchemaJSON = `{
  "schemaName" : "baseballStats",
  "enableColumnBasedNullHandling" : false,
  "dimensionFieldSpecs" : [ {
    "name" : "playerID",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  }, {
    "name" : "yearID",
    "dataType" : "INT",
    "fieldType" : "DIMENSION"
  }, {
    "name" : "teamID",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  } ],
  "metricFieldSpecs" : [ {
    "name" : "numberOfGames",
    "dataType" : "INT",
    "fieldType" : "METRIC"
  }, {
    "name" : "homeRuns",
    "dataType" : "INT",
    "fieldType" : "METRIC"
  } ]
}`

const baseballStatsSizeJSON = `{"tableName":"baseballStats","reportedSizeInBytes":3538585,"estimatedSizeInBytes":3538585,"reportedSizePerReplicaInBytes":3538585,"offlineSegments":{"reportedSizeInBytes":3538585,"estimatedSizeInBytes":3538585,"missingSegments":0,"reportedSizePerReplicaInBytes":3538585,"segments":{}},"realtimeSegments":null}`

const baseballStatsSegmentsJSON = `[{"OFFLINE":["baseballStats_OFFLINE_0_f82686a5-aa47-4b80-9612-0d4e08a5780b"]}]`

const dimBaseballTeamsConfigJSON = `{"OFFLINE":{"tableName":"dimBaseballTeams_OFFLINE","tableType":"OFFLINE","segmentsConfig":{"replication":"1","minimizeDataMovement":false,"segmentPushType":"REFRESH"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"rangeIndexVersion":2,"loadMode":"MMAP","nullHandlingEnabled":false},"metadata":{"customConfigs":{}},"quota":{"storage":"200M"},"isDimTable":true,"isMaterializedView":false}}`

const dimBaseballTeamsSchemaJSON = `{
  "schemaName" : "dimBaseballTeams",
  "enableColumnBasedNullHandling" : false,
  "dimensionFieldSpecs" : [ {
    "name" : "teamID",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  }, {
    "name" : "teamName",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  } ],
  "primaryKeyColumns" : [ "teamID" ]
}`

const dimBaseballTeamsSampleJSON = `{"dataSchema":{"columnNames":["teamID","teamName"],"columnDataTypes":["STRING","STRING"]},"rows":[["ANA","Anaheim Angels"],["ARI","Arizona Diamondbacks"]]}`

// ordersEvents is a REALTIME table created against the QuickStart with a
// Kafka stream config, as Pinot 1.6 stores it under ingestionConfig.
const ordersEventsConfigJSON = `{"REALTIME":{"tableName":"orders_events_REALTIME","tableType":"REALTIME","segmentsConfig":{"retentionTimeUnit":"DAYS","retentionTimeValue":"30","replicasPerPartition":"1","timeType":"MILLISECONDS","minimizeDataMovement":false,"timeColumnName":"event_ts"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"rangeIndexVersion":2,"loadMode":"MMAP","nullHandlingEnabled":false,"invertedIndexColumns":["order_id"],"sortedColumn":["customer_id"]},"metadata":{"customConfigs":{}},"ingestionConfig":{"continueOnError":false,"streamIngestionConfig":{"streamConfigMaps":[{"streamType":"kafka","stream.kafka.topic.name":"orders-events","stream.kafka.broker.list":"localhost:9092","stream.kafka.consumer.type":"lowlevel","stream.kafka.consumer.factory.class.name":"org.apache.pinot.plugin.stream.kafka20.KafkaConsumerFactory","stream.kafka.decoder.class.name":"org.apache.pinot.plugin.stream.kafka.KafkaJSONMessageDecoder","realtime.segment.flush.threshold.rows":"0","realtime.segment.flush.threshold.time":"24h","realtime.segment.flush.threshold.segment.size":"50M"}],"disasterRecoveryMode":"DEFAULT","pauselessConsumptionEnabled":false}},"isDimTable":false,"isMaterializedView":false}}`

const ordersEventsSchemaJSON = `{
  "schemaName" : "orders_events",
  "enableColumnBasedNullHandling" : false,
  "dimensionFieldSpecs" : [ {
    "name" : "order_id",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  }, {
    "name" : "customer_id",
    "dataType" : "STRING",
    "fieldType" : "DIMENSION"
  }, {
    "name" : "item_ids",
    "dataType" : "INT",
    "fieldType" : "DIMENSION",
    "singleValueField" : false
  } ],
  "metricFieldSpecs" : [ {
    "name" : "amount",
    "dataType" : "DOUBLE",
    "fieldType" : "METRIC"
  } ],
  "dateTimeFieldSpecs" : [ {
    "name" : "event_ts",
    "dataType" : "LONG",
    "fieldType" : "DATE_TIME",
    "format" : "1:MILLISECONDS:EPOCH",
    "granularity" : "1:MILLISECONDS"
  } ],
  "primaryKeyColumns" : [ "order_id" ]
}`

// A consuming segment has no size yet; the controller reports -1.
const ordersEventsSizeJSON = `{"tableName":"orders_events","reportedSizeInBytes":-1,"estimatedSizeInBytes":-1,"reportedSizePerReplicaInBytes":-1,"offlineSegments":null,"realtimeSegments":{"reportedSizeInBytes":-1,"estimatedSizeInBytes":-1,"missingSegments":1,"reportedSizePerReplicaInBytes":-1,"segments":{}}}`

const ordersEventsSegmentsJSON = `[{"REALTIME":["orders_events__0__0__20260907T2124Z"]}]`

// clicks is a hybrid table: the same logical table with an OFFLINE and a
// REALTIME side. Pinot lists both configs under one name.
const clicksConfigJSON = `{"OFFLINE":{"tableName":"clicks_OFFLINE","tableType":"OFFLINE","segmentsConfig":{"replication":"2","timeType":"MILLISECONDS","timeColumnName":"ts","retentionTimeUnit":"DAYS","retentionTimeValue":"365","schemaName":"clicks","segmentPushType":"APPEND"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP"},"metadata":{"customConfigs":{}},"isDimTable":false},"REALTIME":{"tableName":"clicks_REALTIME","tableType":"REALTIME","segmentsConfig":{"replicasPerPartition":"2","timeType":"MILLISECONDS","timeColumnName":"ts","retentionTimeUnit":"DAYS","retentionTimeValue":"7","schemaName":"clicks"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP"},"metadata":{"customConfigs":{}},"ingestionConfig":{"streamIngestionConfig":{"streamConfigMaps":[{"streamType":"kafka","stream.kafka.topic.name":"clicks","stream.kafka.broker.list":"kafka-1:9092,kafka-2:9092","stream.kafka.consumer.type":"lowlevel"}]}},"isDimTable":false}}`

const clicksSchemaJSON = `{"schemaName":"clicks","dimensionFieldSpecs":[{"name":"url","dataType":"STRING","fieldType":"DIMENSION"}],"dateTimeFieldSpecs":[{"name":"ts","dataType":"LONG","fieldType":"DATE_TIME","format":"1:MILLISECONDS:EPOCH","granularity":"1:MILLISECONDS"}]}`

const clicksSegmentsJSON = `[{"OFFLINE":["clicks_OFFLINE_0","clicks_OFFLINE_1"]},{"REALTIME":["clicks__0__0__20260901T0000Z"]}]`

// legacyEvents keeps its stream settings under tableIndexConfig, the way
// Pinot versions before 0.12 wrote them.
const legacyEventsConfigJSON = `{"REALTIME":{"tableName":"legacy_events_REALTIME","tableType":"REALTIME","segmentsConfig":{"replicasPerPartition":"1","timeType":"MILLISECONDS","timeColumnName":"ts"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP","streamConfigs":{"streamType":"kafka","stream.kafka.topic.name":"legacy-events","stream.kafka.broker.list":"kafka:9092","stream.kafka.consumer.type":"lowlevel"}},"metadata":{"customConfigs":{}},"isDimTable":false}}`

const kinesisOrdersConfigJSON = `{"REALTIME":{"tableName":"kinesis_orders_REALTIME","tableType":"REALTIME","segmentsConfig":{"replicasPerPartition":"1","timeType":"MILLISECONDS","timeColumnName":"ts"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP"},"metadata":{"customConfigs":{}},"ingestionConfig":{"streamIngestionConfig":{"streamConfigMaps":[{"streamType":"kinesis","stream.kinesis.topic.name":"orders-stream","region":"eu-west-1","stream.kinesis.consumer.type":"lowlevel","stream.kinesis.consumer.factory.class.name":"org.apache.pinot.plugin.stream.kinesis.KinesisConsumerFactory"}]}},"isDimTable":false}}`

const pulsarLogsConfigJSON = `{"REALTIME":{"tableName":"pulsar_logs_REALTIME","tableType":"REALTIME","segmentsConfig":{"replicasPerPartition":"1","timeType":"MILLISECONDS","timeColumnName":"ts"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP"},"metadata":{"customConfigs":{}},"ingestionConfig":{"streamIngestionConfig":{"streamConfigMaps":[{"streamType":"pulsar","stream.pulsar.topic.name":"persistent://public/default/logs","stream.pulsar.bootstrap.servers":"pulsar://pulsar:6650"}]}},"isDimTable":false}}`

// renamedSchemaConfigJSON points at a schema that is not named after the
// table, the case the /schemas/{name} fallback exists for.
const renamedSchemaConfigJSON = `{"OFFLINE":{"tableName":"renamed_OFFLINE","tableType":"OFFLINE","segmentsConfig":{"replication":"1","schemaName":"renamed_schema"},"tenants":{"broker":"DefaultTenant","server":"DefaultTenant"},"tableIndexConfig":{"loadMode":"MMAP"},"metadata":{"customConfigs":{}},"isDimTable":false}}`

const renamedSchemaJSON = `{"schemaName":"renamed_schema","dimensionFieldSpecs":[{"name":"id","dataType":"STRING","fieldType":"DIMENSION"}]}`

const notNullSchemaJSON = `{"schemaName":"strict","dimensionFieldSpecs":[{"name":"id","dataType":"STRING","fieldType":"DIMENSION","notNull":true},{"name":"note","dataType":"STRING","fieldType":"DIMENSION","defaultNullValue":"n/a","maxLength":64}]}`

// The broker answers a query for an unknown table with a 200 and an
// exception in the body, not with an HTTP error.
const tableDoesNotExistJSON = `{"numRowsResultSet":0,"partialResult":true,"exceptions":[{"message":"TableDoesNotExistError","errorCode":190}],"timeUsedMs":0,"requestId":"287231035000000021","brokerId":"Broker_172.17.0.8_8000","tablesQueried":[]}`

// recordedRequest is what the fake remembers about each request so tests
// can check headers and which endpoint took a query.
type recordedRequest struct {
	Method string
	Path   string
	Header http.Header
}

// fakeController is a stand-in for a Pinot controller (and, on the
// /query/sql path, a broker). Tests register the tables it should know
// about and then run a real discovery against it.
type fakeController struct {
	mu sync.Mutex

	health   string
	tables   []string
	configs  map[string]string
	schemas  map[string]string
	named    map[string]string
	sizes    map[string]string
	segments map[string]string
	counts   map[string]int64
	samples  map[string]string
	requests []recordedRequest
}

func newFakeController() *fakeController {
	return &fakeController{
		health:   "OK",
		configs:  make(map[string]string),
		schemas:  make(map[string]string),
		named:    make(map[string]string),
		sizes:    make(map[string]string),
		segments: make(map[string]string),
		counts:   make(map[string]int64),
		samples:  make(map[string]string),
	}
}

// withTable registers a logical table with its config. Schema, size,
// segments and row count are added by the with* helpers below; a table
// without them answers 404 (or a query exception) the way a half-created
// table on a real controller does.
func (f *fakeController) withTable(name, configJSON string) *fakeController {
	f.tables = append(f.tables, name)
	f.configs[name] = configJSON
	return f
}

func (f *fakeController) withSchema(table, schemaJSON string) *fakeController {
	f.schemas[table] = schemaJSON
	return f
}

// withNamedSchema registers a schema under its own name, for the
// /schemas/{name} fallback.
func (f *fakeController) withNamedSchema(name, schemaJSON string) *fakeController {
	f.named[name] = schemaJSON
	return f
}

func (f *fakeController) withSize(table, sizeJSON string) *fakeController {
	f.sizes[table] = sizeJSON
	return f
}

func (f *fakeController) withSegments(table, segmentsJSON string) *fakeController {
	f.segments[table] = segmentsJSON
	return f
}

func (f *fakeController) withRowCount(table string, count int64) *fakeController {
	f.counts[table] = count
	return f
}

// withSample registers the resultTable a SELECT * query returns.
func (f *fakeController) withSample(table, resultTableJSON string) *fakeController {
	f.samples[table] = resultTableJSON
	return f
}

// withHealth overrides the /health body; anything but OK is unhealthy.
func (f *fakeController) withHealth(body string) *fakeController {
	f.health = body
	return f
}

// withBaseballStats registers the batch QuickStart's baseballStats table
// with everything discovery reads for it.
func (f *fakeController) withBaseballStats() *fakeController {
	return f.withTable("baseballStats", baseballStatsConfigJSON).
		withSchema("baseballStats", baseballStatsSchemaJSON).
		withSize("baseballStats", baseballStatsSizeJSON).
		withSegments("baseballStats", baseballStatsSegmentsJSON).
		withRowCount("baseballStats", 97889)
}

func (f *fakeController) withDimBaseballTeams() *fakeController {
	return f.withTable("dimBaseballTeams", dimBaseballTeamsConfigJSON).
		withSchema("dimBaseballTeams", dimBaseballTeamsSchemaJSON).
		withSegments("dimBaseballTeams", `[{"OFFLINE":["dimBaseballTeams_OFFLINE_0"]}]`).
		withRowCount("dimBaseballTeams", 51).
		withSample("dimBaseballTeams", dimBaseballTeamsSampleJSON)
}

func (f *fakeController) withOrdersEvents() *fakeController {
	return f.withTable("orders_events", ordersEventsConfigJSON).
		withSchema("orders_events", ordersEventsSchemaJSON).
		withSize("orders_events", ordersEventsSizeJSON).
		withSegments("orders_events", ordersEventsSegmentsJSON).
		withRowCount("orders_events", 3)
}

var fromTable = regexp.MustCompile(`FROM "((?:[^"]|"")+)"`)

func (f *fakeController) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()})
		f.mu.Unlock()

		path := strings.TrimPrefix(r.URL.Path, "/")

		switch {
		case path == "health":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(f.health))
		case path == "version":
			writeJSON(w, versionJSON)
		case path == "tables" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string][]string{"tables": f.tables})
		case path == "sql" || path == "query/sql":
			f.serveQuery(w, r)
		case strings.HasPrefix(path, "tables/") && strings.HasSuffix(path, "/schema"):
			writeJSONOr404(w, f.schemas, strings.TrimSuffix(strings.TrimPrefix(path, "tables/"), "/schema"))
		case strings.HasPrefix(path, "tables/") && strings.HasSuffix(path, "/size"):
			writeJSONOr404(w, f.sizes, strings.TrimSuffix(strings.TrimPrefix(path, "tables/"), "/size"))
		case strings.HasPrefix(path, "tables/"):
			writeJSONOr404(w, f.configs, strings.TrimPrefix(path, "tables/"))
		case strings.HasPrefix(path, "schemas/"):
			writeJSONOr404(w, f.named, strings.TrimPrefix(path, "schemas/"))
		case strings.HasPrefix(path, "segments/"):
			writeJSONOr404(w, f.segments, strings.TrimPrefix(path, "segments/"))
		default:
			http.Error(w, `{"code":404,"error":"not found"}`, http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)
	return server
}

// serveQuery answers the two statements the plugin sends, COUNT(*) and
// SELECT * ... LIMIT 20, with the broker's response shape.
func (f *fakeController) serveQuery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"code":400,"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	match := fromTable.FindStringSubmatch(body.SQL)
	if match == nil {
		writeJSON(w, tableDoesNotExistJSON)
		return
	}
	table := strings.ReplaceAll(match[1], `""`, `"`)

	switch {
	case strings.HasPrefix(body.SQL, "SELECT COUNT(*)"):
		count, ok := f.counts[table]
		if !ok {
			writeJSON(w, tableDoesNotExistJSON)
			return
		}
		writeJSON(w, fmt.Sprintf(`{"resultTable":{"dataSchema":{"columnNames":["count(*)"],"columnDataTypes":["LONG"]},"rows":[[%d]]},"numRowsResultSet":1,"partialResult":false,"exceptions":[],"timeUsedMs":15,"brokerId":"Broker_172.17.0.8_8000","tablesQueried":["%s"]}`, count, table))
	case strings.HasPrefix(body.SQL, "SELECT *"):
		sample, ok := f.samples[table]
		if !ok {
			writeJSON(w, tableDoesNotExistJSON)
			return
		}
		writeJSON(w, fmt.Sprintf(`{"resultTable":%s,"numRowsResultSet":2,"partialResult":false,"exceptions":[],"timeUsedMs":33,"brokerId":"Broker_172.17.0.8_8000","tablesQueried":["%s"]}`, sample, table))
	default:
		writeJSON(w, tableDoesNotExistJSON)
	}
}

// requestsTo returns the recorded requests for one path.
func (f *fakeController) requestsTo(path string) []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	var matches []recordedRequest
	for _, req := range f.requests {
		if req.Path == path {
			matches = append(matches, req)
		}
	}
	return matches
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func writeJSONOr404(w http.ResponseWriter, bodies map[string]string, key string) {
	body, ok := bodies[key]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"code":404,"error":"%s not found"}`, key), http.StatusNotFound)
		return
	}
	writeJSON(w, body)
}

// discover runs a full discovery against the fake. Extra config keys
// override the defaults.
func discover(t *testing.T, f *fakeController, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"controller_url": server.URL}
	for key, value := range overrides {
		config[key] = value
	}

	s := &Source{}
	result, err := s.Discover(t.Context(), config)
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

// columnsOf decodes an asset's column list, keyed by column name, with
// the order preserved in the returned slice.
func columnsOf(t *testing.T, a *pluginsdk.Asset) ([]map[string]interface{}, map[string]map[string]interface{}) {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column list on %s", *a.Name)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return columns, byName
}

// statsOf returns the statistics recorded for one MRN by metric name.
func statsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}
