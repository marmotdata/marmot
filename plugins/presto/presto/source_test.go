package presto

import (
	"database/sql"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidMinimalConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "presto.example.com", "user": "marmot"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"user": "marmot"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_MissingUserFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestValidate_PortOutOfRangeFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "marmot", "port": 99999})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_PasswordWithoutHTTPSFails(t *testing.T) {
	// The driver drops the password on plain HTTP, so a config that
	// looks authenticated would silently connect anonymously.
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "marmot", "password": "secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secure")
}

func TestValidate_PasswordWithHTTPSIsAccepted(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "marmot", "password": "secret", "secure": true})
	require.NoError(t, err)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "marmot"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 8080, s.config.Port)
	assert.False(t, s.config.Secure)
	assert.Equal(t, []string{"system", "jmx"}, s.config.ExcludeCatalogs)
	assert.Empty(t, s.config.ExcludeSchemas)
	assert.True(t, s.config.IncludeCatalogs)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludeStats)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":             "localhost",
		"user":             "marmot",
		"include_catalogs": false,
		"include_views":    false,
		"include_columns":  false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeCatalogs)
	assert.False(t, s.config.IncludeViews)
	assert.False(t, s.config.IncludeColumns)
}

func TestValidate_KeepsExplicitExclusions(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":             "localhost",
		"user":             "marmot",
		"exclude_catalogs": []any{"system"},
		"exclude_schemas":  []any{"sf1", "sf100"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"system"}, s.config.ExcludeCatalogs)
	assert.Equal(t, []string{"sf1", "sf100"}, s.config.ExcludeSchemas)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "localhost",
		"user": "marmot",
		"filter": map[string]any{
			"include": []any{"^orders"},
		},
	})
	require.NoError(t, err)
}

func TestIsExcludedSchema_AlwaysSkipsInformationSchema(t *testing.T) {
	s := &Source{config: &Config{}}
	assert.True(t, s.isExcludedSchema("information_schema"))
	assert.True(t, s.isExcludedSchema("INFORMATION_SCHEMA"))
	assert.False(t, s.isExcludedSchema("tiny"))
}

func TestIsExcludedSchema_IgnoresCase(t *testing.T) {
	s := &Source{config: &Config{ExcludeSchemas: []string{"sf1"}}}
	assert.True(t, s.isExcludedSchema("SF1"))
	assert.False(t, s.isExcludedSchema("sf100"))
}

func TestExcludedSchemaList_RendersLowercasedQuotedNames(t *testing.T) {
	s := &Source{config: &Config{ExcludeSchemas: []string{"SF1", "it's"}}}
	assert.Equal(t, `'information_schema', 'sf1', 'it''s'`, s.excludedSchemaList())
}

func TestContainsFold(t *testing.T) {
	assert.True(t, containsFold([]string{"system", "jmx"}, "JMX"))
	assert.False(t, containsFold([]string{"system", "jmx"}, "hive"))
	assert.False(t, containsFold(nil, "hive"))
}

func TestNewColumn_KeepsNestedTypesVerbatim(t *testing.T) {
	col := newColumn(columnRow{
		Name:            "r",
		OrdinalPosition: 1,
		IsNullable:      "YES",
		DataType:        `row("id" integer, "name" varchar)`,
	})

	assert.Equal(t, `row("id" integer, "name" varchar)`, col.DataType)
	assert.Equal(t, "r", col.Name)
	assert.Equal(t, int64(1), col.OrdinalPosition)
	assert.True(t, col.Nullable)
}

func TestNewColumn_MapsNullabilityCommentAndDefault(t *testing.T) {
	col := newColumn(columnRow{
		Name:       "nationkey",
		IsNullable: "NO",
		DataType:   "bigint",
		Default:    sql.NullString{String: "0", Valid: true},
		Comment:    sql.NullString{String: "Nation key", Valid: true},
	})

	assert.False(t, col.Nullable)
	assert.Equal(t, "Nation key", col.Description)
	assert.Equal(t, "0", col.Default)
}

