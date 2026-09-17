package vertexai

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validated(t *testing.T, raw pluginsdk.RawConfig) *Config {
	t.Helper()

	// Every valid config needs the two required fields, so tests only
	// state what they are actually about.
	if _, ok := raw["project_id"]; !ok {
		raw["project_id"] = testProject
	}
	if _, ok := raw["locations"]; !ok {
		raw["locations"] = []string{testLocation}
	}

	source := &Source{}
	_, err := source.Validate(raw)
	require.NoError(t, err)

	return source.config
}

func TestMeta_DeclaresTheFeaturesDiscoverEmits(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "vertexai", meta.ID)
	assert.Equal(t, "Google Vertex AI", meta.Name)
	assert.Equal(t, "ml", meta.Category)
	assert.Equal(t, "vertex-ai", meta.Icon)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
}

func TestValidate_RejectsAConfigWithoutAProject(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"locations": []string{testLocation}})

	require.Error(t, err)
}

func TestValidate_RejectsAConfigWithoutLocations(t *testing.T) {
	// There is no wildcard location, so a run with no location would scan
	// nothing at all.
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"project_id": testProject})

	require.Error(t, err)
}

func TestValidate_RejectsAnEmptyLocationList(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"project_id": testProject,
		"locations":  []string{},
	})

	require.Error(t, err)
}

func TestValidate_EnablesEndpointsByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeEndpoints)
}

func TestValidate_EnablesDatasetsByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeDatasets)
}

func TestValidate_EnablesFeatureGroupsByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeFeatureGroups)
}

func TestValidate_LeavesPipelineJobsOffByDefault(t *testing.T) {
	// A project keeps every job it has ever run, so opting in is the
	// caller's decision.
	assert.False(t, validated(t, pluginsdk.RawConfig{}).IncludePipelineJobs)
}

func TestValidate_DefaultsThePipelineJobLimit(t *testing.T) {
	assert.Equal(t, 50, validated(t, pluginsdk.RawConfig{}).MaxPipelineJobs)
}

func TestValidate_KeepsAnExplicitFalse(t *testing.T) {
	assert.False(t, validated(t, pluginsdk.RawConfig{"include_datasets": false}).IncludeDatasets)
}

func TestValidate_KeepsAnExplicitTrue(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{"include_pipeline_jobs": true}).IncludePipelineJobs)
}

func TestValidate_RejectsAPipelineJobLimitAboveTheCap(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"project_id":        testProject,
		"locations":         []string{testLocation},
		"max_pipeline_jobs": 5000,
	})

	require.Error(t, err)
}

func TestValidate_GivesTheEndpointATrailingSlash(t *testing.T) {
	// The client appends its own path to the endpoint, and Google writes
	// every published endpoint with the slash.
	config := validated(t, pluginsdk.RawConfig{"endpoint": "http://localhost:18821"})

	assert.Equal(t, "http://localhost:18821/", config.Endpoint)
}

func TestValidate_DoesNotDoubleTheEndpointsTrailingSlash(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{"endpoint": "http://localhost:18821/"})

	assert.Equal(t, "http://localhost:18821/", config.Endpoint)
}

func TestValidate_LeavesAnUnsetEndpointEmpty(t *testing.T) {
	assert.Empty(t, validated(t, pluginsdk.RawConfig{}).Endpoint)
}

func TestValidate_TrimsWhitespaceFromLocations(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{"locations": []string{" us-central1 "}})

	assert.Equal(t, []string{"us-central1"}, config.Locations)
}

func TestRegionalEndpoint_UsesTheLocationsOwnHost(t *testing.T) {
	// Vertex AI does not serve regional resources from the API's default
	// host, so this shape is what makes discovery find anything at all.
	assert.Equal(t, "https://us-central1-aiplatform.googleapis.com/", regionalEndpoint("us-central1"))
}

func TestRegionalEndpoint_HandlesEveryOrdinaryRegion(t *testing.T) {
	assert.Equal(t, "https://europe-west4-aiplatform.googleapis.com/", regionalEndpoint("europe-west4"))
}

func TestRegionalEndpoint_SpellsTheMultiRegionHostsTheOtherWayRound(t *testing.T) {
	// The us and eu multi-region endpoints do not follow the prefix shape.
	assert.Equal(t, "https://aiplatform.us.rep.googleapis.com/", regionalEndpoint("us"))
	assert.Equal(t, "https://aiplatform.eu.rep.googleapis.com/", regionalEndpoint("eu"))
}

func TestRegionalEndpoint_FallsBackToTheDefaultHostForGlobal(t *testing.T) {
	assert.Equal(t, defaultEndpoint, regionalEndpoint("global"))
}

