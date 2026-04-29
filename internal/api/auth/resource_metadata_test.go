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

func (m *mockIssuerProvider) GetAuthURL(_ string) string                                          { return "" }
func (m *mockIssuerProvider) HandleCallback(_ context.Context, _ string) (*user.User, error)      { return nil, nil }
func (m *mockIssuerProvider) Name() string                                                        { return m.typ }
func (m *mockIssuerProvider) Type() string                                                        { return m.typ }
func (m *mockIssuerProvider) IssuerURL() string                                                   { return m.issuerURL }

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

	var meta protectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(meta.AuthorizationServers) != 1 {
		t.Fatalf("expected 1 authorization server (self), got %d", len(meta.AuthorizationServers))
	}
	if meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected self URL, got %q", meta.AuthorizationServers[0])
	}
}

func TestHandleProtectedResourceMetadata_SingleProvider(t *testing.T) {
	mgr := coreauth.NewOAuthManager()
	mgr.RegisterProvider(&mockIssuerProvider{
		typ:       "keycloak",
		issuerURL: "http://localhost:8180/realms/marmot",
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

	if len(meta.AuthorizationServers) != 2 {
		t.Fatalf("expected 2 authorization servers (self + keycloak), got %d: %v", len(meta.AuthorizationServers), meta.AuthorizationServers)
	}
	if meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected self URL first, got %q", meta.AuthorizationServers[0])
	}
	if meta.AuthorizationServers[1] != "http://localhost:8180/realms/marmot" {
		t.Fatalf("expected keycloak second, got %q", meta.AuthorizationServers[1])
	}

	if len(meta.ScopesSupported) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(meta.ScopesSupported))
	}

	if len(meta.BearerMethodsSupported) != 1 || meta.BearerMethodsSupported[0] != "header" {
		t.Fatalf("expected bearer_methods_supported=[header], got %v", meta.BearerMethodsSupported)
	}
}

func TestHandleProtectedResourceMetadata_MultipleProviders(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var meta protectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(meta.AuthorizationServers) != 3 {
		t.Fatalf("expected 3 authorization servers (self + 2 issuers), got %d: %v", len(meta.AuthorizationServers), meta.AuthorizationServers)
	}
	if meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected self URL first, got %q", meta.AuthorizationServers[0])
	}
}

func TestHandleProtectedResourceMetadata_DeduplicatesIssuers(t *testing.T) {
	mgr := coreauth.NewOAuthManager()
	mgr.RegisterProvider(&mockIssuerProvider{
		typ:       "generic_oidc",
		issuerURL: "https://accounts.google.com",
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

	if len(meta.AuthorizationServers) != 2 {
		t.Fatalf("expected 2 authorization servers (self + deduplicated), got %d: %v", len(meta.AuthorizationServers), meta.AuthorizationServers)
	}
	if meta.AuthorizationServers[0] != "https://marmot.example.com" {
		t.Fatalf("expected self URL first, got %q", meta.AuthorizationServers[0])
	}
}
