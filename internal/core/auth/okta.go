package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// OktaProvider represents the OAuth provider for Okta.
type OktaProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	config       *config.Config
	userService  user.Service
	authService  Service
	verifier     *oidc.IDTokenVerifier
	userInfoURL  string
	oauthConfig  *oauth2.Config
	oidcProvider *oidc.Provider
}

// NewOktaProvider creates a new OktaProvider.
func NewOktaProvider(cfg *config.Config, userService user.Service, authService Service) *OktaProvider {
	providerCfg, ok := cfg.Auth.Providers["okta"]
	if !ok || providerCfg == nil {
		log.Fatal().Msg("okta provider config not found")
		return nil // Unreachable but makes linter happy
	}

	// We can now safely use providerCfg
	p := &OktaProvider{
		clientID:     providerCfg.ClientID,
		clientSecret: providerCfg.ClientSecret,
		redirectURL:  cfg.Server.RootURL + "/api/v1/auth/okta/callback",
		config:       cfg,
		userService:  userService,
		authService:  authService,
	}

	p.oauthConfig = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  providerCfg.URL + "/oauth2/v1/authorize",
			TokenURL: providerCfg.URL + "/oauth2/v1/token",
		},
		Scopes: providerCfg.Scopes,
	}

	var err error
	p.oidcProvider, err = oidc.NewProvider(context.Background(), providerCfg.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create OIDC provider")
		return nil // Unreachable but makes linter happy
	}

	p.verifier = p.oidcProvider.Verifier(&oidc.Config{
		ClientID: p.clientID,
	})

	p.userInfoURL = providerCfg.URL + "/oauth2/v1/userinfo"

	return p
}

func (p *OktaProvider) GetAuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *OktaProvider) HandleCallback(ctx context.Context, code string) (*user.User, error) {
	log.Debug().Str("code_length", fmt.Sprintf("%d", len(code))).Msg("exchanging Okta code for token")

	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	log.Debug().Str("token_type", token.TokenType).Msg("token exchange successful")

	log.Debug().Msg("fetching user info from Okta")
	userInfo, err := p.getUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	email, ok := userInfo["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email not provided by Okta")
	}
	log.Debug().Str("email", email).Msg("got user email from Okta")

	// Try to find existing user by email
	log.Debug().Str("email", email).Msg("looking up existing user by email")
	usr, err := p.userService.GetUserByUsername(ctx, email)
	if err != nil && err != user.ErrUserNotFound {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if err == user.ErrUserNotFound {
		// Get user details from Okta
		name, ok := userInfo["name"].(string)
		if !ok || name == "" {
			name = email // Fallback to email if name not provided
			log.Debug().Str("email", email).Msg("name not provided, using email as name")
		} else {
			log.Debug().Str("name", name).Str("email", email).Msg("got user name from Okta")
		}

		providerUserID, ok := userInfo["sub"].(string)
		if !ok || providerUserID == "" {
			return nil, fmt.Errorf("provider user ID not provided by Okta")
		}

		// Create new user
		log.Debug().Str("name", name).Str("email", email).Msg("creating new user")
		newUser := user.CreateUserInput{
			Username:          email,
			Name:              name,
			OAuthProvider:     "okta",
			OAuthProviderData: userInfo,
			OAuthProviderID:   providerUserID,
			RoleNames:         []string{"user"},
		}

		usr, err = p.userService.Create(ctx, newUser)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		log.Debug().Str("user_id", usr.ID).Str("name", name).Str("email", email).Msg("created new user")
	} else {
		log.Debug().Str("user_id", usr.ID).Str("email", email).Msg("found existing user")
	}

	return usr, nil
}

// getUserInfo fetches the user's information from Okta.
func (p *OktaProvider) getUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	// Verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract custom claims into a map
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Merge claims from userinfo endpoint
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Merge the claims from the ID token into the userInfo map
	for key, value := range claims {
		userInfo[key] = value
	}

	return userInfo, nil
}

// Name returns the name of the provider.
func (p *OktaProvider) Name() string {
	return "Okta"
}

// Type returns the type of the provider.
func (p *OktaProvider) Type() string {
	return "okta"
}
