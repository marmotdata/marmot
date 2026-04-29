package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	jwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type testJWKS struct {
	key    *rsa.PrivateKey
	keyID  string
	server *httptest.Server
}

func newTestJWKS(t *testing.T) *testJWKS {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tj := &testJWKS{key: key, keyID: "test-key-1"}

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &key.PublicKey,
				KeyID:     tj.keyID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}

	tj.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			disc := map[string]interface{}{
				"issuer":                 tj.server.URL,
				"jwks_uri":              tj.server.URL + "/jwks",
				"authorization_endpoint": tj.server.URL + "/auth",
				"token_endpoint":         tj.server.URL + "/token",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(disc)
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
		}
	}))

	return tj
}

func (tj *testJWKS) signToken(t *testing.T, claims map[string]interface{}, audience string) string {
	t.Helper()

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: tj.key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), tj.keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	now := time.Now()
	registered := jwt.Claims{
		Issuer:    tj.server.URL,
		Subject:   claims["sub"].(string),
		Audience:  jwt.Audience{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(time.Hour)),
		NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
	}

	raw, err := jwt.Signed(sig).Claims(registered).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func (tj *testJWKS) verifier(t *testing.T) *oidc.IDTokenVerifier {
	t.Helper()
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, tj.server.URL)
	if err != nil {
		t.Fatalf("create oidc provider: %v", err)
	}
	return provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
}

type mockUserService struct {
	getUserByProviderIDFn func(ctx context.Context, provider, providerUserID string) (*user.User, error)
	createFn              func(ctx context.Context, input user.CreateUserInput) (*user.User, error)
	updateFn              func(ctx context.Context, id string, input user.UpdateUserInput) (*user.User, error)
}

func (m *mockUserService) GetUserByProviderID(ctx context.Context, provider, providerUserID string) (*user.User, error) {
	return m.getUserByProviderIDFn(ctx, provider, providerUserID)
}
func (m *mockUserService) Create(ctx context.Context, input user.CreateUserInput) (*user.User, error) {
	return m.createFn(ctx, input)
}
func (m *mockUserService) Update(ctx context.Context, id string, input user.UpdateUserInput) (*user.User, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return nil, nil
}

func (m *mockUserService) Delete(_ context.Context, _, _ string) error                           { return nil }
func (m *mockUserService) Get(_ context.Context, _ string) (*user.User, error)                   { return nil, nil }
func (m *mockUserService) GetUserByUsername(_ context.Context, _ string) (*user.User, error)      { return nil, nil }
func (m *mockUserService) FindSimilarUsernames(_ context.Context, _ string, _ int) ([]string, error) { return nil, nil }
func (m *mockUserService) List(_ context.Context, _ user.Filter) ([]*user.User, int, error)      { return nil, 0, nil }
func (m *mockUserService) Authenticate(_ context.Context, _, _ string) (*user.User, error)       { return nil, nil }
func (m *mockUserService) ValidateAPIKey(_ context.Context, _ string) (*user.User, error)        { return nil, nil }
func (m *mockUserService) HasPermission(_ context.Context, _, _ string, _ string) (bool, error)  { return false, nil }
func (m *mockUserService) GetPermissionsByRoleName(_ context.Context, _ string) ([]user.Permission, error) { return nil, nil }
func (m *mockUserService) AuthenticateOAuth(_ context.Context, _, _ string, _ map[string]interface{}) (*user.User, error) { return nil, nil }
func (m *mockUserService) LinkOAuthAccount(_ context.Context, _, _, _ string, _ map[string]interface{}) error { return nil }
func (m *mockUserService) UnlinkOAuthAccount(_ context.Context, _, _ string) error               { return nil }
func (m *mockUserService) CreateAPIKey(_ context.Context, _, _ string, _ *time.Duration) (*user.APIKey, error) { return nil, nil }
func (m *mockUserService) DeleteAPIKey(_ context.Context, _, _ string) error                     { return nil }
func (m *mockUserService) ListAPIKeys(_ context.Context, _ string) ([]*user.APIKey, error)       { return nil, nil }
func (m *mockUserService) UpdatePreferences(_ context.Context, _ string, _ map[string]interface{}) error { return nil }
func (m *mockUserService) UpdatePassword(_ context.Context, _, _ string) (*user.User, error)     { return nil, nil }

func TestExchangeIDToken_ExistingUser(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	clientID := "test-client"
	existingUser := &user.User{ID: "existing-123", Name: "Existing User", ProfilePicture: "old.png"}

	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, provider, providerUserID string) (*user.User, error) {
			if provider != "okta" || providerUserID != "sub-1" {
				t.Fatalf("unexpected provider=%q id=%q", provider, providerUserID)
			}
			return existingUser, nil
		},
	}

	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":     "sub-1",
		"email":   "user@example.com",
		"name":    "Existing User",
		"picture": "old.png",
	}, clientID)

	verifier := tjwks.verifier(t)

	usr, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{clientID},
		userService:      userSvc,
	}, rawToken)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usr.ID != "existing-123" {
		t.Fatalf("expected user ID 'existing-123', got %q", usr.ID)
	}
}

