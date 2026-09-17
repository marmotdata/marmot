package sagemaker

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// featureColumnsOf reads the columns back out of an asset's schema, the way
// the Marmot UI does.
func featureColumnsOf(t *testing.T, asset pluginsdk.Asset) []pluginsdk.Column {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.True(t, ok, "expected a columns entry in the schema")

	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	return columns
}

func TestDiscoverFeatureGroups_CatalogsTheGroupAsADataset(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Equal(t, []string{"SageMaker"}, group.Providers)
	assert.Equal(t, "Created", group.Metadata["status"])
}

func TestDiscoverFeatureGroups_UsesTheGroupDescription(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	require.NotNil(t, group.Description)
	assert.Equal(t, "Customer features for churn", *group.Description)
}

func TestDiscoverFeatureGroups_RecordsTheRecordIdentifierAndEventTime(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Equal(t, "age", group.Metadata["record_identifier"])
	assert.Equal(t, "event_time", group.Metadata["event_time_feature"])
}

func TestDiscoverFeatureGroups_RecordsTheOnlineStoreFlag(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Equal(t, true, group.Metadata["online_store"])
}

func TestDiscoverFeatureGroups_RecordsTheOfflineStoreLocation(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Equal(t, "s3://ml-features/customer-features", group.Metadata["offline_store_s3_uri"])
}

func TestDiscoverFeatureGroups_RecordsTheGlueTableTheOfflineStoreIsQueriedThrough(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Equal(t, "sagemaker_featurestore.customer_features", group.Metadata["glue_table"])
}

func TestDiscoverFeatureGroups_TurnsFeatureDefinitionsIntoColumns(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	columns := featureColumnsOf(t, assetNamed(t, result, "Dataset", "customer-features"))

	require.Len(t, columns, 3)
	assert.Equal(t, "age", columns[0].Name)
	assert.Equal(t, "Integral", columns[0].DataType)
	assert.Equal(t, "plan", columns[1].Name)
	assert.Equal(t, "String", columns[1].DataType)
	assert.Equal(t, "event_time", columns[2].Name)
	assert.Equal(t, "Fractional", columns[2].DataType)
}

func TestDiscoverFeatureGroups_MarksTheRecordIdentifierAsThePrimaryKey(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	columns := featureColumnsOf(t, assetNamed(t, result, "Dataset", "customer-features"))

	assert.True(t, columns[0].PrimaryKey)
	assert.False(t, columns[1].PrimaryKey)
}

func TestDiscoverFeatureGroups_MarksTheRequiredFeaturesAsNotNullable(t *testing.T) {
	// Every record has to carry its identifier and its event time.
	result := discoverWith(t, fullFake(), nil)

	columns := featureColumnsOf(t, assetNamed(t, result, "Dataset", "customer-features"))

	assert.False(t, columns[0].Nullable)
	assert.True(t, columns[1].Nullable)
	assert.False(t, columns[2].Nullable)
}

func TestDiscoverFeatureGroups_LinksTheGroupToItsGlueTable(t *testing.T) {
	// The Glue plugin names its table assets with the bare table name.
	result := discoverWith(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://dataset/sagemaker/customer-features", "mrn://table/glue/customer_features", "PRODUCES"))
}

func TestDiscoverFeatureGroups_LinksTheGroupToItsOfflineStoreBucket(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://dataset/sagemaker/customer-features", "mrn://bucket/s3/ml-features", "PRODUCES"))
}

func TestDiscoverFeatureGroups_HasNoConsoleLink(t *testing.T) {
	// The feature store has no confirmed route in the classic console.
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Dataset", "customer-features")

	assert.Empty(t, group.ExternalLinks)
	assert.NotContains(t, group.Metadata, "url")
}

func TestDiscoverFeatureGroups_OmitsTheGlueTableWhenTheOfflineStoreHasNoCatalog(t *testing.T) {
	fake := fullFake()
	fake.featureDetails["customer-features"].OfflineStoreConfig.DataCatalogConfig = nil

	result := discoverWith(t, fake, nil)

	group := assetNamed(t, result, "Dataset", "customer-features")
	assert.NotContains(t, group.Metadata, "glue_table")
}

func TestDiscoverFeatureGroups_KeepsTheGroupWhenTaggingIsUnsupported(t *testing.T) {
	// Tagging a feature group is not supported everywhere, and the group
	// itself is still worth cataloguing.
	fake := fullFake()
	fake.errs = map[string]error{"ListTags": errors.New("not implemented")}

	result := discoverWith(t, fake, nil)

	assetNamed(t, result, "Dataset", "customer-features")
}

func TestDiscoverFeatureGroups_SkipsAGroupItCannotDescribe(t *testing.T) {
	fake := fullFake()
	fake.featureGroups[0].FeatureGroupName = aws.String("ghost")

	result := discoverWith(t, fake, nil)

	for _, a := range result.Assets {
		assert.NotEqual(t, "Dataset", a.Type)
	}
}

func TestDiscoverFeatureGroups_AreSkippedWhenTurnedOff(t *testing.T) {
	result := discoverWith(t, fullFake(), pluginsdk.RawConfig{"include_feature_groups": false})

	for _, a := range result.Assets {
		assert.NotEqual(t, "Dataset", a.Type)
	}
}

func TestDiscoverFeatureGroups_FailureKeepsTheRestOfTheRun(t *testing.T) {
	// Not every SageMaker-compatible API implements ListFeatureGroups.
	fake := fullFake()
	fake.errs = map[string]error{"ListFeatureGroups": errors.New("not implemented")}

	result := discoverWith(t, fake, nil)

	assetNamed(t, result, "Model", "churn-xgb")
	assetNamed(t, result, "Endpoint", "churn-prod")
}
