package vertexai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/aiplatform/v1"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, driving the real aiplatform client
// library and the real HTTP transport. Google has no Vertex AI emulator,
// so the API on the other end is served by this test:
//
//	MARMOT_TEST_VERTEXAI_ENDPOINT=http://127.0.0.1:18821 go test ./...
//
// Set the variable to an address the test may listen on. It is the same
// address the plugin is pointed at.
const endpointEnv = "MARMOT_TEST_VERTEXAI_ENDPOINT"

const (
	project     = "acme-ml"
	location    = "us-central1"
	bucket      = "marmot-vertexai-artifacts"
	table       = "customer_events"
	modelID     = "1000000000000000001"
	endpointID  = "4000000000000000004"
	datasetID   = "5000000000000000005"
	jobID       = "8000000000000000008"
	featureName = "customer_features"
)

func requireEndpoint(t *testing.T) string {
	t.Helper()

	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to an address to serve the Vertex AI API on, for example http://127.0.0.1:18821, to run the end to end tests", endpointEnv)
	}

	return endpoint
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func testConfig(endpoint string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"project_id":            project,
		"locations":             []string{location},
		"endpoint":              endpoint,
		"disable_auth":          true,
		"include_pipeline_jobs": true,
	}
}

var (
	discoverOnce sync.Once
	discovered   *pluginsdk.DiscoveryResult
	discoverErr  error
)

// discoverSeeded serves the API once and returns the plugin's discovery of
// it. Every test looks at the same project, and each Discover call starts
// a fresh plugin process.
func discoverSeeded(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	endpoint := requireEndpoint(t)
	bin := buildBinary(t)

	discoverOnce.Do(func() {
		server, err := serveAPI(endpoint)
		if err != nil {
			discoverErr = err
			return
		}
		defer server.Close()

		discovered, discoverErr = bin.Discover(context.Background(), testConfig(endpoint))
	})

	require.NoError(t, discoverErr)
	require.NotNil(t, discovered)

	return discovered
}

// serveAPI listens on the address the test was given and answers the list
// calls the plugin makes. Responses are built by marshalling the generated
// API structs, so they cannot drift from the API the plugin reads.
func serveAPI(endpoint string) (*http.Server, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", endpointEnv, err)
	}

	listener, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", parsed.Host, err)
	}

	server := &http.Server{Handler: http.HandlerFunc(serve)}
	go server.Serve(listener)

	return server, nil
}

func serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	cut := strings.LastIndex(path, "/")
	if cut < 0 {
		http.NotFound(w, r)
		return
	}

	var body any
	switch path[cut+1:] {
	case "models":
		body = &aiplatform.GoogleCloudAiplatformV1ListModelsResponse{
			Models: []*aiplatform.GoogleCloudAiplatformV1Model{seedModel()},
		}
	case "endpoints":
		body = &aiplatform.GoogleCloudAiplatformV1ListEndpointsResponse{
			Endpoints: []*aiplatform.GoogleCloudAiplatformV1Endpoint{seedEndpoint()},
		}
	case "datasets":
		body = &aiplatform.GoogleCloudAiplatformV1ListDatasetsResponse{
			Datasets: []*aiplatform.GoogleCloudAiplatformV1Dataset{seedDataset()},
		}
	case "featureGroups":
		body = &aiplatform.GoogleCloudAiplatformV1ListFeatureGroupsResponse{
			FeatureGroups: []*aiplatform.GoogleCloudAiplatformV1FeatureGroup{seedFeatureGroup()},
		}
	case "features":
		body = &aiplatform.GoogleCloudAiplatformV1ListFeaturesResponse{Features: seedFeatures()}
	case "pipelineJobs":
		body = &aiplatform.GoogleCloudAiplatformV1ListPipelineJobsResponse{
			PipelineJobs: []*aiplatform.GoogleCloudAiplatformV1PipelineJob{seedPipelineJob()},
		}
	default:
		http.NotFound(w, r)
		return
	}

	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func resourceName(collection, id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/%s/%s", project, location, collection, id)
}

