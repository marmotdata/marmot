package cloudrun_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	run "google.golang.org/api/run/v2"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses: plugintest.Build compiles the main package
// and every call spawns the process, runs one RPC and kills it again.
//
// Google publishes no Cloud Run emulator, so the plugin is pointed at an HTTP
// server started here that answers the three Cloud Run Admin API list calls
// with the generated response structs. That exercises the real client library,
// the real HTTP transport and the real plugin process; the only part it does
// not exercise is Google's own server.
//
//	MARMOT_TEST_CLOUDRUN_ENDPOINT=http://127.0.0.1:18801 go test ./...

const (
	e2eProject = "marmot-e2e"
	e2eRegion  = "europe-west1"
	e2eBucket  = "marmot-e2e-archive"
)

func e2eEndpoint(t *testing.T) string {
	t.Helper()

	endpoint := os.Getenv("MARMOT_TEST_CLOUDRUN_ENDPOINT")
	if endpoint == "" {
		t.Skip("set MARMOT_TEST_CLOUDRUN_ENDPOINT to the address the test Cloud Run API server should listen on, for example http://127.0.0.1:18801")
	}
	return endpoint
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// serveAPI binds the Cloud Run Admin API responses to the host and port in
// endpoint. The plugin runs as its own process, so an in-memory handler would
// be unreachable: it has to be a real listening socket.
func serveAPI(t *testing.T, endpoint string) {
	t.Helper()

	parsed, err := url.Parse(endpoint)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", parsed.Host)
	require.NoError(t, err, "port %s is already in use", parsed.Host)

	server := &http.Server{Handler: http.HandlerFunc(handleAPI)}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")

	var body any
	switch {
	case strings.HasSuffix(path, "/executions"):
		body = &run.GoogleCloudRunV2ListExecutionsResponse{
			Executions: []*run.GoogleCloudRunV2Execution{e2eSucceededExecution(), e2eRunningExecution()},
		}
	case strings.HasSuffix(path, "/services"):
		body = &run.GoogleCloudRunV2ListServicesResponse{
			Services:    []*run.GoogleCloudRunV2Service{e2eService()},
			Unreachable: []string{"projects/" + e2eProject + "/locations/asia-south2"},
		}
	case strings.HasSuffix(path, "/jobs"):
		body = &run.GoogleCloudRunV2ListJobsResponse{Jobs: []*run.GoogleCloudRunV2Job{e2eJob()}}
	default:
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func e2eServiceName() string {
	return "projects/" + e2eProject + "/locations/" + e2eRegion + "/services/checkout-api"
}

func e2eJobName() string {
	return "projects/" + e2eProject + "/locations/" + e2eRegion + "/jobs/nightly-export"
}

func e2eService() *run.GoogleCloudRunV2Service {
	return &run.GoogleCloudRunV2Service{
		Name:                e2eServiceName(),
		Uid:                 "e2e-service-uid",
		Generation:          4,
		Description:         "Takes checkout requests from the storefront",
		Uri:                 "https://checkout-api-e2e-ew.a.run.app",
		Ingress:             "INGRESS_TRAFFIC_ALL",
		CreateTime:          "2026-01-04T09:12:33Z",
		UpdateTime:          "2026-09-01T18:02:11Z",
		LatestReadyRevision: e2eServiceName() + "/revisions/checkout-api-00042-abc",
		Labels:              map[string]string{"team": "payments"},
		TerminalCondition:   &run.GoogleCloudRunV2Condition{Type: "Ready", State: "CONDITION_SUCCEEDED"},
		TrafficStatuses: []*run.GoogleCloudRunV2TrafficTargetStatus{
			{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", Percent: 100},
		},
		Template: &run.GoogleCloudRunV2RevisionTemplate{
			ServiceAccount:                "checkout-api@marmot-e2e.iam.gserviceaccount.com",
			Timeout:                       "300s",
			MaxInstanceRequestConcurrency: 80,
			Scaling:                       &run.GoogleCloudRunV2RevisionScaling{MinInstanceCount: 1, MaxInstanceCount: 10},
			Containers: []*run.GoogleCloudRunV2Container{{
				Name:  "app",
				Image: "europe-docker.pkg.dev/marmot-e2e/services/checkout-api:2.11.0",
				Ports: []*run.GoogleCloudRunV2ContainerPort{{Name: "http1", ContainerPort: 8080}},
				Env:   []*run.GoogleCloudRunV2EnvVar{{Name: "DATABASE_PASSWORD", Value: "hunter2"}},
			}},
			Volumes: []*run.GoogleCloudRunV2Volume{
				{Name: "archive", Gcs: &run.GoogleCloudRunV2GCSVolumeSource{Bucket: e2eBucket}},
				{Name: "sql", CloudSqlInstance: &run.GoogleCloudRunV2CloudSqlInstance{Instances: []string{"marmot-e2e:europe-west1:orders-db"}}},
			},
		},
	}
}

func e2eJob() *run.GoogleCloudRunV2Job {
	return &run.GoogleCloudRunV2Job{
		Name:           e2eJobName(),
		Uid:            "e2e-job-uid",
		ExecutionCount: 2,
		CreateTime:     "2026-02-11T07:00:00Z",
		Labels:         map[string]string{"team": "analytics"},
		LatestCreatedExecution: &run.GoogleCloudRunV2ExecutionReference{
			Name: e2eJobName() + "/executions/nightly-export-x9k2t",
		},
		Template: &run.GoogleCloudRunV2ExecutionTemplate{
			TaskCount:   4,
			Parallelism: 2,
			Template: &run.GoogleCloudRunV2TaskTemplate{
				ServiceAccount: "nightly-export@marmot-e2e.iam.gserviceaccount.com",
				MaxRetries:     3,
				Containers: []*run.GoogleCloudRunV2Container{{
					Image: "europe-docker.pkg.dev/marmot-e2e/jobs/nightly-export:1.4.2",
				}},
				Volumes: []*run.GoogleCloudRunV2Volume{
					{Name: "archive", Gcs: &run.GoogleCloudRunV2GCSVolumeSource{Bucket: e2eBucket}},
				},
			},
		},
	}
}

func e2eSucceededExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:           e2eJobName() + "/executions/nightly-export-x9k2t",
		Job:            e2eJobName(),
		StartTime:      "2026-08-30T07:00:04Z",
		CompletionTime: "2026-08-30T07:11:48Z",
		TaskCount:      4,
		SucceededCount: 4,
		LogUri:         "https://console.cloud.google.com/logs/viewer?project=marmot-e2e",
	}
}

func e2eRunningExecution() *run.GoogleCloudRunV2Execution {
	return &run.GoogleCloudRunV2Execution{
		Name:      e2eJobName() + "/executions/nightly-export-r1n2g",
		Job:       e2eJobName(),
		StartTime: "2026-09-01T07:00:01Z",
		TaskCount: 4,
	}
}

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	endpoint := e2eEndpoint(t)
	serveAPI(t, endpoint)

	return pluginsdk.RawConfig{
		"project_id":   e2eProject,
		"endpoint":     endpoint,
		"disable_auth": true,
	}
}

func e2eDiscover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := buildBinary(t).Discover(t.Context(), e2eConfig(t))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func e2eAssetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	t.Fatalf("no asset named %q in %d assets", name, len(result.Assets))
	return pluginsdk.Asset{}
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "cloudrun", meta.ID)
	assert.Equal(t, "Google Cloud Run", meta.Name)
	assert.Equal(t, "container", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestE2E_ValidateMissingProjectIDFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAProjectID(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"project_id": e2eProject})
	require.NoError(t, err)
}

