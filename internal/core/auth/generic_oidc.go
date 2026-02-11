package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type GenericOIDCProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	name         string
	config       *config.Config
	userService  user.Service
	authService  Service
	teamService  *team.Service
	verifier     *oidc.IDTokenVerifier
	oauthConfig  *oauth2.Config
	oidcProvider *oidc.Provider
}

func NewGenericOIDCProvider(cfg *config.Config, userService user.Service, authService Service, teamService *team.Service) (*GenericOIDCProvider, error) {
	providerCfg := cfg.Auth.GenericOIDC
	if providerCfg == nil {
		return nil, fmt.Errorf("generic_oidc provider config not found")
	}

	issuerURL := strings.TrimRight(providerCfg.URL, "/")

	name := providerCfg.Name
	if name == "" {
		name = "SSO"
	}

	p := &GenericOIDCProvider{
		clientID:     providerCfg.ClientID,
		clientSecret: providerCfg.ClientSecret,
		redirectURL:  cfg.Server.RootURL + "/auth/generic_oidc/callback",
		name:         name,
		config:       cfg,
		userService:  userService,
		authService:  authService,
		teamService:  teamService,
	}

	var err error
	p.oidcProvider, err = oidc.NewProvider(context.Background(), issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Generic OIDC provider: %w", err)
	}

	p.oauthConfig = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint:     p.oidcProvider.Endpoint(),
		Scopes:       providerCfg.Scopes,
	}

	p.verifier = p.oidcProvider.Verifier(&oidc.Config{
		ClientID: p.clientID,
	})

	return p, nil
}

func (p *GenericOIDCProvider) GetAuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *GenericOIDCProvider) HandleCallback(ctx context.Context, code string) (*user.User, error) {
	log.Debug().Str("code_length", fmt.Sprintf("%d", len(code))).Msg("exchanging Generic OIDC code for token")

	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	log.Debug().Str("token_type", token.TokenType).Msg("token exchange successful")

	log.Debug().Msg("fetching user info from Generic OIDC provider")
	userInfo, err := p.getUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	providerUserID, ok := userInfo["sub"].(string)
	if !ok || providerUserID == "" {
		return nil, fmt.Errorf("provider user ID not provided by Generic OIDC provider")
	}

	usr, err := p.userService.GetUserByProviderID(ctx, "generic_oidc", providerUserID)
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
	case errors.Is(err, user.ErrUserNotFound):
		email, ok := userInfo["email"].(string)
		if !ok || email == "" {
			return nil, fmt.Errorf("email not provided by Generic OIDC provider")
		}
		log.Debug().Str("email", email).Msg("got user email from Generic OIDC provider")

		name, ok := userInfo["name"].(string)
		if !ok || name == "" {
			name = email
			log.Debug().Str("email", email).Msg("name not provided, using email as name")
		} else {
			log.Debug().Str("name", name).Str("email", email).Msg("got user name from Generic OIDC provider")
		}

		profilePicture, _ := userInfo["picture"].(string)

		log.Debug().Str("name", name).Str("email", email).Msg("creating new user")
		newUser := user.CreateUserInput{
			Username:          email,
			Name:              name,
			ProfilePicture:    profilePicture,
			OAuthProvider:     "generic_oidc",
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
		providerCfg := p.config.Auth.GenericOIDC
		if providerCfg != nil {
			groupClaim := "groups"
			if providerCfg.TeamSync.Group.Claim != "" {
				groupClaim = providerCfg.TeamSync.Group.Claim
			}

			groups := extractGroups(userInfo, groupClaim)
			if len(groups) > 0 {
				log.Debug().Strs("groups", groups).Str("user_id", usr.ID).Msg("syncing team memberships from SSO")
				if err := p.teamService.SyncUserTeamsFromSSO(ctx, usr.ID, "generic_oidc", groups, providerCfg.TeamSync); err != nil {
					log.Error().Err(err).Str("user_id", usr.ID).Msg("failed to sync teams from SSO")
				}
			}
		}
	}

	return usr, nil
}

func (p *GenericOIDCProvider) getUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
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

	oidcUserInfo, err := p.oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	var userInfo map[string]interface{}
	if err := oidcUserInfo.Claims(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info claims: %w", err)
	}

	for key, value := range claims {
		userInfo[key] = value
	}

	return userInfo, nil
}

func (p *GenericOIDCProvider) Name() string {
	return p.name
}

func (p *GenericOIDCProvider) Type() string {
	return "generic_oidc"
}
