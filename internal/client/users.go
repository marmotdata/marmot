package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/marmotdata/marmot/internal/core/user"
)

// UsersListResponse represents the response from the users list endpoint.
type UsersListResponse struct {
	Users  []user.User `json:"users"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// GetCurrentUser retrieves the currently authenticated user.
func (c *Client) GetCurrentUser(ctx context.Context) (*user.User, error) {
	var u user.User
	if err := c.get(ctx, "/users/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers lists all users with pagination.
func (c *Client) ListUsers(ctx context.Context, limit, offset int) (*UsersListResponse, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp UsersListResponse
	if err := c.get(ctx, "/users", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetUser retrieves a user by ID.
func (c *Client) GetUser(ctx context.Context, id string) (*user.User, error) {
	var u user.User
	if err := c.get(ctx, "/users/"+id, nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListAPIKeys lists the current user's API keys.
func (c *Client) ListAPIKeys(ctx context.Context) ([]user.APIKey, error) {
	var keys []user.APIKey
	if err := c.get(ctx, "/users/apikeys", nil, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// CreateAPIKey creates a new API key.
func (c *Client) CreateAPIKey(ctx context.Context, name string) (*user.APIKey, error) {
	body := map[string]string{"name": name}
	var key user.APIKey
	if err := c.post(ctx, "/users/apikeys", body, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// DeleteAPIKey deletes an API key by ID.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.del(ctx, "/users/apikeys/"+id, nil, nil)
}

// GetCurrentUserRaw retrieves the current user as raw JSON.
func (c *Client) GetCurrentUserRaw(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/users/me", nil)
}

// ListUsersRaw lists users and returns raw JSON.
func (c *Client) ListUsersRaw(ctx context.Context, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return c.getRaw(ctx, "/users", q)
}

// GetUserRaw retrieves a user as raw JSON.
func (c *Client) GetUserRaw(ctx context.Context, id string) ([]byte, error) {
	return c.getRaw(ctx, "/users/"+id, nil)
}

// ListAPIKeysRaw lists API keys and returns raw JSON.
func (c *Client) ListAPIKeysRaw(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/users/apikeys", nil)
}
