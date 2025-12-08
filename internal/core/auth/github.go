package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	config       *config.Config
	userService  user.Service
	oauthConfig  *oauth2.Config
}

func NewGitHubProvider(cfg *config.Config, userService user.Service) *GitHubProvider {
	providerCfg := cfg.Auth.GitHub
	if providerCfg == nil {
		log.Fatal().Msg("github provider config not found")
		return nil
	}

	p := &GitHubProvider{
		clientID:     providerCfg.ClientID,
		clientSecret: providerCfg.ClientSecret,
		redirectURL:  cfg.Server.RootURL + "/auth/github/callback",
		config:       cfg,
		userService:  userService,
	}

	p.oauthConfig = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint:     github.Endpoint,
		Scopes:       providerCfg.Scopes,
	}

	return p
}

func (p *GitHubProvider) GetAuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *GitHubProvider) HandleCallback(ctx context.Context, code string) (*user.User, error) {
	log.Debug().Str("code_length", fmt.Sprintf("%d", len(code))).Msg("exchanging GitHub code for token")

	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	userInfo, err := p.getUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	providerUserIDFloat, ok := userInfo["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("provider user ID not provided by GitHub")
	}
	providerUserID := fmt.Sprintf("%d", int64(providerUserIDFloat))

	usr, err := p.userService.GetUserByProviderID(ctx, "github", providerUserID)
	if err == nil {
		profilePicture, _ := userInfo["avatar_url"].(string)
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

	if err != user.ErrUserNotFound {
		return nil, fmt.Errorf("failed to get user by provider ID: %w", err)
	}

	email, ok := userInfo["email"].(string)
	if !ok || email == "" {
		emails, err := p.getUserEmails(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to get user emails: %w", err)
		}

		if len(emails) == 0 {
			return nil, fmt.Errorf("no email found for GitHub user")
		}

		for _, e := range emails {
			if verified, ok := e["verified"].(bool); ok && verified {
				if primary, ok := e["primary"].(bool); ok && primary {
					if emailStr, ok := e["email"].(string); ok {
						email = emailStr
						break
					}
				}
			}
		}

		if email == "" {
			for _, e := range emails {
				if verified, ok := e["verified"].(bool); ok && verified {
					if emailStr, ok := e["email"].(string); ok {
						email = emailStr
						break
					}
				}
			}
		}

		if email == "" {
			return nil, fmt.Errorf("no verified email found for GitHub user")
		}

		userInfo["email"] = email
	}

	name, ok := userInfo["name"].(string)
	if !ok || name == "" {
		name, ok = userInfo["login"].(string)
		if !ok || name == "" {
			name = email
		}
	}

	profilePicture, _ := userInfo["avatar_url"].(string)

	newUser := user.CreateUserInput{
		Username:          email,
		Name:              name,
		ProfilePicture:    profilePicture,
		OAuthProvider:     "github",
		OAuthProviderData: userInfo,
		OAuthProviderID:   providerUserID,
		RoleNames:         []string{"user"},
	}

	usr, err = p.userService.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return usr, nil
}

func (p *GitHubProvider) getUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

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

	return userInfo, nil
}

func (p *GitHubProvider) getUserEmails(ctx context.Context, token *oauth2.Token) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var emails []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return nil, fmt.Errorf("failed to decode user emails: %w", err)
	}

	return emails, nil
}

func (p *GitHubProvider) Name() string {
	return "GitHub"
}

func (p *GitHubProvider) Type() string {
	return "github"
}
