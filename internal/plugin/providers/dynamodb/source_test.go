package dynamodb

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    plugin.RawPluginConfig
		expectErr bool
	}{
		{
			name: "valid config with credentials",
			config: plugin.RawPluginConfig{
				"credentials": map[string]interface{}{
					"region": "us-east-1",
					"id":     "AKIAIOSFODNN7EXAMPLE",
					"secret": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				},
			},
			expectErr: false,
		},
		{
			name: "valid config with profile",
			config: plugin.RawPluginConfig{
				"credentials": map[string]interface{}{
					"region":  "us-west-2",
					"profile": "production",
				},
			},
			expectErr: false,
		},
		{
			name: "valid config with role assumption",
			config: plugin.RawPluginConfig{
				"credentials": map[string]interface{}{
					"region": "eu-west-1",
					"role":   "arn:aws:iam::123456789012:role/MyRole",
				},
			},
			expectErr: false,
		},
		{
			name:      "empty config",
			config:    plugin.RawPluginConfig{},
			expectErr: false,
		},
		{
			name: "config with tags",
			config: plugin.RawPluginConfig{
				"tags": []interface{}{"aws", "dynamodb"},
			},
			expectErr: false,
		},
		{
			name: "config with filter",
			config: plugin.RawPluginConfig{
				"filter": map[string]interface{}{
					"include": []interface{}{"^prod-.*"},
					"exclude": []interface{}{".*-temp$"},
				},
			},
			expectErr: false,
		},
		{
			name: "config with tags_to_metadata",
			config: plugin.RawPluginConfig{
				"tags_to_metadata": true,
				"credentials": map[string]interface{}{
					"region": "us-east-1",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_ValidateStoresConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{})
	require.NoError(t, err)
	assert.NotNil(t, s.config)
}
