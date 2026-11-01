package vertexai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/aiplatform/v1"
)

// The tests drive the plugin against a fake aiplatform API. Every response
// is built by marshalling the generated API structs, so a fixture cannot
// drift from the API the plugin reads.

const (
	testProject      = "acme-ml"
	testLocation     = "us-central1"
	emptyLocation    = "europe-west4"
	testBucket       = "marmot-vertexai-artifacts"
	testTable        = "customer_events"
	churnModelID     = "1000000000000000001"
	fraudModelIDOne  = "2000000000000000002"
	fraudModelIDTwo  = "3000000000000000003"
	churnEndpointID  = "4000000000000000004"
	tabularDatasetID = "5000000000000000005"
	imageDatasetID   = "6000000000000000006"
	opaqueDatasetID  = "7000000000000000007"
	trainJobID       = "8000000000000000008"
	failedJobID      = "9000000000000000009"
	runningJobID     = "1100000000000000011"
	cancelledJobID   = "1200000000000000012"
	queuedJobID      = "1300000000000000013"
)

func modelName(id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/models/%s", testProject, testLocation, id)
}

func endpointName(id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/endpoints/%s", testProject, testLocation, id)
}

func datasetName(id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/datasets/%s", testProject, testLocation, id)
}

func featureGroupName(id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/featureGroups/%s", testProject, testLocation, id)
}

func pipelineJobName(id string) string {
	return fmt.Sprintf("projects/%s/locations/%s/pipelineJobs/%s", testProject, testLocation, id)
}

// fake is an aiplatform API served over HTTP, keyed by location for the
// project-wide collections and by resource name for a feature group's
// features.
type fake struct {
	models        map[string][]*aiplatform.GoogleCloudAiplatformV1Model
	endpoints     map[string][]*aiplatform.GoogleCloudAiplatformV1Endpoint
	datasets      map[string][]*aiplatform.GoogleCloudAiplatformV1Dataset
	featureGroups map[string][]*aiplatform.GoogleCloudAiplatformV1FeatureGroup
	pipelineJobs  map[string][]*aiplatform.GoogleCloudAiplatformV1PipelineJob
	features      map[string][]*aiplatform.GoogleCloudAiplatformV1Feature
	// featureFailures are the feature groups whose feature list returns an
	// error, so one group failing can be told apart from all of them.
	featureFailures map[string]bool
	// modelPageSize splits the model list into pages when it is set.
	modelPageSize int
}

func (f *fake) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)

	return server
}

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	cut := strings.LastIndex(path, "/")
	if cut < 0 {
		http.NotFound(w, r)
		return
	}
	parent, collection := path[:cut], path[cut+1:]

	location := ""
	if parsed, ok := parseResourceName(parent + "/placeholder/1"); ok {
		location = parsed.location
	}

	switch collection {
	case "models":
		f.writeModels(w, r, location)
	case "endpoints":
		writeJSON(w, &aiplatform.GoogleCloudAiplatformV1ListEndpointsResponse{Endpoints: f.endpoints[location]})
	case "datasets":
		writeJSON(w, &aiplatform.GoogleCloudAiplatformV1ListDatasetsResponse{Datasets: f.datasets[location]})
	case "featureGroups":
		writeJSON(w, &aiplatform.GoogleCloudAiplatformV1ListFeatureGroupsResponse{FeatureGroups: f.featureGroups[location]})
	case "pipelineJobs":
		writeJSON(w, &aiplatform.GoogleCloudAiplatformV1ListPipelineJobsResponse{PipelineJobs: f.pipelineJobs[location]})
	case "features":
		if f.featureFailures[parent] {
			http.Error(w, `{"error":{"code":403,"message":"permission denied on feature group"}}`, http.StatusForbidden)
			return
		}
		writeJSON(w, &aiplatform.GoogleCloudAiplatformV1ListFeaturesResponse{Features: f.features[parent]})
	default:
		http.NotFound(w, r)
	}
}

// writeModels pages the model list when the fake is set up to, so the
// plugin's pagination is exercised against a token it did not choose.
func (f *fake) writeModels(w http.ResponseWriter, r *http.Request, location string) {
	models := f.models[location]

	size := f.modelPageSize
	if size <= 0 {
		size = len(models) + 1
	}

	start := 0
	if token := r.URL.Query().Get("pageToken"); token != "" {
		start, _ = strconv.Atoi(token)
	}
	end := min(start+size, len(models))

	response := &aiplatform.GoogleCloudAiplatformV1ListModelsResponse{Models: models[start:end]}
	if end < len(models) {
		response.NextPageToken = strconv.Itoa(end)
	}

	writeJSON(w, response)
}

