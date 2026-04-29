package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/ory/fosite"
	"github.com/marmotdata/marmot/pkg/config"
)

func newAuthorizeHandler() *Handler {
	secret := []byte("test-secret-key-for-authorize--!")
	provider := marmotOAuth2.NewProvider(secret)
	store := marmotOAuth2.NewAuthorizeSessionStore()

	cfg := &config.Config{}
	cfg.Server.RootURL = "http://localhost:8080"

	provider.Store.RegisterClient(&fosite.DefaultClient{
		ID:            "test-client-1",
		Public:        true,
		RedirectURIs:  []string{"http://localhost:9999/callback"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	})

	return &Handler{
		oauthProvider:         provider,
		authorizeSessionStore: store,
		config:                cfg,
	}
}

func TestAuthorize_RedirectsToLogin(t *testing.T) {
	h := newAuthorizeHandler()

	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client-1"},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"test-state"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil)
	rec := httptest.NewRecorder()

	h.handleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location != "http://localhost:8080/login?oauth_pending=1" {
		t.Fatalf("expected redirect to /login?oauth_pending=1, got %q", location)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected oauth_session cookie")
	}
	if sessionCookie.Value == "" {
		t.Fatal("expected non-empty session cookie value")
	}
}

func TestAuthorize_MissingPKCE_RejectedAtTokenEndpoint(t *testing.T) {
	h := newAuthorizeHandler()

	params := url.Values{
		"response_type": {"code"},
		"client_id":     {"test-client-1"},
		"redirect_uri":  {"http://localhost:9999/callback"},
		"state":         {"test-state"},
		"scope":         {"openid"},
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil)
	rec := httptest.NewRecorder()

	h.handleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
}

func TestAuthorize_InvalidClient(t *testing.T) {
	h := newAuthorizeHandler()

	hash := sha256.Sum256([]byte("verifier"))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {"nonexistent-client"},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"test-state"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil)
	rec := httptest.NewRecorder()

	h.handleAuthorize(rec, req)

	location := rec.Header().Get("Location")
	if location == "http://localhost:8080/login?oauth_pending=1" {
		t.Fatal("should not redirect to login for invalid client")
	}
}

func TestHasPendingAuthorize_NoCookie(t *testing.T) {
	h := newAuthorizeHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if h.HasPendingAuthorize(req) {
		t.Fatal("expected false without cookie")
	}
}

func TestCompleteAuthorize_Flow(t *testing.T) {
	h := newAuthorizeHandler()

	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client-1"},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"test-state-123"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil)
	rec := httptest.NewRecorder()

	h.handleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("authorize: expected 302, got %d", rec.Code)
	}

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no oauth_session cookie")
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", nil)
	loginReq.AddCookie(sessionCookie)
	if !h.HasPendingAuthorize(loginReq) {
		t.Fatal("expected pending authorize")
	}

	completeRec := httptest.NewRecorder()
	completeReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", nil)
	completeReq.AddCookie(sessionCookie)

	redirectURL, err := h.CompleteAuthorize(completeRec, completeReq, "user-123", "alice")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}

	if parsed.Host != "localhost:9999" {
		t.Fatalf("expected redirect to localhost:9999, got %q", parsed.Host)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		t.Fatal("expected code in redirect URL")
	}

	state := parsed.Query().Get("state")
	if state != "test-state-123" {
		t.Fatalf("expected state 'test-state-123', got %q", state)
	}

	checkReq := httptest.NewRequest(http.MethodPost, "/", nil)
	checkReq.AddCookie(sessionCookie)
	if h.HasPendingAuthorize(checkReq) {
		t.Fatal("expected no pending authorize after completion")
	}
}
