package pinot

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"controller_url": "http://localhost:9000"})
	require.NoError(t, err)
}

func TestValidate_MissingControllerURLFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "controller_url")
}

func TestValidate_RejectsAControllerURLThatIsNotAURL(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"controller_url": "localhost:9000"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "controller_url")
}

func TestValidate_RejectsABrokerURLThatIsNotAURL(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"controller_url": "http://localhost:9000",
		"broker_url":     "broker:8099",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broker_url")
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"controller_url": "http://localhost:9000"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeRowCounts)
	assert.True(t, s.config.IncludeSizes)
	assert.True(t, s.config.DiscoverLineage)
	assert.True(t, s.config.ExcludeSystemTables)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"controller_url":     "http://localhost:9000",
		"include_row_counts": false,
		"verify_ssl":         false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeRowCounts)
	assert.False(t, s.config.VerifySSL)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.DiscoverLineage)
}

func TestValidate_TrimsTrailingSlashesFromURLs(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"controller_url": "http://localhost:9000/",
		"broker_url":     "http://localhost:8099/",
	})
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:9000", s.config.ControllerURL)
	assert.Equal(t, "http://localhost:8099", s.config.BrokerURL)
}

func TestValidate_RejectsTokenAndUsernameTogether(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"controller_url": "http://localhost:9000",
		"username":       "admin",
		"password":       "secret",
		"token":          "abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.NotContains(t, err.Error(), "secret", "the error must not leak the password")
	assert.NotContains(t, err.Error(), "abc", "the error must not leak the token")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"controller_url": "http://localhost:9000",
		"filter": map[string]interface{}{
			"include": []interface{}{"^orders.*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "pinot", meta.ID)
	assert.Equal(t, "Apache Pinot", meta.Name)
	assert.Equal(t, "pinot", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksSecretsSensitive(t *testing.T) {
	sensitive := make(map[string]bool)
	for _, field := range Meta().ConfigSpec {
		sensitive[field.Name] = field.Sensitive
	}

	assert.True(t, sensitive["password"])
	assert.True(t, sensitive["token"])
	assert.False(t, sensitive["username"])
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{ControllerURL: "http://localhost:9000"}

	assert.False(t, config.VerifySSL)
	assert.False(t, config.IncludeColumns)
	assert.False(t, config.IncludeRowCounts)
	assert.False(t, config.IncludeSizes)
	assert.False(t, config.DiscoverLineage)
	assert.False(t, config.ExcludeSystemTables)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = (*Source)(nil)
}
