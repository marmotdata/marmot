package timescale

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host": "timescale.internal",
		"user": "marmot_reader",
	}
}

func TestValidate_AcceptsAMinimalConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"user": "marmot_reader"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestValidate_MissingUserFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "timescale.internal"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestValidate_DefaultsThePort(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.Equal(t, 5432, s.config.Port)
}

func TestValidate_DefaultsSSLModeToDisable(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.Equal(t, "disable", s.config.SSLMode)
}

func TestValidate_RejectsAnUnknownSSLMode(t *testing.T) {
	config := validConfig()
	config["ssl_mode"] = "sometimes"

	s := &Source{}
	_, err := s.Validate(config)
	require.Error(t, err)
}

func TestValidate_DefaultsDiscoveryFlagsToOn(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeStatistics)
	assert.True(t, s.config.DiscoverForeignKeys)
	assert.True(t, s.config.ExcludeSystemSchemas)
}

func TestValidate_DefaultsTimescaleFlagsToOn(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.True(t, s.config.IncludeHypertables)
	assert.True(t, s.config.IncludeContinuousAggregates)
	assert.True(t, s.config.IncludeCompression)
}

// Reading every chunk of a busy hypertable is expensive, so the chunk range
// is opt in while the cheap chunk count is always recorded.
func TestValidate_DefaultsIncludeChunksToOff(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.False(t, s.config.IncludeChunks)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	config := validConfig()
	config["include_columns"] = false
	config["include_compression"] = false

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.False(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludeCompression)
	// Untouched flags still default to on.
	assert.True(t, s.config.IncludeStatistics)
	assert.True(t, s.config.IncludeHypertables)
}

func TestValidate_RespectsExplicitTrueForIncludeChunks(t *testing.T) {
	config := validConfig()
	config["include_chunks"] = true

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.True(t, s.config.IncludeChunks)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	config := validConfig()
	config["filter"] = map[string]interface{}{
		"include": []interface{}{"^conditions.*"},
		"exclude": []interface{}{".*_tmp$"},
	}

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)
}

func TestValidate_AcceptsExcludeDatabases(t *testing.T) {
	config := validConfig()
	config["exclude_databases"] = []interface{}{"postgres", "scratch"}

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.Equal(t, []string{"postgres", "scratch"}, s.config.ExcludeDatabases)
}

func TestMeta_DeclaresTheTimescaleIdentity(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "timescale", meta.ID)
	assert.Equal(t, "TimescaleDB", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, "postgresql", meta.Icon)
}

func TestMeta_DeclaresAssetsAndLineage(t *testing.T) {
	meta := Meta()

	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestMeta_ExposesTheTimescaleConfigFields(t *testing.T) {
	names := make(map[string]bool)
	for _, field := range Meta().ConfigSpec {
		names[field.Name] = true
	}

	assert.True(t, names["include_hypertables"])
	assert.True(t, names["include_continuous_aggregates"])
	assert.True(t, names["include_compression"])
	assert.True(t, names["include_chunks"])
}

// Source has to satisfy DataFetcher for the host to offer a data preview.
func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = (*Source)(nil)
}

func TestBuildAssets_TableBecomesATableAsset(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable, Owner: "postgres"},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "Table", assets[0].Type)
	assert.Equal(t, []string{"PostgreSQL"}, assets[0].Providers)
	assert.Equal(t, "public", assets[0].Metadata["schema"])
	assert.Equal(t, "shop", assets[0].Metadata["database"])
}

func TestBuildAssets_ViewBecomesAViewAssetWithItsQuery(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customer_readings", Kind: kindView,
			Definition: "SELECT c.name FROM customers c"},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "View", assets[0].Type)
	require.NotNil(t, assets[0].Query)
	assert.Equal(t, "SELECT c.name FROM customers c", *assets[0].Query)
	require.NotNil(t, assets[0].QueryLanguage)
	assert.Equal(t, "sql", *assets[0].QueryLanguage)
}

func TestBuildAssets_MaterializedViewBecomesAViewAsset(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "monthly", Kind: kindMaterializedView},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "View", assets[0].Type)
	assert.Equal(t, "materialized_view", assets[0].Metadata["object_type"])
}

func TestBuildAssets_TableWithoutAQueryLeavesQueryUnset(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable},
	}, nil)

	require.Len(t, assets, 1)
	assert.Nil(t, assets[0].Query)
}

func TestBuildAssets_ColumnsLandInTheAssetSchema(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable, Columns: []column{
			{Column: pluginsdk.Column{Name: "id", DataType: "integer", PrimaryKey: true}},
			{Column: pluginsdk.Column{Name: "email", DataType: "text", Nullable: true, Description: "Contact email address"}},
		}},
	}, nil)

	require.Len(t, assets, 1)
	columns := assets[0].Schema["columns"]
	assert.Contains(t, columns, `"column_name":"id"`)
	assert.Contains(t, columns, `"data_type":"integer"`)
	assert.Contains(t, columns, `"is_primary_key":true`)
	assert.Contains(t, columns, `"description":"Contact email address"`)
}

func TestBuildAssets_CommentBecomesTheMetadataComment(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable, Comment: "Shop customers"},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "Shop customers", assets[0].Metadata["comment"])
}

func TestBuildAssets_NoCommentLeavesTheKeyOut(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable},
	}, nil)

	require.Len(t, assets, 1)
	_, present := assets[0].Metadata["comment"]
	assert.False(t, present)
}

func TestDescribeKind_NamesAHypertable(t *testing.T) {
	assert.Equal(t, "TimescaleDB hypertable", describeKind(map[string]interface{}{
		"hypertable": true, "object_type": "table",
	}))
}

func TestDescribeKind_NamesAContinuousAggregate(t *testing.T) {
	assert.Equal(t, "TimescaleDB continuous aggregate", describeKind(map[string]interface{}{
		"continuous_aggregate": true, "object_type": "view",
	}))
}

func TestDescribeKind_NamesAPlainTable(t *testing.T) {
	assert.Equal(t, "PostgreSQL table", describeKind(map[string]interface{}{"object_type": "table"}))
}

func TestDescribeKind_NamesAPlainView(t *testing.T) {
	assert.Equal(t, "PostgreSQL view", describeKind(map[string]interface{}{"object_type": "view"}))
}

func TestConvertValue_RendersTimestampsAsRFC3339(t *testing.T) {
	value := convertValue(mustTime(t, "2026-09-08T10:30:00Z"))
	assert.Equal(t, "2026-09-08T10:30:00Z", value)
}

func TestConvertValue_RendersAUUIDArray(t *testing.T) {
	var id [16]byte
	for i := range id {
		id[i] = byte(i)
	}

	assert.Equal(t, "00010203-0405-0607-0809-0a0b0c0d0e0f", convertValue(id))
}

func TestConvertValue_PassesScalarsThrough(t *testing.T) {
	assert.Equal(t, 42, convertValue(42))
	assert.Equal(t, "hello", convertValue("hello"))
	assert.Nil(t, convertValue(nil))
}
