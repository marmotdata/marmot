package cloudrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
	run "google.golang.org/api/run/v2"
)

// The fixtures below are the generated Cloud Run Admin API structs themselves,
// marshalled to JSON by the fake server. Building them that way means a test
// fixture cannot describe a field shape the real API never sends.

const (
	testProject = "acme"
	testRegion  = "europe-west1"
	testBucket  = "order-archive"
)

// fakeAPI serves the three list endpoints this plugin calls. Pages are handed
// out in order, one per request, so following page tokens can be tested.
type fakeAPI struct {
	servicePages []*run.GoogleCloudRunV2ListServicesResponse
	jobPages     []*run.GoogleCloudRunV2ListJobsResponse
	// executions is keyed by the bare job id.
	executions map[string]*run.GoogleCloudRunV2ListExecutionsResponse

	serviceCalls int
	jobCalls     int
	// parents records the parent of every services request, so the wildcard
	// location can be checked.
	parents []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		servicePages: []*run.GoogleCloudRunV2ListServicesResponse{
			{Services: []*run.GoogleCloudRunV2Service{checkoutService()}},
		},
		jobPages: []*run.GoogleCloudRunV2ListJobsResponse{
			{Jobs: []*run.GoogleCloudRunV2Job{nightlyExportJob()}},
		},
		executions: map[string]*run.GoogleCloudRunV2ListExecutionsResponse{
			"nightly-export": {Executions: []*run.GoogleCloudRunV2Execution{
				succeededExecution(),
				failedExecution(),
				cancelledExecution(),
			}},
		},
	}
}

// start runs the fake and returns its base URL.
func (f *fakeAPI) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(server.Close)
	return server.URL
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/executions"):
		jobID := resourceID(strings.TrimSuffix(path, "/executions"))
		response := f.executions[jobID]
		if response == nil {
			response = &run.GoogleCloudRunV2ListExecutionsResponse{}
		}
		writeJSON(w, response)

	case strings.HasSuffix(path, "/services"):
		f.parents = append(f.parents, strings.TrimSuffix(strings.TrimPrefix(path, "/v2/"), "/services"))
		writeJSON(w, page(f.servicePages, &f.serviceCalls, &run.GoogleCloudRunV2ListServicesResponse{}))

	case strings.HasSuffix(path, "/jobs"):
		writeJSON(w, page(f.jobPages, &f.jobCalls, &run.GoogleCloudRunV2ListJobsResponse{}))

	default:
		http.NotFound(w, r)
	}
}

