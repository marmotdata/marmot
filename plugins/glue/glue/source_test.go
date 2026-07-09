package glue

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    pluginsdk.RawConfig
		expectErr bool
	}{
		{
			name: "valid config with credentials",
			config: pluginsdk.RawConfig{
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
			config: pluginsdk.RawConfig{
				"credentials": map[string]interface{}{
					"region":  "us-west-2",
					"profile": "production",
				},
			},
			expectErr: false,
		},
		{
			name: "valid config with role assumption",
			config: pluginsdk.RawConfig{
				"credentials": map[string]interface{}{
					"region": "eu-west-1",
					"role":   "arn:aws:iam::123456789012:role/MyRole",
				},
			},
			expectErr: false,
		},
		{
			name:      "empty config",
			config:    pluginsdk.RawConfig{},
			expectErr: false,
		},
		{
			name: "config with tags",
			config: pluginsdk.RawConfig{
				"tags": []interface{}{"aws", "glue"},
			},
			expectErr: false,
		},
		{
			name: "config with filter",
			config: pluginsdk.RawConfig{
				"filter": map[string]interface{}{
					"include": []interface{}{"^prod-.*"},
					"exclude": []interface{}{".*-temp$"},
				},
			},
			expectErr: false,
		},
		{
			name: "config with tags_to_metadata",
			config: pluginsdk.RawConfig{
				"tags_to_metadata": true,
				"credentials": map[string]interface{}{
					"region": "us-east-1",
				},
			},
			expectErr: false,
		},
		{
			name: "config with discovery flags",
			config: pluginsdk.RawConfig{
				"discover_jobs":      true,
				"discover_databases": false,
				"discover_tables":    false,
				"discover_crawlers":  true,
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
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)
	assert.NotNil(t, s.config)
}

func TestSource_ValidateDefaultBoolFields(t *testing.T) {
	t.Run("defaults to true when not set", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{})
		require.NoError(t, err)
		assert.True(t, s.config.DiscoverJobs)
		assert.True(t, s.config.DiscoverDatabases)
		assert.True(t, s.config.DiscoverTables)
		assert.True(t, s.config.DiscoverCrawlers)
	})

	t.Run("respects explicit false", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
			"discover_jobs":      false,
			"discover_databases": false,
			"discover_tables":    false,
			"discover_crawlers":  false,
		})
		require.NoError(t, err)
		assert.False(t, s.config.DiscoverJobs)
		assert.False(t, s.config.DiscoverDatabases)
		assert.False(t, s.config.DiscoverTables)
		assert.False(t, s.config.DiscoverCrawlers)
	})

	t.Run("respects explicit true", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
			"discover_jobs":      true,
			"discover_databases": true,
			"discover_tables":    true,
			"discover_crawlers":  true,
		})
		require.NoError(t, err)
		assert.True(t, s.config.DiscoverJobs)
		assert.True(t, s.config.DiscoverDatabases)
		assert.True(t, s.config.DiscoverTables)
		assert.True(t, s.config.DiscoverCrawlers)
	})
}

func TestIsIcebergTable(t *testing.T) {
	tests := []struct {
		name     string
		table    types.Table
		expected bool
	}{
		{
			name: "ICEBERG uppercase",
			table: types.Table{
				Parameters: map[string]string{
					"table_type": "ICEBERG",
				},
			},
			expected: true,
		},
		{
			name: "iceberg lowercase",
			table: types.Table{
				Parameters: map[string]string{
					"table_type": "iceberg",
				},
			},
			expected: true,
		},
		{
			name: "non-iceberg table",
			table: types.Table{
				Parameters: map[string]string{
					"table_type": "HIVE",
				},
			},
			expected: false,
		},
		{
			name: "nil parameters",
			table: types.Table{
				Parameters: nil,
			},
			expected: false,
		},
		{
			name: "empty parameters",
			table: types.Table{
				Parameters: map[string]string{},
			},
			expected: false,
		},
		{
			name: "no table_type key",
			table: types.Table{
				Parameters: map[string]string{
					"classification": "parquet",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIcebergTable(tt.table)
			assert.Equal(t, tt.expected, result)
		})
	}
}
