package cmd

import (
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

	contexts := getContexts()
	assert.Contains(t, contexts, "example.com")
	assert.Equal(t, "https://example.com", contexts["example.com"].Host)
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
