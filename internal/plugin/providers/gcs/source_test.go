package gcs

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
			name: "valid config with credentials file",
			config: map[string]interface{}{
				"project_id":       "my-project",
				"credentials_file": "/path/to/credentials.json",
			},
			expectErr: false,
		},
		{
			name: "valid config with credentials JSON",
			config: map[string]interface{}{
				"project_id":       "my-project",
				"credentials_json": `{"type": "service_account"}`,
			},
			expectErr: false,
		},
		{
			name: "valid config with custom endpoint",
			config: map[string]interface{}{
				"project_id":   "test-project",
				"endpoint":     "http://localhost:4443/storage/v1/",
				"disable_auth": true,
			},
			expectErr: false,
		},
		{
			name: "missing project_id",
			config: map[string]interface{}{
				"credentials_file": "/path/to/credentials.json",
			},
			expectErr: true,
			errMsg:    "project_id is required",
		},
		{
			name: "config with filter",
			config: map[string]interface{}{
				"project_id":       "my-project",
				"credentials_file": "/path/to/credentials.json",
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
				"project_id":           "my-project",
				"credentials_file":     "/path/to/credentials.json",
				"include_metadata":     true,
				"include_object_count": false,
				"tags":                 []string{"gcs", "storage"},
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
		"project_id":       "test-project",
		"credentials_file": "/path/to/creds.json",
	}

	_, err := s.Validate(plugin.RawPluginConfig(config))
	require.NoError(t, err)

	assert.NotNil(t, s.config)
	assert.Equal(t, "test-project", s.config.ProjectID)
	assert.Equal(t, "/path/to/creds.json", s.config.CredentialsFile)
}
