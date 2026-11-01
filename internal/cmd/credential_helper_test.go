package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHelperContext saves a context for a local Marmot with a token that
// expires in expiresIn seconds.
func setupHelperContext(t *testing.T, expiresIn int) {
	t.Helper()
	_, cleanup := setupTestConfigDir(t)
	t.Cleanup(cleanup)
	setupTestViper(t)
	require.NoError(t, setContext("localhost:8080", ContextEntry{Host: "http://localhost:8080"}))
	require.NoError(t, setCachedToken("localhost:8080", "tok-live", "Bearer", expiresIn))
}

func TestCredentialHelperGet(t *testing.T) {
	setupHelperContext(t, 3600)

	var out bytes.Buffer
	err := runCredentialHelper("get", strings.NewReader("localhost:8080\n"), &out)
	require.NoError(t, err)

	var cred helperCredential
	require.NoError(t, json.Unmarshal(out.Bytes(), &cred))
	assert.Equal(t, "localhost:8080", cred.ServerURL)
	assert.Equal(t, registryUsername, cred.Username)
	assert.Equal(t, "tok-live", cred.Secret)
}

func TestCredentialHelperGetMatchesSchemeAndPath(t *testing.T) {
	setupHelperContext(t, 3600)

	var out bytes.Buffer
	err := runCredentialHelper("get", strings.NewReader("https://localhost:8080/v2/\n"), &out)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "tok-live")
}

func TestCredentialHelperGetUnknownRegistry(t *testing.T) {
	setupHelperContext(t, 3600)

	var out bytes.Buffer
	err := runCredentialHelper("get", strings.NewReader("ghcr.io\n"), &out)
	assert.ErrorIs(t, err, errHelperFailed)
	// Docker matches this exact text to fall back to anonymous access.
	assert.Equal(t, errCredentialsNotFound.Error()+"\n", out.String())
}

func TestCredentialHelperGetExpired(t *testing.T) {
	setupHelperContext(t, 5) // inside the 30 second safety margin

	var out bytes.Buffer
	err := runCredentialHelper("get", strings.NewReader("localhost:8080\n"), &out)
	assert.ErrorIs(t, err, errHelperFailed)
	assert.Contains(t, out.String(), "marmot login localhost:8080")
	assert.NotContains(t, out.String(), "tok-live")
}

func TestCredentialHelperStoreIsRefused(t *testing.T) {
	setupHelperContext(t, 3600)

	var out bytes.Buffer
	err := runCredentialHelper("store", strings.NewReader(`{"ServerURL":"localhost:8080","Username":"x","Secret":"y"}`), &out)
	assert.ErrorIs(t, err, errHelperFailed)
	assert.Contains(t, out.String(), "marmot login")

	token, ok := getCachedToken("localhost:8080")
	assert.True(t, ok)
	assert.Equal(t, "tok-live", token, "a refused store must not touch the cached token")
}

func TestCredentialHelperEraseAndList(t *testing.T) {
	setupHelperContext(t, 3600)

	var out bytes.Buffer
	require.NoError(t, runCredentialHelper("list", strings.NewReader(""), &out))
	var listed map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &listed))
	assert.Equal(t, map[string]string{"localhost:8080": registryUsername}, listed)

	require.NoError(t, runCredentialHelper("erase", strings.NewReader("localhost:8080\n"), &bytes.Buffer{}))
	_, ok := getCachedToken("localhost:8080")
	assert.False(t, ok)

	out.Reset()
	require.NoError(t, runCredentialHelper("list", strings.NewReader(""), &out))
	assert.Equal(t, "{}\n", out.String())
}

func TestCredentialHelperUnknownOperation(t *testing.T) {
	err := runCredentialHelper("frobnicate", strings.NewReader(""), &bytes.Buffer{})
	assert.Error(t, err)
}
