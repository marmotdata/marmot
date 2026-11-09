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

func mappedConfig() *Config {
	return &Config{
		Host: "trino.example.com",
		Port: 8080,
		CatalogMap: map[string]CatalogMapping{
			"pg":   {Zone: "prod-eu", Database: "shop"},
			"my":   {Zone: "billing"},
			"ice":  {},
			"ch":   {},
			"dl":   {},
			"pgno": {Zone: "x"},
		},
	}
}

func TestTargetFor_MappedCatalogsGetTheNativeIdentity(t *testing.T) {
	cfg := mappedConfig()

	tests := []struct {
		name      string
		catalog   string
		connector string
		schema    string
		table     string
		wantMRN   string
	}{
		{"postgresql database comes from the map", "pg", "postgresql", "public", "products", "mrn://asset/table/postgresql@prod-eu/shop.public.products"},
		{"mysql schema is the database", "my", "mysql", "mydb", "users", "mrn://asset/table/mysql@billing/mydb.users"},
		{"clickhouse database and table", "ch", "clickhouse", "analytics", "events", "mrn://asset/table/clickhouse/analytics.events"},
		{"iceberg namespace and table", "ice", "iceberg", "warehouse", "orders", "mrn://asset/table/iceberg/warehouse.orders"},
		{"delta lake is named by table", "dl", "delta_lake", "default", "events", "mrn://asset/table/delta-lake/events"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := cfg.targetFor(tt.catalog, tt.connector, tt.schema, tt.table)
			assert.True(t, target.Native)
			assert.Equal(t, tt.wantMRN, target.mrn("Table"))
		})
	}
}

func TestTargetFor_UnmappedCatalogIsATrinoAsset(t *testing.T) {
	cfg := mappedConfig()

	// Two catalogs reading different PostgreSQL servers must never merge, so an unmapped one keeps its catalog in the path.
	a := cfg.targetFor("other_pg", "postgresql", "public", "products")
	b := cfg.targetFor("another_pg", "postgresql", "public", "products")

	assert.False(t, a.Native)
	assert.Equal(t, "mrn://asset/table/trino/other_pg.public.products", a.mrn("Table"))
	assert.NotEqual(t, a.mrn("Table"), b.mrn("Table"))
}

func TestTargetFor_ConnectorNeedingADatabaseWithoutOneIsATrinoAsset(t *testing.T) {
	target := mappedConfig().targetFor("pgno", "postgresql", "public", "products")

	assert.False(t, target.Native)
	assert.Equal(t, "mrn://asset/table/trino/pgno.public.products", target.mrn("Table"))
}

func TestTargetFor_UnknownConnectorIsATrinoAsset(t *testing.T) {
	cfg := mappedConfig()
	cfg.CatalogMap["future"] = CatalogMapping{Zone: "x"}

	target := cfg.targetFor("future", "some_future_connector", "s", "t")

	assert.False(t, target.Native)
	assert.Equal(t, "mrn://asset/table/trino/future.s.t", target.mrn("Table"))
}

func TestTargetFor_TrinoAssetsCarryTheTrinoZone(t *testing.T) {
	cfg := mappedConfig()
	cfg.Zone = "data-platform"

	target := cfg.targetFor("unmapped", "hive", "sales", "orders")

	assert.Equal(t, "mrn://asset/table/trino@data-platform/unmapped.sales.orders", target.mrn("Table"))
	// A mapped catalog uses the mapped zone, not Trino's own.
	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/shop.public.p", cfg.targetFor("pg", "postgresql", "public", "p").mrn("Table"))
}

func TestTargetFor_CatalogMapIgnoresCase(t *testing.T) {
	target := mappedConfig().targetFor("PG", "postgresql", "public", "products")
	assert.True(t, target.Native)
}

func TestTargetFor_InternalConnectorsAreNeverNative(t *testing.T) {
	cfg := &Config{CatalogMap: map[string]CatalogMapping{"mem": {}}}
	assert.False(t, cfg.targetFor("mem", "memory", "default", "t").Native)
}

