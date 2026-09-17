package sagemaker

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validated(t *testing.T, raw pluginsdk.RawConfig) *Config {
	t.Helper()

	source := &Source{}
	_, err := source.Validate(raw)
	require.NoError(t, err)

	return source.config
}

func TestMeta_DeclaresTheFeaturesDiscoverEmits(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "sagemaker", meta.ID)
	assert.Equal(t, "AWS SageMaker", meta.Name)
	assert.Equal(t, "ml", meta.Category)
	assert.Equal(t, "sagemaker", meta.Icon)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
}

func TestValidate_EnablesEndpointsByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeEndpoints)
}

func TestValidate_EnablesModelPackagesByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeModelPackages)
}

func TestValidate_EnablesFeatureGroupsByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).IncludeFeatureGroups)
}

func TestValidate_LeavesTrainingJobsOffByDefault(t *testing.T) {
	// An account keeps months of training history, so opting in is the
	// caller's decision.
	assert.False(t, validated(t, pluginsdk.RawConfig{}).IncludeTrainingJobs)
}

func TestValidate_KeepsAnExplicitFalse(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{"include_endpoints": false})

	assert.False(t, config.IncludeEndpoints)
}

func TestValidate_KeepsAnExplicitTrue(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{"include_training_jobs": true})

	assert.True(t, config.IncludeTrainingJobs)
}

func TestValidate_ConvertsTagsToMetadataByDefault(t *testing.T) {
	assert.True(t, validated(t, pluginsdk.RawConfig{}).TagsToMetadata)
}

func TestValidate_KeepsTagsToMetadataOffWhenAsked(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{"tags_to_metadata": false})

	assert.False(t, config.TagsToMetadata)
}

func TestValidate_AllocatesTheAWSSectionWhenTheConfigOmitsIt(t *testing.T) {
	// The embedded AWS section is a pointer, and reading its tag settings
	// would panic if a config without any AWS key left it nil.
	config := validated(t, pluginsdk.RawConfig{})

	require.NotNil(t, config.AWSConfig)
}

func TestValidate_ReadsTheConfiguredRegion(t *testing.T) {
	config := validated(t, pluginsdk.RawConfig{
		"credentials": map[string]any{"region": "eu-west-1"},
	})

	assert.Equal(t, "eu-west-1", config.Credentials.Region)
}

func TestValidate_RejectsAnEndpointThatIsNotAURL(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"credentials": map[string]any{"endpoint": "not a url"},
	})

	require.Error(t, err)
}

func TestS3Bucket_ReturnsTheBucketOfAnObjectURI(t *testing.T) {
	assert.Equal(t, "ml-artifacts", s3Bucket("s3://ml-artifacts/churn/model.tar.gz"))
}

func TestS3Bucket_ReturnsTheBucketWhenTheURIHasNoKey(t *testing.T) {
	assert.Equal(t, "ml-artifacts", s3Bucket("s3://ml-artifacts"))
}

func TestS3Bucket_IgnoresANonS3URI(t *testing.T) {
	assert.Empty(t, s3Bucket("https://example.com/model.tar.gz"))
}

func TestS3Bucket_IgnoresAnEmptyValue(t *testing.T) {
	assert.Empty(t, s3Bucket(""))
}

func TestMaskEnvironment_HidesValuesThatLookLikeCredentials(t *testing.T) {
	masked := maskEnvironment(map[string]string{
		"API_SECRET":     "hunter2",
		"AUTH_TOKEN":     "abc",
		"DB_PASSWORD":    "pw",
		"AWS_ACCESS_KEY": "AKIA",
	})

	assert.Equal(t, maskedValue, masked["API_SECRET"])
	assert.Equal(t, maskedValue, masked["AUTH_TOKEN"])
	assert.Equal(t, maskedValue, masked["DB_PASSWORD"])
	assert.Equal(t, maskedValue, masked["AWS_ACCESS_KEY"])
}

func TestMaskEnvironment_KeepsOrdinaryValues(t *testing.T) {
	masked := maskEnvironment(map[string]string{"LOG_LEVEL": "info"})

	assert.Equal(t, "info", masked["LOG_LEVEL"])
}

func TestMaskEnvironment_MatchesRegardlessOfCase(t *testing.T) {
	masked := maskEnvironment(map[string]string{"api_secret": "hunter2"})

	assert.Equal(t, maskedValue, masked["api_secret"])
}

func TestMaskEnvironment_ReturnsNothingForAnEmptyEnvironment(t *testing.T) {
	assert.Nil(t, maskEnvironment(nil))
}

func TestConsoleURL_PointsAtTheConfiguredRegion(t *testing.T) {
	source := &Source{region: "eu-west-1"}

	assert.Equal(t,
		"https://eu-west-1.console.aws.amazon.com/sagemaker/home?region=eu-west-1#/models/churn-xgb",
		source.consoleURL("models/churn-xgb"))
}

func TestConsoleURL_IsEmptyWithoutAKnownRegion(t *testing.T) {
	source := &Source{}

	assert.Empty(t, source.consoleURL("models/churn-xgb"))
}

func TestBareTableName_DropsTheDatabaseQualifier(t *testing.T) {
	assert.Equal(t, "customer_features", bareTableName("sagemaker_featurestore.customer_features"))
}

func TestBareTableName_LeavesAnUnqualifiedNameAlone(t *testing.T) {
	assert.Equal(t, "customer_features", bareTableName("customer_features"))
}