func seedModel() *aiplatform.GoogleCloudAiplatformV1Model {
	return &aiplatform.GoogleCloudAiplatformV1Model{
		Name:        resourceName("models", modelID),
		DisplayName: "churn-predictor",
		Description: "Predicts subscription churn",
		ArtifactUri: "gs://" + bucket + "/models/churn/",
		ContainerSpec: &aiplatform.GoogleCloudAiplatformV1ModelContainerSpec{
			ImageUri: "us-docker.pkg.dev/vertex-ai/prediction/sklearn-cpu.1-5:latest",
		},
		PipelineJob: resourceName("pipelineJobs", jobID),
		VersionId:   "1",
		CreateTime:  "2026-08-01T10:00:00Z",
		Labels:      map[string]string{"team": "growth"},
	}
}

func seedEndpoint() *aiplatform.GoogleCloudAiplatformV1Endpoint {
	return &aiplatform.GoogleCloudAiplatformV1Endpoint{
		Name:         resourceName("endpoints", endpointID),
		DisplayName:  "churn-prod",
		TrafficSplit: map[string]int64{"deployed-1": 100},
		DeployedModels: []*aiplatform.GoogleCloudAiplatformV1DeployedModel{
			{Id: "deployed-1", DisplayName: "churn-predictor", Model: resourceName("models", modelID)},
		},
		CreateTime: "2026-08-03T08:00:00Z",
	}
}

func seedDataset() *aiplatform.GoogleCloudAiplatformV1Dataset {
	return &aiplatform.GoogleCloudAiplatformV1Dataset{
		Name:              resourceName("datasets", datasetID),
		DisplayName:       "churn-training-data",
		MetadataSchemaUri: "gs://google-cloud-aiplatform/schema/dataset/metadata/tabular_1.0.0.yaml",
		DataItemCount:     125000,
		Metadata: map[string]any{
			"inputConfig": map[string]any{
				"type": "bigquery_source",
				"uri":  "bq://" + project + ".analytics." + table,
			},
		},
		CreateTime: "2026-06-01T10:00:00Z",
	}
}

func seedFeatureGroup() *aiplatform.GoogleCloudAiplatformV1FeatureGroup {
	return &aiplatform.GoogleCloudAiplatformV1FeatureGroup{
		Name:        resourceName("featureGroups", featureName),
		Description: "Customer features for churn",
		BigQuery: &aiplatform.GoogleCloudAiplatformV1FeatureGroupBigQuery{
			BigQuerySource: &aiplatform.GoogleCloudAiplatformV1BigQuerySource{
				InputUri: "bq://" + project + ".features." + table,
			},
			EntityIdColumns: []string{"customer_id"},
		},
		CreateTime: "2026-05-01T10:00:00Z",
	}
}

func seedFeatures() []*aiplatform.GoogleCloudAiplatformV1Feature {
	group := resourceName("featureGroups", featureName)
	return []*aiplatform.GoogleCloudAiplatformV1Feature{
		{Name: group + "/features/plan_tier", ValueType: "STRING", Description: "Subscription tier"},
		{Name: group + "/features/age", ValueType: "INT64"},
	}
}

func seedPipelineJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:        resourceName("pipelineJobs", jobID),
		DisplayName: "churn-training",
		State:       "PIPELINE_STATE_SUCCEEDED",
		StartTime:   "2026-08-01T09:00:00Z",
		EndTime:     "2026-08-01T09:45:00Z",
		CreateTime:  "2026-08-01T08:59:00Z",
	}
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	t.Fatalf("no %s asset named %q in %d discovered assets", assetType, name, len(result.Assets))
	return pluginsdk.Asset{}
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "vertexai", meta.ID)
	assert.Equal(t, "Vertex AI", meta.Name)
	assert.Equal(t, "ml", meta.Category)
	assert.Equal(t, "vertex-ai", meta.Icon)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateRejectsAConfigWithoutAProject(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(context.Background(), pluginsdk.RawConfig{"locations": []string{location}})

	require.Error(t, err)
}

