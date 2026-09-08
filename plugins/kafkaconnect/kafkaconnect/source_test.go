package kafkaconnect

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "kafkaconnect", meta.ID)
	assert.Equal(t, "Apache Kafka Connect", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "kafka", meta.Icon)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_ValidConfig(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "http://connect:8083"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "connect:8083"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_TrimsTrailingSlashFromHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://connect:8083/"})
	require.NoError(t, err)

	assert.Equal(t, "http://connect:8083", s.config.Host)
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://connect:8083"})
	require.NoError(t, err)

	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeTasks)
	assert.True(t, s.config.IncludeTopics)
	assert.True(t, s.config.IncludeConfig)
	assert.True(t, s.config.DiscoverLineage)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":          "http://connect:8083",
		"verify_ssl":    false,
		"include_tasks": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.VerifySSL)
	assert.False(t, s.config.IncludeTasks)
	assert.True(t, s.config.IncludeTopics)
	assert.True(t, s.config.DiscoverLineage)
}

func TestValidate_PasswordWithoutUsernameFails(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "http://connect:8083", "password": "s3cret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_AcceptsBasicAuth(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://connect:8083", "username": "u", "password": "p"})
	require.NoError(t, err)

	assert.Equal(t, "u", s.config.Username)
	assert.Equal(t, "p", s.config.Password)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "http://connect:8083",
		"filter": map[string]any{
			"include": []any{"^orders.*"},
			"exclude": []any{".*-test$"},
		},
	})
	require.NoError(t, err)
}
