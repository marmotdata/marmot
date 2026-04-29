package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

// --- mocks ---

type mockAuthService struct{}

func (m *mockAuthService) GenerateToken(_ context.Context, _ *user.User, _ map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockAuthService) ValidateToken(_ context.Context, _ string) (*auth.Claims, error) {
	return nil, auth.ErrNotFound
}
func (m *mockAuthService) GetSigningKey(_ context.Context) ([]byte, error) {
	return nil, nil
}

type mockUserService struct {
	validateAPIKeyFn func(ctx context.Context, key string) (*user.User, error)
	getFn            func(ctx context.Context, id string) (*user.User, error)
}

func (m *mockUserService) ValidateAPIKey(ctx context.Context, key string) (*user.User, error) {
	if m.validateAPIKeyFn != nil {
		return m.validateAPIKeyFn(ctx, key)
	}
	return nil, user.ErrInvalidAPIKey
}
func (m *mockUserService) Get(ctx context.Context, id string) (*user.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, user.ErrUserNotFound
}

// Unused methods to satisfy user.Service interface
func (m *mockUserService) Create(_ context.Context, _ user.CreateUserInput) (*user.User, error) { return nil, nil }
func (m *mockUserService) Update(_ context.Context, _ string, _ user.UpdateUserInput) (*user.User, error) { return nil, nil }
func (m *mockUserService) Delete(_ context.Context, _, _ string) error                                    { return nil }
func (m *mockUserService) GetUserByUsername(_ context.Context, _ string) (*user.User, error)               { return nil, nil }
func (m *mockUserService) FindSimilarUsernames(_ context.Context, _ string, _ int) ([]string, error)      { return nil, nil }
func (m *mockUserService) List(_ context.Context, _ user.Filter) ([]*user.User, int, error)               { return nil, 0, nil }
func (m *mockUserService) Authenticate(_ context.Context, _, _ string) (*user.User, error)                { return nil, nil }
func (m *mockUserService) HasPermission(_ context.Context, _, _ string, _ string) (bool, error)           { return false, nil }
func (m *mockUserService) GetPermissionsByRoleName(_ context.Context, _ string) ([]user.Permission, error) { return nil, nil }
func (m *mockUserService) GetUserByProviderID(_ context.Context, _, _ string) (*user.User, error)         { return nil, nil }
func (m *mockUserService) AuthenticateOAuth(_ context.Context, _, _ string, _ map[string]interface{}) (*user.User, error) { return nil, nil }
func (m *mockUserService) LinkOAuthAccount(_ context.Context, _, _, _ string, _ map[string]interface{}) error { return nil }
func (m *mockUserService) UnlinkOAuthAccount(_ context.Context, _, _ string) error                        { return nil }
func (m *mockUserService) CreateAPIKey(_ context.Context, _, _ string, _ *time.Duration) (*user.APIKey, error) { return nil, nil }
func (m *mockUserService) DeleteAPIKey(_ context.Context, _, _ string) error                              { return nil }
func (m *mockUserService) ListAPIKeys(_ context.Context, _ string) ([]*user.APIKey, error)                { return nil, nil }
func (m *mockUserService) UpdatePreferences(_ context.Context, _ string, _ map[string]interface{}) error  { return nil }
func (m *mockUserService) UpdatePassword(_ context.Context, _, _ string) (*user.User, error)              { return nil, nil }

type mockOAuthProvider struct {
	typ string
}

func (m *mockOAuthProvider) GetAuthURL(_ string) string                                      { return "" }
func (m *mockOAuthProvider) HandleCallback(_ context.Context, _ string) (*user.User, error)  { return nil, nil }
func (m *mockOAuthProvider) Name() string                                                    { return m.typ }
func (m *mockOAuthProvider) Type() string                                                    { return m.typ }

type mockExchangeProvider struct {
	mockOAuthProvider
	exchangeFn func(ctx context.Context, rawIDToken string) (*user.User, error)
}

func (m *mockExchangeProvider) ExchangeToken(ctx context.Context, rawIDToken string) (*user.User, error) {
	return m.exchangeFn(ctx, rawIDToken)
}

// --- helpers ---

func makeTestJWT(iss string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload, _ := json.Marshal(map[string]string{"iss": iss, "sub": "user1"})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".fakesig"
}

// --- looksLikeOIDCToken tests ---

func TestLooksLikeOIDCToken_WithIss(t *testing.T) {
	token := makeTestJWT("https://example.auth0.com")
	if !looksLikeOIDCToken(token) {
		t.Fatal("expected true for token with iss claim")
	}
}

func TestLooksLikeOIDCToken_NotJWT(t *testing.T) {
	if looksLikeOIDCToken("not-a-jwt") {
		t.Fatal("expected false for non-JWT")
	}
}

func TestLooksLikeOIDCToken_NoIss(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user1"}`))
	token := header + "." + payload + ".sig"

	if looksLikeOIDCToken(token) {
		t.Fatal("expected false for JWT without iss claim")
	}
}

// --- tryOIDCExchange tests ---