// page returns the response for the next request, or an empty one once every
// prepared page has been served.
func page[T any](pages []*T, calls *int, empty *T) *T {
	index := *calls
	*calls++
	if index >= len(pages) {
		return empty
	}
	return pages[index]
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

// discover runs a full discovery against the fake and fails the test if it
// errors.
func discover(t *testing.T, fake *fakeAPI, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := pluginsdk.RawConfig{
		"project_id":   testProject,
		"endpoint":     fake.start(t),
		"disable_auth": true,
	}
	for key, value := range overrides {
		config[key] = value
	}

	result, err := (&Source{}).Discover(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	t.Fatalf("no asset named %q in %d assets", name, len(result.Assets))
	return pluginsdk.Asset{}
}

func serviceName(location, id string) string {
	return "projects/" + testProject + "/locations/" + location + "/services/" + id
}

func jobName(location, id string) string {
	return "projects/" + testProject + "/locations/" + location + "/jobs/" + id
}

// checkoutService is a service with the shape the plugin cares most about: a
// GCS volume, a Cloud SQL volume, a secret volume, a split traffic
// configuration and environment variables whose values must never be stored.
func checkoutService() *run.GoogleCloudRunV2Service {
	return &run.GoogleCloudRunV2Service{
		Name:                  serviceName(testRegion, "checkout-api"),
		Uid:                   "3fdbba8e-6a51-4f18-9b0a-1b6b1f1a2c3d",
		Generation:            7,
		Description:           "Takes checkout requests from the storefront",
		Uri:                   "https://checkout-api-abcdef-ew.a.run.app",
		Ingress:               "INGRESS_TRAFFIC_ALL",
		LaunchStage:           "GA",
		Creator:               "deployer@acme.iam.gserviceaccount.com",
		LastModifier:          "release-bot@acme.iam.gserviceaccount.com",
		CreateTime:            "2026-01-04T09:12:33.421Z",
		UpdateTime:            "2026-09-01T18:02:11.008Z",
		LatestReadyRevision:   serviceName(testRegion, "checkout-api") + "/revisions/checkout-api-00042-abc",
		LatestCreatedRevision: serviceName(testRegion, "checkout-api") + "/revisions/checkout-api-00043-def",
		Labels:                map[string]string{"team": "payments", "env": "prod"},
		Reconciling:           true,
		TerminalCondition:     &run.GoogleCloudRunV2Condition{Type: "Ready", State: "CONDITION_SUCCEEDED"},
		TrafficStatuses: []*run.GoogleCloudRunV2TrafficTargetStatus{
			{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION", Revision: serviceName(testRegion, "checkout-api") + "/revisions/checkout-api-00042-abc", Percent: 90},
			{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION", Revision: serviceName(testRegion, "checkout-api") + "/revisions/checkout-api-00043-def", Percent: 10},
		},
		Template: &run.GoogleCloudRunV2RevisionTemplate{
			Revision:                      "checkout-api-00043-def",
			ExecutionEnvironment:          "EXECUTION_ENVIRONMENT_GEN2",
			ServiceAccount:                "checkout-api@acme.iam.gserviceaccount.com",
			Timeout:                       "300s",
			MaxInstanceRequestConcurrency: 80,
			Scaling:                       &run.GoogleCloudRunV2RevisionScaling{MinInstanceCount: 1, MaxInstanceCount: 25},
			VpcAccess: &run.GoogleCloudRunV2VpcAccess{
				Connector: "projects/acme/locations/europe-west1/connectors/serverless-egress",
				Egress:    "PRIVATE_RANGES_ONLY",
			},
			Containers: []*run.GoogleCloudRunV2Container{{
				Name:  "app",
				Image: "europe-docker.pkg.dev/acme/services/checkout-api:2.11.0",
				Ports: []*run.GoogleCloudRunV2ContainerPort{{Name: "http1", ContainerPort: 8080}},
				Env: []*run.GoogleCloudRunV2EnvVar{
					{Name: "DATABASE_PASSWORD", Value: "hunter2"},
					{Name: "LOG_LEVEL", Value: "info"},
				},
			}},
			Volumes: []*run.GoogleCloudRunV2Volume{
				{Name: "archive", Gcs: &run.GoogleCloudRunV2GCSVolumeSource{Bucket: testBucket, ReadOnly: true}},
				{Name: "archive-again", Gcs: &run.GoogleCloudRunV2GCSVolumeSource{Bucket: testBucket}},
				{Name: "sql", CloudSqlInstance: &run.GoogleCloudRunV2CloudSqlInstance{Instances: []string{"acme:europe-west1:orders-db"}}},
				{Name: "creds", Secret: &run.GoogleCloudRunV2SecretVolumeSource{Secret: "checkout-signing-key"}},
				{Name: "shared", Nfs: &run.GoogleCloudRunV2NFSVolumeSource{Server: "10.0.0.4", Path: "/exports/shared"}},
			},
		},
	}
}

// nightlyExportJob is a job with the double-nested template Cloud Run uses:
// Job.Template is an execution template whose Template is the task template
// that actually holds the containers and volumes.
func nightlyExportJob() *run.GoogleCloudRunV2Job {
	return &run.GoogleCloudRunV2Job{
		Name:           jobName(testRegion, "nightly-export"),
		Uid:            "9c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f",
		Generation:     3,
		Creator:        "deployer@acme.iam.gserviceaccount.com",
		LastModifier:   "deployer@acme.iam.gserviceaccount.com",
		CreateTime:     "2026-02-11T07:00:00Z",
		UpdateTime:     "2026-08-30T07:00:00Z",
		LaunchStage:    "GA",
		ExecutionCount: 3,
		Labels:         map[string]string{"team": "analytics"},
		LatestCreatedExecution: &run.GoogleCloudRunV2ExecutionReference{
			Name:             jobName(testRegion, "nightly-export") + "/executions/nightly-export-x9k2t",
			CompletionStatus: "EXECUTION_SUCCEEDED",
		},
		Template: &run.GoogleCloudRunV2ExecutionTemplate{
			TaskCount:   4,
			Parallelism: 2,
			Template: &run.GoogleCloudRunV2TaskTemplate{
				ServiceAccount:       "nightly-export@acme.iam.gserviceaccount.com",
				Timeout:              "3600s",
				MaxRetries:           3,
				ExecutionEnvironment: "EXECUTION_ENVIRONMENT_GEN2",
				Containers: []*run.GoogleCloudRunV2Container{{
					Name:  "exporter",
					Image: "europe-docker.pkg.dev/acme/jobs/nightly-export:1.4.2",
					Env:   []*run.GoogleCloudRunV2EnvVar{{Name: "EXPORT_TOKEN", Value: "s3cr3t"}},
				}},
				Volumes: []*run.GoogleCloudRunV2Volume{
					{Name: "archive", Gcs: &run.GoogleCloudRunV2GCSVolumeSource{Bucket: testBucket}},
				},
			},
		},
	}
}

func executionName(id string) string {
	return jobName(testRegion, "nightly-export") + "/executions/" + id
}

func succeededExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:           executionName("nightly-export-x9k2t"),
		Job:            jobName(testRegion, "nightly-export"),
		StartTime:      "2026-08-30T07:00:04Z",
		CompletionTime: "2026-08-30T07:11:48Z",
		TaskCount:      4,
		SucceededCount: 4,
		LogUri:         "https://console.cloud.google.com/logs/viewer?project=acme",
	}
}

func failedExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:           executionName("nightly-export-b4m1p"),
		Job:            jobName(testRegion, "nightly-export"),
		StartTime:      "2026-08-29T07:00:03Z",
		CompletionTime: "2026-08-29T07:04:19Z",
		TaskCount:      4,
		SucceededCount: 3,
		FailedCount:    1,
		RetriedCount:   2,
	}
}

func cancelledExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:           executionName("nightly-export-q7w3z"),
		Job:            jobName(testRegion, "nightly-export"),
		StartTime:      "2026-08-28T07:00:02Z",
		CompletionTime: "2026-08-28T07:02:55Z",
		TaskCount:      4,
		SucceededCount: 1,
		CancelledCount: 3,
	}
}

func runningExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:         executionName("nightly-export-r1n2g"),
		Job:          jobName(testRegion, "nightly-export"),
		StartTime:    "2026-09-01T07:00:01Z",
		TaskCount:    4,
		RunningCount: 4,
	}
}
