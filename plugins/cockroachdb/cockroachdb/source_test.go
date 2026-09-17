package cockroachdb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validate(t *testing.T, raw pluginsdk.RawConfig) *Config {
	t.Helper()
	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	require.NotNil(t, s.config)
	return s.config
}

func TestMeta_DeclaresIdentity(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "cockroachdb", meta.ID)
	assert.Equal(t, "CockroachDB", meta.Name)
	assert.Equal(t, "cockroachdb", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"user": "root"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_MissingUserFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestValidate_PasswordIsOptional(t *testing.T) {
	// Insecure clusters and certificate authentication have no password.
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	assert.Empty(t, config.Password)
}

func TestValidate_DefaultsPort(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	assert.Equal(t, 26257, config.Port)
}

func TestValidate_KeepsExplicitPort(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root", "port": 26258})
	assert.Equal(t, 26258, config.Port)
}

func TestValidate_RejectsPortOutOfRange(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "root", "port": 70000})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_DefaultsSSLModeToDisable(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	assert.Equal(t, "disable", config.SSLMode)
}

func TestValidate_RejectsUnknownSSLMode(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost", "user": "root", "ssl_mode": "prefer"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssl_mode")
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root"})

	assert.True(t, config.IncludeColumns)
	assert.True(t, config.IncludeViews)
	assert.True(t, config.DiscoverForeignKeys)
	assert.True(t, config.IncludeStatistics)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{
		"host":               "localhost",
		"user":               "root",
		"include_views":      false,
		"include_statistics": false,
	})

	assert.False(t, config.IncludeViews)
	assert.False(t, config.IncludeStatistics)
	// Untouched flags still default to true.
	assert.True(t, config.IncludeColumns)
	assert.True(t, config.DiscoverForeignKeys)
}

func TestValidate_ExcludesSystemDatabaseByDefault(t *testing.T) {
	// defaultdb and postgres stay in: they are empty on a fresh cluster but
	// people do create tables there.
	config := validate(t, pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	assert.Equal(t, []string{"system"}, config.ExcludeDatabases)
}

func TestValidate_KeepsExplicitExcludeDatabases(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{
		"host":              "localhost",
		"user":              "root",
		"exclude_databases": []interface{}{"system", "defaultdb", "postgres"},
	})
	assert.Equal(t, []string{"system", "defaultdb", "postgres"}, config.ExcludeDatabases)
}

func TestValidate_EmptyExcludeDatabasesExcludesNothing(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{
		"host":              "localhost",
		"user":              "root",
		"exclude_databases": []interface{}{},
	})
	assert.Empty(t, config.ExcludeDatabases)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "localhost",
		"user": "root",
		"filter": map[string]interface{}{
			"include": []interface{}{"^shop\\..*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

func TestConnString_EncodesReservedCharactersInThePassword(t *testing.T) {
	s := &Source{config: &Config{Host: "db.internal", Port: 26257, User: "marmot", Password: "p@ss/w:rd", SSLMode: "require"}}

	assert.Equal(t, "postgres://marmot:p%40ss%2Fw%3Ard@db.internal:26257/shop?sslmode=require", s.connString("shop"))
}

func TestConnString_OmitsThePasswordWhenEmpty(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 26257, User: "root", SSLMode: "disable"}}

	assert.Equal(t, "postgres://root@localhost:26257/defaultdb?sslmode=disable", s.connString("defaultdb"))
}

func TestConnString_CarriesTheCertificatePaths(t *testing.T) {
	s := &Source{config: &Config{
		Host: "localhost", Port: 26257, User: "marmot", SSLMode: "verify-full",
		SSLRootCert: "/certs/ca.crt", SSLCert: "/certs/client.marmot.crt", SSLKey: "/certs/client.marmot.key",
	}}

	conn := s.connString("shop")

	assert.Contains(t, conn, "sslmode=verify-full")
	assert.Contains(t, conn, "sslrootcert=%2Fcerts%2Fca.crt")
	assert.Contains(t, conn, "sslcert=%2Fcerts%2Fclient.marmot.crt")
	assert.Contains(t, conn, "sslkey=%2Fcerts%2Fclient.marmot.key")
}

func TestConnString_BracketsIPv6Hosts(t *testing.T) {
	s := &Source{config: &Config{Host: "::1", Port: 26257, User: "root", SSLMode: "disable"}}

	assert.Equal(t, "postgres://root@[::1]:26257/shop?sslmode=disable", s.connString("shop"))
}
