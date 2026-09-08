package oracle

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host":         "db.example.com",
		"user":         "marmot",
		"password":     "secret",
		"service_name": "FREEPDB1",
	}
}

func TestValidate_ValidConfigWithServiceName(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
	assert.Equal(t, "FREEPDB1", s.config.ServiceName)
}

func TestValidate_ValidConfigWithSID(t *testing.T) {
	raw := validConfig()
	delete(raw, "service_name")
	raw["sid"] = "ORCL"

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, "ORCL", s.config.SID)
}

func TestValidate_MissingHostFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "host")

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_MissingUserFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "user")

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestValidate_MissingPasswordFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "password")

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestValidate_RequiresServiceNameOrSID(t *testing.T) {
	raw := validConfig()
	delete(raw, "service_name")

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_name or sid")
}

func TestValidate_ServiceNameAndSIDAreMutuallyExclusive(t *testing.T) {
	raw := validConfig()
	raw["sid"] = "ORCL"

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_BlankServiceNameCountsAsUnset(t *testing.T) {
	raw := validConfig()
	raw["service_name"] = "   "
	raw["sid"] = "ORCL"

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, "", s.config.ServiceName)
	assert.Equal(t, "ORCL", s.config.SID)
}

func TestValidate_DefaultsPortTo1521(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
	assert.Equal(t, 1521, s.config.Port)
}

func TestValidate_RejectsPortOutOfRange(t *testing.T) {
	raw := validConfig()
	raw["port"] = 70000

	s := &Source{}
	_, err := s.Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_DefaultsDiscoveryFlagsToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeMaterializedViews)
	assert.True(t, s.config.IncludeProcedures)
	assert.True(t, s.config.DiscoverForeignKeys)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_DefaultsConnectionFlags(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.False(t, s.config.UseDBAViews)
	assert.False(t, s.config.SSL)
	assert.True(t, s.config.SSLVerify)
	assert.Empty(t, s.config.WalletPath)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	raw := validConfig()
	raw["include_procedures"] = false
	raw["include_statistics"] = false
	raw["ssl_verify"] = false

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)

	assert.False(t, s.config.IncludeProcedures)
	assert.False(t, s.config.IncludeStatistics)
	assert.False(t, s.config.SSLVerify)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.DiscoverForeignKeys)
}

func TestValidate_DefaultExcludeListCoversOracleSchemas(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.Contains(t, s.config.ExcludeSchemas, "SYS")
	assert.Contains(t, s.config.ExcludeSchemas, "SYSTEM")
	assert.Contains(t, s.config.ExcludeSchemas, "CTXSYS")
	assert.Contains(t, s.config.ExcludeSchemas, "DBSNMP")
	assert.Contains(t, s.config.ExcludeSchemas, "OUTLN")
	assert.Contains(t, s.config.ExcludeSchemas, "XDB")
}

func TestValidate_EmptyExcludeListOverridesTheDefault(t *testing.T) {
	raw := validConfig()
	raw["exclude_schemas"] = []interface{}{}

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	assert.Empty(t, s.config.ExcludeSchemas)
}

func TestValidate_UpperCasesConfiguredSchemas(t *testing.T) {
	raw := validConfig()
	raw["schemas"] = []interface{}{"hr", "Sales"}
	raw["exclude_schemas"] = []interface{}{"scratch"}

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)

	assert.Equal(t, []string{"HR", "SALES"}, s.config.Schemas)
	assert.Equal(t, []string{"SCRATCH"}, s.config.ExcludeSchemas)
}

func TestValidate_QuotedSchemaKeepsItsCase(t *testing.T) {
	raw := validConfig()
	raw["schemas"] = []interface{}{`"MixedCase"`}

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, []string{"MixedCase"}, s.config.Schemas)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	raw := validConfig()
	raw["filter"] = map[string]interface{}{
		"include": []interface{}{"^HR\\..*"},
		"exclude": []interface{}{".*_TMP$"},
	}

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
}

func TestMeta_DeclaresAssetsAndLineage(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "oracle", meta.ID)
	assert.Equal(t, "Oracle Database", meta.Name)
	assert.Equal(t, "oracle", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

// The Source implements the optional sample data interface, which the SDK
// turns into SupportsDataPreview on the manifest.
func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}