func TestTryOIDCExchange_Success(t *testing.T) {
	testUser := &user.User{ID: "exchanged-user", Active: true}
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "auth0"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return testUser, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	token := makeTestJWT("https://example.auth0.com")
	result := tryOIDCExchange(context.Background(), mgr, token)

	if result == nil {
		t.Fatal("expected user, got nil")
	}
	if result.ID != "exchanged-user" {
		t.Fatalf("expected user ID 'exchanged-user', got %q", result.ID)
	}
}

func TestTryOIDCExchange_AllProvidersFail(t *testing.T) {
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "auth0"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, errors.New("verification failed")
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	token := makeTestJWT("https://unknown.example.com")
	result := tryOIDCExchange(context.Background(), mgr, token)

	if result != nil {
		t.Fatalf("expected nil when all providers fail, got %v", result)
	}
}

func TestTryOIDCExchange_NotAJWT(t *testing.T) {
	mgr := auth.NewOAuthManager()
	result := tryOIDCExchange(context.Background(), mgr, "just-an-api-key")

	if result != nil {
		t.Fatalf("expected nil for non-JWT token, got %v", result)
	}
}

func TestTryOIDCExchange_MarmotJWT(t *testing.T) {
	// Marmot JWTs have no iss claim — should not trigger exchange
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "auth0"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			t.Fatal("exchange should not be called for Marmot JWT")
			return nil, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	// Marmot JWT: has sub but no iss
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user-id-123","roles":["user"]}`))
	token := header + "." + payload + ".hmac-sig"

	result := tryOIDCExchange(context.Background(), mgr, token)
	if result != nil {
		t.Fatalf("expected nil for Marmot JWT, got %v", result)
	}
}

func TestTryOIDCExchange_TriesMultipleProviders(t *testing.T) {
	testUser := &user.User{ID: "found-user", Active: true}
	failProvider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, errors.New("wrong provider")
		},
	}
	successProvider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "keycloak"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return testUser, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(failProvider)
	mgr.RegisterProvider(successProvider)

	token := makeTestJWT("https://keycloak.example.com/realms/prod")
	result := tryOIDCExchange(context.Background(), mgr, token)

	if result == nil {
		t.Fatal("expected user, got nil")
	}
	if result.ID != "found-user" {
		t.Fatalf("expected user ID 'found-user', got %q", result.ID)
	}
}

// --- WithAuth middleware integration tests ---

func TestWithAuth_OIDCTokenExchange(t *testing.T) {
	testUser := &user.User{ID: "oidc-user", Active: true}
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return testUser, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	// Set global OAuth manager for middleware
	oldMgr := globalOAuthManager
	globalOAuthManager = mgr
	defer func() { globalOAuthManager = oldMgr }()

	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}

	var capturedUser *user.User
	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(UserContextKey).(*user.User)
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	})

	token := makeTestJWT("https://dev.okta.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil || capturedUser.ID != "oidc-user" {
		t.Fatalf("expected user 'oidc-user' in context, got %v", capturedUser)
	}
}

func TestWithAuth_OIDCInactiveUser(t *testing.T) {
	inactiveUser := &user.User{ID: "inactive", Active: false}
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return inactiveUser, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	oldMgr := globalOAuthManager
	globalOAuthManager = mgr
	defer func() { globalOAuthManager = oldMgr }()

	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}

	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for inactive user")
	})

	token := makeTestJWT("https://dev.okta.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// --- WWW-Authenticate header tests ---

func TestWithAuth_WWWAuthenticate_NoAuth(t *testing.T) {
	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	expected := `Bearer resource_metadata="https://marmot.example.com/.well-known/oauth-protected-resource"`
	if wwwAuth != expected {
		t.Fatalf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestWithAuth_WWWAuthenticate_InvalidBearer(t *testing.T) {
	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Fatal("expected WWW-Authenticate header on 401 response")
	}
	expected := `Bearer resource_metadata="https://marmot.example.com/.well-known/oauth-protected-resource"`
	if wwwAuth != expected {
		t.Fatalf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestWithAuth_WWWAuthenticate_NotOnSuccess(t *testing.T) {
	testUser := &user.User{ID: "oidc-user", Active: true}
	provider := &mockExchangeProvider{
		mockOAuthProvider: mockOAuthProvider{typ: "okta"},
		exchangeFn: func(_ context.Context, _ string) (*user.User, error) {
			return testUser, nil
		},
	}

	mgr := auth.NewOAuthManager()
	mgr.RegisterProvider(provider)

	oldMgr := globalOAuthManager
	globalOAuthManager = mgr
	defer func() { globalOAuthManager = oldMgr }()

	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}
	cfg.Server.RootURL = "https://marmot.example.com"

	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token := makeTestJWT("https://dev.okta.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Fatalf("expected no WWW-Authenticate header on 200, got %q", wwwAuth)
	}
}

func TestWithAuth_WWWAuthenticate_NoRootURL(t *testing.T) {
	userSvc := &mockUserService{}
	authSvc := &mockAuthService{}
	cfg := &config.Config{} // no RootURL set

	handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Fatalf("expected no WWW-Authenticate header when RootURL is empty, got %q", wwwAuth)
	}
}
