package prefect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakePrefect stands in for a Prefect server. Every fixture below is JSON
// captured from prefecthq/prefect:3-latest (Prefect 3.8.5), trimmed to
// the fields that matter, so a test proves the plugin against the shapes
// a real server sends.
type fakePrefect struct {
	flows            []map[string]any
	deployments      map[string][]map[string]any
	flowRuns         map[string][]map[string]any
	taskRuns         map[string][]map[string]any
	materializations map[string][]map[string]any
	// assetsAPI is off by default, the way a self-hosted server behaves:
	// it answers 404 for the Cloud-only Assets API.
	assetsAPI bool
	// failDeploymentsFor makes one flow's deployments call fail, so a test
	// can prove discovery carries on past it.
	failDeploymentsFor string

	mu       sync.Mutex
	requests map[string][]map[string]any
}

func newFakePrefect() *fakePrefect {
	return &fakePrefect{
		deployments:      make(map[string][]map[string]any),
		flowRuns:         make(map[string][]map[string]any),
		taskRuns:         make(map[string][]map[string]any),
		materializations: make(map[string][]map[string]any),
		requests:         make(map[string][]map[string]any),
	}
}

// withFlows registers the workspace's flows.
func (f *fakePrefect) withFlows(t *testing.T, raw string) *fakePrefect {
	f.flows = decodeList(t, raw)
	return f
}

// withDeployments registers the deployments of one flow.
func (f *fakePrefect) withDeployments(t *testing.T, flowID, raw string) *fakePrefect {
	f.deployments[flowID] = decodeList(t, raw)
	return f
}

// withFlowRuns registers the runs of one flow, newest first.
func (f *fakePrefect) withFlowRuns(t *testing.T, flowID, raw string) *fakePrefect {
	f.flowRuns[flowID] = decodeList(t, raw)
	return f
}

// withTaskRuns registers the task runs of one flow run.
func (f *fakePrefect) withTaskRuns(t *testing.T, flowRunID, raw string) *fakePrefect {
	f.taskRuns[flowRunID] = decodeList(t, raw)
	return f
}

// withMaterializations turns on the Cloud-only Assets API and registers
// what one flow run wrote.
func (f *fakePrefect) withMaterializations(t *testing.T, flowRunID, raw string) *fakePrefect {
	f.assetsAPI = true
	f.materializations[flowRunID] = decodeList(t, raw)
	return f
}

