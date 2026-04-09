package duckdb

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      plugin.RawPluginConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: plugin.RawPluginConfig{
				"path": "/data/analytics.duckdb",
			},
			wantErr: false,
		},
		{
			name: "valid config with all options",
			config: plugin.RawPluginConfig{
				"path":                   "/data/analytics.duckdb",
				"include_columns":        true,
				"enable_metrics":         true,
				"discover_foreign_keys":  true,
				"exclude_system_schemas": true,
			},
			wantErr: false,
		},
		{
			name:        "missing path",
			config:      plugin.RawPluginConfig{},
			wantErr:     true,
			errContains: "path",
		},
		{
			name: "valid config with filters",
			config: plugin.RawPluginConfig{
				"path": "/data/analytics.duckdb",
				"filter": map[string]interface{}{
					"include": []interface{}{"^main\\..*"},
					"exclude": []interface{}{".*_temp$"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_ValidateDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"path": "/data/analytics.duckdb",
	})

	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.EnableMetrics)
	assert.True(t, s.config.DiscoverForeignKeys)
	assert.True(t, s.config.ExcludeSystemSchemas)
}

func TestConfig_Defaults(t *testing.T) {
	config := &Config{
		Path: "/data/analytics.duckdb",
	}

	assert.False(t, config.IncludeColumns)
	assert.False(t, config.EnableMetrics)
	assert.False(t, config.DiscoverForeignKeys)
	assert.False(t, config.ExcludeSystemSchemas)
}