func TestE2E_DiscoverReturnsTheServiceAndTheJob(t *testing.T) {
	result := e2eDiscover(t)

	service := e2eAssetNamed(t, result, "europe-west1/checkout-api")
	assert.Equal(t, "Service", service.Type)
	assert.Equal(t, []string{"Cloud Run"}, service.Providers)
	require.NotNil(t, service.MRN)
	assert.Equal(t, "mrn://service/cloud-run/europe-west1-checkout-api", *service.MRN)

	job := e2eAssetNamed(t, result, "europe-west1/nightly-export")
	assert.Equal(t, "Job", job.Type)
	require.NotNil(t, job.MRN)
	assert.Equal(t, "mrn://job/cloud-run/europe-west1-nightly-export", *job.MRN)
}

func TestE2E_DiscoverCarriesServiceMetadataOverTheWire(t *testing.T) {
	result := e2eDiscover(t)

	metadata := e2eAssetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "europe-west1", metadata["location"])
	assert.Equal(t, "marmot-e2e", metadata["project_id"])
	assert.Equal(t, "https://checkout-api-e2e-ew.a.run.app", metadata["uri"])
	assert.Equal(t, "checkout-api-00042-abc", metadata["latest_ready_revision"])
	assert.Equal(t, "latest=100", metadata["traffic"])
	assert.Equal(t, "CONDITION_SUCCEEDED", metadata["ready"])
	assert.Equal(t, "payments", metadata["label_team"])
	assert.Equal(t, "europe-docker.pkg.dev/marmot-e2e/services/checkout-api:2.11.0", metadata["container_image"])
	// JSON is the wire format, so numbers arrive back as float64.
	assert.EqualValues(t, 80, metadata["max_instance_request_concurrency"])
}

