package dagster

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "dagster", meta.ID)
	assert.Equal(t, "Dagster", meta.Name)
	assert.Equal(t, "dagster", meta.Icon)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:3000"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_NonURLHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "not a url"})
	require.Error(t, err)
}

func TestValidate_TrimsTheTrailingSlashFromHost(t *testing.T) {
	// The GraphQL path is appended to the host, so a trailing slash would
	// produce a double slash.
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:3000/"})
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:3000", s.config.Host)
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:3000"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeOps)
	assert.True(t, s.config.IncludeAssets)
	assert.True(t, s.config.IncludeRunHistory)
}

func TestValidate_DefaultsRunHistoryLimit(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:3000"})
	require.NoError(t, err)

	assert.Equal(t, 10, s.config.RunHistoryLimit)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":                "http://localhost:3000",
		"include_ops":         false,
		"include_run_history": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeOps)
	assert.False(t, s.config.IncludeRunHistory)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeAssets)
	assert.True(t, s.config.VerifySSL)
}

func TestValidate_RejectsARunHistoryLimitAboveTheMaximum(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "http://localhost:3000",
		"run_history_limit": 500,
	})
	require.Error(t, err)
}

func TestValidate_RejectsAZeroRunHistoryLimit(t *testing.T) {
	// Zero would ask Dagster for no runs at all, which is never what a user
	// who set the field meant.
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "http://localhost:3000",
		"run_history_limit": 0,
	})
	require.Error(t, err)
}

func TestValidate_AcceptsCodeLocations(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":           "http://localhost:3000",
		"code_locations": []any{"analytics", "ml"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"analytics", "ml"}, s.config.CodeLocations)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "http://localhost:3000",
		"filter": map[string]any{
			"include": []any{"^orders_.*"},
			"exclude": []any{".*_test$"},
		},
	})
	require.NoError(t, err)
}

func TestValidate_AcceptsACloudToken(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":  "https://my-org.dagster.cloud/prod",
		"token": "secret-token",
	})
	require.NoError(t, err)

	assert.Equal(t, "secret-token", s.config.Token)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes the
// flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Host: "http://localhost:3000"}

	assert.False(t, config.VerifySSL)
	assert.False(t, config.IncludeOps)
	assert.False(t, config.IncludeAssets)
	assert.False(t, config.IncludeRunHistory)
}
