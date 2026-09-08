package firebase

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "firebase", meta.ID)
	assert.Equal(t, "Google Firebase", meta.Name)
	assert.Equal(t, "firebase", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)
}

func TestValidate_MissingProjectIDFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeRealtimeDatabase)
	assert.True(t, s.config.IncludeProjectDetails)
	assert.True(t, s.config.IncludeSubcollections)
	assert.Equal(t, 2, s.config.MaxCollectionDepth)
	assert.Equal(t, 20, s.config.SampleDocuments)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":                "marmot-demo",
		"include_realtime_database": false,
		"include_subcollections":    false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeRealtimeDatabase)
	assert.False(t, s.config.IncludeSubcollections)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeProjectDetails)
}

func TestValidate_KeepsExplicitSampleSize(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":           "marmot-demo",
		"sample_documents":     100,
		"max_collection_depth": 4,
	})
	require.NoError(t, err)

	assert.Equal(t, 100, s.config.SampleDocuments)
	assert.Equal(t, 4, s.config.MaxCollectionDepth)
}

func TestValidate_RejectsTooManySampleDocuments(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":       "marmot-demo",
		"sample_documents": 5000,
	})
	require.Error(t, err)
}

func TestValidate_RejectsTooDeepACollectionWalk(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":           "marmot-demo",
		"max_collection_depth": 9,
	})
	require.Error(t, err)
}

func TestValidate_AddsTheTrailingSlashTheGoogleClientsNeed(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id": "marmot-demo",
		"endpoint":   "http://localhost:8080",
	})
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/", s.config.Endpoint)
}

func TestValidate_LeavesATrailingSlashAlone(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id": "marmot-demo",
		"endpoint":   "http://localhost:8080/",
	})
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/", s.config.Endpoint)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id": "marmot-demo",
		"filter": map[string]interface{}{
			"include": []interface{}{"^firestore/.*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

func TestValidate_AcceptsAListOfDatabases(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id": "marmot-demo",
		"databases":  []interface{}{"(default)", "analytics"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"(default)", "analytics"}, s.config.Databases)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes the
// flags, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreUnset(t *testing.T) {
	config := &Config{ProjectID: "marmot-demo"}

	assert.False(t, config.IncludeRealtimeDatabase)
	assert.False(t, config.IncludeProjectDetails)
	assert.False(t, config.IncludeSubcollections)
	assert.Zero(t, config.MaxCollectionDepth)
	assert.Zero(t, config.SampleDocuments)
}

func TestEmulatorHost_StripsTheSchemeAndTrailingSlash(t *testing.T) {
	assert.Equal(t, "localhost:8080", emulatorHost("http://localhost:8080/"))
	assert.Equal(t, "127.0.0.1:9000", emulatorHost("https://127.0.0.1:9000"))
}

// An empty endpoint leaves the Google library's own FIRESTORE_EMULATOR_HOST
// handling in charge of where the data client connects.
func TestEmulatorHost_IsEmptyWithoutAnEndpoint(t *testing.T) {
	assert.Equal(t, "", emulatorHost(""))
}

// Dialling the emulator means no TLS and a token Google would reject, so an
// endpoint on its own must never be enough to turn it on.

func TestUseEmulator_NeedsBothTheEndpointAndNoAuthentication(t *testing.T) {
	assert.True(t, useEmulator("http://localhost:8080", true))
}

func TestUseEmulator_LeavesARealEndpointAlone(t *testing.T) {
	assert.False(t, useEmulator("https://firestore.googleapis.com", false))
}

func TestUseEmulator_IsOffWithoutAnEndpoint(t *testing.T) {
	assert.False(t, useEmulator("", true))
}

func TestLastSegment_ReadsTheIDOutOfAResourceName(t *testing.T) {
	assert.Equal(t, "(default)", lastSegment("projects/marmot-demo/databases/(default)"))
	assert.Equal(t, "events", lastSegment("projects/451/locations/europe-west1/instances/events"))
	assert.Equal(t, "orders", lastSegment("orders"))
}
