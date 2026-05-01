package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	coreauth "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type mockIssuerProvider struct {
	typ       string
	issuerURL string
}

func (m *mockIssuerProvider) GetAuthURL(_ string) string                                     { return "" }
func (m *mockIssuerProvider) HandleCallback(_ context.Context, _ string) (*user.User, error) { return nil, nil }
func (m *mockIssuerProvider) Name() string                                                   { return m.typ }
func (m *mockIssuerProvider) Type() string                                                   { return m.typ }
func (m *mockIssuerProvider) IssuerURL() string                                              { return m.issuerURL }

func TestHandleProtectedResourceMetadata_NoProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	h := &Handler{
		oauthManager: coreauth.NewOAuthManager(),
		config:       cfg,
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()

	h.handleProtectedResourceMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var meta protectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if meta.Resource != "https://marmot.example.com" {
		t.Fatalf("expected resource 'https://marmot.example.com', got %q", meta.Resource)
	}

	if len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected authorization_servers=[self], got %v", meta.AuthorizationServers)
	}

	if len(meta.ScopesSupported) != 1 || meta.ScopesSupported[0] != "openid" {
		t.Fatalf("expected scopes_supported=[openid], got %v", meta.ScopesSupported)
	}

	if len(meta.BearerMethodsSupported) != 1 || meta.BearerMethodsSupported[0] != "header" {
		t.Fatalf("expected bearer_methods_supported=[header], got %v", meta.BearerMethodsSupported)
	}
}

func TestHandleProtectedResourceMetadata_DoesNotAdvertiseUpstreamIssuers(t *testing.T) {
	mgr := coreauth.NewOAuthManager()
	mgr.RegisterProvider(&mockIssuerProvider{
		typ:       "keycloak",
		issuerURL: "http://localhost:8180/realms/marmot",
	})
	mgr.RegisterProvider(&mockIssuerProvider{
		typ:       "google",
		issuerURL: "https://accounts.google.com",
	})

	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	h := &Handler{
		oauthManager: mgr,
		config:       cfg,
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()

	h.handleProtectedResourceMetadata(rec, req)

	var meta protectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected authorization_servers=[self] only, got %v", meta.AuthorizationServers)
	}
}
