package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/marmotdata/marmot/pkg/config"
)

func newDCRHandler() *Handler {
	provider := marmotOAuth2.NewProvider([]byte("test-secret-key-for-dcr-tests"))
	return &Handler{
		oauthProvider: provider,
		config:        &config.Config{},
	}
}

func TestDCR_Success(t *testing.T) {
	h := newDCRHandler()

	body := `{"redirect_uris":["http://localhost:9999/callback"],"client_name":"Test Client"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleDCR(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp dcrResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ClientID == "" {
		t.Fatal("expected non-empty client_id")
	}
	if resp.ClientName != "Test Client" {
		t.Fatalf("expected client_name 'Test Client', got %q", resp.ClientName)
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "http://localhost:9999/callback" {
		t.Fatalf("unexpected redirect_uris: %v", resp.RedirectURIs)
	}
	if resp.TokenEndpointAuthMethod != "none" {
		t.Fatalf("expected token_endpoint_auth_method 'none', got %q", resp.TokenEndpointAuthMethod)
	}

	_, err := h.oauthProvider.Store.GetClient(req.Context(), resp.ClientID)
	if err != nil {
		t.Fatalf("client not found in store: %v", err)
	}
}

func TestDCR_MissingRedirectURIs(t *testing.T) {
	h := newDCRHandler()

	body := `{"client_name":"Bad Client"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleDCR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp oauthErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_client_metadata" {
		t.Fatalf("expected error 'invalid_client_metadata', got %q", resp.Error)
	}
}

func TestDCR_InvalidAuthMethod(t *testing.T) {
	h := newDCRHandler()

	body := `{"redirect_uris":["http://localhost:9999/cb"],"token_endpoint_auth_method":"client_secret_basic"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleDCR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDCR_InvalidBody(t *testing.T) {
	h := newDCRHandler()

	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleDCR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDCR_NonLoopbackRedirectURI_Rejected(t *testing.T) {
	cases := []string{
		`{"redirect_uris":["https://attacker.com/cb"]}`,
		`{"redirect_uris":["http://attacker.com/cb"]}`,
		`{"redirect_uris":["http://example.com/cb"]}`,
		`{"redirect_uris":["http://localhost.attacker.com/cb"]}`,
		`{"redirect_uris":["javascript:alert(1)"]}`,
		`{"redirect_uris":["http://localhost/cb","http://attacker.com/cb"]}`,
	}

	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			h := newDCRHandler()
			req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.handleDCR(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %q, got %d: %s", body, rec.Code, rec.Body.String())
			}

			var resp oauthErrorResponse
			_ = json.NewDecoder(rec.Body).Decode(&resp)
			if resp.Error != "invalid_redirect_uri" {
				t.Fatalf("expected error 'invalid_redirect_uri' for %q, got %q", body, resp.Error)
			}
		})
	}
}

func TestDCR_LoopbackRedirectURIs_Accepted(t *testing.T) {
	cases := []string{
		`{"redirect_uris":["http://localhost:9999/callback"]}`,
		`{"redirect_uris":["http://localhost/callback"]}`,
		`{"redirect_uris":["http://127.0.0.1:54321/cb"]}`,
		`{"redirect_uris":["http://[::1]:8080/cb"]}`,
	}

	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			h := newDCRHandler()
			req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.handleDCR(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("expected 201 for %q, got %d: %s", body, rec.Code, rec.Body.String())
			}
		})
	}
}
