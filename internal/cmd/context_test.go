package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	viper.SetDefault("host", "http://localhost:8080")
	viper.SetDefault("output", "table")
}

func TestSetAndGetContext(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()
	setupTestViper(t)

	err := setContext("example.com", ContextEntry{Host: "https://example.com"})
	require.NoError(t, err)

	contexts, err := loadContexts()
	require.NoError(t, err)
	assert.Contains(t, contexts, "example.com")
	assert.Equal(t, "https://example.com", contexts["example.com"].Host)
}

// A context name is a hostname. Viper is dot-delimited, so a name kept in the
// config would come back split; and any command that rewrites the config must
// leave the contexts alone.
func TestContextsSurviveAConfigWrite(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()
	setupTestViper(t)

	require.NoError(t, setContext("example.com", ContextEntry{Host: "https://example.com"}))
	require.NoError(t, setContext("other.com", ContextEntry{Host: "https://other.com"}))

	// A later process reads the config, then writes it for an unrelated key.
	dir, err := configDir()
	require.NoError(t, err)
	viper.Reset()
	viper.SetConfigFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, viper.ReadInConfig())
	viper.Set("output", "json")
	require.NoError(t, writeConfig())

	contexts, err := loadContexts()
	require.NoError(t, err)
	assert.Equal(t, map[string]ContextEntry{
		"example.com": {Host: "https://example.com"},
		"other.com":   {Host: "https://other.com"},
	}, contexts)
}

func TestLoadContextsReportsABrokenFile(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	p, err := contextsPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte("{not json"), 0o600))

	_, err = loadContexts()
	assert.Error(t, err, "a broken file is not silently an empty one")
}

func TestLoadContextsWithoutAFile(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	contexts, err := loadContexts()
	require.NoError(t, err)
	assert.Empty(t, contexts)
}

func TestCurrentContextName(t *testing.T) {
	setupTestViper(t)

	viper.Set("current_context", "myctx")
	assert.Equal(t, "myctx", currentContextName())
}

func TestGetActiveContext(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()
	setupTestViper(t)

	// No context set
	name, ctx := getActiveContext()
	assert.Empty(t, name)
	assert.Nil(t, ctx)

	// Set a context
	require.NoError(t, setContext("test.dev", ContextEntry{Host: "https://test.dev"}))

	name, ctx = getActiveContext()
	assert.Equal(t, "test.dev", name)
	require.NotNil(t, ctx)
	assert.Equal(t, "https://test.dev", ctx.Host)
}

func TestResolveHostPriority(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()
	setupTestViper(t)

	// Default
	assert.Equal(t, "http://localhost:8080", resolveHost())

	// Context takes priority over default
	require.NoError(t, setContext("ctx.dev", ContextEntry{Host: "https://ctx.dev"}))
	assert.Equal(t, "https://ctx.dev", resolveHost())

	// --host flag takes highest priority
	globalHost = "http://override:9090"
	defer func() { globalHost = "" }()
	assert.Equal(t, "http://override:9090", resolveHost())
}
