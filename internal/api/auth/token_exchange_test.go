package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	coreauth "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

// --- mocks ---

type mockAuthService struct {
	generateTokenFn func(ctx context.Context, u *user.User, prefs map[string]interface{}) (string, error)
}

func (m *mockAuthService) GenerateToken(ctx context.Context, u *user.User, prefs map[string]interface{}) (string, error) {
	return m.generateTokenFn(ctx, u, prefs)
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (*coreauth.Claims, error) {
	return nil, nil
}

func (m *mockAuthService) GetSigningKey(ctx context.Context) ([]byte, error) {
	return nil, nil
}

type mockOAuthProvider struct {
	name string
	typ  string
}

func (m *mockOAuthProvider) GetAuthURL(state string) string                                      { return "" }
func (m *mockOAuthProvider) HandleCallback(ctx context.Context, code string) (*user.User, error) { return nil, nil }
func (m *mockOAuthProvider) Name() string                                                        { return m.name }
func (m *mockOAuthProvider) Type() string                                                        { return m.typ }

type mockTokenExchangerProvider struct {
	mockOAuthProvider
	exchangeFn func(ctx context.Context, rawIDToken string) (*user.User, error)
}

func (m *mockTokenExchangerProvider) ExchangeToken(ctx context.Context, rawIDToken string) (*user.User, error) {
	return m.exchangeFn(ctx, rawIDToken)
}

type mockAccessTokenExchangerProvider struct {
	mockOAuthProvider
	exchangeFn func(ctx context.Context, accessToken string) (*user.User, error)
}

func (m *mockAccessTokenExchangerProvider) ExchangeAccessToken(ctx context.Context, accessToken string) (*user.User, error) {
	return m.exchangeFn(ctx, accessToken)
}

// --- helpers ---

func makeJWT(iss string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload, _ := json.Marshal(map[string]string{"iss": iss, "sub": "user123"})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + payloadB64 + ".fakesig"
}

func newTestHandler(providers map[string]coreauth.OAuthProvider, authSvc coreauth.Service) *Handler {
	mgr := coreauth.NewOAuthManager()
	for _, p := range providers {
		mgr.RegisterProvider(p)
	}
	return &Handler{
		authService:  authSvc,
		oauthManager: mgr,
		config:       &config.Config{},
	}
}

func makeExchangeForm(subjectToken, subjectTokenType string) string {
	v := url.Values{}
	v.Set("grant_type", grantTypeTokenExchange)
	if subjectToken != "" {
		v.Set("subject_token", subjectToken)
	}
	if subjectTokenType != "" {
		v.Set("subject_token_type", subjectTokenType)
	}
	return v.Encode()
}

func postExchange(h *Handler, formBody string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(formBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.handleToken(rec, req)
	return rec
}

func decodeOAuthError(t *testing.T, rec *httptest.ResponseRecorder) oauthErrorResponse {
	t.Helper()
	var resp oauthErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return resp
}

// --- tests ---

func TestTokenExchange_WrongGrantType(t *testing.T) {
	h := newTestHandler(nil, nil)
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("subject_token", "x.y.z")
	form.Set("subject_token_type", tokenTypeIDToken)

	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.handleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "unsupported_grant_type" {
		t.Fatalf("expected error 'unsupported_grant_type', got %q", resp.Error)
	}
}

func TestTokenExchange_MissingSubjectToken(t *testing.T) {
	h := newTestHandler(nil, nil)
	rec := postExchange(h, makeExchangeForm("", tokenTypeIDToken))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_request" {
		t.Fatalf("expected error 'invalid_request', got %q", resp.Error)
	}
}

func TestTokenExchange_UnsupportedSubjectTokenType(t *testing.T) {
	h := newTestHandler(nil, nil)
	form := url.Values{}
	form.Set("grant_type", grantTypeTokenExchange)
	form.Set("subject_token", "x.y.z")
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:saml2")

	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.handleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_request" {
		t.Fatalf("expected error 'invalid_request', got %q", resp.Error)
	}
}

func TestTokenExchange_Success(t *testing.T) {
	testUser := &user.User{ID: "u1", Name: "Test"}
	provider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn:        func(_ context.Context, _ string) (*user.User, error) { return testUser, nil },
	}
	authSvc := &mockAuthService{
		generateTokenFn: func(_ context.Context, _ *user.User, _ map[string]interface{}) (string, error) {
			return "marmot-jwt-token", nil
		},
	}

	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": provider}, authSvc)

	token := makeJWT("https://dev.okta.com")
	rec := postExchange(h, makeExchangeForm(token, tokenTypeIDToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp tokenExchangeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken != "marmot-jwt-token" {
		t.Fatalf("expected access_token 'marmot-jwt-token', got %q", resp.AccessToken)
	}
	if resp.IssuedTokenType != tokenTypeAccessToken {
		t.Fatalf("expected issued_token_type %q, got %q", tokenTypeAccessToken, resp.IssuedTokenType)
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("expected token_type 'Bearer', got %q", resp.TokenType)
	}
}

func TestTokenExchange_AllProvidersFail(t *testing.T) {
	provider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, errors.New("verification failed")
		},
	}
	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": provider}, nil)

	token := makeJWT("https://dev.okta.com")
	rec := postExchange(h, makeExchangeForm(token, tokenTypeIDToken))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_grant" {
		t.Fatalf("expected error 'invalid_grant', got %q", resp.Error)
	}
}

