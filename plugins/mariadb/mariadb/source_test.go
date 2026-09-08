package mariadb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host":     "db.internal",
		"user":     "marmot",
		"password": "secret",
		"database": "shop",
	}
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "mariadb", meta.ID)
	assert.Equal(t, "MariaDB", meta.Name)
	assert.Equal(t, "mariadb", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "host")

	_, err := (&Source{}).Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_MissingUserFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "user")

	_, err := (&Source{}).Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestValidate_MissingDatabaseFails(t *testing.T) {
	raw := validConfig()
	delete(raw, "database")

	_, err := (&Source{}).Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database")
}

func TestValidate_PasswordIsOptional(t *testing.T) {
	raw := validConfig()
	delete(raw, "password")

	_, err := (&Source{}).Validate(raw)
	require.NoError(t, err)
}

func TestValidate_DefaultsPortAndTLS(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 3306, s.config.Port)
	assert.Equal(t, "false", s.config.TLS)
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeSequences)
	assert.True(t, s.config.IncludeRowCounts)
	assert.True(t, s.config.IncludeStatistics)
	assert.True(t, s.config.DiscoverForeignKeys)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	raw := validConfig()
	raw["include_views"] = false
	raw["include_statistics"] = false

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)

	assert.False(t, s.config.IncludeViews)
	assert.False(t, s.config.IncludeStatistics)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeSequences)
	assert.True(t, s.config.IncludeColumns)
}

func TestValidate_KeepsAnExplicitPort(t *testing.T) {
	raw := validConfig()
	raw["port"] = 13307

	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)

	assert.Equal(t, 13307, s.config.Port)
}

func TestValidate_RejectsAPortOutOfRange(t *testing.T) {
	raw := validConfig()
	raw["port"] = 70000

	_, err := (&Source{}).Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_AcceptsEveryTLSMode(t *testing.T) {
	for _, mode := range []string{"false", "true", "skip-verify", "preferred"} {
		raw := validConfig()
		raw["tls"] = mode

		_, err := (&Source{}).Validate(raw)
		assert.NoErrorf(t, err, "tls mode %q should be accepted", mode)
	}
}

func TestValidate_RejectsAnUnknownTLSMode(t *testing.T) {
	raw := validConfig()
	raw["tls"] = "maybe"

	_, err := (&Source{}).Validate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	raw := validConfig()
	raw["filter"] = map[string]interface{}{
		"include": []interface{}{"^order.*"},
		"exclude": []interface{}{".*_tmp$"},
	}

	_, err := (&Source{}).Validate(raw)
	require.NoError(t, err)
}

func TestBuildDSN_HasTheDriverShape(t *testing.T) {
	config := &Config{Host: "db.internal", Port: 3306, User: "marmot", Password: "secret", TLS: "true"}

	assert.Equal(t, "marmot:secret@tcp(db.internal:3306)/shop?parseTime=true&timeout=15s&tls=true",
		buildDSN(config, "shop"))
}

func TestBuildDSN_OmitsTheColonWithoutAPassword(t *testing.T) {
	config := &Config{Host: "db.internal", Port: 3306, User: "marmot", TLS: "false"}

	assert.Equal(t, "marmot@tcp(db.internal:3306)/shop?parseTime=true&timeout=15s&tls=false",
		buildDSN(config, "shop"))
}

func TestBuildDSN_BracketsAnIPv6Host(t *testing.T) {
	config := &Config{Host: "::1", Port: 3306, User: "marmot", TLS: "false"}

	assert.Contains(t, buildDSN(config, "shop"), "@tcp([::1]:3306)/")
}

func TestBuildDSN_EscapesTheDatabaseName(t *testing.T) {
	config := &Config{Host: "db.internal", Port: 3306, User: "marmot", TLS: "false"}

	assert.Contains(t, buildDSN(config, "odd?name"), "/odd%3Fname?")
}

func TestQuoteIdentifier_WrapsInBackticks(t *testing.T) {
	assert.Equal(t, "`orders`", quoteIdentifier("orders"))
}

func TestQuoteIdentifier_EscapesEmbeddedBackticks(t *testing.T) {
	assert.Equal(t, "`we``ird`", quoteIdentifier("we`ird"))
}
