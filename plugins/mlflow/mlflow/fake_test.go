package mlflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// The fixtures below are trimmed copies of what MLflow 3.16.0 returned
// for the objects the e2e tests seed, so the fake answers with the real
// wire shapes: latest_versions on a registered model, metrics with a
// step, the log-model history tag, dataset inputs with JSON stored as
// strings, and a logged model behind a models:/ source.

const churnRunID = "b47b7fabd5df4064a0c9df7fcf5bba89"
const fraudRunID = "fc6a943b43a643a6983ec2eb2aedc27c"
const fraudV1RunID = "5d940fb1943a472d8f263cbc90b32f53"
const loggedModelID = "m-6e8aac52e8224bec84b24369b274f665"

const churnModelJSON = `{
  "name": "churn-predictor",
  "creation_timestamp": 1788815465218,
  "last_updated_timestamp": 1788815465566,
  "description": "Predicts churn",
  "latest_versions": [` + churnVersionJSON + `],
  "tags": [{"key": "team", "value": "growth"}],
  "aliases": [{"alias": "champion", "version": "1"}]
}`

const churnVersionJSON = `{
  "name": "churn-predictor",
  "version": "1",
  "creation_timestamp": 1788815465566,
  "last_updated_timestamp": 1788815465566,
  "current_stage": "None",
  "description": "v1",
  "source": "runs:/` + churnRunID + `/model",
  "run_id": "` + churnRunID + `",
  "status": "READY",
  "run_link": ""
}`

const churnRunJSON = `{
  "info": {
    "run_uuid": "` + churnRunID + `",
    "experiment_id": "1",
    "run_name": "baseline",
    "user_id": "",
    "status": "FINISHED",
    "start_time": 1788815456000,
    "end_time": 1788815461000,
    "artifact_uri": "mlflow-artifacts:/1/` + churnRunID + `/artifacts",
    "lifecycle_stage": "active",
    "run_id": "` + churnRunID + `"
  },
  "data": {
    "metrics": [
      {"key": "accuracy", "value": 0.91, "timestamp": 1788815456000, "step": 0},
      {"key": "f1", "value": 0.87, "timestamp": 1788815456000, "step": 0}
    ],
    "params": [
      {"key": "max_depth", "value": "6"},
      {"key": "learning_rate", "value": "0.1"}
    ],
    "tags": [
      {"key": "mlflow.runName", "value": "baseline"},
      {"key": "mlflow.log-model.history", "value": "[{\"run_id\": \"` + churnRunID + `\", \"artifact_path\": \"model\", \"flavors\": {\"sklearn\": {}}, \"signature\": {\"inputs\": \"[{\\\"name\\\": \\\"age\\\", \\\"type\\\": \\\"long\\\", \\\"required\\\": true}, {\\\"name\\\": \\\"plan\\\", \\\"type\\\": \\\"string\\\", \\\"required\\\": false}]\", \"outputs\": \"[{\\\"type\\\": \\\"double\\\"}]\"}}]"},
      {"key": "mlflow.user", "value": "marmot"}
    ]
  },
  "inputs": {
    "dataset_inputs": [
      {
        "tags": [{"key": "mlflow.data.context", "value": "training"}],
        "dataset": {
          "name": "customers",
          "digest": "abc123",
          "source_type": "s3",
          "source": "{\"uri\": \"s3://ml-data/customers.parquet\"}",
          "schema": "{\"mlflow_colspec\": [{\"name\": \"age\", \"type\": \"long\", \"required\": true}, {\"name\": \"plan\", \"type\": \"string\", \"required\": false}]}",
          "profile": "{\"num_rows\": 1200, \"num_elements\": 2400}"
        }
      }
    ]
  },
  "outputs": {}
}`

const fraudModelJSON = `{
  "name": "fraud-detector",
  "creation_timestamp": 1788815505861,
  "last_updated_timestamp": 1788815506187,
  "description": "Flags fraudulent transactions",
  "latest_versions": [` + fraudV2JSON + `]
}`

const fraudV2JSON = `{
  "name": "fraud-detector",
  "version": "2",
  "creation_timestamp": 1788815506187,
  "last_updated_timestamp": 1788815506187,
  "current_stage": "None",
  "description": "",
  "source": "runs:/` + fraudRunID + `/model",
  "run_id": "` + fraudRunID + `",
  "status": "READY",
  "tags": [{"key": "validated", "value": "true"}],
  "run_link": ""
}`

