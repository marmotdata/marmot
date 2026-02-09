package trino

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
			name: "valid minimal config",
			config: plugin.RawPluginConfig{
				"host": "trino.example.com",
				"user": "marmot",
			},
		},
		{
			name: "valid full config",
			config: plugin.RawPluginConfig{
				"host":             "trino.example.com",
				"port":             8443,
				"user":             "marmot",
				"secure":           true,
				"catalog":          "hive",
				"exclude_catalogs": []interface{}{"system"},
				"include_columns":  false,
			},
		},
		{
			name:    "missing host",
			config:  plugin.RawPluginConfig{"user": "marmot"},
			wantErr: "host",
		},
		{
			name:    "missing user",
			config:  plugin.RawPluginConfig{"host": "localhost"},
			wantErr: "user",
		},
		{
			name: "invalid port",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"user": "marmot",
				"port": 99999,
			},
			wantErr: "port",
		},
		{
			name: "with filter",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"user": "marmot",
				"filter": map[string]interface{}{
					"include": []interface{}{"^orders"},
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
		"user": "marmot",
	})
	require.NoError(t, err)

	assert.Equal(t, 8080, s.config.Port)
	assert.True(t, s.config.IncludeCatalogs)
	assert.True(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludeStats)
	assert.Equal(t, []string{"system", "jmx"}, s.config.ExcludeCatalogs)
	assert.Equal(t, 100, s.config.AIMaxEnrichments)
}

func TestSource_ValidateBoolOverrides(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"host":             "localhost",
		"user":             "marmot",
		"include_catalogs": false,
		"include_columns":  false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeCatalogs)
	assert.False(t, s.config.IncludeColumns)
}

func TestIsExcludedCatalog(t *testing.T) {
	s := &Source{
		config: &Config{
			ExcludeCatalogs: []string{"system", "jmx"},
		},
	}

	assert.True(t, s.isExcludedCatalog("system"))
	assert.True(t, s.isExcludedCatalog("System"))
	assert.True(t, s.isExcludedCatalog("JMX"))
	assert.False(t, s.isExcludedCatalog("hive"))
	assert.False(t, s.isExcludedCatalog("iceberg"))
}

func TestBuildColumnSummary(t *testing.T) {
	t.Run("with columns", func(t *testing.T) {
		schema := map[string]string{
			"columns": `[{"column_name":"id","data_type":"bigint"},{"column_name":"name","data_type":"varchar"},{"column_name":"created_at","data_type":"timestamp"}]`,
		}
		result := buildColumnSummary(schema)
		assert.Equal(t, "id bigint, name varchar, created_at timestamp", result)
	})

	t.Run("no columns key", func(t *testing.T) {
		schema := map[string]string{}
		result := buildColumnSummary(schema)
		assert.Equal(t, "(no column info)", result)
	})

	t.Run("empty columns", func(t *testing.T) {
		schema := map[string]string{
			"columns": `[]`,
		}
		result := buildColumnSummary(schema)
		assert.Equal(t, "(no column info)", result)
	})
}

func TestConnectorMRNNames(t *testing.T) {
	tests := []struct {
		name      string
		connector string
		catalog   string
		schema    string
		table     string
		wantName  string
	}{
		{"postgresql table only", "postgresql", "pg", "public", "products", "products"},
		{"mysql table only", "mysql", "my", "mydb", "users", "users"},
		{"clickhouse schema.table", "clickhouse", "ch", "analytics", "events", "analytics.events"},
		{"mongodb table only", "mongodb", "mongo", "mydb", "users", "users"},
		{"iceberg full path", "iceberg", "ice", "warehouse", "orders", "ice.warehouse.orders"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := connectorMap[tt.connector]
			got := info.MRNName(tt.catalog, tt.schema, tt.table)
			assert.Equal(t, tt.wantName, got)
		})
	}
}

func TestCreateTableAsset_NativeProvider(t *testing.T) {
	s := &Source{
		config: &Config{
			BaseConfig: plugin.BaseConfig{},
			Host:       "trino.example.com",
			Port:       8080,
		},
	}

	tests := []struct {
		name         string
		connector    string
		catalog      string
		schema       string
		table        string
		wantMRN      string
		wantName     string
		wantProvider string
	}{
		{
			name:         "postgresql produces native MRN",
			connector:    "postgresql",
			catalog:      "pg",
			schema:       "public",
			table:        "products",
			wantMRN:      "mrn://table/postgresql/products",
			wantName:     "products",
			wantProvider: "PostgreSQL",
		},
		{
			name:         "clickhouse produces native MRN",
			connector:    "clickhouse",
			catalog:      "ch",
			schema:       "analytics",
			table:        "events",
			wantMRN:      "mrn://table/clickhouse/analytics.events",
			wantName:     "analytics.events",
			wantProvider: "ClickHouse",
		},
		{
			name:         "iceberg produces native MRN",
			connector:    "iceberg",
			catalog:      "ice",
			schema:       "warehouse",
			table:        "orders",
			wantMRN:      "mrn://table/iceberg/ice.warehouse.orders",
			wantName:     "ice.warehouse.orders",
			wantProvider: "Iceberg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := connectorMap[tt.connector]

			a := s.createTableAsset(tt.catalog, tt.schema, tt.table, "BASE TABLE", info)
			assert.Equal(t, tt.wantMRN, *a.MRN)
			assert.Equal(t, tt.wantName, *a.Name)
			assert.Equal(t, []string{tt.wantProvider}, a.Providers)
			assert.Equal(t, "Trino", a.Sources[0].Name, "source is always Trino")
			assert.Equal(t, tt.catalog, a.Metadata["catalog"])
			assert.Equal(t, tt.schema, a.Metadata["schema"])
			assert.Equal(t, tt.table, a.Metadata["table_name"])
		})
	}

	t.Run("view type", func(t *testing.T) {
		info := connectorMap["postgresql"]
		a := s.createTableAsset("pg", "public", "my_view", "VIEW", info)
		assert.Equal(t, "View", a.Type)
		assert.Equal(t, "mrn://view/postgresql/my_view", *a.MRN)
	})
}

func TestConnectorInfoForName(t *testing.T) {
	// Internal connectors are skipped
	for _, c := range []string{"memory", "tpch", "tpcds", "blackhole", "localfile"} {
		_, ok := connectorInfoForName(c)
		assert.False(t, ok, "%s should be skipped", c)
	}

	// Known connectors return their native provider
	info, ok := connectorInfoForName("postgresql")
	assert.True(t, ok)
	assert.Equal(t, "PostgreSQL", info.Provider)

	info, ok = connectorInfoForName("snowflake")
	assert.True(t, ok)
	assert.Equal(t, "Snowflake", info.Provider)

	// Unknown external connectors get a default mapping
	info, ok = connectorInfoForName("some_future_connector")
	assert.True(t, ok)
	assert.Equal(t, "some_future_connector", info.Provider)
	assert.Equal(t, "cat.sch.tbl", info.MRNName("cat", "sch", "tbl"))
}

func TestQuoteIdentifier(t *testing.T) {
	assert.Equal(t, `"catalog"`, quoteIdentifier("catalog"))
	assert.Equal(t, `"my""catalog"`, quoteIdentifier(`my"catalog`))
}

func TestEscapeString(t *testing.T) {
	assert.Equal(t, "hello", escapeString("hello"))
	assert.Equal(t, "it''s", escapeString("it's"))
	assert.Equal(t, "a''b''c", escapeString("a'b'c"))
}
