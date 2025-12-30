package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type Auth0Provider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	config       *config.Config
	userService  user.Service
	authService  Service
	teamService  *team.Service
	verifier     *oidc.IDTokenVerifier
	userInfoURL  string
	oauthConfig  *oauth2.Config
	oidcProvider *oidc.Provider
}

func NewAuth0Provider(cfg *config.Config, userService user.Service, authService Service, teamService *team.Service) *Auth0Provider {
	providerCfg := cfg.Auth.Auth0
	if providerCfg == nil {
		log.Fatal().Msg("auth0 provider config not found")
		return nil
	}

	p := &Auth0Provider{
		clientID:     providerCfg.ClientID,
		clientSecret: providerCfg.ClientSecret,
		redirectURL:  cfg.Server.RootURL + "/auth/auth0/callback",
		config:       cfg,
		userService:  userService,
		authService:  authService,
		teamService:  teamService,
	}

	p.oauthConfig = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  providerCfg.URL + "/authorize",
			TokenURL: providerCfg.URL + "/oauth/token",
		},
		Scopes: providerCfg.Scopes,
	}

	var err error
	p.oidcProvider, err = oidc.NewProvider(context.Background(), providerCfg.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create OIDC provider")
		return nil
	}

	p.verifier = p.oidcProvider.Verifier(&oidc.Config{
		ClientID: p.clientID,
	})

	p.userInfoURL = providerCfg.URL + "/userinfo"

	return p
}

func (p *Auth0Provider) GetAuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *Auth0Provider) HandleCallback(ctx context.Context, code string) (*user.User, error) {
	log.Debug().Str("code_length", fmt.Sprintf("%d", len(code))).Msg("exchanging Auth0 code for token")

	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	log.Debug().Str("token_type", token.TokenType).Msg("token exchange successful")

	log.Debug().Msg("fetching user info from Auth0")
	userInfo, err := p.getUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	providerUserID, ok := userInfo["sub"].(string)
	if !ok || providerUserID == "" {
		return nil, fmt.Errorf("provider user ID not provided by Auth0")
	}

	usr, err := p.userService.GetUserByProviderID(ctx, "auth0", providerUserID)
	switch {
	case err == nil:
		log.Debug().Str("user_id", usr.ID).Msg("found existing user")
		profilePicture, _ := userInfo["picture"].(string)
		if profilePicture != "" && usr.ProfilePicture != profilePicture {
			input := user.UpdateUserInput{
				ProfilePicture: &profilePicture,
			}
			if _, err := p.userService.Update(ctx, usr.ID, input); err != nil {
				log.Warn().Err(err).Str("user_id", usr.ID).Msg("failed to update profile picture")
			}
		}
	case err == user.ErrUserNotFound:
		email, ok := userInfo["email"].(string)
		if !ok || email == "" {
			return nil, fmt.Errorf("email not provided by Auth0")
		}
		log.Debug().Str("email", email).Msg("got user email from Auth0")

		name, ok := userInfo["name"].(string)
		if !ok || name == "" {
			name = email
			log.Debug().Str("email", email).Msg("name not provided, using email as name")
		} else {
			log.Debug().Str("name", name).Str("email", email).Msg("got user name from Auth0")
		}

		profilePicture, _ := userInfo["picture"].(string)

		log.Debug().Str("name", name).Str("email", email).Msg("creating new user")
		newUser := user.CreateUserInput{
			Username:          email,
			Name:              name,
			ProfilePicture:    profilePicture,
			OAuthProvider:     "auth0",
			OAuthProviderData: userInfo,
			OAuthProviderID:   providerUserID,
			RoleNames:         []string{"user"},
		}

		usr, err = p.userService.Create(ctx, newUser)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		log.Debug().Str("user_id", usr.ID).Str("name", name).Str("email", email).Msg("created new user")
	default:
		return nil, fmt.Errorf("failed to get user by provider ID: %w", err)
	}

	if p.teamService != nil {
		providerCfg := p.config.Auth.Auth0
		if providerCfg != nil {
			groupClaim := "groups"
			if providerCfg.TeamSync.Group.Claim != "" {
				groupClaim = providerCfg.TeamSync.Group.Claim
			}

			groups := p.extractGroups(userInfo, groupClaim)
			if len(groups) > 0 {
				log.Debug().Strs("groups", groups).Str("user_id", usr.ID).Msg("syncing team memberships from SSO")
				if err := p.teamService.SyncUserTeamsFromSSO(ctx, usr.ID, "auth0", groups, providerCfg.TeamSync); err != nil {
					log.Error().Err(err).Str("user_id", usr.ID).Msg("failed to sync teams from SSO")
				}
			}
		}
	}

	return usr, nil
}

func (p *Auth0Provider) getUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

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

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	for key, value := range claims {
		userInfo[key] = value
	}

	return userInfo, nil
}

func (p *Auth0Provider) Name() string {
	return "Auth0"
}

func (p *Auth0Provider) Type() string {
	return "auth0"
}

func (p *Auth0Provider) extractGroups(userInfo map[string]interface{}, groupClaim string) []string {
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