func TestE2E_ValidateRejectsAConfigWithoutLocations(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(context.Background(), pluginsdk.RawConfig{"project_id": project})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsACompleteConfig(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(context.Background(), pluginsdk.RawConfig{
		"project_id": project,
		"locations":  []string{location},
	})

	require.NoError(t, err)
}

func TestE2E_DiscoversTheModel(t *testing.T) {
	result := discoverSeeded(t)

	model := assetNamed(t, result, "Model", "churn-predictor")

	assert.Equal(t, []string{"Vertex AI"}, model.Providers)
	assert.Equal(t, "mrn://model/vertex-ai/churn-predictor", *model.MRN)
	assert.Equal(t, modelID, model.Metadata["resource_id"])
	assert.Equal(t, location, model.Metadata["location"])
	assert.Equal(t, "us-docker.pkg.dev/vertex-ai/prediction/sklearn-cpu.1-5:latest", model.Metadata["container_image"])
	assert.Equal(t, "growth", model.Metadata["label_team"])
}

func TestE2E_DiscoversTheEndpoint(t *testing.T) {
	result := discoverSeeded(t)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "mrn://endpoint/vertex-ai/churn-prod", *endpoint.MRN)
	assert.Equal(t, "deployed-1=100", endpoint.Metadata["traffic_split"])
	assert.Equal(t, "churn-predictor", endpoint.Metadata["deployed_models"])
}

func TestE2E_LinksTheModelToTheEndpoint(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://model/vertex-ai/churn-predictor",
		"mrn://endpoint/vertex-ai/churn-prod",
		"FEEDS"))
}

func TestE2E_LinksTheArtifactBucketToTheModel(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://bucket/gcs/"+bucket,
		"mrn://model/vertex-ai/churn-predictor",
		"FEEDS"))
}

func TestE2E_LinksThePipelineJobToTheModelItProduced(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://job/vertex-ai/churn-training",
		"mrn://model/vertex-ai/churn-predictor",
		"PRODUCES"))
}

func TestE2E_DiscoversTheDatasetAndItsItemCount(t *testing.T) {
	result := discoverSeeded(t)

	dataset := assetNamed(t, result, "Dataset", "churn-training-data")

	assert.Equal(t, "tabular_1.0.0", dataset.Metadata["dataset_kind"])

	var value float64
	found := false
	for _, statistic := range result.Statistics {
		if statistic.AssetMRN == *dataset.MRN && statistic.MetricName == "asset.data_item_count" {
			value, found = statistic.Value, true
		}
	}
	require.True(t, found, "expected a data item count statistic")
	assert.Equal(t, float64(125000), value)
}

func TestE2E_LinksTheBigQueryTableToTheDataset(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://table/bigquery/"+table,
		"mrn://dataset/vertex-ai/churn-training-data",
		"FEEDS"))
}

func TestE2E_DiscoversTheFeatureGroupAndItsColumns(t *testing.T) {
	result := discoverSeeded(t)

	group := assetNamed(t, result, "Dataset", featureName)

	// Metadata crosses the plugin wire as JSON, so a count arrives as a
	// float, which is how the Marmot host sees it too.
	assert.Equal(t, float64(2), group.Metadata["feature_count"])

	columns, ok := group.Schema["columns"]
	require.True(t, ok, "expected a column list")
	assert.Contains(t, columns, "plan_tier")
	assert.Contains(t, columns, "int64")
}

func TestE2E_LinksTheBigQueryTableToTheFeatureGroup(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://table/bigquery/"+table,
		"mrn://dataset/vertex-ai/"+featureName,
		"FEEDS"))
}

func TestE2E_EmitsRunHistoryForThePipelineJob(t *testing.T) {
	result := discoverSeeded(t)

	var history *pluginsdk.AssetRunHistory
	for i := range result.RunHistory {
		if result.RunHistory[i].AssetMRN == "mrn://job/vertex-ai/churn-training" {
			history = &result.RunHistory[i]
		}
	}
	require.NotNil(t, history, "expected run history for the pipeline job")

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
	assert.Equal(t, jobID, history.Runs[0].RunID)
	assert.Equal(t, project, history.Runs[0].JobNamespace)
}

func TestE2E_EveryAssetMRNMatchesTheServersDerivation(t *testing.T) {
	result := discoverSeeded(t)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)

		assert.Equal(t, strings.ToLower("mrn://"+asset.Type+"/Vertex AI/"+*asset.Name), *asset.MRN)
	}
}
