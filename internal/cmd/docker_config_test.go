package cmd

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryHost(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://marmot.example.com", "marmot.example.com"},
		{"https://marmot.example.com/", "marmot.example.com"},
		{"http://localhost:8080", "localhost:8080"},
		{"https://marmot.example.com/some/prefix", "marmot.example.com"},
		{"marmot.example.com", "marmot.example.com"},
		{"https://marmot.example.com/v2/", "marmot.example.com"},
		{"  localhost:8080\n", "localhost:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, registryHost(tt.in))
		})
	}
}

// setupTestDockerConfig points DOCKER_CONFIG at a scratch directory and
// empties PATH so the credential helper is not found unless a test installs
// one.
func setupTestDockerConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("PATH", "")
	return filepath.Join(dir, "config.json")
}

func readDockerConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	return cfg
}

func TestConfigureRegistryAuthStatic(t *testing.T) {
	path := setupTestDockerConfig(t)

	reg, err := configureRegistryAuth("https://marmot.example.com", "tok-123")
	require.NoError(t, err)
	assert.False(t, reg.Helper)
	assert.Equal(t, "marmot.example.com", reg.Registry)
	assert.Equal(t, path, reg.Path)
	assert.Empty(t, reg.CredsStore)

	cfg := readDockerConfig(t, path)
	auths := cfg["auths"].(map[string]any)
	entry := auths["marmot.example.com"].(map[string]any)
	decoded, err := base64.StdEncoding.DecodeString(entry["auth"].(string))
	require.NoError(t, err)
	assert.Equal(t, registryUsername+":tok-123", string(decoded))
	assert.NotContains(t, cfg, "credHelpers")
}

func TestConfigureRegistryAuthPreservesOtherKeys(t *testing.T) {
	path := setupTestDockerConfig(t)
	require.NoError(t, os.WriteFile(path, []byte(`{
		"auths": {"ghcr.io": {"auth": "Zm9vOmJhcg=="}},
		"credHelpers": {"gcr.io": "gcloud"},
		"credsStore": "desktop",
		"currentContext": "default"
	}`), 0o600))

	reg, err := configureRegistryAuth("http://localhost:8080", "tok-123")
	require.NoError(t, err)
	assert.Equal(t, "desktop", reg.CredsStore, "login should know a credsStore will shadow the static entry")

	cfg := readDockerConfig(t, path)
	auths := cfg["auths"].(map[string]any)
	assert.Contains(t, auths, "ghcr.io")
	assert.Contains(t, auths, "localhost:8080")
	assert.Equal(t, "gcloud", cfg["credHelpers"].(map[string]any)["gcr.io"])
	assert.Equal(t, "desktop", cfg["credsStore"])
	assert.Equal(t, "default", cfg["currentContext"])

	require.NoError(t, removeRegistryAuth("http://localhost:8080"))
	cfg = readDockerConfig(t, path)
	assert.NotContains(t, cfg["auths"].(map[string]any), "localhost:8080")
	assert.Contains(t, cfg["auths"].(map[string]any), "ghcr.io")
	assert.Equal(t, "gcloud", cfg["credHelpers"].(map[string]any)["gcr.io"])
}

func TestConfigureRegistryAuthHelper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH lookup of a fake helper is unix-specific")
	}
	path := setupTestDockerConfig(t)

	// A stale static entry from a machine without the helper.
	require.NoError(t, os.WriteFile(path, []byte(`{"auths": {"marmot.example.com": {"auth": "old"}}}`), 0o600))

	bin := t.TempDir()
	helper := filepath.Join(bin, credentialHelperBinary)
	require.NoError(t, os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", bin)

	reg, err := configureRegistryAuth("https://marmot.example.com", "tok-123")
	require.NoError(t, err)
	assert.True(t, reg.Helper)

	cfg := readDockerConfig(t, path)
	assert.Equal(t, credentialHelperName, cfg["credHelpers"].(map[string]any)["marmot.example.com"])
	assert.NotContains(t, cfg, "auths", "the static entry is replaced, not kept beside the helper")

	// Logging out takes the mapping away again and leaves an empty config.
	require.NoError(t, removeRegistryAuth("https://marmot.example.com"))
	cfg = readDockerConfig(t, path)
	assert.Empty(t, cfg)
}

func TestRemoveRegistryAuthLeavesForeignHelper(t *testing.T) {
	path := setupTestDockerConfig(t)
	require.NoError(t, os.WriteFile(path, []byte(`{"credHelpers": {"marmot.example.com": "osxkeychain"}}`), 0o600))

	require.NoError(t, removeRegistryAuth("https://marmot.example.com"))
	cfg := readDockerConfig(t, path)
	assert.Equal(t, "osxkeychain", cfg["credHelpers"].(map[string]any)["marmot.example.com"])
}

func TestRemoveRegistryAuthWithoutConfig(t *testing.T) {
	path := setupTestDockerConfig(t)
	require.NoError(t, removeRegistryAuth("https://marmot.example.com"))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "nothing to remove should not create a config file")
}
