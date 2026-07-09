package nats

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  pluginsdk.RawConfig
		wantErr string
	}{
		{
			name: "valid host config",
			config: pluginsdk.RawConfig{
				"host": "localhost",
				"port": 4222,
			},
		},
		{
			name:    "missing host",
			config:  pluginsdk.RawConfig{},
			wantErr: "host",
		},
		{
			name: "invalid port",
			config: pluginsdk.RawConfig{
				"host": "localhost",
				"port": 99999,
			},
			wantErr: "port",
		},
		{
			name: "with token",
			config: pluginsdk.RawConfig{
				"host":  "localhost",
				"token": "s3cr3t",
			},
		},
		{
			name: "with credentials file",
			config: pluginsdk.RawConfig{
				"host":             "localhost",
				"credentials_file": "/path/to/creds.creds",
			},
		},
		{
			name: "with filter",
			config: pluginsdk.RawConfig{
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
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "localhost",
	})
	require.NoError(t, err)
	assert.Equal(t, 4222, s.config.Port)
}
