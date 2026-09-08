package vertexai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpoint_IsNamedAfterItsDisplayName(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "mrn://endpoint/vertex ai/churn-prod", *endpoint.MRN)
}

func TestEndpoint_CarriesItsDescription(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	require.NotNil(t, endpoint.Description)
	assert.Equal(t, "Production churn serving", *endpoint.Description)
}

func TestEndpoint_RecordsItsNetworking(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "projects/12345/global/networks/ml-vpc", endpoint.Metadata["network"])
	assert.Equal(t, true, endpoint.Metadata["dedicated_endpoint_enabled"])
	assert.Equal(t, "4000000000000000004.us-central1-12345.prediction.vertexai.goog", endpoint.Metadata["dedicated_endpoint_dns"])
}

func TestEndpoint_RecordsWhatItServes(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, 2, endpoint.Metadata["deployed_model_count"])
	assert.Equal(t, "churn-predictor, shared-ranker", endpoint.Metadata["deployed_models"])
}

func TestEndpoint_RecordsItsMonitoringJobAsABareID(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "77", endpoint.Metadata["model_deployment_monitoring_job"])
}

func TestEndpoint_LinksEachModelItServes(t *testing.T) {
	result := discoverWith(t, fullFake())

	assert.True(t, hasEdge(result,
		"mrn://model/vertex ai/churn-predictor",
		"mrn://endpoint/vertex ai/churn-prod",
		"FEEDS"))
}

func TestEndpoint_SkipsAModelThisRunDidNotFind(t *testing.T) {
	// A model deployed from another project has no asset name here, so
	// there is nothing to point the edge at.
	result := discoverWith(t, fullFake())

	count := 0
	for _, edge := range result.Lineage {
		if edge.Target == "mrn://endpoint/vertex ai/churn-prod" {
			count++
		}
	}

	assert.Equal(t, 1, count)
}

func TestTrafficSplit_IsSortedByDeployedModelID(t *testing.T) {
	// Go randomises map order, so an unsorted value would change between
	// runs and rewrite the asset every time.
	assert.Equal(t, "deployed-1=70,deployed-2=30", trafficSplit(map[string]int64{
		"deployed-2": 30,
		"deployed-1": 70,
	}))
}

func TestTrafficSplit_IsEmptyWithoutASplit(t *testing.T) {
	assert.Empty(t, trafficSplit(nil))
}

func TestEndpoint_RecordsItsTrafficSplit(t *testing.T) {
	result := discoverWith(t, fullFake())

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "deployed-1=70,deployed-2=30", endpoint.Metadata["traffic_split"])
}

func TestModelResourceName_DropsAVersionSuffix(t *testing.T) {
	// A deployed model may name a specific version, but the models list
	// is keyed on the model.
	assert.Equal(t,
		"projects/acme/locations/us-central1/models/123",
		modelResourceName("projects/acme/locations/us-central1/models/123@2"))
}

func TestModelResourceName_LeavesAPlainNameAlone(t *testing.T) {
	assert.Equal(t,
		"projects/acme/locations/us-central1/models/123",
		modelResourceName("projects/acme/locations/us-central1/models/123"))
}

func TestEndpoint_ResolvesAModelDeployedByVersion(t *testing.T) {
	fake := fullFake()
	fake.endpoints[testLocation][0].DeployedModels[0].Model = modelName(churnModelID) + "@1"

	result := discoverWith(t, fake)

	assert.True(t, hasEdge(result,
		"mrn://model/vertex ai/churn-predictor",
		"mrn://endpoint/vertex ai/churn-prod",
		"FEEDS"))
}

func TestEndpoints_QualifyBothSidesOfADisplayNameCollision(t *testing.T) {
	fake := fullFake()
	second := churnEndpoint()
	second.Name = endpointName("5555")
	fake.endpoints[testLocation] = append(fake.endpoints[testLocation], second)

	result := discoverWith(t, fake)

	assetNamed(t, result, "Endpoint", "churn-prod ("+churnEndpointID+")")
	assetNamed(t, result, "Endpoint", "churn-prod (5555)")
}

func TestEndpoints_AreSkippedWhenTheConfigTurnsThemOff(t *testing.T) {
	server := fullFake().start(t)

	source := &Source{}
	result, err := source.Discover(t.Context(), map[string]any{
		"project_id":        testProject,
		"locations":         []string{testLocation},
		"endpoint":          server.URL,
		"disable_auth":      true,
		"include_endpoints": false,
	})
	require.NoError(t, err)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Endpoint", asset.Type)
	}
}

func TestDeployedModelNames_SkipsAModelWithoutADisplayName(t *testing.T) {
	assert.Empty(t, deployedModelNames(nil))
}
