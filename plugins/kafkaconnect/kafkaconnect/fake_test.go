package kafkaconnect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// The fixtures below are responses captured from confluentinc/cp-kafka-connect:7.9.0
// running two FileStream connectors, trimmed only where noted.

const rootFixture = `{"version":"7.9.0-ccs","commit":"ebe6df624d6bc758c37aba7053cf868c1e532ccd","kafka_cluster_id":"5L6g3nShT-eMCtK--X86sw"}`

const pluginsFixture = `[{"class":"org.apache.kafka.connect.file.FileStreamSinkConnector","type":"sink","version":"7.9.0-ccs"},{"class":"org.apache.kafka.connect.file.FileStreamSourceConnector","type":"source","version":"7.9.0-ccs"},{"class":"org.apache.kafka.connect.mirror.MirrorCheckpointConnector","type":"source","version":"7.9.0-ccs"},{"class":"org.apache.kafka.connect.mirror.MirrorHeartbeatConnector","type":"source","version":"7.9.0-ccs"},{"class":"org.apache.kafka.connect.mirror.MirrorSourceConnector","type":"source","version":"7.9.0-ccs"}]`

// GET /connectors?expand=status&expand=info, both connectors RUNNING.
const fileSourceFixture = `{"status":{"name":"orders-file-source","connector":{"state":"RUNNING","worker_id":"marmot-test-kafkaconnect-connect:8083"},"tasks":[{"id":0,"state":"RUNNING","worker_id":"marmot-test-kafkaconnect-connect:8083"}],"type":"source"},"info":{"name":"orders-file-source","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSourceConnector","file":"/tmp/orders.txt","tasks.max":"1","name":"orders-file-source","topic":"orders-events"},"tasks":[{"connector":"orders-file-source","task":0}],"type":"source"}}`

const fileSinkFixture = `{"status":{"name":"orders-file-sink","connector":{"state":"RUNNING","worker_id":"marmot-test-kafkaconnect-connect:8083"},"tasks":[{"id":0,"state":"RUNNING","worker_id":"marmot-test-kafkaconnect-connect:8083"}],"type":"sink"},"info":{"name":"orders-file-sink","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSinkConnector","file":"/tmp/orders-out.txt","tasks.max":"1","topics":"orders-events","name":"orders-file-sink"},"tasks":[{"connector":"orders-file-sink","task":0}],"type":"sink"}}`

// A sink whose task died at start. The connector itself stays RUNNING;
// only the task carries the trace (trimmed to its first lines here).
const brokenSinkFixture = `{"status":{"name":"broken-sink","connector":{"state":"RUNNING","worker_id":"marmot-test-kafkaconnect-connect:8083"},"tasks":[{"id":0,"state":"FAILED","worker_id":"marmot-test-kafkaconnect-connect:8083","trace":"org.apache.kafka.connect.errors.ConnectException: Couldn't find or create file '/nonexistent-dir/out.txt' for FileStreamSinkTask\n\tat org.apache.kafka.connect.file.FileStreamSinkTask.start(FileStreamSinkTask.java:74)\n\tat org.apache.kafka.connect.runtime.WorkerSinkTask.initializeAndStart(WorkerSinkTask.java:324)\n\tat org.apache.kafka.connect.runtime.WorkerTask.doStart(WorkerTask.java:176)\n\tat org.apache.kafka.connect.runtime.WorkerTask.doRun(WorkerTask.java:225)\n\tat org.apache.kafka.connect.runtime.WorkerTask.run(WorkerTask.java:281)\n\tat org.apache.kafka.connect.runtime.isolation.Plugins.lambda$withClassLoader$1(Plugins.java:238)\n\tat java.base/java.util.concurrent.Executors$RunnableAdapter.call(Executors.java:539)\n\tat java.base/java.util.concurrent.FutureTask.run(FutureTask.java:264)\n\tat java.base/java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1136)\n\tat java.base/java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:635)\n\tat java.base/java.lang.Thread.run(Thread.java:840)\n"}],"type":"sink"},"info":{"name":"broken-sink","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSinkConnector","file":"/nonexistent-dir/out.txt","tasks.max":"1","topics":"orders-events","name":"broken-sink"},"tasks":[{"connector":"broken-sink","task":0}],"type":"sink"}}`

// The sink after PUT /connectors/orders-file-sink/pause.
const pausedSinkFixture = `{"status":{"name":"orders-file-sink","connector":{"state":"PAUSED","worker_id":"marmot-test-kafkaconnect-connect:8083"},"tasks":[{"id":0,"state":"PAUSED","worker_id":"marmot-test-kafkaconnect-connect:8083"}],"type":"sink"},"info":{"name":"orders-file-sink","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSinkConnector","file":"/tmp/orders-out.txt","tasks.max":"1","topics":"orders-events","name":"orders-file-sink"},"tasks":[{"connector":"orders-file-sink","task":0}],"type":"sink"}}`

