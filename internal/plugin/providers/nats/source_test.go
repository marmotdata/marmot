package nats

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  plugin.RawPluginConfig
		wantErr string
	}{
		{
			name: "valid host config",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"port": 4222,
			},
		},
		{
			name:    "missing host",
			config:  plugin.RawPluginConfig{},
			wantErr: "host",
		},
		{
			name: "invalid port",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"port": 99999,
			},
			wantErr: "port",
		},
		{
			name: "with token",
			config: plugin.RawPluginConfig{
				"host":  "localhost",
				"token": "s3cr3t",
			},
		},
		{
			name: "with credentials file",
			config: plugin.RawPluginConfig{
				"host":             "localhost",
				"credentials_file": "/path/to/creds.creds",
			},
		},
		{
			name: "with filter",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"filter": map[string]interface{}{
					"include": []interface{}{"^ORDERS"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
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
	})
	require.NoError(t, err)
	assert.Equal(t, 4222, s.config.Port)
}
