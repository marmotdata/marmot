package trino

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
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
			name: "valid minimal config",
			config: pluginsdk.RawConfig{
				"host": "trino.example.com",
				"user": "marmot",
			},
		},
		{
			name: "valid full config",
			config: pluginsdk.RawConfig{
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
			config:  pluginsdk.RawConfig{"user": "marmot"},
			wantErr: "host",
		},
		{
			name:    "missing user",
			config:  pluginsdk.RawConfig{"host": "localhost"},
			wantErr: "user",
		},
		{
			name: "invalid port",
			config: pluginsdk.RawConfig{
				"host": "localhost",
				"user": "marmot",
				"port": 99999,
			},
			wantErr: "port",
		},
		{
			name: "with filter",
			config: pluginsdk.RawConfig{
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
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "localhost",
		"user": "marmot",
	})
	require.NoError(t, err)

	assert.Equal(t, 8080, s.config.Port)
	assert.True(t, s.config.IncludeCatalogs)
	assert.True(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludeStats)
	assert.Equal(t, []string{"system", "jmx"}, s.config.ExcludeCatalogs)
	assert.Equal(t, 0, s.config.AIMaxEnrichments)
}

func TestSource_ValidateBoolOverrides(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
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
		// PostgreSQL, MySQL and MongoDB all have a Marmot plugin that
		// names a table by its bare object name, so this map matches them
		// and the two sources land on one asset. See the mrn_test.go in
		// each of those plugins for why they stay bare.
		{"postgresql bare table", "postgresql", "pg", "public", "products", "products"},
		{"mysql bare table", "mysql", "my", "mydb", "users", "users"},
		{"mongodb bare collection", "mongodb", "mongo", "mydb", "users", "users"},
		// ClickHouse's plugin declares schema.table, so this follows it.
		{"clickhouse schema.table", "clickhouse", "ch", "analytics", "events", "analytics.events"},
		// Connectors with no matching convention fall back to the full
		// catalog.schema.table path, with Trino as the authority.
		{"iceberg full path", "iceberg", "ice", "warehouse", "orders", "ice.warehouse.orders"},
		{"delta lake full path", "delta_lake", "dl", "default", "events", "dl.default.events"},
		{"hive full path", "hive", "hv", "warehouse", "orders", "hv.warehouse.orders"},
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
			BaseConfig: pluginsdk.BaseConfig{},
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
			name:         "iceberg falls back to the full path",
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

func TestCreateTableAsset_NameIsTheStringTheMRNIsBuiltFrom(t *testing.T) {
	// Name and the MRN's name segment carry the same string, even where
	// that means a Trino asset reads as "analytics.events".
	s := &Source{config: &Config{Host: "trino.example.com", Port: 8080}}

	for _, connector := range []string{"postgresql", "clickhouse", "iceberg", "hive"} {
		info := connectorMap[connector]
		a := s.createTableAsset("cat", "sch", "orders", "BASE TABLE", info)

		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		assert.Equal(t, mrn.New("Table", a.Providers[0], *a.Name), *a.MRN,
			"%s: MRN and Name must agree", connector)
	}
}

func TestCreateTableAsset_ProviderWithASpaceLandsInTheMRN(t *testing.T) {
	// mrn.New sanitizes the name it is given but not the service, so
	// "Delta Lake" puts a literal space in the MRN. This is ugly and it is
	// deliberate: it is what the published Trino plugin already emits, and
	// slugging it here would rename every Delta Lake asset a previous run
	// wrote without a migration to move them.
	s := &Source{config: &Config{Host: "trino.example.com", Port: 8080}}

	a := s.createTableAsset("dl", "default", "events", "BASE TABLE", connectorMap["delta_lake"])

	assert.Equal(t, "mrn://table/delta lake/dl.default.events", *a.MRN)
	assert.Equal(t, []string{"Delta Lake"}, a.Providers)
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
