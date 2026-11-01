package kinesis

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesTheKinesisPlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "kinesis", meta.ID)
	assert.Equal(t, "AWS Kinesis", meta.Name)
	assert.Equal(t, "kinesis", meta.Icon)
	assert.Equal(t, "messaging", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets"}, meta.Features, "no lineage: Kinesis knows nothing about producers")
}

func TestSource_IsADataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}

func TestValidate_EmptyConfigUsesTheDefaultCredentialChain(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)

	require.NotNil(t, s.config)
	require.NotNil(t, s.config.AWSConfig, "the AWS section is allocated so callers never nil-check it")
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]interface{}{"region": "us-east-1"},
	})
	require.NoError(t, err)

	assert.True(t, s.config.IncludeConsumers)
	assert.True(t, s.config.IncludeShards)
	assert.True(t, s.config.TagsToMetadata)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"include_consumers": false,
		"tags_to_metadata":  false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeConsumers)
	assert.False(t, s.config.TagsToMetadata)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeShards)
}

func TestValidate_CarriesTheAWSCredentials(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]interface{}{
			"region":   "eu-west-1",
			"id":       "AKIA",
			"secret":   "shh",
			"endpoint": "http://localhost:15555",
		},
		"include_tags": []interface{}{"team"},
	})
	require.NoError(t, err)

	assert.Equal(t, "eu-west-1", s.config.Credentials.Region)
	assert.Equal(t, "AKIA", s.config.Credentials.ID)
	assert.Equal(t, "http://localhost:15555", s.config.Credentials.Endpoint)
	assert.Equal(t, []string{"team"}, s.config.IncludeTags)
}

func TestValidate_RejectsAnEndpointThatIsNotAURL(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]interface{}{"endpoint": "not a url"},
	})
	require.Error(t, err)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"filter": map[string]interface{}{
			"include": []interface{}{"^orders.*"},
			"exclude": []interface{}{".*-tmp$"},
		},
	})
	require.NoError(t, err)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{}

	assert.False(t, config.IncludeConsumers)
	assert.False(t, config.IncludeShards)
}
