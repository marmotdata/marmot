package flink

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// fakeJob is one job as the JobManager reports it across its endpoints.
type fakeJob struct {
	overview   map[string]interface{}
	details    map[string]interface{}
	config     map[string]interface{}
	exceptions map[string]interface{}
}

func (j fakeJob) jid() string {
	return j.details["jid"].(string)
}

// renamed gives the job another name everywhere the name appears.
func (j fakeJob) renamed(name string) fakeJob {
	j.overview["name"] = name
	j.details["name"] = name
	j.config["name"] = name
	return j
}

// withJID gives the job another id everywhere the id appears.
func (j fakeJob) withJID(jid string) fakeJob {
	j.overview["jid"] = jid
	j.details["jid"] = jid
	j.config["jid"] = jid
	return j
}

// startedAt moves the job's submission time.
func (j fakeJob) startedAt(millis int64) fakeJob {
	j.overview["start-time"] = millis
	j.details["start-time"] = millis
	return j
}

// jobFixture decodes one captured job. Every call decodes afresh so a test
// can mutate its copy without touching another test's.
func jobFixture(detailsJSON, configJSON, exceptionsJSON string) fakeJob {
	var job fakeJob
	mustDecode(detailsJSON, &job.details)
	mustDecode(configJSON, &job.config)
	mustDecode(exceptionsJSON, &job.exceptions)

	var overview struct {
		Jobs []map[string]interface{} `json:"jobs"`
	}
	mustDecode(jobsOverviewJSON, &overview)
	for _, entry := range overview.Jobs {
		if entry["jid"] == job.details["jid"] {
			job.overview = entry
		}
	}
	if job.overview == nil {
		panic("no overview entry for job " + job.details["jid"].(string))
	}
	return job
}

func mustDecode(data string, out interface{}) {
	if err := json.Unmarshal([]byte(data), out); err != nil {
		panic(err)
	}
}

// The four jobs captured from the live cluster: the StateMachineExample
// left running, WordCount run to completion, TopSpeedWindowing cancelled
// by hand and SocketWindowWordCount pointed at a host that does not exist.

func runningJob() fakeJob {
	return jobFixture(runningJobJSON, runningConfigJSON, runningExceptionsJSON)
}

func finishedJob() fakeJob {
	return jobFixture(finishedJobJSON, finishedConfigJSON, finishedExceptionsJSON)
}

func canceledJob() fakeJob {
	return jobFixture(canceledJobJSON, canceledConfigJSON, canceledExceptionsJSON)
}

func failedJob() fakeJob {
	return jobFixture(failedJobJSON, failedConfigJSON, failedExceptionsJSON)
}

// fakeJobManager is a stand-in for a Flink JobManager. Tests register the
// jobs it should report and then run a real discovery against it.
type fakeJobManager struct {
	jobs    []fakeJob
	broken  map[string]bool
	mu      sync.Mutex
	headers []http.Header
}

func newFakeJobManager() *fakeJobManager {
	return &fakeJobManager{broken: make(map[string]bool)}
}

// with registers jobs for the fake to report.
func (f *fakeJobManager) with(jobs ...fakeJob) *fakeJobManager {
	f.jobs = append(f.jobs, jobs...)
	return f
}

// failing makes a path answer 500, the way a JobManager mid-restart does.
func (f *fakeJobManager) failing(path string) *fakeJobManager {
	f.broken[path] = true
	return f
}

// requestHeaders returns the headers of every request the fake served.
func (f *fakeJobManager) requestHeaders() []http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]http.Header(nil), f.headers...)
}

func (f *fakeJobManager) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.headers = append(f.headers, r.Header.Clone())
		f.mu.Unlock()

		if f.broken[r.URL.Path] {
			http.Error(w, "Service temporarily unavailable due to an ongoing leader election.", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/config":
			_, _ = w.Write([]byte(clusterConfigJSON))
			return
		case "/jobs/overview":
			entries := make([]map[string]interface{}, 0, len(f.jobs))
			for _, job := range f.jobs {
				entries = append(entries, job.overview)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"jobs": entries})
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/jobs/"), "/")
		for _, job := range f.jobs {
			if job.jid() != parts[0] {
				continue
			}
			switch strings.Join(parts[1:], "/") {
			case "":
				_ = json.NewEncoder(w).Encode(job.details)
			case "config":
				_ = json.NewEncoder(w).Encode(job.config)
			case "exceptions":
				_ = json.NewEncoder(w).Encode(job.exceptions)
			default:
				http.NotFound(w, r)
			}
			return
		}

		http.Error(w, `{"errors":["Job could not be found."]}`, http.StatusNotFound)
	}))

	t.Cleanup(server.Close)
	return server
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeJobManager, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
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

// findAsset looks an asset up by type and name.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

// hasEdge reports whether the result contains an edge between two MRNs.
func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

// runsOf returns the run history recorded for one asset.
func runsOf(result *pluginsdk.DiscoveryResult, assetMRN string) []pluginsdk.RunHistoryEvent {
	for _, history := range result.RunHistory {
		if history.AssetMRN == assetMRN {
			return history.Runs
		}
	}
	return nil
}

// eventTypes lists the event types of a run in the order they were emitted.
func eventTypes(runs []pluginsdk.RunHistoryEvent) []string {
	types := make([]string, 0, len(runs))
	for _, run := range runs {
		types = append(types, run.EventType)
	}
	return types
}

func pipelineMRN(name string) string {
	return mrn.New("Pipeline", provider, name)
}

func taskMRN(name string) string {
	return mrn.New("Task", provider, name)
}