// start serves the fake and returns its base address, which is what a
// user would put in the host field: no /api path on it.
func (f *fakePrefect) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(path, "/assets/materializations") {
			if !f.assetsAPI {
				http.Error(w, `{"detail":"Not Found"}`, http.StatusNotFound)
				return
			}
			flowRunID := strings.TrimSuffix(strings.TrimPrefix(path, "/flow_runs/"), "/assets/materializations")
			writeList(w, f.materializations[flowRunID])
			return
		}

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		resource := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/filter")
		f.record(resource, body)

		switch resource {
		case "flows":
			writeList(w, paginate(f.flows, body))
		case "deployments":
			flowID := anyID(body, "flows", "id")
			if flowID != "" && flowID == f.failDeploymentsFor {
				http.Error(w, `{"exception_message":"Internal Server Error"}`, http.StatusInternalServerError)
				return
			}
			writeList(w, paginate(f.deployments[flowID], body))
		case "flow_runs":
			writeList(w, paginate(f.flowRuns[anyID(body, "flows", "id")], body))
		case "task_runs":
			writeList(w, paginate(f.taskRuns[anyID(body, "task_runs", "flow_run_id")], body))
		default:
			http.Error(w, `{"detail":"Not Found"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// record keeps every filter body the plugin sent, so a test can assert on
// the query and not only on the result.
func (f *fakePrefect) record(resource string, body map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests[resource] = append(f.requests[resource], body)
}

// filters returns the bodies posted to one resource's filter endpoint.
func (f *fakePrefect) filters(resource string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any{}, f.requests[resource]...)
}

// requestCount is how many times one resource's filter endpoint was hit.
func (f *fakePrefect) requestCount(resource string) int {
	return len(f.filters(resource))
}

func decodeList(t *testing.T, raw string) []map[string]any {
	t.Helper()

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &items))

	return items
}

func writeList(w http.ResponseWriter, items []map[string]any) {
	if items == nil {
		items = []map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(items)
}

// paginate applies the request's limit and offset, the way Prefect does.
func paginate(items []map[string]any, body map[string]any) []map[string]any {
	limit, offset := len(items), 0
	if value, ok := body["limit"].(float64); ok {
		limit = int(value)
	}
	if value, ok := body["offset"].(float64); ok {
		offset = int(value)
	}

	if offset >= len(items) {
		return nil
	}

	end := offset + limit
	if end > len(items) {
		end = len(items)
	}

	return items[offset:end]
}

// anyID digs the single id out of a filter body such as
// {"flows": {"id": {"any_": ["..."]}}}.
func anyID(body map[string]any, section, field string) string {
	sectionValue, ok := body[section].(map[string]any)
	if !ok {
		return ""
	}
	fieldValue, ok := sectionValue[field].(map[string]any)
	if !ok {
		return ""
	}
	ids, ok := fieldValue["any_"].([]any)
	if !ok || len(ids) == 0 {
		return ""
	}
	id, _ := ids[0].(string)

	return id
}

// The ids below are the real ones from the captured run, kept so the
// fixtures stay cross-referenced exactly as the server returned them.
const (
	nightlyFlowID = "70fe693d-da43-4c8b-9435-64a7c8a5ac6b"
	brokenFlowID  = "065a03cb-119d-45ad-92f0-9952a02a80b6"
	nightlyRunID  = "3f4e4829-6775-4e10-bfc4-d76b0639e113"
	brokenRunID   = "1feab620-6670-444e-9bdd-12491359d6a9"
)

const flowsFixture = `[
  {"id":"065a03cb-119d-45ad-92f0-9952a02a80b6","created":"2026-09-08T07:40:30.437912Z","updated":"2026-09-08T07:40:30.437918Z","name":"broken-report","tags":[],"labels":{}},
  {"id":"70fe693d-da43-4c8b-9435-64a7c8a5ac6b","created":"2026-09-08T07:40:29.369625Z","updated":"2026-09-08T07:40:29.369629Z","name":"nightly-etl","tags":[],"labels":{}}
]`

const nightlyDeploymentsFixture = `[
  {
    "id":"911349bb-e15b-42cf-8189-242181806401",
    "created":"2026-09-08T07:40:52.169405Z",
    "updated":"2026-09-08T07:40:52.168219Z",
    "name":"nightly",
    "version":"c1a71d57257c70ebb02b85d760fe460c",
    "description":"Nightly ETL that refreshes the order rollups.",
    "flow_id":"70fe693d-da43-4c8b-9435-64a7c8a5ac6b",
    "paused":false,
    "schedules":[
      {
        "id":"6402f0fb-cb37-46ca-8b87-ddb20e96e54e",
        "deployment_id":"911349bb-e15b-42cf-8189-242181806401",
        "schedule":{"cron":"0 2 * * *","timezone":null,"day_or":true},
        "active":true,
        "max_scheduled_runs":null,
        "parameters":{}
      }
    ],
    "job_variables":{},
    "parameters":{},
    "tags":["etl","om-source:shop.public.orders"],
    "labels":{"prefect.flow.id":"70fe693d-da43-4c8b-9435-64a7c8a5ac6b"},
    "work_queue_name":null,
    "path":".",
    "entrypoint":"flows.py:nightly_etl",
    "work_pool_name":null,
    "status":"NOT_READY",
    "enforce_parameter_schema":true
  }
]`

const brokenDeploymentsFixture = `[
  {
    "id":"6d223afb-a643-4698-9112-2f720cad3702",
    "created":"2026-09-08T07:40:52.222000Z",
    "updated":"2026-09-08T07:40:52.222000Z",
    "name":"hourly",
    "version":"c1a71d57257c70ebb02b85d760fe460c",
    "description":"Hourly report build.",
    "flow_id":"065a03cb-119d-45ad-92f0-9952a02a80b6",
    "paused":false,
    "schedules":[
      {
        "id":"81840d78-2300-484a-b909-26fd2d1d45eb",
        "deployment_id":"6d223afb-a643-4698-9112-2f720cad3702",
        "schedule":{"interval":3600.0,"anchor_date":"2026-09-08T07:40:52.196729Z","timezone":"UTC"},
        "active":true,
        "parameters":{}
      }
    ],
    "parameters":{},
    "tags":["reporting"],
    "labels":{"prefect.flow.id":"065a03cb-119d-45ad-92f0-9952a02a80b6"},
    "work_queue_name":null,
    "path":".",
    "entrypoint":"flows.py:broken_report",
    "work_pool_name":null,
    "status":"NOT_READY"
  }
]`

const nightlyRunsFixture = `[
  {
    "id":"3f4e4829-6775-4e10-bfc4-d76b0639e113",
    "created":"2026-09-08T07:40:29.376126Z",
    "name":"papaya-mayfly",
    "flow_id":"70fe693d-da43-4c8b-9435-64a7c8a5ac6b",
    "deployment_id":"911349bb-e15b-42cf-8189-242181806401",
    "work_queue_name":null,
    "flow_version":"c1a71d57257c70ebb02b85d760fe460c",
    "parameters":{},
    "tags":[],
    "state_type":"COMPLETED",
    "state_name":"Completed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:29.375976Z",
    "start_time":"2026-09-08T07:40:29.400518Z",
    "end_time":"2026-09-08T07:40:29.437435Z",
    "total_run_time":0.036917,
    "state":{
      "id":"01a07ff6-177d-79c3-9c92-87d987354ca2",
      "type":"COMPLETED",
      "name":"Completed",
      "timestamp":"2026-09-08T07:40:29.437435Z",
      "message":null
    }
  }
]`

const brokenRunsFixture = `[
  {
    "id":"1feab620-6670-444e-9bdd-12491359d6a9",
    "created":"2026-09-08T07:40:30.443000Z",
    "name":"discreet-whale",
    "flow_id":"065a03cb-119d-45ad-92f0-9952a02a80b6",
    "deployment_id":null,
    "parameters":{},
    "tags":[],
    "state_type":"FAILED",
    "state_name":"Failed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:30.443000Z",
    "start_time":"2026-09-08T07:40:30.462866Z",
    "end_time":"2026-09-08T07:40:31.485426Z",
    "total_run_time":1.02256,
    "state":{
      "type":"FAILED",
      "name":"Failed",
      "timestamp":"2026-09-08T07:40:31.485426Z",
      "message":"Flow run encountered an exception: ValueError: intentional failure"
    }
  }
]`

// The task runs of the nightly-etl run, newest first, exactly as the
// server returned them. load ran twice: once on transform's result and
// once on extract's, so it is one task with two runs and two upstreams.
const nightlyTaskRunsFixture = `[
  {
    "id":"01a07ff6-1773-7411-8ce8-3f0e054cf870",
    "name":"load-201",
    "flow_run_id":"3f4e4829-6775-4e10-bfc4-d76b0639e113",
    "task_key":"load-d6bfc39f",
    "dynamic_key":"201777f1-9a12-46b3-8f87-d853a325151e",
    "tags":[],
    "task_inputs":{"rows":[{"input_type":"task_run","id":"01a07ff6-176d-7a7e-92fd-1c1d1642aa99"}]},
    "state_type":"COMPLETED",
    "state_name":"Completed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:29.427323Z",
    "start_time":"2026-09-08T07:40:29.427497Z",
    "end_time":"2026-09-08T07:40:29.427924Z",
    "total_run_time":0.000427
  },
  {
    "id":"01a07ff6-1772-7bf6-9336-7a144d89dfba",
    "name":"load-080",
    "flow_run_id":"3f4e4829-6775-4e10-bfc4-d76b0639e113",
    "task_key":"load-d6bfc39f",
    "dynamic_key":"080682cd-192a-48c0-8a3f-749b37896599",
    "tags":[],
    "task_inputs":{"rows":[{"input_type":"task_run","id":"01a07ff6-1770-73fd-a8f5-3e69c8abad85"}]},
    "state_type":"COMPLETED",
    "state_name":"Completed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:29.426135Z",
    "start_time":"2026-09-08T07:40:29.426340Z",
    "end_time":"2026-09-08T07:40:29.426900Z",
    "total_run_time":0.00056
  },
  {
    "id":"01a07ff6-1770-73fd-a8f5-3e69c8abad85",
    "name":"transform-eef",
    "flow_run_id":"3f4e4829-6775-4e10-bfc4-d76b0639e113",
    "task_key":"transform-cb5df7f5",
    "dynamic_key":"eefea802-146b-4484-baec-4b0433d20f06",
    "tags":["shaping"],
    "task_inputs":{"rows":[{"input_type":"task_run","id":"01a07ff6-176d-7a7e-92fd-1c1d1642aa99"}]},
    "state_type":"COMPLETED",
    "state_name":"Completed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:29.424777Z",
    "start_time":"2026-09-08T07:40:29.425084Z",
    "end_time":"2026-09-08T07:40:29.425591Z",
    "total_run_time":0.000507
  },
  {
    "id":"01a07ff6-176d-7a7e-92fd-1c1d1642aa99",
    "name":"extract-ed7",
    "flow_run_id":"3f4e4829-6775-4e10-bfc4-d76b0639e113",
    "task_key":"extract-bf522387",
    "dynamic_key":"ed7f6a1c-3d8e-4a5b-9c2f-0e1d2a3b4c5d",
    "tags":[],
    "task_inputs":{},
    "state_type":"COMPLETED",
    "state_name":"Completed",
    "run_count":1,
    "expected_start_time":"2026-09-08T07:40:29.418000Z",
    "start_time":"2026-09-08T07:40:29.419000Z",
    "end_time":"2026-09-08T07:40:29.424000Z",
    "total_run_time":0.005
  }
]`

// Prefect Cloud's Assets API. A self-hosted server has no such endpoint,
// so this shape comes from Prefect's documented Assets responses and is
// exercised here rather than in the end to end test.
const materializationsFixture = `[
  {
    "asset_key":"postgres://warehouse.internal/shop/public/order_rollups",
    "upstream_assets":[
      "postgres://warehouse.internal/shop/public/orders",
      "s3://raw-events/orders/2026-09-08.json"
    ]
  }
]`