// fakeConnect is a stand-in for a Connect worker. Tests register
// connectors as the JSON the expand endpoint returns for them and then
// run a real discovery against it.
type fakeConnect struct {
	connectors map[string]map[string]json.RawMessage
	order      []string
	topics     map[string][]string

	// topicsStatus and topicsBody make the active topics endpoint fail.
	topicsStatus int
	topicsBody   string
	topicsCalls  int

	// pluginsStatus makes the connector plugin list fail.
	pluginsStatus int

	// rejectDoubleExpand answers 400 to expand=status&expand=info, the
	// way a worker that only takes one expand does.
	rejectDoubleExpand bool
	// noExpand answers the bare list of names whatever is asked, the way
	// a worker older than Kafka 2.3 does.
	noExpand bool

	username, password string

	requests []string
}

func newFakeConnect() *fakeConnect {
	return &fakeConnect{
		connectors: make(map[string]map[string]json.RawMessage),
		topics:     make(map[string][]string),
	}
}

// with registers one connector from its expanded JSON.
func (f *fakeConnect) with(fixture string) *fakeConnect {
	var entry map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fixture), &entry); err != nil {
		panic(err)
	}
	var info struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(entry["info"], &info); err != nil {
		panic(err)
	}
	f.connectors[info.Name] = entry
	f.order = append(f.order, info.Name)
	return f
}

// withConnector registers a connector built from a config map, with
// every task RUNNING, for connector families the test system does not
// have installed.
func (f *fakeConnect) withConnector(name, connectorType string, config map[string]string) *fakeConnect {
	config["name"] = name
	entry := map[string]any{
		"info": map[string]any{
			"name":   name,
			"config": config,
			"tasks":  []map[string]any{{"connector": name, "task": 0}},
			"type":   connectorType,
		},
		"status": map[string]any{
			"name":      name,
			"connector": map[string]any{"state": "RUNNING", "worker_id": "worker:8083"},
			"tasks":     []map[string]any{{"id": 0, "state": "RUNNING", "worker_id": "worker:8083"}},
			"type":      connectorType,
		},
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		panic(err)
	}
	return f.with(string(raw))
}

// withTopics registers what the active topics endpoint reports for a
// connector.
func (f *fakeConnect) withTopics(name string, topics ...string) *fakeConnect {
	f.topics[name] = topics
	return f
}

func (f *fakeConnect) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")

		if f.username != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != f.username || pass != f.password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		path := r.URL.Path
		switch {
		case path == "/":
			_, _ = w.Write([]byte(rootFixture))
		case path == "/connector-plugins":
			if f.pluginsStatus != 0 {
				http.Error(w, `{"error_code":500,"message":"boom"}`, f.pluginsStatus)
				return
			}
			_, _ = w.Write([]byte(pluginsFixture))
		case path == "/connectors":
			f.serveList(w, r)
		case strings.HasPrefix(path, "/connectors/"):
			f.serveConnector(w, strings.TrimPrefix(path, "/connectors/"))
		default:
			http.Error(w, `{"error_code":404,"message":"HTTP 404 Not Found"}`, http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)
	return server
}

func (f *fakeConnect) serveList(w http.ResponseWriter, r *http.Request) {
	expands := r.URL.Query()["expand"]
	if f.noExpand || len(expands) == 0 {
		_ = json.NewEncoder(w).Encode(f.order)
		return
	}
	if f.rejectDoubleExpand && len(expands) > 1 {
		http.Error(w, `{"error_code":400,"message":"Invalid expand"}`, http.StatusBadRequest)
		return
	}
	out := make(map[string]map[string]json.RawMessage, len(f.connectors))
	for name, entry := range f.connectors {
		out[name] = make(map[string]json.RawMessage)
		for _, expand := range expands {
			out[name][expand] = entry[expand]
		}
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeConnect) serveConnector(w http.ResponseWriter, rest string) {
	name, sub, _ := strings.Cut(rest, "/")
	entry, ok := f.connectors[name]
	if !ok && sub != "topics" {
		http.Error(w, `{"error_code":404,"message":"Connector `+name+` not found"}`, http.StatusNotFound)
		return
	}
	switch sub {
	case "":
		_, _ = w.Write(entry["info"])
	case "status":
		_, _ = w.Write(entry["status"])
	case "topics":
		f.topicsCalls++
		if f.topicsStatus != 0 {
			http.Error(w, f.topicsBody, f.topicsStatus)
			return
		}
		topics := f.topics[name]
		if topics == nil {
			topics = []string{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{name: map[string]any{"topics": topics}})
	default:
		http.Error(w, `{"error_code":404,"message":"HTTP 404 Not Found"}`, http.StatusNotFound)
	}
}

// discover runs a full discovery against the fake worker. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeConnect, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL}
	for key, value := range overrides {
		config[key] = value
	}

	result, err := (&Source{}).Discover(t.Context(), config)
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

// countRequests counts how often one exact request URI was served.
func countRequests(f *fakeConnect, uri string) int {
	n := 0
	for _, r := range f.requests {
		if r == uri {
			n++
		}
	}
	return n
}
