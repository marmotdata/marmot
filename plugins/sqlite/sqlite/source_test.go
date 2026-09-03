package sqlite

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"path": "/data/app.db"})
	require.NoError(t, err)
}

func TestValidate_MissingPathFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"path": "/data/app.db"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.EnableMetrics)
	assert.True(t, s.config.DiscoverForeignKeys)
	assert.True(t, s.config.ExcludeSystemTables)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"path":                  "/data/app.db",
		"exclude_system_tables": false,
		"enable_metrics":        false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.ExcludeSystemTables)
	assert.False(t, s.config.EnableMetrics)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.DiscoverForeignKeys)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"path": "/data/app.db",
		"filter": map[string]interface{}{
			"include": []interface{}{"^user.*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes the
// flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Path: "/data/app.db"}

	assert.False(t, config.IncludeColumns)
	assert.False(t, config.EnableMetrics)
	assert.False(t, config.DiscoverForeignKeys)
	assert.False(t, config.ExcludeSystemTables)
}