func TestExchangeIDToken_NewUser(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	clientID := "test-client"
	createdUser := &user.User{ID: "new-456", Name: "New User"}

	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return nil, user.ErrUserNotFound
		},
		createFn: func(_ context.Context, input user.CreateUserInput) (*user.User, error) {
			if input.Username != "new@example.com" {
				t.Fatalf("expected username 'new@example.com', got %q", input.Username)
			}
			if input.OAuthProvider != "okta" {
				t.Fatalf("expected provider 'okta', got %q", input.OAuthProvider)
			}
			if input.OAuthProviderID != "sub-2" {
				t.Fatalf("expected provider ID 'sub-2', got %q", input.OAuthProviderID)
			}
			return createdUser, nil
		},
	}

	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":   "sub-2",
		"email": "new@example.com",
		"name":  "New User",
	}, clientID)

	verifier := tjwks.verifier(t)

	usr, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{clientID},
		userService:      userSvc,
	}, rawToken)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usr.ID != "new-456" {
		t.Fatalf("expected user ID 'new-456', got %q", usr.ID)
	}
}

func TestExchangeIDToken_InvalidToken(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	verifier := tjwks.verifier(t)

	userSvc := &mockUserService{}

	_, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{"test-client"},
		userService:      userSvc,
	}, "invalid.token.here")

	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestExchangeIDToken_AudienceMismatch(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	existingUser := &user.User{ID: "aud-user", Name: "Aud User"}
	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return existingUser, nil
		},
	}

	// Token issued for "other-client", but we only allow "my-client"
	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":   "sub-aud",
		"email": "aud@example.com",
	}, "other-client")

	verifier := tjwks.verifier(t)

	_, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{"my-client"},
		userService:      userSvc,
	}, rawToken)

	if err == nil {
		t.Fatal("expected error for audience mismatch")
	}
}

func TestExchangeIDToken_MultipleAllowedAudiences(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	existingUser := &user.User{ID: "multi-aud", Name: "Multi Aud User"}
	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return existingUser, nil
		},
	}

	// Token issued for "ci-client", allowed list includes it
	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":   "sub-multi",
		"email": "multi@example.com",
	}, "ci-client")

	verifier := tjwks.verifier(t)

	usr, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{"marmot-client", "ci-client", "agent-client"},
		userService:      userSvc,
	}, rawToken)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usr.ID != "multi-aud" {
		t.Fatalf("expected user ID 'multi-aud', got %q", usr.ID)
	}
}

func TestExchangeIDToken_TeamSync(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	clientID := "test-client"
	existingUser := &user.User{ID: "team-user-1", Name: "Team User"}

	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return existingUser, nil
		},
	}

	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":    "sub-3",
		"email":  "team@example.com",
		"groups": []string{"engineering", "platform"},
	}, clientID)

	verifier := tjwks.verifier(t)

	// Without a team service, groups are just ignored (no panic)
	usr, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{clientID},
		userService:      userSvc,
		teamSync: config.TeamSyncConfig{
			Enabled: true,
		},
	}, rawToken)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usr.ID != "team-user-1" {
		t.Fatalf("expected user ID 'team-user-1', got %q", usr.ID)
	}
}

func TestExchangeIDToken_ProfilePictureUpdate(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	clientID := "test-client"
	existingUser := &user.User{ID: "pic-user", Name: "Pic User", ProfilePicture: "old.png"}

	var updatedPic string
	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return existingUser, nil
		},
		updateFn: func(_ context.Context, id string, input user.UpdateUserInput) (*user.User, error) {
			if input.ProfilePicture != nil {
				updatedPic = *input.ProfilePicture
			}
			return existingUser, nil
		},
	}

	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":     "sub-pic",
		"email":   "pic@example.com",
		"picture": "new.png",
	}, clientID)

	verifier := tjwks.verifier(t)

	_, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{clientID},
		userService:      userSvc,
	}, rawToken)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updatedPic != "new.png" {
		t.Fatalf("expected profile picture update to 'new.png', got %q", updatedPic)
	}
}

func TestExchangeIDToken_LookupError(t *testing.T) {
	tjwks := newTestJWKS(t)
	defer tjwks.server.Close()

	clientID := "test-client"
	userSvc := &mockUserService{
		getUserByProviderIDFn: func(_ context.Context, _, _ string) (*user.User, error) {
			return nil, errors.New("database connection failed")
		},
	}

	rawToken := tjwks.signToken(t, map[string]interface{}{
		"sub":   "sub-err",
		"email": "err@example.com",
	}, clientID)

	verifier := tjwks.verifier(t)

	_, err := exchangeIDToken(context.Background(), oidcExchangeParams{
		providerType:     "okta",
		providerName:     "Okta",
		verifier:         verifier,
		allowedAudiences: []string{clientID},
		userService:      userSvc,
	}, rawToken)

	if err == nil {
		t.Fatal("expected error for database failure")
	}
}
