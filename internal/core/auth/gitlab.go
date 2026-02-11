package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type GitLabProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	gitlabURL    string
	config       *config.Config
	userService  user.Service
	verifier     *oidc.IDTokenVerifier
	oauthConfig  *oauth2.Config
	oidcProvider *oidc.Provider
}

func NewGitLabProvider(cfg *config.Config, userService user.Service) (*GitLabProvider, error) {
	providerCfg := cfg.Auth.GitLab
	if providerCfg == nil {
		return nil, fmt.Errorf("gitlab provider config not found")
	}

	gitlabURL := providerCfg.URL
	if gitlabURL == "" {
		gitlabURL = "https://gitlab.com"
	}

	ctx := context.Background()
	oidcProvider, err := oidc.NewProvider(ctx, gitlabURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab OIDC provider: %w", err)
	}

	p := &GitLabProvider{
		clientID:     providerCfg.ClientID,
		clientSecret: providerCfg.ClientSecret,
		redirectURL:  cfg.Server.RootURL + "/auth/gitlab/callback",
		gitlabURL:    gitlabURL,
		config:       cfg,
		userService:  userService,
		oidcProvider: oidcProvider,
	}

	p.verifier = oidcProvider.Verifier(&oidc.Config{
		ClientID: p.clientID,
	})

	p.oauthConfig = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       providerCfg.Scopes,
	}

	return p, nil
}

func (p *GitLabProvider) GetAuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *GitLabProvider) HandleCallback(ctx context.Context, code string) (*user.User, error) {
	log.Debug().Str("code_length", fmt.Sprintf("%d", len(code))).Msg("exchanging GitLab code for token")

	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

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

	userInfo, err := p.oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	var userInfoClaims map[string]interface{}
	if err := userInfo.Claims(&userInfoClaims); err != nil {
		return nil, fmt.Errorf("failed to parse user info claims: %w", err)
	}

	for key, value := range claims {
		userInfoClaims[key] = value
	}

	providerUserID, ok := userInfoClaims["sub"].(string)
	if !ok || providerUserID == "" {
		return nil, fmt.Errorf("provider user ID not provided by GitLab")
	}

	usr, err := p.userService.GetUserByProviderID(ctx, "gitlab", providerUserID)
	if err == nil {
		profilePicture, _ := userInfoClaims["picture"].(string)
		if profilePicture != "" && usr.ProfilePicture != profilePicture {
			input := user.UpdateUserInput{
				ProfilePicture: &profilePicture,
			}
			if _, err := p.userService.Update(ctx, usr.ID, input); err != nil {
				log.Warn().Err(err).Str("user_id", usr.ID).Msg("failed to update profile picture")
			}
		}
		return usr, nil
	}

	if !errors.Is(err, user.ErrUserNotFound) {
		return nil, fmt.Errorf("failed to get user by provider ID: %w", err)
	}

	email, ok := userInfoClaims["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email not provided by GitLab")
	}

	name, ok := userInfoClaims["name"].(string)
	if !ok || name == "" {
		name = email
	}

	profilePicture, _ := userInfoClaims["picture"].(string)

	newUser := user.CreateUserInput{
		Username:          email,
		Name:              name,
		ProfilePicture:    profilePicture,
		OAuthProvider:     "gitlab",
		OAuthProviderData: userInfoClaims,
		OAuthProviderID:   providerUserID,
		RoleNames:         []string{"user"},
	}

	usr, err = p.userService.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return usr, nil
}

func (p *GitLabProvider) Name() string {
	return "GitLab"
}

func (p *GitLabProvider) Type() string {
	return "gitlab"
}
