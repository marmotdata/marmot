package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	coreauth "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/marmotdata/marmot/pkg/config"
)

func TestFullPKCEFlow(t *testing.T) {
	secret := []byte("test-secret-key-for-full-flow--!")

	cfg := &config.Config{}
	cfg.Server.RootURL = "http://localhost:8080"

	provider := marmotOAuth2.NewProvider(secret)
	sessionStore := marmotOAuth2.NewAuthorizeSessionStore()

	testUser := &user.User{
		ID:       "user-pkce-1",
		Username: "alice",
		Active:   true,
	}

	authSvc := &mockAuthService{
		generateTokenFn: func(_ context.Context, u *user.User, _ map[string]interface{}) (string, error) {
			return "marmot-jwt-for-" + u.ID, nil
		},
	}

	userSvc := &mockUserService{
		getFn: func(_ context.Context, id string) (*user.User, error) {
			if id == testUser.ID {
				return testUser, nil
			}
			return nil, user.ErrUserNotFound
		},
	}

	h := &Handler{
		authService:           authSvc,
		userService:           userSvc,
		oauthManager:          coreauth.NewOAuthManager(),
		oauthProvider:         provider,
		authorizeSessionStore: sessionStore,
		config:                cfg,
	}

	dcrBody := `{"redirect_uris":["http://localhost:9999/callback"],"client_name":"MCP Test"}`
	dcrReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(dcrBody))
	dcrReq.Header.Set("Content-Type", "application/json")
	dcrRec := httptest.NewRecorder()

	h.handleDCR(dcrRec, dcrReq)

	if dcrRec.Code != http.StatusCreated {
		t.Fatalf("DCR: expected 201, got %d: %s", dcrRec.Code, dcrRec.Body.String())
	}

	var dcrResp dcrResponse
	_ = json.NewDecoder(dcrRec.Body).Decode(&dcrResp)
	clientID := dcrResp.ClientID
	if clientID == "" {
		t.Fatal("DCR: empty client_id")
	}
	t.Logf("DCR client_id: %s", clientID)

	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	authorizeParams := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"random-state-xyz"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}

	authzReq := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+authorizeParams.Encode(), nil)
	authzRec := httptest.NewRecorder()

	h.handleAuthorize(authzRec, authzReq)

	if authzRec.Code != http.StatusFound {
		t.Fatalf("Authorize: expected 302, got %d: %s", authzRec.Code, authzRec.Body.String())
	}

	location := authzRec.Header().Get("Location")
	if location != "http://localhost:8080/login?oauth_pending=1" {
		t.Fatalf("Authorize: expected redirect to /login?oauth_pending=1, got %q", location)
	}

	var oauthCookie *http.Cookie
	for _, c := range authzRec.Result().Cookies() {
		if c.Name == "oauth_session" {
			oauthCookie = c
			break
		}
	}
	if oauthCookie == nil {
		t.Fatal("Authorize: no oauth_session cookie")
	}

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", nil)
	loginReq.AddCookie(oauthCookie)

	if !h.HasPendingAuthorize(loginReq) {
		t.Fatal("Login: expected pending authorize")
	}

	redirectURL, err := h.CompleteAuthorize(loginRec, loginReq, testUser.ID, testUser.Username)
	if err != nil {
		t.Fatalf("CompleteAuthorize: %v", err)
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}

	authCode := parsed.Query().Get("code")
	if authCode == "" {
		t.Fatal("CompleteAuthorize: no code in redirect URL")
	}
	returnedState := parsed.Query().Get("state")
	if returnedState != "random-state-xyz" {
		t.Fatalf("expected state 'random-state-xyz', got %q", returnedState)
	}
	t.Logf("Auth code: %s", authCode)

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"redirect_uri":  {"http://localhost:9999/callback"},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}

	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()

	h.handleToken(tokenRec, tokenReq)

	if tokenRec.Code != http.StatusOK {
		t.Fatalf("Token: expected 200, got %d: %s", tokenRec.Code, tokenRec.Body.String())
	}

	var tokenResp map[string]interface{}
	if err := json.NewDecoder(tokenRec.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("Token: failed to decode response: %v", err)
	}

	accessToken, ok := tokenResp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("Token: missing access_token in response: %v", tokenResp)
	}
	if accessToken != "marmot-jwt-for-user-pkce-1" {
		t.Fatalf("expected 'marmot-jwt-for-user-pkce-1', got %q", accessToken)
	}

	tokenType, _ := tokenResp["token_type"].(string)
	if tokenType != "Bearer" {
		t.Fatalf("expected token_type 'Bearer', got %q", tokenType)
	}

	t.Logf("Full PKCE flow completed. Access token: %s", accessToken)
}

type mockUserService struct {
	getFn func(ctx context.Context, id string) (*user.User, error)
}

func (m *mockUserService) Get(ctx context.Context, id string) (*user.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, user.ErrUserNotFound
}

func (m *mockUserService) Create(_ context.Context, _ user.CreateUserInput) (*user.User, error) { return nil, nil }
func (m *mockUserService) Update(_ context.Context, _ string, _ user.UpdateUserInput) (*user.User, error) { return nil, nil }
func (m *mockUserService) Delete(_ context.Context, _, _ string) error                          { return nil }
func (m *mockUserService) GetUserByUsername(_ context.Context, _ string) (*user.User, error)     { return nil, nil }
func (m *mockUserService) FindSimilarUsernames(_ context.Context, _ string, _ int) ([]string, error) { return nil, nil }
func (m *mockUserService) List(_ context.Context, _ user.Filter) ([]*user.User, int, error)     { return nil, 0, nil }
func (m *mockUserService) Authenticate(_ context.Context, _, _ string) (*user.User, error)      { return nil, nil }
func (m *mockUserService) ValidateAPIKey(_ context.Context, _ string) (*user.User, error)       { return nil, nil }
func (m *mockUserService) HasPermission(_ context.Context, _, _ string, _ string) (bool, error)  { return false, nil }
func (m *mockUserService) GetPermissionsByRoleName(_ context.Context, _ string) ([]user.Permission, error) { return nil, nil }
func (m *mockUserService) GetUserByProviderID(_ context.Context, _, _ string) (*user.User, error) { return nil, nil }
func (m *mockUserService) AuthenticateOAuth(_ context.Context, _, _ string, _ map[string]interface{}) (*user.User, error) { return nil, nil }
func (m *mockUserService) LinkOAuthAccount(_ context.Context, _, _, _ string, _ map[string]interface{}) error { return nil }
func (m *mockUserService) UnlinkOAuthAccount(_ context.Context, _, _ string) error               { return nil }
func (m *mockUserService) CreateAPIKey(_ context.Context, _, _ string, _ *time.Duration) (*user.APIKey, error) { return nil, nil }
func (m *mockUserService) DeleteAPIKey(_ context.Context, _, _ string) error                     { return nil }
func (m *mockUserService) ListAPIKeys(_ context.Context, _ string) ([]*user.APIKey, error)       { return nil, nil }
func (m *mockUserService) UpdatePreferences(_ context.Context, _ string, _ map[string]interface{}) error { return nil }
func (m *mockUserService) UpdatePassword(_ context.Context, _, _ string) (*user.User, error)     { return nil, nil }