func TestUnmappedReason(t *testing.T) {
	cfg := mappedConfig()

	assert.Empty(t, cfg.unmappedReason("pg", "postgresql"))
	assert.Contains(t, cfg.unmappedReason("other", "postgresql"), "not in catalog_map")
	assert.Contains(t, cfg.unmappedReason("pgno", "postgresql"), "no database")
	assert.Contains(t, cfg.unmappedReason("x", "some_future_connector"), "no native plugin")
}

func TestCreateTableAsset_MappedCatalog(t *testing.T) {
	s := &Source{config: mappedConfig()}

	a := s.createTableAsset("pg", "public", "products", "BASE TABLE", "postgresql")

	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/shop.public.products", *a.MRN)
	assert.Equal(t, "products", *a.Name)
	assert.Equal(t, []string{"PostgreSQL"}, a.Providers)
	assert.Equal(t, "Trino", a.Sources[0].Name, "source is always Trino")
	assert.Equal(t, "pg", a.Metadata["catalog"])
	assert.Equal(t, "public", a.Metadata["schema"])
	assert.Equal(t, "products", a.Metadata["table_name"])
	assert.Empty(t, a.PreviousMRNs)
}

func TestCreateTableAsset_ViewType(t *testing.T) {
	s := &Source{config: mappedConfig()}

	a := s.createTableAsset("pg", "public", "my_view", "VIEW", "postgresql")

	assert.Equal(t, "View", a.Type)
	assert.Equal(t, "mrn://asset/view/postgresql@prod-eu/shop.public.my_view", *a.MRN)
}

func TestCreateTableAsset_UnmappedCatalogIsTrinos(t *testing.T) {
	s := &Source{config: mappedConfig()}

	a := s.createTableAsset("lake", "sales", "orders", "BASE TABLE", "hive")

	assert.Equal(t, "mrn://asset/table/trino/lake.sales.orders", *a.MRN)
	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, []string{"Trino"}, a.Providers)
	assert.Equal(t, []string{"mrn://table/hive/lake.sales.orders"}, a.PreviousMRNs, "the catalog qualified identity earlier releases gave it")
}

func TestPreviousMRNs_OnlyTheCatalogQualifiedOnesAreClaimed(t *testing.T) {
	s := &Source{config: mappedConfig()}

	// A bare postgresql name was shared with the native plugin's own asset, so a Trino asset must not take it over.
	pg := s.createTableAsset("other_pg", "public", "products", "BASE TABLE", "postgresql")
	assert.Empty(t, pg.PreviousMRNs)

	unknown := s.createTableAsset("x", "s", "t", "BASE TABLE", "some_future_connector")
	assert.Equal(t, []string{"mrn://table/some_future_connector/x.s.t"}, unknown.PreviousMRNs)
}

func TestCreateCatalogAsset(t *testing.T) {
	s := &Source{config: mappedConfig()}
	s.config.Zone = "data-platform"

	a := s.createCatalogAsset("lake")

	assert.Equal(t, "mrn://asset/catalog/trino@data-platform/lake", *a.MRN)
	assert.Equal(t, "lake", *a.Name)
}

func TestCreateTableAsset_NameAndMRNAgree(t *testing.T) {
	s := &Source{config: mappedConfig()}

	for _, tc := range []struct{ catalog, connector string }{{"pg", "postgresql"}, {"ice", "iceberg"}, {"nope", "hive"}} {
		a := s.createTableAsset(tc.catalog, "sch", "orders", "BASE TABLE", tc.connector)

		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.Equal(t, *a.Name, parsed.Path()[len(parsed.Path())-1], tc.catalog)
	}
}

func TestParseConfig_CatalogMap(t *testing.T) {
	cfg, err := parseConfig(pluginsdk.RawConfig{
		"host": "localhost",
		"user": "marmot",
		"catalog_map": map[string]any{
			"pg": map[string]any{"zone": "prod-eu", "database": "shop"},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, CatalogMapping{Zone: "prod-eu", Database: "shop"}, cfg.CatalogMap["pg"])
}

func TestMeta_FederatesAndDeclaresCatalogMap(t *testing.T) {
	meta := Meta()
	assert.True(t, meta.Federates)

	var found bool
	for _, f := range meta.ConfigSpec {
		if f.Name == "catalog_map" {
			found = true
			assert.Equal(t, "Discovery", f.Group)
		}
	}
	assert.True(t, found)
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
