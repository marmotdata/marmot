package vertexai

import (
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/aiplatform/v1"
)

// columnsOf reads back the schema the plugin set on an asset.
func columnsOf(t *testing.T, asset pluginsdk.Asset) []pluginsdk.Column {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.True(t, ok, "asset has no columns")

	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	return columns
}

func TestFeatureGroup_IsNamedAfterItsID(t *testing.T) {
	// A feature group id is chosen by the user and is unique within a
	// project and location, so it needs no qualifying.
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")

	assert.Equal(t, "mrn://dataset/vertex ai/customer_features", *group.MRN)
}

func TestFeatureGroup_CarriesItsDescription(t *testing.T) {
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")

	require.NotNil(t, group.Description)
	assert.Equal(t, "Customer features for churn", *group.Description)
}

func TestFeatureGroup_RecordsItsSourceTable(t *testing.T) {
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")

	assert.Equal(t, "bq://"+testProject+".features."+testTable, group.Metadata["big_query_source_uri"])
	assert.Equal(t, []string{"customer_id"}, group.Metadata["entity_id_columns"])
	assert.Equal(t, true, group.Metadata["dense"])
}

func TestFeatureGroup_RecordsItsServiceAccount(t *testing.T) {
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")

	assert.Equal(t, "features@acme-ml.iam.gserviceaccount.com", group.Metadata["service_account_email"])
}

func TestFeatureGroup_SetsOneColumnPerFeature(t *testing.T) {
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")
	columns := columnsOf(t, group)

	require.Len(t, columns, 2)
	assert.Equal(t, "age", columns[0].Name)
	assert.Equal(t, "int64", columns[0].DataType)
	assert.Equal(t, "plan_tier", columns[1].Name)
	assert.Equal(t, "string", columns[1].DataType)
	assert.Equal(t, "Subscription tier", columns[1].Description)
}

func TestFeatureGroup_SortsItsColumnsByName(t *testing.T) {
	// The API returns features in its own order, and a schema that
	// reshuffles would rewrite the asset on every run.
	result := discoverWith(t, fullFake())

	columns := columnsOf(t, assetNamed(t, result, "Dataset", "customer_features"))

	require.Len(t, columns, 2)
	assert.Less(t, columns[0].Name, columns[1].Name)
}

func TestFeatureGroup_EmitsItsFeatureCountAsAStatistic(t *testing.T) {
	result := discoverWith(t, fullFake())

	group := assetNamed(t, result, "Dataset", "customer_features")

	value, ok := statisticFor(result, *group.MRN, "asset.column_count")
	require.True(t, ok)
	assert.Equal(t, float64(2), value)
	assert.Equal(t, 2, group.Metadata["feature_count"])
}

func TestFeatureGroup_LinksTheBigQueryTableThatFeedsIt(t *testing.T) {
	// A Vertex feature group reads from a table that already exists, so
	// the table feeds the group. This is the opposite direction to
	// SageMaker, where a feature group writes its offline store out.
	result := discoverWith(t, fullFake())

	assert.True(t, hasEdge(result,
		"mrn://table/bigquery/"+testTable,
		"mrn://dataset/vertex ai/customer_features",
		"FEEDS"))
}

func TestFeatureGroup_SurvivesFeaturesItCannotRead(t *testing.T) {
	// One group whose features are denied still belongs in the catalog,
	// and must not cost the other groups theirs.
	result := discoverWith(t, fullFake())

	locked := assetNamed(t, result, "Dataset", "locked_features")
	assert.NotContains(t, locked.Schema, "columns")

	assert.Len(t, columnsOf(t, assetNamed(t, result, "Dataset", "customer_features")), 2)
}

func TestFeatureGroup_ClaimsNoFeatureCountWhenItCannotReadThem(t *testing.T) {
	// A zero would read as an empty group, which is a different fact from
	// a group whose features were denied.
	result := discoverWith(t, fullFake())

	locked := assetNamed(t, result, "Dataset", "locked_features")

	assert.NotContains(t, locked.Metadata, "feature_count")
	_, ok := statisticFor(result, *locked.MRN, "asset.column_count")
	assert.False(t, ok)
}

func TestFeatureGroups_AreSkippedWhenTheConfigTurnsThemOff(t *testing.T) {
	server := fullFake().start(t)

	source := &Source{}
	result, err := source.Discover(t.Context(), map[string]any{
		"project_id":             testProject,
		"locations":              []string{testLocation},
		"endpoint":               server.URL,
		"disable_auth":           true,
		"include_feature_groups": false,
	})
	require.NoError(t, err)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "customer_features", *asset.Name)
	}
}

func TestFeatureColumns_SkipsAFeatureWithNoName(t *testing.T) {
	columns := featureColumns([]*aiplatform.GoogleCloudAiplatformV1Feature{{ValueType: "STRING"}})

	assert.Empty(t, columns)
}

func TestBigQuerySourceURI_IsEmptyWithoutASource(t *testing.T) {
	assert.Empty(t, bigQuerySourceURI(&aiplatform.GoogleCloudAiplatformV1FeatureGroup{}))
}
