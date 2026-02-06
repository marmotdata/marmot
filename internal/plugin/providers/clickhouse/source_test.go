package clickhouse

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
				"host":     "localhost",
				"port":     9000,
				"user":     "default",
				"password": "password",
				"database": "default",
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"user": "default",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: plugin.RawPluginConfig{
				"user":     "default",
				"password": "password",
			},
			wantErr:     true,
			errContains: "host",
		},
		{
			name: "missing user",
			config: plugin.RawPluginConfig{
				"host":     "localhost",
				"password": "password",
			},
			wantErr:     true,
			errContains: "user",
		},
		{
			name: "valid config with secure connection",
			config: plugin.RawPluginConfig{
				"host":   "clickhouse.example.com",
				"user":   "admin",
				"secure": true,
			},
			wantErr: false,
		},
		{
			name: "valid config with filters",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"user": "default",
				"filter": map[string]interface{}{
					"include": []interface{}{"^analytics.*"},
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
		"host": "localhost",
		"user": "default",
	})

	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 9000, s.config.Port)
	assert.Equal(t, "default", s.config.Database)
	assert.False(t, s.config.Secure)
}

func TestConfig_Defaults(t *testing.T) {
	config := &Config{
		Host: "localhost",
		User: "default",
	}

	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.Port)
	assert.Equal(t, "", config.Database)
	assert.False(t, config.Secure)
	assert.False(t, config.IncludeDatabases)
	assert.False(t, config.IncludeColumns)
	assert.False(t, config.EnableMetrics)
	assert.False(t, config.ExcludeSystemTables)
}