func TestParseResourceName_ReadsTheProjectLocationAndID(t *testing.T) {
	parsed, ok := parseResourceName("projects/acme/locations/us-central1/models/1234567890")

	require.True(t, ok)
	assert.Equal(t, "acme", parsed.project)
	assert.Equal(t, "us-central1", parsed.location)
	assert.Equal(t, "1234567890", parsed.id)
}

func TestParseResourceName_ReadsANestedResource(t *testing.T) {
	// A feature is identified by the last segment, under its group.
	parsed, ok := parseResourceName("projects/acme/locations/us-central1/featureGroups/customers/features/age")

	require.True(t, ok)
	assert.Equal(t, "us-central1", parsed.location)
	assert.Equal(t, "age", parsed.id)
}

func TestParseResourceName_RejectsAShortName(t *testing.T) {
	_, ok := parseResourceName("projects/acme/locations/us-central1")

	assert.False(t, ok)
}

func TestParseResourceName_RejectsANameThatIsNotAResourcePath(t *testing.T) {
	_, ok := parseResourceName("models/1234567890")

	assert.False(t, ok)
}

func TestParseResourceName_RejectsAnEmptyName(t *testing.T) {
	_, ok := parseResourceName("")

	assert.False(t, ok)
}

func TestParseResourceName_RejectsANameWithAnEmptySegment(t *testing.T) {
	_, ok := parseResourceName("projects//locations/us-central1/models/123")

	assert.False(t, ok)
}

func TestResourceID_IsTheLastSegment(t *testing.T) {
	assert.Equal(t, "77", resourceID("projects/acme/locations/us-central1/pipelineJobs/77"))
}

func TestResourceID_LeavesABareIDAlone(t *testing.T) {
	assert.Equal(t, "77", resourceID("77"))
}

func TestResourceID_IsEmptyForAnEmptyName(t *testing.T) {
	assert.Empty(t, resourceID(""))
}

func TestLocate_FallsBackToTheScannedLocation(t *testing.T) {
	// A name that does not parse must not lose the asset, only the detail
	// the name would have carried.
	source := &Source{config: &Config{ProjectID: testProject}}

	id := source.locate("some-unexpected-name", testLocation)

	assert.Equal(t, testProject, id.project)
	assert.Equal(t, testLocation, id.location)
	assert.Equal(t, "some-unexpected-name", id.id)
}

func TestBigQueryTable_ReturnsTheTableOfAnURI(t *testing.T) {
	assert.Equal(t, "events", bigQueryTable("bq://acme.analytics.events"))
}

func TestBigQueryTable_IgnoresAURIWithoutATable(t *testing.T) {
	assert.Empty(t, bigQueryTable("bq://acme.analytics"))
}

func TestBigQueryTable_IgnoresAURIWithTooManyParts(t *testing.T) {
	assert.Empty(t, bigQueryTable("bq://acme.analytics.events.extra"))
}

func TestBigQueryTable_IgnoresAURIWithAnEmptyPart(t *testing.T) {
	assert.Empty(t, bigQueryTable("bq://acme..events"))
}

func TestBigQueryTable_IgnoresAnotherScheme(t *testing.T) {
	assert.Empty(t, bigQueryTable("gs://acme/analytics/events"))
}

func TestBigQueryTable_IgnoresAnEmptyValue(t *testing.T) {
	assert.Empty(t, bigQueryTable(""))
}

func TestBigQueryTable_IgnoresTheSchemeOnItsOwn(t *testing.T) {
	assert.Empty(t, bigQueryTable("bq://"))
}

func TestGCSBucket_ReturnsTheBucketOfAnObjectURI(t *testing.T) {
	assert.Equal(t, "ml-artifacts", gcsBucket("gs://ml-artifacts/models/churn/model.pkl"))
}

func TestGCSBucket_ReturnsTheBucketWhenTheURIHasNoObject(t *testing.T) {
	assert.Equal(t, "ml-artifacts", gcsBucket("gs://ml-artifacts"))
}

func TestGCSBucket_IgnoresAnotherScheme(t *testing.T) {
	assert.Empty(t, gcsBucket("https://storage.googleapis.com/ml-artifacts"))
}

func TestGCSBucket_IgnoresAnEmptyValue(t *testing.T) {
	assert.Empty(t, gcsBucket(""))
}

func TestGCSBucket_IgnoresTheSchemeOnItsOwn(t *testing.T) {
	assert.Empty(t, gcsBucket("gs://"))
}

