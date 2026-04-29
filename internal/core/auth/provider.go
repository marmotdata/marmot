package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// extractGroups extracts group names from a userInfo claims map using the specified claim key.
func extractGroups(userInfo map[string]interface{}, groupClaim string) []string {
	groups := []string{}

	groupsRaw, ok := userInfo[groupClaim]
	if !ok {
		return groups
	}

	switch v := groupsRaw.(type) {
	case []interface{}:
		for _, g := range v {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, groupStr)
			}
		}
	case []string:
		groups = v
	case string:
		groups = []string{v}
	}

	return groups
}

type OAuthProvider interface {
	GetAuthURL(state string) string
	HandleCallback(ctx context.Context, code string) (*user.User, error)
	Name() string
	Type() string
}

// TokenExchanger exchanges an upstream ID token for a local Marmot user.
type TokenExchanger interface {
	ExchangeToken(ctx context.Context, rawIDToken string) (*user.User, error)
}

// AccessTokenExchanger validates an OAuth access token via the UserInfo endpoint
// and resolves a local Marmot user. Used by MCP clients that send access tokens.
type AccessTokenExchanger interface {
	ExchangeAccessToken(ctx context.Context, accessToken string) (*user.User, error)
}

// IssuerProvider returns the provider's OIDC issuer URL.
type IssuerProvider interface {
	IssuerURL() string
}

func trimIssuer(s string) string {
	return strings.TrimRight(s, "/")
}

type OAuthManager struct {
	providers map[string]OAuthProvider
}

func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers: make(map[string]OAuthProvider),
	}
}

func (m *OAuthManager) RegisterProvider(provider OAuthProvider) {
	m.providers[provider.Type()] = provider
}

func (m *OAuthManager) GetProvider(providerType string) (OAuthProvider, bool) {
	provider, exists := m.providers[providerType]
	return provider, exists
}

func (m *OAuthManager) GetProviderNames() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

func (m *OAuthManager) GetProviders() map[string]OAuthProvider {
	return m.providers
}

type oidcExchangeParams struct {
	providerType     string
	providerName     string
	verifier         *oidc.IDTokenVerifier
	allowedAudiences []string
	httpClient       *http.Client
	userService      user.Service
	teamService      *team.Service
	teamSync         config.TeamSyncConfig
}

func exchangeIDToken(ctx context.Context, p oidcExchangeParams, rawIDToken string) (*user.User, error) {
	if p.httpClient != nil {
		ctx = oidc.ClientContext(ctx, p.httpClient)
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	if len(p.allowedAudiences) > 0 {
		if !audienceMatches(idToken.Audience, p.allowedAudiences) {
			return nil, fmt.Errorf("token audience %v not in allowed audiences", idToken.Audience)
		}
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	return resolveUserFromClaims(ctx, p.providerType, p.providerName, p.userService, p.teamService, p.teamSync, claims)
}

// userinfoExchangeParams holds the fields needed to validate an access token via UserInfo.
type userinfoExchangeParams struct {
	providerType string
	providerName string
	oidcProvider *oidc.Provider
	httpClient   *http.Client
	userService  user.Service
	teamService  *team.Service
	teamSync     config.TeamSyncConfig
}

// exchangeViaUserinfo validates an access token via the issuer's UserInfo endpoint.
func exchangeViaUserinfo(ctx context.Context, p userinfoExchangeParams, accessToken string) (*user.User, error) {
	if p.httpClient != nil {
		ctx = oidc.ClientContext(ctx, p.httpClient)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	info, err := p.oidcProvider.UserInfo(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}

	var claims map[string]interface{}
	if err := info.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo claims: %w", err)
	}

	if _, ok := claims["sub"]; !ok {
		claims["sub"] = info.Subject
	}

	return resolveUserFromClaims(ctx, p.providerType, p.providerName, p.userService, p.teamService, p.teamSync, claims)
}

// resolveUserFromClaims looks up or creates a Marmot user from OIDC claims.
func resolveUserFromClaims(ctx context.Context, providerType, providerName string, userSvc user.Service, teamSvc *team.Service, teamSync config.TeamSyncConfig, claims map[string]interface{}) (*user.User, error) {
	providerUserID, ok := claims["sub"].(string)
	if !ok || providerUserID == "" {
		return nil, fmt.Errorf("provider user ID (sub) not present in token")
	}

	usr, err := userSvc.GetUserByProviderID(ctx, providerType, providerUserID)
	switch {
	case err == nil:
		log.Debug().Str("user_id", usr.ID).Msg("found existing user via token exchange")
		profilePicture, _ := claims["picture"].(string)
		if profilePicture != "" && usr.ProfilePicture != profilePicture {
			input := user.UpdateUserInput{
				ProfilePicture: &profilePicture,
			}
			if _, err := userSvc.Update(ctx, usr.ID, input); err != nil {
				log.Warn().Err(err).Str("user_id", usr.ID).Msg("failed to update profile picture")
			}
		}
	case errors.Is(err, user.ErrUserNotFound):
		email, ok := claims["email"].(string)
		if !ok || email == "" {
			return nil, fmt.Errorf("email not provided in token from %s", providerName)
		}

		name, _ := claims["name"].(string)
		if name == "" {
			name = email
		}

		profilePicture, _ := claims["picture"].(string)

		newUser := user.CreateUserInput{
			Username:          email,
			Name:              name,
			ProfilePicture:    profilePicture,
			OAuthProvider:     providerType,
			OAuthProviderData: claims,
			OAuthProviderID:   providerUserID,
			RoleNames:         []string{"user"},
		}

		usr, err = userSvc.Create(ctx, newUser)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		log.Debug().Str("user_id", usr.ID).Str("email", email).Msg("created new user via token exchange")
	default:
		return nil, fmt.Errorf("failed to get user by provider ID: %w", err)
	}

	if teamSvc != nil {
		groupClaim := "groups"
		if teamSync.Group.Claim != "" {
			groupClaim = teamSync.Group.Claim
		}

		groups := extractGroups(claims, groupClaim)
		if len(groups) > 0 {
			log.Debug().Strs("groups", groups).Str("user_id", usr.ID).Msg("syncing team memberships from token exchange")
			if err := teamSvc.SyncUserTeamsFromSSO(ctx, usr.ID, providerType, groups, teamSync); err != nil {
				log.Error().Err(err).Str("user_id", usr.ID).Msg("failed to sync teams from SSO")
			}
		}
	}

	return usr, nil
}

// audienceMatches returns true if any of the token's audiences appear in the allowed set.
func audienceMatches(tokenAudiences, allowed []string) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	for _, a := range tokenAudiences {
		if _, ok := set[a]; ok {
			return true
		}
	}
	return false
}

// newExchangeVerifier builds an IDTokenVerifier that skips the client ID check,
// allowing manual audience validation against allowed_audiences.
func newExchangeVerifier(provider *oidc.Provider) *oidc.IDTokenVerifier {
	return provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})
}

// exchangeAudiences returns the audiences to validate during token exchange.
// Falls back to the provider's client_id if allowed_audiences is not configured.
func exchangeAudiences(cfg *config.OAuthProviderConfig) []string {
	if len(cfg.AllowedAudiences) > 0 {
		return cfg.AllowedAudiences
	}
	return []string{cfg.ClientID}
}

