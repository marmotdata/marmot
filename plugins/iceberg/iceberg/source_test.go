package iceberg

import (
	"testing"

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
			name: "valid REST config",
			config: pluginsdk.RawConfig{
				"uri": "http://localhost:8181",
			},
			expectErr: false,
		},
		{
			name: "valid config with all options",
			config: pluginsdk.RawConfig{
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
			config:    pluginsdk.RawConfig{},
			expectErr: true,
		},
		{
			name: "invalid URI",
			config: pluginsdk.RawConfig{
				"uri": "not-a-valid-url",
			},
			expectErr: true,
		},
		{
			name: "config with tags",
			config: pluginsdk.RawConfig{
				"uri":  "http://localhost:8181",
				"tags": []interface{}{"iceberg", "data-lake"},
			},
			expectErr: false,
		},
		{
			name: "config with filter",
			config: pluginsdk.RawConfig{
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
			config: pluginsdk.RawConfig{
				"catalog_type": "glue",
				"credentials": map[string]interface{}{
					"region": "us-east-1",
				},
			},
			expectErr: false,
		},
		{
			name: "valid Glue config with catalog ID",
			config: pluginsdk.RawConfig{
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
			config: pluginsdk.RawConfig{
				"catalog_type": "glue",
			},
			expectErr: false,
		},
		{
			name: "REST config requires URI",
			config: pluginsdk.RawConfig{
				"catalog_type": "rest",
			},
			expectErr: true,
		},
		{
			name: "invalid catalog_type",
			config: pluginsdk.RawConfig{
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
		_, err := s.Validate(pluginsdk.RawConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.NotNil(t, s.config)
		assert.Equal(t, "http://localhost:8181", s.config.URI)
		assert.Equal(t, "rest", s.config.CatalogType)
	})

	t.Run("Glue config", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
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
		_, err := s.Validate(pluginsdk.RawConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.Equal(t, "rest", s.config.CatalogType)
	})

	t.Run("respects explicit glue", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
			"catalog_type": "glue",
		})
		require.NoError(t, err)
		assert.Equal(t, "glue", s.config.CatalogType)
	})
}

func TestSource_ValidateDefaultBoolFields(t *testing.T) {
	t.Run("defaults to true when not set", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
			"uri": "http://localhost:8181",
		})
		require.NoError(t, err)
		assert.True(t, s.config.IncludeNamespaces)
		assert.True(t, s.config.IncludeViews)
	})

	t.Run("respects explicit false", func(t *testing.T) {
		s := &Source{}
		_, err := s.Validate(pluginsdk.RawConfig{
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
		_, err := s.Validate(pluginsdk.RawConfig{
			"uri":                "http://localhost:8181",
			"include_namespaces": true,
			"include_views":      true,
		})
		require.NoError(t, err)
		assert.True(t, s.config.IncludeNamespaces)
		assert.True(t, s.config.IncludeViews)
	})
}

// namespaceFromAssetMRN cuts the last level off the child's parsed path. A level holding a dot is quoted in the MRN, so it is never split in two.
func TestNamespaceFromAssetMRN(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected string
	}{
		{"single namespace", "mrn://asset/table/iceberg/db.orders", "mrn://asset/namespace/iceberg/db"},
		{"nested namespace", "mrn://asset/table/iceberg/catalog.schema.orders", "mrn://asset/namespace/iceberg/catalog.schema"},
		{"view MRN", "mrn://asset/view/iceberg/db.my_view", "mrn://asset/namespace/iceberg/db"},
		{"table name containing a dot", `mrn://asset/table/iceberg/db."orders.2024"`, "mrn://asset/namespace/iceberg/db"},
		{"zone is kept", "mrn://asset/table/iceberg@prod/db.orders", "mrn://asset/namespace/iceberg@prod/db"},
		{"table at the catalog root has no namespace", "mrn://asset/table/iceberg/tablename", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pluginsdk.Asset{MRN: &tt.mrn}
			assert.Equal(t, tt.expected, namespaceFromAssetMRN(a))
		})
	}
}

func newMRNSource(zone string) *Source {
	cfg := &Config{}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestAssetMRN_IsEveryIdentifierLevel(t *testing.T) {
	s := newMRNSource("")

	table, previous := s.assetMRN("Table", []string{"sales", "orders"})
	assert.Equal(t, "mrn://asset/table/iceberg/sales.orders", table)
	assert.Nil(t, previous, "a plain name keeps the MRN earlier releases gave it")

	ns, _ := s.assetMRN("Namespace", []string{"lake", "sales"})
	assert.Equal(t, "mrn://asset/namespace/iceberg/lake.sales", ns)
}

func TestAssetMRN_SameTableInTwoNamespacesDiffers(t *testing.T) {
	s := newMRNSource("")
	a, _ := s.assetMRN("Table", []string{"sales", "orders"})
	b, _ := s.assetMRN("Table", []string{"archive", "orders"})
	assert.NotEqual(t, a, b)
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	got, _ := newMRNSource("prod-eu").assetMRN("Table", []string{"sales", "orders"})
	assert.Equal(t, "mrn://asset/table/iceberg@prod-eu/sales.orders", got)
}

func TestAssetMRN_DottedLevelKeepsThePreviousMRN(t *testing.T) {
	got, previous := newMRNSource("").assetMRN("Table", []string{"db", "orders.2024"})
	assert.Equal(t, `mrn://asset/table/iceberg/db."orders.2024"`, got)
	assert.Equal(t, []string{"mrn://table/iceberg/db.orders.2024"}, previous)
}

func TestZoneBindings_RESTHostOnly(t *testing.T) {
	cfg := &Config{CatalogType: "rest", URI: "https://Catalog.Internal:8181/api"}
	cfg.Zone = "prod"
	assert.Equal(t, []pluginsdk.ZoneBinding{{Provider: "Iceberg", Host: "catalog.internal:8181", Zone: "prod"}},
		(&Source{config: cfg}).zoneBindings())

	cfg = &Config{CatalogType: "rest", URI: "https://catalog.internal"}
	assert.Equal(t, "catalog.internal:443", (&Source{config: cfg}).zoneBindings()[0].Host)

	assert.Nil(t, (&Source{config: &Config{CatalogType: "glue"}}).zoneBindings())
}

// namespacePath is what createTableAsset and createViewAsset record, so it
// has to be the levels above the object's own name.
func TestNamespacePath_IsEveryLevelAboveTheObject(t *testing.T) {
	assert.Equal(t, "db", namespacePath([]string{"db", "orders"}))
	assert.Equal(t, "catalog.schema", namespacePath([]string{"catalog", "schema", "orders"}))
	assert.Equal(t, "", namespacePath([]string{"orders"}),
		"a table at the catalog root has no namespace")
}
