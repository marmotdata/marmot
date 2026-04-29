package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/pkg/config"
)

func TestHandleASMetadata(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	h := &Handler{config: cfg}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()

	h.handleASMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var meta asMetadata
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if meta.Issuer != "https://marmot.example.com" {
		t.Fatalf("expected issuer 'https://marmot.example.com', got %q", meta.Issuer)
	}
	if meta.AuthorizationEndpoint != "https://marmot.example.com/oauth/authorize" {
		t.Fatalf("unexpected authorization_endpoint: %q", meta.AuthorizationEndpoint)
	}
	if meta.TokenEndpoint != "https://marmot.example.com/oauth/token" {
		t.Fatalf("unexpected token_endpoint: %q", meta.TokenEndpoint)
	}
	if meta.RegistrationEndpoint != "https://marmot.example.com/oauth/register" {
		t.Fatalf("unexpected registration_endpoint: %q", meta.RegistrationEndpoint)
	}

	if len(meta.ResponseTypesSupported) != 1 || meta.ResponseTypesSupported[0] != "code" {
		t.Fatalf("expected response_types_supported=[code], got %v", meta.ResponseTypesSupported)
	}
	if len(meta.GrantTypesSupported) != 1 || meta.GrantTypesSupported[0] != "authorization_code" {
		t.Fatalf("expected grant_types_supported=[authorization_code], got %v", meta.GrantTypesSupported)
	}
	if len(meta.CodeChallengeMethodsSupported) != 1 || meta.CodeChallengeMethodsSupported[0] != "S256" {
		t.Fatalf("expected code_challenge_methods_supported=[S256], got %v", meta.CodeChallengeMethodsSupported)
	}
	if len(meta.TokenEndpointAuthMethodsSupported) != 1 || meta.TokenEndpointAuthMethodsSupported[0] != "none" {
		t.Fatalf("expected token_endpoint_auth_methods_supported=[none], got %v", meta.TokenEndpointAuthMethodsSupported)
	}
	if len(meta.ScopesSupported) != 1 || meta.ScopesSupported[0] != "openid" {
		t.Fatalf("expected scopes_supported=[openid], got %v", meta.ScopesSupported)
	}
}
