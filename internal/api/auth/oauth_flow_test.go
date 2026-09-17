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
	"sync"
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

	provider := marmotOAuth2.NewProvider(secret, marmotOAuth2.NewMemoryRepository())

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
		authService:   authSvc,
		userService:   userSvc,
		oauthManager:  coreauth.NewOAuthManager(),
		oauthProvider: provider,
		config:        cfg,
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

// replicaCluster hands out Handlers that share nothing but the repository and
// the HMAC secret, the way the replicas of one instance do. A load balancer
// with no session affinity can serve each request of a login from any of them.
type replicaCluster struct {
	t    *testing.T
	repo marmotOAuth2.Repository
	user *user.User
}

func newReplicaCluster(t *testing.T) *replicaCluster {
	t.Helper()
	return &replicaCluster{
		t:    t,
		repo: marmotOAuth2.NewMemoryRepository(),
		user: &user.User{ID: "user-replica-1", Username: "alice", Active: true},
	}
}

func (c *replicaCluster) replica(secret string) *Handler {
	cfg := &config.Config{}
	cfg.Server.RootURL = "http://localhost:8080"
	return &Handler{
		authService: &mockAuthService{
			generateTokenFn: func(_ context.Context, u *user.User, _ map[string]interface{}) (string, error) {
				return "marmot-jwt-for-" + u.ID, nil
			},
		},
		userService: &mockUserService{
			getFn: func(_ context.Context, id string) (*user.User, error) {
				if id == c.user.ID {
					return c.user, nil
				}
				return nil, user.ErrUserNotFound
			},
		},
		oauthManager:  coreauth.NewOAuthManager(),
		oauthProvider: marmotOAuth2.NewProvider([]byte(secret), c.repo),
		config:        cfg,
	}
}

const testCodeVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