const fraudV1JSON = `{
  "name": "fraud-detector",
  "version": "1",
  "creation_timestamp": 1788815505989,
  "last_updated_timestamp": 1788815505989,
  "current_stage": "None",
  "description": "",
  "source": "runs:/` + fraudV1RunID + `/model",
  "run_id": "` + fraudV1RunID + `",
  "status": "READY",
  "run_link": ""
}`

// The fraud run logs no model history tag, so its signature has to be
// read from the MLmodel file. It also carries a metric MLflow serialises
// as a string.
const fraudRunJSON = `{
  "info": {
    "run_uuid": "` + fraudRunID + `",
    "experiment_id": "2",
    "run_name": "fraud-v2",
    "user_id": "",
    "status": "FINISHED",
    "start_time": 1788815501000,
    "end_time": 1788815506000,
    "artifact_uri": "mlflow-artifacts:/2/` + fraudRunID + `/artifacts",
    "lifecycle_stage": "active",
    "run_id": "` + fraudRunID + `"
  },
  "data": {
    "metrics": [
      {"key": "auc", "value": 0.97, "timestamp": 1788815502000, "step": 1},
      {"key": "nan_metric", "value": "NaN", "timestamp": 1788815636000, "step": 0}
    ],
    "params": [{"key": "n_estimators", "value": "200"}],
    "tags": [{"key": "mlflow.runName", "value": "fraud-v2"}]
  },
  "inputs": {
    "dataset_inputs": [
      {
        "tags": [{"key": "mlflow.data.context", "value": "training"}],
        "dataset": {
          "name": "transactions",
          "digest": "def456",
          "source_type": "gs",
          "source": "{\"uri\": \"gs://fraud-data/transactions.parquet\"}",
          "schema": "{\"mlflow_colspec\": [{\"name\": \"amount\", \"type\": \"double\", \"required\": true}]}"
        }
      }
    ]
  },
  "outputs": {}
}`

// evalModelJSON registers a model from a run that evaluated on the same
// transactions dataset the fraud run trained on.
const evalModelJSON = `{
  "name": "fraud-eval",
  "creation_timestamp": 1788815600000,
  "last_updated_timestamp": 1788815600000,
  "latest_versions": [` + evalVersionJSON + `]
}`

const evalVersionJSON = `{
  "name": "fraud-eval",
  "version": "1",
  "current_stage": "None",
  "source": "runs:/evalrun/model",
  "run_id": "evalrun",
  "status": "READY"
}`

const evalRunJSON = `{
  "info": {"run_id": "evalrun", "run_name": "eval", "experiment_id": "2", "status": "FINISHED",
    "artifact_uri": "mlflow-artifacts:/2/evalrun/artifacts", "lifecycle_stage": "active"},
  "data": {},
  "inputs": {
    "dataset_inputs": [
      {
        "tags": [{"key": "mlflow.data.context", "value": "eval"}],
        "dataset": {
          "name": "transactions",
          "digest": "def456",
          "source_type": "gs",
          "source": "{\"uri\": \"gs://fraud-data/transactions.parquet\"}"
        }
      }
    ]
  }
}`

const loggedModelRegJSON = `{
  "name": "fraud-lm-reg",
  "creation_timestamp": 1788815526260,
  "last_updated_timestamp": 1788815526487,
  "description": "Registered from a logged model",
  "latest_versions": [` + loggedModelVersionJSON + `]
}`

const loggedModelVersionJSON = `{
  "name": "fraud-lm-reg",
  "version": "1",
  "creation_timestamp": 1788815526487,
  "last_updated_timestamp": 1788815526487,
  "current_stage": "None",
  "description": "",
  "source": "models:/` + loggedModelID + `",
  "run_id": "` + fraudRunID + `",
  "status": "READY",
  "run_link": ""
}`

const loggedModelJSON = `{
  "info": {
    "model_id": "` + loggedModelID + `",
    "experiment_id": "2",
    "name": "fraud-lm",
    "creation_timestamp_ms": 1788815506322,
    "last_updated_timestamp_ms": 1788815506322,
    "artifact_uri": "mlflow-artifacts:/2/models/` + loggedModelID + `/artifacts",
    "status": "LOGGED_MODEL_PENDING",
    "model_type": "sklearn",
    "source_run_id": "` + fraudRunID + `"
  },
  "data": {"params": [{"key": "n_estimators", "value": "200"}]}
}`

