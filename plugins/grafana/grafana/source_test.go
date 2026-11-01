package grafana

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_RequiresHost(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"api_key": "glsa_x"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RequiresAPIKey(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://grafana.example.com"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "grafana.example.com", "api_key": "glsa_x"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://grafana.example.com", "api_key": "glsa_x"})
	require.NoError(t, err)

	assert.True(t, source.config.VerifySSL)
	assert.True(t, source.config.IncludePanels)
	assert.True(t, source.config.IncludeDatasources)
	assert.True(t, source.config.DiscoverLineage)
	assert.Equal(t, 100, source.config.PageSize)
}

func TestValidate_KeepsExplicitValues(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://grafana.example.com", "api_key": "glsa_x",
		"verify_ssl": false, "include_panels": false, "page_size": 10,
	})
	require.NoError(t, err)

	assert.False(t, source.config.VerifySSL)
	assert.False(t, source.config.IncludePanels)
	assert.True(t, source.config.IncludeDatasources, "untouched flags still default to true")
	assert.Equal(t, 10, source.config.PageSize)
}

func TestValidate_RejectsAnExplicitZeroPageSize(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://grafana.example.com", "api_key": "glsa_x", "page_size": 0,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_size must be at least 1")
}

func TestValidate_RejectsAPageSizeAboveGrafanasCap(t *testing.T) {
	// Grafana answers 422 to a limit above 5000, so the config refuses
	// it up front instead of failing on the first request.
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://grafana.example.com", "api_key": "glsa_x", "page_size": 6000,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_size must be at most 5000")
}

func TestValidate_TrimsATrailingSlashFromTheHost(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://grafana.example.com/", "api_key": "glsa_x"})
	require.NoError(t, err)

	assert.Equal(t, "https://grafana.example.com", source.config.Host)
}

func TestValidate_AcceptsATokenWithoutTheServiceAccountPrefix(t *testing.T) {
	// Legacy API keys still work against Grafana, so they only warn.
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://grafana.example.com", "api_key": "eyJrIjoi"})

	require.NoError(t, err)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://grafana.example.com", "api_key": "glsa_x",
		"filter": map[string]any{
			"include": []any{"^Sales/.*"},
			"exclude": []any{".*tmp$"},
		},
	})

	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "grafana", meta.ID)
	assert.Equal(t, "Grafana", meta.Name)
	assert.Equal(t, "grafana", meta.Icon)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksTheTokenSensitive(t *testing.T) {
	for _, field := range Meta().ConfigSpec {
		if field.Name == "api_key" {
			assert.True(t, field.Sensitive)
			assert.True(t, field.Required)
			assert.Equal(t, "Service Account Token", field.Label)
			return
		}
	}
	t.Fatal("api_key missing from the config spec")
}
