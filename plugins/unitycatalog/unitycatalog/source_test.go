package unitycatalog

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8080"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "uc.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_TrimsTrailingSlashFromHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8080/"})
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", s.config.Host)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8080"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, []string{"system", "__databricks_internal"}, s.config.ExcludeCatalogs)
	assert.Empty(t, s.config.Catalogs)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeVolumes)
	assert.True(t, s.config.IncludeFunctions)
	assert.True(t, s.config.IncludeModels)
	assert.True(t, s.config.VerifySSL)
	assert.Equal(t, 100, s.config.PageSize)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":            "http://localhost:8080",
		"include_volumes": false,
		"verify_ssl":      false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeVolumes)
	assert.False(t, s.config.VerifySSL)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeFunctions)
	assert.True(t, s.config.IncludeModels)
}

func TestValidate_AnEmptyExcludeListMeansNothingIsExcluded(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":             "http://localhost:8080",
		"exclude_catalogs": []any{},
	})
	require.NoError(t, err)
	assert.Empty(t, s.config.ExcludeCatalogs)
}

func TestValidate_RejectsAPageSizeOutOfRange(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8080", "page_size": 5000})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_size")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "http://localhost:8080",
		"filter": map[string]any{
			"include": []any{"^shop\\..*"},
			"exclude": []any{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "unitycatalog", meta.ID)
	assert.Equal(t, "Unity Catalog", meta.Name)
	assert.Equal(t, "catalog", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.ElementsMatch(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksTheTokenSensitive(t *testing.T) {
	for _, field := range Meta().ConfigSpec {
		if field.Name == "token" {
			assert.True(t, field.Sensitive)
			return
		}
	}
	t.Fatal("token field missing from the config spec")
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Host: "http://localhost:8080"}

	assert.False(t, config.IncludeColumns)
	assert.False(t, config.IncludeVolumes)
	assert.False(t, config.IncludeFunctions)
	assert.False(t, config.IncludeModels)
	assert.False(t, config.VerifySSL)
}