func writeJSON(w http.ResponseWriter, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// fullFake is the project every test looks at unless it needs something
// narrower: two locations, one of them empty, models whose display names
// collide, an endpoint serving a model this run did not find, datasets in
// three different metadata shapes, a feature group whose features cannot
// be read, and a pipeline job in every state.
func fullFake() *fake {
	return &fake{
		modelPageSize: 2,
		models: map[string][]*aiplatform.GoogleCloudAiplatformV1Model{
			testLocation: {churnModel(), fraudModelOne(), fraudModelTwo()},
		},
		endpoints: map[string][]*aiplatform.GoogleCloudAiplatformV1Endpoint{
			testLocation: {churnEndpoint()},
		},
		datasets: map[string][]*aiplatform.GoogleCloudAiplatformV1Dataset{
			testLocation: {tabularDataset(), imageDataset(), opaqueDataset()},
		},
		featureGroups: map[string][]*aiplatform.GoogleCloudAiplatformV1FeatureGroup{
			testLocation: {customerFeatureGroup(), lockedFeatureGroup()},
		},
		pipelineJobs: map[string][]*aiplatform.GoogleCloudAiplatformV1PipelineJob{
			testLocation: {trainJob(), failedJob(), runningJob(), cancelledJob(), queuedJob()},
		},
		features: map[string][]*aiplatform.GoogleCloudAiplatformV1Feature{
			featureGroupName("customer_features"): {
				{
					Name:        featureGroupName("customer_features") + "/features/plan_tier",
					ValueType:   "STRING",
					Description: "Subscription tier",
				},
				{
					Name:      featureGroupName("customer_features") + "/features/age",
					ValueType: "INT64",
				},
			},
		},
		featureFailures: map[string]bool{
			featureGroupName("locked_features"): true,
		},
	}
}

func churnModel() *aiplatform.GoogleCloudAiplatformV1Model {
	return &aiplatform.GoogleCloudAiplatformV1Model{
		Name:              modelName(churnModelID),
		DisplayName:       "churn-predictor",
		Description:       "Predicts subscription churn",
		ArtifactUri:       "gs://" + testBucket + "/models/churn/",
		MetadataSchemaUri: "gs://google-cloud-aiplatform/schema/model/metadata/automl_tabular_1.0.0.yaml",
		ContainerSpec: &aiplatform.GoogleCloudAiplatformV1ModelContainerSpec{
			ImageUri: "us-docker.pkg.dev/vertex-ai/prediction/sklearn-cpu.1-5:latest",
		},
		PredictSchemata: &aiplatform.GoogleCloudAiplatformV1PredictSchemata{
			InstanceSchemaUri:   "gs://" + testBucket + "/schema/instance.yaml",
			ParametersSchemaUri: "gs://" + testBucket + "/schema/parameters.yaml",
			PredictionSchemaUri: "gs://" + testBucket + "/schema/prediction.yaml",
		},
		SupportedDeploymentResourcesTypes: []string{"DEDICATED_RESOURCES"},
		SupportedInputStorageFormats:      []string{"jsonl", "bigquery"},
		SupportedOutputStorageFormats:     []string{"jsonl"},
		PipelineJob:                       pipelineJobName(trainJobID),
		DeployedModels: []*aiplatform.GoogleCloudAiplatformV1DeployedModelRef{
			{DeployedModelId: "deployed-1", Endpoint: endpointName(churnEndpointID)},
		},
		VersionId:          "1",
		VersionAliases:     []string{"default"},
		VersionDescription: "First release",
		VersionCreateTime:  "2026-08-01T10:00:00Z",
		CreateTime:         "2026-08-01T10:00:00Z",
		UpdateTime:         "2026-08-02T11:30:00Z",
		Labels:             map[string]string{"team": "growth", "tier": "gold"},
	}
}

// fraudModelOne and fraudModelTwo share a display name, which Vertex AI
// allows, so both have to be qualified by their id.
func fraudModelOne() *aiplatform.GoogleCloudAiplatformV1Model {
	return &aiplatform.GoogleCloudAiplatformV1Model{
		Name:        modelName(fraudModelIDOne),
		DisplayName: "fraud-detector",
		ArtifactUri: "gs://" + testBucket + "/models/fraud/v1/",
		CreateTime:  "2026-07-01T09:00:00Z",
		BaseModelSource: &aiplatform.GoogleCloudAiplatformV1ModelBaseModelSource{
			ModelGardenSource: &aiplatform.GoogleCloudAiplatformV1ModelGardenSource{
				PublicModelName: "publishers/google/models/gemma",
			},
		},
	}
}

func fraudModelTwo() *aiplatform.GoogleCloudAiplatformV1Model {
	return &aiplatform.GoogleCloudAiplatformV1Model{
		Name:        modelName(fraudModelIDTwo),
		DisplayName: "fraud-detector",
		ArtifactUri: "gs://" + testBucket + "/models/fraud/v2/",
		CreateTime:  "2026-07-20T09:00:00Z",
	}
}

func churnEndpoint() *aiplatform.GoogleCloudAiplatformV1Endpoint {
	return &aiplatform.GoogleCloudAiplatformV1Endpoint{
		Name:                     endpointName(churnEndpointID),
		DisplayName:              "churn-prod",
		Description:              "Production churn serving",
		Network:                  "projects/12345/global/networks/ml-vpc",
		DedicatedEndpointEnabled: true,
		DedicatedEndpointDns:     "4000000000000000004.us-central1-12345.prediction.vertexai.goog",
		TrafficSplit:             map[string]int64{"deployed-2": 30, "deployed-1": 70},
		DeployedModels: []*aiplatform.GoogleCloudAiplatformV1DeployedModel{
			{Id: "deployed-1", DisplayName: "churn-predictor", Model: modelName(churnModelID), ModelVersionId: "1"},
			// A model deployed from another project cannot be resolved to
			// an asset name, so it gets no edge.
			{Id: "deployed-2", DisplayName: "shared-ranker", Model: "projects/other/locations/us-central1/models/999"},
		},
		ModelDeploymentMonitoringJob: fmt.Sprintf("projects/%s/locations/%s/modelDeploymentMonitoringJobs/77", testProject, testLocation),
		CreateTime:                   "2026-08-03T08:00:00Z",
		UpdateTime:                   "2026-08-04T08:00:00Z",
		Labels:                       map[string]string{"env": "prod"},
	}
}

func tabularDataset() *aiplatform.GoogleCloudAiplatformV1Dataset {
	return &aiplatform.GoogleCloudAiplatformV1Dataset{
		Name:              datasetName(tabularDatasetID),
		DisplayName:       "churn-training-data",
		Description:       "Labelled churn events",
		MetadataSchemaUri: "gs://google-cloud-aiplatform/schema/dataset/metadata/tabular_1.0.0.yaml",
		DataItemCount:     125000,
		Metadata: map[string]any{
			"inputConfig": map[string]any{
				"type": "bigquery_source",
				"uri":  "bq://" + testProject + ".analytics." + testTable,
			},
		},
		SavedQueries: []*aiplatform.GoogleCloudAiplatformV1SavedQuery{{DisplayName: "labelled"}},
		CreateTime:   "2026-06-01T10:00:00Z",
		UpdateTime:   "2026-06-02T10:00:00Z",
		Labels:       map[string]string{"team": "growth"},
	}
}

func imageDataset() *aiplatform.GoogleCloudAiplatformV1Dataset {
	return &aiplatform.GoogleCloudAiplatformV1Dataset{
		Name:              datasetName(imageDatasetID),
		DisplayName:       "receipt-scans",
		MetadataSchemaUri: "gs://google-cloud-aiplatform/schema/dataset/metadata/image_1.0.0.yaml",
		DataItemCount:     0,
		Metadata: map[string]any{
			"dataItemSchemaUri": "gs://google-cloud-aiplatform/schema/dataset/dataitem/image_1.0.0.yaml",
			"gcsBucket":         testBucket,
		},
		CreateTime: "2026-06-05T10:00:00Z",
	}
}

// opaqueDataset carries a metadata shape this plugin does not recognise,
// which has to leave the asset intact and produce no edge.
func opaqueDataset() *aiplatform.GoogleCloudAiplatformV1Dataset {
	return &aiplatform.GoogleCloudAiplatformV1Dataset{
		Name:              datasetName(opaqueDatasetID),
		DisplayName:       "experimental-corpus",
		MetadataSchemaUri: "gs://acme-schemas/private/corpus.json",
		Metadata: map[string]any{
			"someFutureConfig": map[string]any{"source": "unknowable"},
		},
		CreateTime: "2026-06-06T10:00:00Z",
	}
}

func customerFeatureGroup() *aiplatform.GoogleCloudAiplatformV1FeatureGroup {
	return &aiplatform.GoogleCloudAiplatformV1FeatureGroup{
		Name:        featureGroupName("customer_features"),
		Description: "Customer features for churn",
		BigQuery: &aiplatform.GoogleCloudAiplatformV1FeatureGroupBigQuery{
			BigQuerySource: &aiplatform.GoogleCloudAiplatformV1BigQuerySource{
				InputUri: "bq://" + testProject + ".features." + testTable,
			},
			EntityIdColumns: []string{"customer_id"},
			Dense:           true,
		},
		ServiceAccountEmail: "features@acme-ml.iam.gserviceaccount.com",
		CreateTime:          "2026-05-01T10:00:00Z",
		UpdateTime:          "2026-05-02T10:00:00Z",
		Labels:              map[string]string{"team": "growth"},
	}
}

// lockedFeatureGroup's features cannot be read, which must cost that group
// its columns and nothing else.
func lockedFeatureGroup() *aiplatform.GoogleCloudAiplatformV1FeatureGroup {
	return &aiplatform.GoogleCloudAiplatformV1FeatureGroup{
		Name:       featureGroupName("locked_features"),
		CreateTime: "2026-05-03T10:00:00Z",
	}
}

func trainJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:           pipelineJobName(trainJobID),
		DisplayName:    "churn-training",
		State:          "PIPELINE_STATE_SUCCEEDED",
		StartTime:      "2026-08-01T09:00:00Z",
		EndTime:        "2026-08-01T09:45:00Z",
		CreateTime:     "2026-08-01T08:59:00Z",
		UpdateTime:     "2026-08-01T09:45:00Z",
		TemplateUri:    "https://us-central1-kfp.pkg.dev/acme-ml/pipelines/churn/v3",
		ServiceAccount: "pipelines@acme-ml.iam.gserviceaccount.com",
		ScheduleName:   fmt.Sprintf("projects/%s/locations/%s/schedules/nightly", testProject, testLocation),
		Labels:         map[string]string{"team": "growth"},
	}
}

func failedJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:        pipelineJobName(failedJobID),
		DisplayName: "churn-training-retry",
		State:       "PIPELINE_STATE_FAILED",
		StartTime:   "2026-07-31T09:00:00Z",
		EndTime:     "2026-07-31T09:05:00Z",
		Error:       &aiplatform.GoogleRpcStatus{Code: 9, Message: "training container exited with code 1"},
	}
}

func runningJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:        pipelineJobName(runningJobID),
		DisplayName: "churn-training-live",
		State:       "PIPELINE_STATE_RUNNING",
		StartTime:   "2026-09-01T09:00:00Z",
	}
}

func cancelledJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:        pipelineJobName(cancelledJobID),
		DisplayName: "churn-training-cancelled",
		State:       "PIPELINE_STATE_CANCELLED",
		StartTime:   "2026-07-30T09:00:00Z",
		EndTime:     "2026-07-30T09:02:00Z",
	}
}

func queuedJob() *aiplatform.GoogleCloudAiplatformV1PipelineJob {
	return &aiplatform.GoogleCloudAiplatformV1PipelineJob{
		Name:        pipelineJobName(queuedJobID),
		DisplayName: "churn-training-queued",
		State:       "PIPELINE_STATE_QUEUED",
		StartTime:   "2026-09-02T09:00:00Z",
		EndTime:     "2026-09-02T09:00:30Z",
	}
}

