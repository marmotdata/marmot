package azureblob

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config with connection string",
			config: map[string]interface{}{
				"connection_string": "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key123;EndpointSuffix=core.windows.net",
			},
			expectErr: false,
		},
		{
			name: "valid config with account name and key",
			config: map[string]interface{}{
				"account_name": "testaccount",
				"account_key":  "testkey123",
			},
			expectErr: false,
		},
		{
			name: "valid config with custom endpoint",
			config: map[string]interface{}{
				"account_name": "devstoreaccount1",
				"account_key":  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
				"endpoint":     "http://localhost:10000/devstoreaccount1",
			},
			expectErr: false,
		},
		{
			name:      "missing connection string and account name",
			config:    map[string]interface{}{},
			expectErr: true,
			errMsg:    "either connection_string or account_name must be provided",
		},
		{
			name: "account name without key",
			config: map[string]interface{}{
				"account_name": "testaccount",
			},
			expectErr: true,
			errMsg:    "account_key is required when using account_name",
		},
		{
			name: "config with filter",
			config: map[string]interface{}{
				"connection_string": "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key123;EndpointSuffix=core.windows.net",
				"filter": map[string]interface{}{
					"include": []string{"^data-.*"},
					"exclude": []string{".*-temp$"},
				},
			},
			expectErr: false,
		},
		{
			name: "config with all options",
			config: map[string]interface{}{
				"connection_string":  "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key123;EndpointSuffix=core.windows.net",
				"include_metadata":   true,
				"include_blob_count": false,
				"tags":               []string{"azure", "storage"},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(plugin.RawPluginConfig(tt.config))

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_ValidateDefaults(t *testing.T) {
	s := &Source{}
	config := map[string]interface{}{
		"connection_string": "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key123;EndpointSuffix=core.windows.net",
	}

	_, err := s.Validate(plugin.RawPluginConfig(config))
	require.NoError(t, err)

	assert.NotNil(t, s.config)
	assert.Equal(t, "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key123;EndpointSuffix=core.windows.net", s.config.ConnectionString)
}
