package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/users"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// APIKey is a personal API key. The Token value is only populated on the Create response.
type APIKey = models.UserAPIKey

// CreateAPIKeyInput is the input for APIKeysService.Create.
type CreateAPIKeyInput struct {
	Name          string
	ExpiresInDays int64
}

// APIKeysService manages personal API keys for the authenticated user.
type APIKeysService struct {
	gen *apiclient.Marmot
}

// List returns the caller's API keys.
func (s *APIKeysService) List(ctx context.Context) ([]*APIKey, error) {
	p := users.NewGetUsersApikeysParams().WithContext(ctx)
	resp, err := s.gen.Users.GetUsersApikeys(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create issues a new API key. The full token is only readable from this response.
func (s *APIKeysService) Create(ctx context.Context, in CreateAPIKeyInput) (*APIKey, error) {
	body := &models.V1UsersCreateAPIKeyRequest{
		Name:          &in.Name,
		ExpiresInDays: in.ExpiresInDays,
	}
	p := users.NewPostUsersApikeysParams().WithContext(ctx).WithKey(body)
	resp, err := s.gen.Users.PostUsersApikeys(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete revokes an API key.
func (s *APIKeysService) Delete(ctx context.Context, id string) error {
	p := users.NewDeleteUsersApikeysIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Users.DeleteUsersApikeysID(p)
	return mapErr(err)
}