// withPipelineJobs turns on the pass that is off by default.
func withPipelineJobs(config pluginsdk.RawConfig) {
	config["include_pipeline_jobs"] = true
}

// withEmptyLocation adds a second location the project has nothing in.
func withEmptyLocation(config pluginsdk.RawConfig) {
	config["locations"] = []string{testLocation, emptyLocation}
}

// discoverWith runs a full discovery against a fake API, through Discover
// so the config path the host uses is the one under test.
func discoverWith(t *testing.T, f *fake, options ...func(pluginsdk.RawConfig)) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{
		"project_id":   testProject,
		"locations":    []string{testLocation},
		"endpoint":     server.URL,
		"disable_auth": true,
	}
	for _, option := range options {
		option(config)
	}

	source := &Source{}
	result, err := source.Discover(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

// assetNamed finds one discovered asset, failing the test when it is not
// there.
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

func statisticFor(result *pluginsdk.DiscoveryResult, assetMRN, metric string) (float64, bool) {
	for _, statistic := range result.Statistics {
		if statistic.AssetMRN == assetMRN && statistic.MetricName == metric {
			return statistic.Value, true
		}
	}
	return 0, false
}

func runHistoryFor(result *pluginsdk.DiscoveryResult, assetMRN string) *pluginsdk.AssetRunHistory {
	for i := range result.RunHistory {
		if result.RunHistory[i].AssetMRN == assetMRN {
			return &result.RunHistory[i]
		}
	}
	return nil
}
