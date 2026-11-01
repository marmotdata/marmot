package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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
	providers   map[string]OAuthProvider
	teamService *team.Service
	switches    ProviderSwitches

	mu        sync.Mutex
	disabled  map[string]bool
	checkedAt time.Time
}

// ProviderSwitches reports which configured providers an administrator has switched off.
type ProviderSwitches interface {
	DisabledProviders(ctx context.Context) (map[string]bool, error)
}

// providerSwitchTTL is how long a switch may take to reach a server that did not make the change, which keeps every sign-in from reading the database.
const providerSwitchTTL = 30 * time.Second

// SetTeamService is passed to providers registered afterwards that create members without group sync, so those members can still join the default team.
func (m *OAuthManager) SetTeamService(t *team.Service) {
	m.teamService = t
}

// SetProviderSwitches lets an administrator switch configured providers off without editing the config file.
func (m *OAuthManager) SetProviderSwitches(s ProviderSwitches) {
	m.switches = s
	m.Refresh()
}

// Refresh drops the cached switches so the next lookup reads them again, for the server that just changed them.
func (m *OAuthManager) Refresh() {
	m.mu.Lock()
	m.checkedAt = time.Time{}
	m.mu.Unlock()
}

func (m *OAuthManager) isDisabled(providerType string) bool {
	if m.switches == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.checkedAt) > providerSwitchTTL {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		disabled, err := m.switches.DisabledProviders(ctx)
		cancel()
		if err != nil {
			// Keep the last answer rather than switching every provider back on because the database blinked.
			log.Warn().Err(err).Msg("Failed to read single sign-on provider switches")
		} else {
			m.disabled = disabled
		}
		m.checkedAt = time.Now()
	}
	return m.disabled[providerType]
}

// Enabled reports whether a configured provider is switched on.
func (m *OAuthManager) Enabled(providerType string) bool {
	return !m.isDisabled(providerType)
}

// AllProviders returns every configured provider, including those switched off, for the settings that list them.
func (m *OAuthManager) AllProviders() map[string]OAuthProvider {
	return m.providers
}

func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers: make(map[string]OAuthProvider),
	}
}

func (m *OAuthManager) RegisterProvider(provider OAuthProvider) {
	if ts, ok := provider.(interface{ SetTeamService(*team.Service) }); ok && m.teamService != nil {
		ts.SetTeamService(m.teamService)
	}
	m.providers[provider.Type()] = provider
}

// GetProvider returns a provider that is configured and switched on.
func (m *OAuthManager) GetProvider(providerType string) (OAuthProvider, bool) {
	provider, exists := m.providers[providerType]
	if !exists || m.isDisabled(providerType) {
		return nil, false
	}
	return provider, true
}

// GetProviderNames lists the providers people can sign in with, leaving out those switched off.
func (m *OAuthManager) GetProviderNames() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		if m.isDisabled(name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// GetProviders returns the providers switched on, so tokens from one switched off are not accepted either.
func (m *OAuthManager) GetProviders() map[string]OAuthProvider {
	out := make(map[string]OAuthProvider, len(m.providers))
	for name, p := range m.providers {
		if m.isDisabled(name) {
			continue
		}
		out[name] = p
	}
	return out
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

		var created bool
		usr, created, err = userSvc.ProvisionSSO(ctx, newUser)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		applyNewMemberDefaults(ctx, userSvc, teamSvc, usr, created)
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
			if err := syncSSOGroups(ctx, userSvc, teamSvc, usr.ID, providerType, groups, teamSync); err != nil {
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

// applyNewMemberDefaults puts a member that single sign-on has just created into the default team. It runs before group sync, and the membership is manual, so a mapped group adds to it rather than replacing it.
func applyNewMemberDefaults(ctx context.Context, userSvc user.Service, teamSvc *team.Service, usr *user.User, created bool) {
	if !created || teamSvc == nil || usr == nil {
		return
	}
	teamID := userSvc.SignInSettings(ctx).DefaultTeamID
	if teamID == "" {
		return
	}
	if err := teamSvc.AddDefaultMember(ctx, teamID, usr.ID); err != nil {
		log.Warn().Err(err).Str("user_id", usr.ID).Str("team_id", teamID).Msg("Failed to add new member to the default team")
	}
}

// syncSSOGroups applies a member's group mappings at sign-in: the teams their groups feed and, when any group is mapped to a role, those roles in place of whatever they had.
func syncSSOGroups(ctx context.Context, userSvc user.Service, teamSvc *team.Service, userID, provider string, groups []string, teamSync config.TeamSyncConfig) error {
	if err := teamSvc.SyncUserTeamsFromSSO(ctx, userID, provider, groups, teamSync); err != nil {
		return err
	}
	roles, err := teamSvc.RoleNamesForGroups(ctx, provider, groups)
	if err != nil {
		return fmt.Errorf("reading role mappings: %w", err)
	}
	if err := userSvc.SetRolesFromSSO(ctx, userID, roles); err != nil {
		return fmt.Errorf("applying mapped roles: %w", err)
	}
	return nil
}