func TestTokenExchange_GenerateTokenFailure(t *testing.T) {
	testUser := &user.User{ID: "u1", Name: "Test"}
	provider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn:        func(_ context.Context, _ string) (*user.User, error) { return testUser, nil },
	}
	authSvc := &mockAuthService{
		generateTokenFn: func(_ context.Context, _ *user.User, _ map[string]interface{}) (string, error) {
			return "", errors.New("signing key error")
		},
	}

	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": provider}, authSvc)

	token := makeJWT("https://dev.okta.com")
	rec := postExchange(h, makeExchangeForm(token, tokenTypeIDToken))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "server_error" {
		t.Fatalf("expected error 'server_error', got %q", resp.Error)
	}
}

func TestTokenExchange_NoOIDCProviders(t *testing.T) {
	// Only non-OIDC provider registered
	nonOIDC := &mockOAuthProvider{name: "GitHub", typ: "github"}
	h := newTestHandler(map[string]coreauth.OAuthProvider{"github": nonOIDC}, nil)

	token := makeJWT("https://dev.okta.com")
	rec := postExchange(h, makeExchangeForm(token, tokenTypeIDToken))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_grant" {
		t.Fatalf("expected error 'invalid_grant', got %q", resp.Error)
	}
}

func TestTokenExchange_AccessTokenSuccess(t *testing.T) {
	testUser := &user.User{ID: "u1", Name: "Test"}
	provider := &mockAccessTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn:        func(_ context.Context, _ string) (*user.User, error) { return testUser, nil },
	}
	authSvc := &mockAuthService{
		generateTokenFn: func(_ context.Context, _ *user.User, _ map[string]interface{}) (string, error) {
			return "marmot-jwt-token", nil
		},
	}

	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": provider}, authSvc)

	rec := postExchange(h, makeExchangeForm("opaque-access-token", tokenTypeAccessToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp tokenExchangeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken != "marmot-jwt-token" {
		t.Fatalf("expected access_token 'marmot-jwt-token', got %q", resp.AccessToken)
	}
	if resp.IssuedTokenType != tokenTypeAccessToken {
		t.Fatalf("expected issued_token_type %q, got %q", tokenTypeAccessToken, resp.IssuedTokenType)
	}
}

func TestTokenExchange_AccessToken_AllProvidersFail(t *testing.T) {
	provider := &mockAccessTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, errors.New("userinfo request failed")
		},
	}
	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": provider}, nil)

	rec := postExchange(h, makeExchangeForm("opaque-access-token", tokenTypeAccessToken))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_grant" {
		t.Fatalf("expected error 'invalid_grant', got %q", resp.Error)
	}
}

// A provider that only implements TokenExchanger (not AccessTokenExchanger) must be
// skipped on the access-token path; with no other providers, the request should 401.
func TestTokenExchange_AccessToken_ProviderDoesNotImplement(t *testing.T) {
	idOnlyProvider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn:        func(_ context.Context, _ string) (*user.User, error) { return &user.User{ID: "u1"}, nil },
	}
	h := newTestHandler(map[string]coreauth.OAuthProvider{"okta": idOnlyProvider}, nil)

	rec := postExchange(h, makeExchangeForm("opaque-access-token", tokenTypeAccessToken))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	resp := decodeOAuthError(t, rec)
	if resp.Error != "invalid_grant" {
		t.Fatalf("expected error 'invalid_grant', got %q", resp.Error)
	}
}

func TestTokenExchange_TriesMultipleProviders(t *testing.T) {
	testUser := &user.User{ID: "u2", Name: "Test"}
	failProvider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Okta", typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, errors.New("wrong issuer")
		},
	}
	successProvider := &mockTokenExchangerProvider{
		mockOAuthProvider: mockOAuthProvider{name: "Keycloak", typ: "keycloak"},
		exchangeFn:        func(_ context.Context, _ string) (*user.User, error) { return testUser, nil },
	}
	authSvc := &mockAuthService{
		generateTokenFn: func(_ context.Context, _ *user.User, _ map[string]interface{}) (string, error) {
			return "marmot-jwt", nil
		},
	}

	h := newTestHandler(map[string]coreauth.OAuthProvider{
		"okta":     failProvider,
		"keycloak": successProvider,
	}, authSvc)

	token := makeJWT("https://keycloak.example.com/realms/prod")
	rec := postExchange(h, makeExchangeForm(token, tokenTypeIDToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