const experimentsJSON = `[
  {
    "experiment_id": "2",
    "name": "fraud",
    "artifact_location": "mlflow-artifacts:/2",
    "lifecycle_stage": "active",
    "last_update_time": 1788815501228,
    "creation_time": 1788815501228,
    "tags": [{"key": "owner", "value": "risk"}],
    "workspace": "default"
  },
  {
    "experiment_id": "1",
    "name": "churn",
    "artifact_location": "mlflow-artifacts:/1",
    "lifecycle_stage": "active",
    "last_update_time": 1788815456343,
    "creation_time": 1788815456343,
    "workspace": "default"
  },
  {
    "experiment_id": "0",
    "name": "Default",
    "artifact_location": "mlflow-artifacts:/0",
    "lifecycle_stage": "active",
    "last_update_time": 1788815429950,
    "creation_time": 1788815429950,
    "workspace": "default"
  }
]`

const fraudMLmodel = `artifact_path: model
flavors:
  sklearn:
    model_format: cloudpickle
    sklearn_version: 1.5.0
mlflow_version: 3.16.0
run_id: ` + fraudRunID + `
signature:
  inputs: '[{"name": "amount", "type": "double", "required": true}, {"name": "merchant", "type": "string", "required": false}]'
  outputs: '[{"type": "long", "required": true}]'
  params: null
utc_time_created: '2026-09-07 21:00:00.000000'
`

// fakeMLflow is a stand-in for an MLflow tracking server with its model
// registry. Tests register the objects each endpoint should return and
// then run a real discovery against it.
type fakeMLflow struct {
	models       []json.RawMessage
	versions     map[string][]json.RawMessage
	runs         map[string]json.RawMessage
	experiments  []json.RawMessage
	loggedModels map[string]json.RawMessage
	runArtifacts map[string]string
	proxied      map[string]string

	// pageSize, when set, pages the registered model listing.
	pageSize int
	// versionSearchFails makes the model version search answer 500.
	versionSearchFails bool
	// experimentSearchFails makes the experiment search answer 500.
	experimentSearchFails bool

	mu       sync.Mutex
	requests []string
	auth     string
}

func newFakeMLflow() *fakeMLflow {
	return &fakeMLflow{
		versions:     make(map[string][]json.RawMessage),
		runs:         make(map[string]json.RawMessage),
		loggedModels: make(map[string]json.RawMessage),
		runArtifacts: make(map[string]string),
		proxied:      make(map[string]string),
	}
}

// seeded returns a fake holding the same objects the e2e tests seed into
// a real server.
func seeded() *fakeMLflow {
	f := newFakeMLflow()
	f.withModel(churnModelJSON, churnVersionJSON)
	f.withModel(fraudModelJSON, fraudV2JSON, fraudV1JSON)
	f.withModel(loggedModelRegJSON, loggedModelVersionJSON)
	f.withRun(churnRunID, churnRunJSON)
	f.withRun(fraudRunID, fraudRunJSON)
	f.withExperiments(experimentsJSON)
	f.loggedModels[loggedModelID] = json.RawMessage(loggedModelJSON)
	f.runArtifacts[fraudRunID+"|model/MLmodel"] = fraudMLmodel
	f.proxied["2/models/"+loggedModelID+"/artifacts/MLmodel"] = fraudMLmodel
	return f
}

func (f *fakeMLflow) withModel(model string, versions ...string) *fakeMLflow {
	f.models = append(f.models, json.RawMessage(model))
	var m struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(model), &m); err != nil {
		panic(err)
	}
	for _, v := range versions {
		f.versions[m.Name] = append(f.versions[m.Name], json.RawMessage(v))
	}
	return f
}

func (f *fakeMLflow) withRun(runID, run string) *fakeMLflow {
	f.runs[runID] = json.RawMessage(run)
	return f
}

func (f *fakeMLflow) withExperiments(list string) *fakeMLflow {
	var experiments []json.RawMessage
	if err := json.Unmarshal([]byte(list), &experiments); err != nil {
		panic(err)
	}
	f.experiments = append(f.experiments, experiments...)
	return f
}