// register runs dynamic client registration and returns the client id.
func (c *replicaCluster) register(h *Handler) string {
	c.t.Helper()
	body := `{"redirect_uris":["http://localhost:9999/callback"],"client_name":"CLI"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.handleDCR(rec, req)
	if rec.Code != http.StatusCreated {
		c.t.Fatalf("DCR: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp dcrResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		c.t.Fatalf("DCR: decode: %v", err)
	}
	return resp.ClientID
}

// authorizeAndComplete signs the user in and returns the authorization code.
func (c *replicaCluster) authorizeAndComplete(authorizeOn, completeOn *Handler, clientID string) string {
	c.t.Helper()

	hash := sha256.Sum256([]byte(testCodeVerifier))
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"state-across-replicas"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(hash[:])},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}
	rec := httptest.NewRecorder()
	authorizeOn.handleAuthorize(rec, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil))
	if rec.Code != http.StatusFound {
		c.t.Fatalf("Authorize: expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	var cookie *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "oauth_session" {
			cookie = ck
			break
		}
	}
	if cookie == nil {
		c.t.Fatal("Authorize: no oauth_session cookie")
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", nil)
	loginReq.AddCookie(cookie)
	if !completeOn.HasPendingAuthorize(loginReq) {
		c.t.Fatal("Complete: expected pending authorize")
	}
	redirectURL, err := completeOn.CompleteAuthorize(httptest.NewRecorder(), loginReq, c.user.ID, c.user.Username)
	if err != nil {
		c.t.Fatalf("Complete: %v", err)
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		c.t.Fatalf("parse redirect: %v", err)
	}
	code := parsed.Query().Get("code")
	if code == "" {
		c.t.Fatalf("Complete: no code in redirect %q", redirectURL)
	}
	return code
}

func (c *replicaCluster) redeem(h *Handler, clientID, code, verifier, redirectURI string) *httptest.ResponseRecorder {
	c.t.Helper()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.handleToken(rec, req)
	return rec
}

// TestPKCEFlowAcrossReplicas runs one sign-in through four separate handlers,
// the way a load balancer spreads the four requests over the replicas of an
// instance. With per-process state this fails at the authorize step with
// "invalid_client: The requested OAuth 2.0 Client does not exist."
func TestPKCEFlowAcrossReplicas(t *testing.T) {
	const secret = "test-secret-key-for-replicas---!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))
	code := c.authorizeAndComplete(c.replica(secret), c.replica(secret), clientID)
	rec := c.redeem(c.replica(secret), clientID, code, testCodeVerifier, "http://localhost:9999/callback")

	if rec.Code != http.StatusOK {
		t.Fatalf("Token on another replica: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Token: decode: %v", err)
	}
	if resp["access_token"] != "marmot-jwt-for-user-replica-1" {
		t.Fatalf("unexpected access_token: %v", resp["access_token"])
	}
}

// A code is good once. The replica that never issued it has to reject the
// replay, which it can only do from shared state.
func TestAuthorizeCodeIsSingleUseAcrossReplicas(t *testing.T) {
	const secret = "test-secret-key-for-replay-----!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))
	code := c.authorizeAndComplete(c.replica(secret), c.replica(secret), clientID)

	first := c.redeem(c.replica(secret), clientID, code, testCodeVerifier, "http://localhost:9999/callback")
	if first.Code != http.StatusOK {
		t.Fatalf("first redemption: expected 200, got %d: %s", first.Code, first.Body.String())
	}

	second := c.redeem(c.replica(secret), clientID, code, testCodeVerifier, "http://localhost:9999/callback")
	if second.Code == http.StatusOK {
		t.Fatalf("a replayed code was accepted on another replica: %s", second.Body.String())
	}
	var resp oauthErrorResponse
	if err := json.NewDecoder(second.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "invalid_grant" {
		t.Fatalf("expected invalid_grant on replay, got %q (%d)", resp.Error, second.Code)
	}
}

// Two replicas redeeming the same code at the same moment must not both
// succeed. This is the race the in-process map used to hide behind a mutex.
func TestConcurrentRedemptionYieldsOneToken(t *testing.T) {
	const secret = "test-secret-key-for-race------!!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))
	code := c.authorizeAndComplete(c.replica(secret), c.replica(secret), clientID)

	replicas := []*Handler{c.replica(secret), c.replica(secret), c.replica(secret), c.replica(secret)}
	results := make([]int, len(replicas))

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, h := range replicas {
		wg.Add(1)
		go func(i int, h *Handler) {
			defer wg.Done()
			<-start
			results[i] = c.redeem(h, clientID, code, testCodeVerifier, "http://localhost:9999/callback").Code
		}(i, h)
	}
	close(start)
	wg.Wait()

	accepted := 0
	for _, code := range results {
		if code == http.StatusOK {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("expected exactly one replica to issue a token, got %d (%v)", accepted, results)
	}
}

func TestTokenEndpointRejectsWrongVerifier(t *testing.T) {
	const secret = "test-secret-key-for-verifier--!!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))
	code := c.authorizeAndComplete(c.replica(secret), c.replica(secret), clientID)

	rec := c.redeem(c.replica(secret), clientID, code, "not-the-verifier-that-was-hashed-earlier", "http://localhost:9999/callback")
	if rec.Code == http.StatusOK {
		t.Fatalf("a wrong code_verifier was accepted: %s", rec.Body.String())
	}
}

func TestTokenEndpointRejectsMismatchedRedirectURI(t *testing.T) {
	const secret = "test-secret-key-for-redirect--!!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))
	code := c.authorizeAndComplete(c.replica(secret), c.replica(secret), clientID)

	rec := c.redeem(c.replica(secret), clientID, code, testCodeVerifier, "http://localhost:9999/other")
	if rec.Code == http.StatusOK {
		t.Fatalf("a mismatched redirect_uri was accepted: %s", rec.Body.String())
	}
}

// handleAuthorizePending backs the consent screen, which the browser can load
// from a replica other than the one that started the flow.
func TestPendingAuthorizeVisibleOnAnotherReplica(t *testing.T) {
	const secret = "test-secret-key-for-pending---!!"
	c := newReplicaCluster(t)

	clientID := c.register(c.replica(secret))

	hash := sha256.Sum256([]byte(testCodeVerifier))
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"http://localhost:9999/callback"},
		"state":                 {"state-pending"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(hash[:])},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid"},
	}
	authzRec := httptest.NewRecorder()
	c.replica(secret).handleAuthorize(authzRec, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil))

	pendingReq := httptest.NewRequest(http.MethodGet, "/oauth/authorize/pending", nil)
	for _, ck := range authzRec.Result().Cookies() {
		pendingReq.AddCookie(ck)
	}
	pendingRec := httptest.NewRecorder()
	c.replica(secret).handleAuthorizePending(pendingRec, pendingReq)

	if pendingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on another replica, got %d: %s", pendingRec.Code, pendingRec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(pendingRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["client_id"] != clientID {
		t.Fatalf("expected client_id %q, got %v", clientID, resp["client_id"])
	}
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

func (m *mockUserService) Create(_ context.Context, _ user.CreateUserInput) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) Update(_ context.Context, _ string, _ user.UpdateUserInput) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) Delete(_ context.Context, _, _ string) error { return nil }
func (m *mockUserService) GetUserByUsername(_ context.Context, _ string) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) FindSimilarUsernames(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, nil
}
func (m *mockUserService) List(_ context.Context, _ user.Filter) ([]*user.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserService) Authenticate(_ context.Context, _, _ string) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) ValidateAPIKey(_ context.Context, _ string) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) HasPermission(_ context.Context, _, _ string, _ string) (bool, error) {
	return false, nil
}
func (m *mockUserService) GetPermissionsByRoleName(_ context.Context, _ string) ([]user.Permission, error) {
	return nil, nil
}
func (m *mockUserService) GetUserByProviderID(_ context.Context, _, _ string) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) AuthenticateOAuth(_ context.Context, _, _ string, _ map[string]interface{}) (*user.User, error) {
	return nil, nil
}
func (m *mockUserService) LinkOAuthAccount(_ context.Context, _, _, _ string, _ map[string]interface{}) error {
	return nil
}
func (m *mockUserService) UnlinkOAuthAccount(_ context.Context, _, _ string) error { return nil }
func (m *mockUserService) CreateAPIKey(_ context.Context, _, _ string, _ *time.Duration) (*user.APIKey, error) {
	return nil, nil
}
func (m *mockUserService) DeleteAPIKey(_ context.Context, _, _ string) error { return nil }
func (m *mockUserService) ListAPIKeys(_ context.Context, _ string) ([]*user.APIKey, error) {
	return nil, nil
}
func (m *mockUserService) UpdatePreferences(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}
func (m *mockUserService) UpdatePassword(_ context.Context, _, _ string) (*user.User, error) {
	return nil, nil
}
