package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestConfigDir(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	return tmpDir, func() {
		_ = os.Setenv("XDG_CONFIG_HOME", origXDG)
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Save a token
	err := setCachedToken("example.com", "tok-abc", "Bearer", 3600)
	require.NoError(t, err)

	// Load it back
	token, ok := getCachedToken("example.com")
	assert.True(t, ok)
	assert.Equal(t, "tok-abc", token)
}

func TestCredentialsMissingHost(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	token, ok := getCachedToken("nonexistent.com")
	assert.False(t, ok)
	assert.Empty(t, token)
}

func TestCredentialsExpired(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Store a token that expires in 10 seconds (within the 30s margin)
	err := setCachedToken("example.com", "tok-expired", "Bearer", 10)
	require.NoError(t, err)

	token, ok := getCachedToken("example.com")
	assert.False(t, ok)
	assert.Empty(t, token)
}

func TestCredentialsValidWithMargin(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Token expires in 120 seconds — well beyond the 30s margin
	err := setCachedToken("example.com", "tok-valid", "Bearer", 120)
	require.NoError(t, err)

	token, ok := getCachedToken("example.com")
	assert.True(t, ok)
	assert.Equal(t, "tok-valid", token)
}

func TestDeleteCachedToken(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	require.NoError(t, setCachedToken("a.com", "tok-a", "Bearer", 3600))
	require.NoError(t, setCachedToken("b.com", "tok-b", "Bearer", 3600))

	// Delete a.com
	require.NoError(t, deleteCachedToken("a.com"))

	_, ok := getCachedToken("a.com")
	assert.False(t, ok)

	// b.com should still exist
	token, ok := getCachedToken("b.com")
	assert.True(t, ok)
	assert.Equal(t, "tok-b", token)
}

func TestCredentialsFilePermissions(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	require.NoError(t, setCachedToken("example.com", "tok", "Bearer", 3600))

	p, err := credentialsPath()
	require.NoError(t, err)

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoadCredentialsMissingFile(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	store, err := loadCredentials()
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.Empty(t, store.Tokens)
}

func TestSaveAndLoadCredentials(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	store := &CredentialStore{
		Tokens: map[string]*TokenEntry{
			"test.com": {
				AccessToken: "tok123",
				TokenType:   "Bearer",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
		},
	}

	require.NoError(t, saveCredentials(store))

	loaded, err := loadCredentials()
	require.NoError(t, err)
	require.Contains(t, loaded.Tokens, "test.com")
	assert.Equal(t, "tok123", loaded.Tokens["test.com"].AccessToken)
}

func TestCredentialsDirCreated(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "deep", "nested")
	t.Setenv("XDG_CONFIG_HOME", nested)

	require.NoError(t, setCachedToken("example.com", "tok", "Bearer", 3600))

	p, err := credentialsPath()
	require.NoError(t, err)
	_, err = os.Stat(p)
	require.NoError(t, err)
}
