package user

import (
	"context"
	"fmt"
	"time"
)

type UserIdentity struct {
	ID             string                 `json:"id"`
	UserID         string                 `json:"user_id"`
	Provider       string                 `json:"provider"`
	ProviderUserID string                 `json:"provider_user_id"`
	ProviderEmail  string                 `json:"provider_email"`
	ProviderData   map[string]interface{} `json:"provider_data"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

func (s *service) AuthenticateOAuth(ctx context.Context, provider string, providerUserID string, userInfo map[string]interface{}) (*User, error) {
	user, err := s.repo.GetUserByProviderID(ctx, provider, providerUserID)
	if err != nil && err != ErrUserNotFound {
		return nil, fmt.Errorf("checking existing user: %w", err)
	}

	if user != nil {
		if !user.Active {
			return nil, ErrUnauthorized
		}
		return user, nil
	}

	email, _ := userInfo["email"].(string)
	name, _ := userInfo["name"].(string)

	input := CreateUserInput{
		Username:          email,
		Name:              name,
		OAuthProvider:     provider,
		OAuthProviderID:   providerUserID,
		OAuthProviderData: userInfo,
		RoleNames:         []string{"user"}, // Default role
	}

	return s.Create(ctx, input)
}

func (s *service) LinkOAuthAccount(ctx context.Context, userID string, provider string, providerUserID string, userInfo map[string]interface{}) error {
	identity := &UserIdentity{
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		ProviderEmail:  userInfo["email"].(string),
		ProviderData:   userInfo,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return s.repo.CreateUserIdentity(ctx, identity)
}

func (s *service) UnlinkOAuthAccount(ctx context.Context, userID string, provider string) error {
	return s.repo.DeleteUserIdentity(ctx, userID, provider)
}
