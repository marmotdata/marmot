package gluepipeline

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_IsAnOrchestrationPlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "gluepipeline", meta.ID)
	assert.Equal(t, "Glue Pipelines", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "glue", meta.Icon)
}

func TestMeta_DeclaresRunHistory(t *testing.T) {
	meta := Meta()

	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestValidate_EmptyConfigIsValid(t *testing.T) {
	// AWS credentials can come from the environment, so nothing has to be
	// set in the ingest file.
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)
}

func TestValidate_DefaultsEverythingOn(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeWorkflows)
	assert.True(t, s.config.IncludeTriggers)
	assert.True(t, s.config.IncludeRunHistory)
	assert.True(t, s.config.IncludeCrawlers)
	assert.Equal(t, 20, s.config.RunHistoryLimit)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"include_crawlers":    false,
		"include_run_history": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeCrawlers)
	assert.False(t, s.config.IncludeRunHistory)
	assert.True(t, s.config.IncludeWorkflows)
	assert.True(t, s.config.IncludeTriggers)
}

func TestValidate_KeepsAnExplicitRunHistoryLimit(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"run_history_limit": 5})
	require.NoError(t, err)

	assert.Equal(t, 5, s.config.RunHistoryLimit)
}

func TestValidate_RejectsARunHistoryLimitAboveTheMaximum(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"run_history_limit": 500})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_history_limit must be at most 200")
}

func TestValidate_RejectsARunHistoryLimitOfZero(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"run_history_limit": 0})
	require.Error(t, err)
}

func TestValidate_ReadsAWSCredentials(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]interface{}{
			"region":   "eu-west-1",
			"id":       "test",
			"secret":   "test",
			"endpoint": "http://localhost:15560",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, s.config.AWSConfig)

	assert.Equal(t, "eu-west-1", s.config.Credentials.Region)
	assert.Equal(t, "http://localhost:15560", s.config.Credentials.Endpoint)
}

func TestValidate_RejectsANonURLEndpoint(t *testing.T) {
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
			"include": []interface{}{"^daily.*"},
			"exclude": []interface{}{".*-test$"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, s.config.Filter)

	assert.Equal(t, []string{"^daily.*"}, s.config.Filter.Include)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{}

	assert.False(t, config.IncludeWorkflows)
	assert.False(t, config.IncludeRunHistory)
	assert.Equal(t, 0, config.RunHistoryLimit)
}

// An ingest file may leave the AWS block out entirely and take credentials
// from the environment, so the config still has to be usable.
func TestValidate_FillsInTheAWSSectionWhenItIsAbsent(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)

	require.NotNil(t, s.config.AWSConfig)
	assert.False(t, s.config.TagsToMetadata)
}