func TestNewColumn_LeavesDefaultUnsetWhenNull(t *testing.T) {
	col := newColumn(columnRow{Name: "x", IsNullable: "YES", DataType: "varchar(25)"})

	assert.Nil(t, col.Default)
	assert.Empty(t, col.Description)
}

func TestCreateTableAsset_PrestoNativeCatalogUsesTheFullPath(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory", connectorInfoForName("memory"))

	assert.Equal(t, "memory.shop.orders", *a.Name)
	assert.Equal(t, "mrn://table/presto/memory.shop.orders", *a.MRN)
	assert.Equal(t, "Table", a.Type)
	assert.Equal(t, []string{"Presto"}, a.Providers)
	assert.Equal(t, "Presto", a.Sources[0].Name)
	assert.Equal(t, "memory", a.Metadata["catalog"])
	assert.Equal(t, "shop", a.Metadata["schema"])
	assert.Equal(t, "orders", a.Metadata["table_name"])
	assert.Equal(t, "BASE TABLE", a.Metadata["table_type"])
	assert.Equal(t, "memory", a.Metadata["connector_name"])
}

func TestCreateTableAsset_MappedConnectorUsesTheNativeProvider(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createTableAsset("pg", "public", "orders", "BASE TABLE", "postgresql", connectorInfoForName("postgresql"))

	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, "mrn://table/postgresql/orders", *a.MRN)
	assert.Equal(t, []string{"PostgreSQL"}, a.Providers)
	assert.Equal(t, "Presto", a.Sources[0].Name, "the source is always Presto")
}

func TestCreateTableAsset_ViewGetsTheViewType(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createTableAsset("memory", "shop", "big_orders", "VIEW", "memory", connectorInfoForName("memory"))

	assert.Equal(t, "View", a.Type)
	assert.Equal(t, "mrn://view/presto/memory.shop.big_orders", *a.MRN)
}

func TestCreateTableAsset_OmitsUnknownConnector(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createTableAsset("x", "s", "t", "BASE TABLE", "", connectorInfoForName(""))

	_, present := a.Metadata["connector_name"]
	assert.False(t, present)
	assert.Equal(t, []string{"Presto"}, a.Providers)
}

func TestCreateCatalogAsset_CountsTablesAndViews(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}
	tables := []pluginsdk.Asset{{Type: "Table"}, {Type: "Table"}, {Type: "View"}}

	a := s.createCatalogAsset("memory", "memory", "0.299", 2, tables)

	assert.Equal(t, "memory", *a.Name)
	assert.Equal(t, "mrn://catalog/presto/memory", *a.MRN)
	assert.Equal(t, "Catalog", a.Type)
	assert.Equal(t, []string{"Presto"}, a.Providers)
	assert.Equal(t, "memory", a.Metadata["connector_name"])
	assert.Equal(t, 2, a.Metadata["schema_count"])
	assert.Equal(t, 2, a.Metadata["table_count"])
	assert.Equal(t, 1, a.Metadata["view_count"])
	assert.Equal(t, "localhost", a.Metadata["host"])
	assert.Equal(t, 8080, a.Metadata["port"])
	assert.Equal(t, "0.299", a.Metadata["presto_version"])
}

func TestCreateCatalogAsset_OmitsEmptyVersionAndConnector(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createCatalogAsset("tpch", "", "", 0, nil)

	_, hasVersion := a.Metadata["presto_version"]
	_, hasConnector := a.Metadata["connector_name"]
	assert.False(t, hasVersion)
	assert.False(t, hasConnector)
}

func TestCreateTableAsset_InterpolatesTags(t *testing.T) {
	s := &Source{config: &Config{BaseConfig: pluginsdk.BaseConfig{Tags: pluginsdk.TagsConfig{"catalog:${catalog}", "presto"}}}}

	a := s.createTableAsset("tpch", "tiny", "nation", "BASE TABLE", "tpch", connectorInfoForName("tpch"))

	assert.Equal(t, []string{"catalog:tpch", "presto"}, a.Tags)
}
