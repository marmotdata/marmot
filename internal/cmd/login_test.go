package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		{"localhost:8080", "http://localhost:8080"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"localhost", "http://localhost"},
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

	t.Run("bare name of a saved context keeps its scheme", func(t *testing.T) {
		require.NoError(t, setContext("localhost:8080", ContextEntry{Host: "http://localhost:8080"}))
		host, name, err := resolveLoginHost([]string{"localhost:8080"})
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

func TestParseCallback(t *testing.T) {
	ok := parseCallback(url.Values{"code": {"abc"}, "state": {"xyz"}})
	assert.Equal(t, callbackResult{Code: "abc", State: "xyz"}, ok)

	denied := parseCallback(url.Values{"error": {"access_denied"}, "error_description": {"user cancelled"}})
	assert.Equal(t, "access_denied: user cancelled", denied.Err)
}

// TestBrowserLoginPastedCallback drives the flow without a browser: a fake
// Marmot registers the client and exchanges the code, and the callback URL is
// pasted on stdin the way a user on a remote machine would.
func TestBrowserLoginPastedCallback(t *testing.T) {
	var gotVerifier, gotCode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/register":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"client_id":"cli-1"}`))
		case "/oauth/token":
			require.NoError(t, r.ParseForm())
			gotVerifier = r.FormValue("code_verifier")
			gotCode = r.FormValue("code")
			_, _ = w.Write([]byte(`{"access_token":"tok-1","token_type":"Bearer","expires_in":3600}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Read the printed sign-in URL to learn the state, then "paste" a callback.
	statusR, statusW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(statusR)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, "/oauth/authorize?") {
				continue
			}
			u, err := url.Parse(strings.TrimSpace(line))
			require.NoError(t, err)
			state := u.Query().Get("state")
			// Write from another goroutine and keep draining status lines:
			// both pipes block until the other side reads.
			go func() {
				_, _ = fmt.Fprintf(stdinW, "http://localhost:1/callback?code=code-1&state=%s\n", state)
			}()
		}
	}()

	tok, err := browserLogin(server.URL, "test", statusW, stdinR, false)
	_ = statusW.Close()
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	assert.Equal(t, 3600, tok.ExpiresIn)
	assert.Equal(t, "code-1", gotCode)
	assert.Len(t, gotVerifier, 43)
}
