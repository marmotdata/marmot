package cloudrun

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "cloudrun", meta.ID)
	assert.Equal(t, "Google Cloud Run", meta.Name)
	// The UI looks an icon up by the provider, lowercased with spaces turned
	// into hyphens, so the icon id has to be the provider in that form.
	assert.Equal(t, "cloud-run", meta.Icon)
	assert.Equal(t, "container", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_AcceptsAProjectID(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"project_id": "acme"})

	require.NoError(t, err)
	assert.Equal(t, "acme", source.config.ProjectID)
}

func TestValidate_RejectsAMissingProjectID(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestValidate_RejectsBothCredentialFormsAtOnce(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"project_id":       "acme",
		"credentials_json": `{"type":"service_account"}`,
		"credentials_file": "/etc/marmot/key.json",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestValidate_DefaultsJobsAndExecutionsOn(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"project_id": "acme"})

	require.NoError(t, err)
	assert.True(t, source.config.IncludeJobs)
	assert.True(t, source.config.IncludeExecutions)
	assert.Equal(t, 10, source.config.MaxExecutionsPerJob)
}

func TestValidate_KeepsJobsOffWhenAskedTo(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"project_id": "acme", "include_jobs": false})

	require.NoError(t, err)
	assert.False(t, source.config.IncludeJobs)
}

func TestValidate_RejectsAnExecutionLimitAboveTheMaximum(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"project_id":             "acme",
		"max_executions_per_job": 500,
	})

	require.Error(t, err)
}

func TestValidate_AddsATrailingSlashToTheEndpoint(t *testing.T) {
	// The Google client library resolves request paths relative to the
	// endpoint, so without the slash the last path segment is replaced.
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"project_id": "acme",
		"endpoint":   "http://127.0.0.1:18801",
	})

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:18801/", source.config.Endpoint)
}

func TestValidate_TrimsWhitespaceFromLocations(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"project_id": "acme",
		"locations":  []string{" europe-west1 "},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"europe-west1"}, source.config.Locations)
}

func TestValidate_CarriesTheFilterForTheHostToApply(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"project_id": "acme",
		"filter": map[string]any{
			"include": []string{"^europe-west1/.*"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, source.config.Filter)
	assert.Equal(t, []string{"^europe-west1/.*"}, source.config.Filter.Include)
}

func TestParents_UsesTheWildcardLocationByDefault(t *testing.T) {
	source := &Source{config: &Config{ProjectID: "acme"}}

	assert.Equal(t, []string{"projects/acme/locations/-"}, source.parents())
}

func TestParents_UsesOneParentPerConfiguredLocation(t *testing.T) {
	source := &Source{config: &Config{ProjectID: "acme", Locations: []string{"europe-west1", "us-central1"}}}

	assert.Equal(t, []string{
		"projects/acme/locations/europe-west1",
		"projects/acme/locations/us-central1",
	}, source.parents())
}
