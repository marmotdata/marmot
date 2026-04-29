package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := generatePKCE()
	require.NoError(t, err)

	// Verifier should be base64url-encoded 32 bytes = 43 characters
	assert.Len(t, verifier, 43)

	// Challenge should be S256 of verifier
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	assert.Equal(t, expected, challenge)

	// Challenge should be base64url-encoded SHA256 = 43 characters
	assert.Len(t, challenge, 43)
}

func TestGeneratePKCEUniqueness(t *testing.T) {
	v1, _, err := generatePKCE()
	require.NoError(t, err)
	v2, _, err := generatePKCE()
	require.NoError(t, err)
	assert.NotEqual(t, v1, v2)
}

func TestGenerateState(t *testing.T) {
	state, err := generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state)

	// base64url-encoded 16 bytes = 22 characters
	assert.Len(t, state, 22)
}

// TestPKCERFC7636AppendixB verifies the S256 transform against the
// RFC 7636 Appendix B test vector.
func TestPKCERFC7636AppendixB(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expectedChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	assert.Equal(t, expectedChallenge, challenge)
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"https://example.com", "https://example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"example.com", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"marmot.dev/", "https://marmot.dev"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeHost(tt.input))
		})
	}
}

func TestResolveLoginHost(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()
	setupTestViper(t)

	t.Run("from argument", func(t *testing.T) {
		host, name, err := resolveLoginHost([]string{"https://marmot.example.com"})
		require.NoError(t, err)
		assert.Equal(t, "https://marmot.example.com", host)
		assert.Equal(t, "marmot.example.com", name)
	})

	t.Run("from argument with port", func(t *testing.T) {
		host, name, err := resolveLoginHost([]string{"http://localhost:8080"})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", host)
		assert.Equal(t, "localhost:8080", name)
	})

	t.Run("from argument bare domain", func(t *testing.T) {
		host, name, err := resolveLoginHost([]string{"marmot.dev"})
		require.NoError(t, err)
		assert.Equal(t, "https://marmot.dev", host)
		assert.Equal(t, "marmot.dev", name)
	})
}