func TestNames_LeavesAUniqueDisplayNameAlone(t *testing.T) {
	naming := newNames([]string{"churn-predictor", "fraud-detector"})

	assert.Equal(t, "churn-predictor", naming.resolve("churn-predictor", "1234"))
}

func TestNames_QualifiesEveryResourceSharingADisplayName(t *testing.T) {
	// Qualifying only the later ones would make a name depend on the order
	// the API happened to list things in.
	naming := newNames([]string{"fraud-detector", "fraud-detector"})

	assert.Equal(t, "fraud-detector (1111)", naming.resolve("fraud-detector", "1111"))
	assert.Equal(t, "fraud-detector (2222)", naming.resolve("fraud-detector", "2222"))
}

func TestNames_FallsBackToTheIDWhenThereIsNoDisplayName(t *testing.T) {
	naming := newNames([]string{""})

	assert.Equal(t, "1234", naming.resolve("", "1234"))
}

func TestEdgeSet_KeepsOneCopyOfARepeatedEdge(t *testing.T) {
	edges := newEdgeSet()

	edges.add("mrn://bucket/gcs/artifacts", "mrn://model/vertex-ai/churn", "FEEDS")
	edges.add("mrn://bucket/gcs/artifacts", "mrn://model/vertex-ai/churn", "FEEDS")

	assert.Len(t, edges.all(), 1)
}

func TestEdgeSet_KeepsTheOrderEdgesWereAddedIn(t *testing.T) {
	edges := newEdgeSet()

	edges.add("a", "b", "FEEDS")
	edges.add("c", "d", "PRODUCES")

	all := edges.all()
	require.Len(t, all, 2)
	assert.Equal(t, "a", all[0].Source)
	assert.Equal(t, "c", all[1].Source)
}

func TestEdgeSet_IgnoresAnEdgeWithAMissingEnd(t *testing.T) {
	edges := newEdgeSet()

	edges.add("", "mrn://model/vertex-ai/churn", "FEEDS")

	assert.Empty(t, edges.all())
}

func TestDiscover_FindsNothingInAnEmptyProject(t *testing.T) {
	result := discoverWith(t, &fake{})

	assert.Empty(t, result.Assets)
	assert.Empty(t, result.Lineage)
}

func TestDiscover_SurvivesALocationWithNothingInIt(t *testing.T) {
	// A project is usually enabled in more regions than it uses.
	result := discoverWith(t, fullFake(), withEmptyLocation)

	assetNamed(t, result, "Model", "churn-predictor")
}

func TestDiscover_ReturnsAnErrorWhenTheAPIIsUnreachable(t *testing.T) {
	source := &Source{}

	_, err := source.Discover(t.Context(), pluginsdk.RawConfig{
		"project_id":   testProject,
		"locations":    []string{testLocation},
		"endpoint":     "http://127.0.0.1:1",
		"disable_auth": true,
	})

	require.Error(t, err)
}

func TestDiscover_CoversEveryTypeThePluginEmits(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)

	counts := map[string]int{}
	for _, asset := range result.Assets {
		counts[asset.Type]++
	}

	assert.Equal(t, 3, counts["Model"], "one churn model and two colliding fraud models")
	assert.Equal(t, 1, counts["Endpoint"])
	assert.Equal(t, 5, counts["Dataset"], "three managed datasets and two feature groups")
	assert.Equal(t, 5, counts["Job"])
}

func TestDiscover_SkipsPipelineJobsUnlessAskedFor(t *testing.T) {
	result := discoverWith(t, fullFake())

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Job", asset.Type)
	}
}

func TestDiscover_FilesEveryAssetUnderTheVertexAIProvider(t *testing.T) {
	result := discoverWith(t, fullFake(), withPipelineJobs)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		assert.Equal(t, []string{"Vertex AI"}, asset.Providers)
	}
}

func TestDiscover_RecordsTheSourceOnEveryAsset(t *testing.T) {
	result := discoverWith(t, fullFake())

	model := assetNamed(t, result, "Model", "churn-predictor")
	require.Len(t, model.Sources, 1)
	assert.Equal(t, "Vertex AI", model.Sources[0].Name)
}

func TestDiscover_InterpolatesTagsFromMetadata(t *testing.T) {
	server := fullFake().start(t)

	source := &Source{}
	result, err := source.Discover(t.Context(), pluginsdk.RawConfig{
		"project_id":   testProject,
		"locations":    []string{testLocation},
		"endpoint":     server.URL,
		"disable_auth": true,
		"tags":         []string{"location:${location}"},
	})
	require.NoError(t, err)

	model := assetNamed(t, result, "Model", "churn-predictor")
	assert.Contains(t, model.Tags, "location:us-central1")
}
