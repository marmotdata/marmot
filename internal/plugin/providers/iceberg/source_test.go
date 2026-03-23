package iceberg

import (
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
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
			name: "valid REST config",
			config: plugin.RawPluginConfig{
				"uri": "http://localhost:8181",
			},
			expectErr: false,
		},
		{
			name: "valid config with all options",
			config: plugin.RawPluginConfig{
				"uri":        "https://catalog.example.com",
				"warehouse":  "my-warehouse",
				"credential": "client-id:client-secret",
				"token":      "my-bearer-token",
				"prefix":     "v1",
				"properties": map[string]interface{}{
					"custom-key": "custom-value",
				},
				"include_namespaces": true,
				"include_views":      false,
			},
			expectErr: false,
		},
		{
			name:      "missing URI defaults to REST and requires URI",
			config:    plugin.RawPluginConfig{},
			expectErr: true,
		},
		{
			name: "invalid URI",
			config: plugin.RawPluginConfig{
				"uri": "not-a-valid-url",
			},
			expectErr: true,
		},
		{
			name: "config with tags",
			config: plugin.RawPluginConfig{
				"uri":  "http://localhost:8181",
				"tags": []interface{}{"iceberg", "data-lake"},
			},
			expectErr: false,
		},
		{
			name: "config with filter",
			config: plugin.RawPluginConfig{
				"uri": "http://localhost:8181",
				"filter": map[string]interface{}{
					"include": []interface{}{"^prod\\..*"},
					"exclude": []interface{}{".*_temp$"},
				},
			},
			expectErr: false,
		},
		{
			name: "valid Glue config",
			config: plugin.RawPluginConfig{
				"catalog_type": "glue",
				"credentials": map[string]interface{}{
					"region": "us-east-1",
				},
			},
			expectErr: false,
		},
		{
			name: "valid Glue config with catalog ID",
			config: plugin.RawPluginConfig{
				"catalog_type":    "glue",
				"glue_catalog_id": "123456789012",
				"credentials": map[string]interface{}{
					"region": "us-east-1",
				},
			},
			expectErr: false,
		},
		{
			name: "Glue config does not require URI",
			config: plugin.RawPluginConfig{
				"catalog_type": "glue",
			},
			expectErr: false,
		},
		{
			name: "REST config requires URI",
			config: plugin.RawPluginConfig{
				"catalog_type": "rest",
			},
			expectErr: true,
		},
		{
			name: "invalid catalog_type",
			config: plugin.RawPluginConfig{
				"catalog_type": "hive",
			},
			expectErr: true,
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
	t.Run("REST config", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.NotNil(t, s.config)
		assert.Equal(t, "http://localhost:8181", s.config.URI)
		assert.Equal(t, "rest", s.config.CatalogType)
	})

	t.Run("Glue config", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"catalog_type":    "glue",
			"glue_catalog_id": "123456789012",
		})
		require.NoError(t, err)
		assert.NotNil(t, s.config)
		assert.Equal(t, "glue", s.config.CatalogType)
		assert.Equal(t, "123456789012", s.config.GlueCatalogID)
	})
}

func TestSource_ValidateDefaultCatalogType(t *testing.T) {
	t.Run("defaults to rest", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.Equal(t, "rest", s.config.CatalogType)
	})

	t.Run("respects explicit glue", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"catalog_type": "glue",
		})
		require.NoError(t, err)
		assert.Equal(t, "glue", s.config.CatalogType)
	})
}

func TestSource_ValidateDefaultBoolFields(t *testing.T) {
	t.Run("defaults to true when not set", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.True(t, s.config.IncludeNamespaces)
		assert.True(t, s.config.IncludeViews)
	})

	t.Run("respects explicit false", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"uri":                "http://localhost:8181",
			"include_namespaces": false,
			"include_views":      false,
		})
		require.NoError(t, err)
		assert.False(t, s.config.IncludeNamespaces)
		assert.False(t, s.config.IncludeViews)
	})

	t.Run("respects explicit true", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(plugin.RawPluginConfig{
			"uri":                "http://localhost:8181",
			"include_namespaces": true,
			"include_views":      true,
		})
		require.NoError(t, err)
		assert.True(t, s.config.IncludeNamespaces)
		assert.True(t, s.config.IncludeViews)
	})
}

func TestNamespaceFromAssetMRN(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected string
	}{
		{
			name:     "single namespace",
			mrn:      "mrn://table/iceberg/db.orders",
			expected: "mrn://namespace/iceberg/db",
		},
		{
			name:     "nested namespace",
			mrn:      "mrn://table/iceberg/catalog.schema.orders",
			expected: "mrn://namespace/iceberg/catalog.schema",
		},
		{
			name:     "view MRN",
			mrn:      "mrn://view/iceberg/db.my_view",
			expected: "mrn://namespace/iceberg/db",
		},
		{
			name:     "no namespace separator",
			mrn:      "mrn://table/iceberg/tablename",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := asset.Asset{
				MRN:      &tt.mrn,
				Metadata: map[string]interface{}{},
			}
			result := namespaceFromAssetMRN(a)
			assert.Equal(t, tt.expected, result)
		})
	}
}