func (f *fakeMLflow) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.RequestURI())
		f.auth = r.Header.Get("Authorization")
		f.mu.Unlock()

		switch {
		case r.URL.Path == "/get-artifact":
			f.serveText(w, f.runArtifacts[r.URL.Query().Get("run_uuid")+"|"+r.URL.Query().Get("path")])
			return
		case strings.HasPrefix(r.URL.Path, "/api/2.0/mlflow-artifacts/artifacts/"):
			f.serveText(w, f.proxied[strings.TrimPrefix(r.URL.Path, "/api/2.0/mlflow-artifacts/artifacts/")])
			return
		}

		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/api/2.0/mlflow/")
		switch {
		case path == "registered-models/search":
			f.serveModels(w, r)
		case path == "model-versions/search":
			if f.versionSearchFails {
				http.Error(w, `{"error_code": "INTERNAL_ERROR", "message": "boom"}`, http.StatusInternalServerError)
				return
			}
			name := filterName(r.URL.Query().Get("filter"))
			writeJSON(w, map[string]any{"model_versions": orEmpty(f.versions[name])})
		case path == "runs/get":
			run, ok := f.runs[r.URL.Query().Get("run_id")]
			if !ok {
				notFound(w)
				return
			}
			writeJSON(w, map[string]any{"run": run})
		case path == "experiments/search":
			if f.experimentSearchFails {
				http.Error(w, `{"error_code": "INTERNAL_ERROR", "message": "boom"}`, http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"experiments": orEmpty(f.experiments)})
		case strings.HasPrefix(path, "logged-models/"):
			model, ok := f.loggedModels[strings.TrimPrefix(path, "logged-models/")]
			if !ok {
				notFound(w)
				return
			}
			writeJSON(w, map[string]any{"model": model})
		default:
			notFound(w)
		}
	}))

	t.Cleanup(server.Close)
	return server
}

// serveModels pages the registered model list the way MLflow does, with
// an opaque token the client has to hand back unchanged.
func (f *fakeMLflow) serveModels(w http.ResponseWriter, r *http.Request) {
	offset := 0
	if token := r.URL.Query().Get("page_token"); token != "" {
		offset, _ = strconv.Atoi(token)
	}
	end := len(f.models)
	if f.pageSize > 0 && offset+f.pageSize < end {
		end = offset + f.pageSize
	}
	if offset > len(f.models) {
		offset = len(f.models)
	}

	resp := map[string]any{"registered_models": orEmpty(f.models[offset:end])}
	if end < len(f.models) {
		resp["next_page_token"] = strconv.Itoa(end)
	}
	writeJSON(w, resp)
}

func (f *fakeMLflow) serveText(w http.ResponseWriter, content string) {
	if content == "" {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(content))
}

// filterName extracts the model name from a name='x' or name="x" filter.
func filterName(filter string) string {
	value := strings.TrimPrefix(filter, "name=")
	return strings.Trim(value, `'"`)
}

func (f *fakeMLflow) requestsTo(path string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		if strings.HasPrefix(r, path) {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeMLflow) lastAuth() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.auth
}

func orEmpty(items []json.RawMessage) []json.RawMessage {
	if items == nil {
		return []json.RawMessage{}
	}
	return items
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter) {
	http.Error(w, `{"error_code": "RESOURCE_DOES_NOT_EXIST", "message": "not found"}`, http.StatusNotFound)
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeMLflow, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"tracking_uri": server.URL}
	for key, value := range overrides {
		config[key] = value
	}

	result, err := (&Source{}).Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

// findAsset looks an asset up by type and name, through the MRN the
// server would rebuild from the asset's own fields.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type != assetType || asset.MRN == nil || len(asset.Providers) == 0 {
			continue
		}
		if *asset.MRN == mrn.New(assetType, asset.Providers[0], name) {
			return &result.Assets[i]
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

// columnsOf decodes an asset's column list, keyed by column name.
func columnsOf(t *testing.T, asset *pluginsdk.Asset) map[string]pluginsdk.Column {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	if !ok {
		return nil
	}
	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]pluginsdk.Column, len(columns))
	for _, c := range columns {
		byName[c.Name] = c
	}
	return byName
}

// statisticsOf returns an asset's statistics keyed by metric name.
func statisticsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}