func TestE2E_DiscoverNeverCarriesAnEnvVarValue(t *testing.T) {
	result := e2eDiscover(t)

	asset := e2eAssetNamed(t, result, "europe-west1/checkout-api")
	assert.Equal(t, []any{"DATABASE_PASSWORD"}, asset.Metadata["env_var_names"])

	encoded, err := json.Marshal(asset)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "hunter2")
}

func TestE2E_DiscoverEmitsTheBucketLineageEdges(t *testing.T) {
	result := e2eDiscover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://bucket/gcs/marmot-e2e-archive",
		Target: "mrn://service/cloud-run/europe-west1-checkout-api",
		Type:   "FEEDS",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://bucket/gcs/marmot-e2e-archive",
		Target: "mrn://job/cloud-run/europe-west1-nightly-export",
		Type:   "FEEDS",
	})
}

func TestE2E_DiscoverEmitsNoCloudSQLEdge(t *testing.T) {
	result := e2eDiscover(t)

	metadata := e2eAssetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, []any{"marmot-e2e:europe-west1:orders-db"}, metadata["cloud_sql_instances"])
	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "orders-db")
	}
}

func TestE2E_DiscoverEmitsTheJobExecutionCount(t *testing.T) {
	result := e2eDiscover(t)

	require.Len(t, result.Statistics, 1)
	assert.Equal(t, "mrn://job/cloud-run/europe-west1-nightly-export", result.Statistics[0].AssetMRN)
	assert.Equal(t, "asset.execution_count", result.Statistics[0].MetricName)
	assert.Equal(t, float64(2), result.Statistics[0].Value)
}

func TestE2E_DiscoverEmitsTheJobRunHistory(t *testing.T) {
	result := e2eDiscover(t)

	require.Len(t, result.RunHistory, 1)
	history := result.RunHistory[0]
	assert.Equal(t, "mrn://job/cloud-run/europe-west1-nightly-export", history.AssetMRN)

	byType := map[string]string{}
	for _, event := range history.Runs {
		byType[event.EventType] = event.RunID
		assert.Equal(t, "marmot-e2e", event.JobNamespace)
		assert.Equal(t, "europe-west1/nightly-export", event.JobName)
		assert.False(t, event.EventTime.IsZero())
	}

	assert.Equal(t, "nightly-export-x9k2t", byType["COMPLETE"])
	assert.Equal(t, "nightly-export-r1n2g", byType["RUNNING"])
}

func TestE2E_DiscoverSurvivesAnUnreachableLocation(t *testing.T) {
	// The fake reports asia-south2 as unreachable in every services response,
	// the same way Cloud Run does. Discovery has to keep what it did get.
	result := e2eDiscover(t)

	assert.Len(t, result.Assets, 2)
}
