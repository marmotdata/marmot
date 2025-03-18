package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Key        string     `json:"key,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (s *service) CreateAPIKey(ctx context.Context, userID string, name string, expiresIn *time.Duration) (*APIKey, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("generating API key: %w", err)
	}

	key := base64.URLEncoding.EncodeToString(keyBytes)
	keyHash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing API key: %w", err)
	}

	var expiresAt *time.Time
	if expiresIn != nil {
		t := time.Now().Add(*expiresIn)
		expiresAt = &t
	}

	apiKey := &APIKey{
		UserID:    userID,
		Name:      name,
		Key:       key,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateAPIKey(ctx, apiKey, string(keyHash)); err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (s *service) DeleteAPIKey(ctx context.Context, userID string, keyID string) error {
	// Verify the key belongs to the user before deleting
	key, err := s.repo.GetAPIKey(ctx, keyID)
	if err != nil {
		return err
	}

	if key.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.DeleteAPIKey(ctx, keyID)
}

func (s *service) ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	return s.repo.ListAPIKeys(ctx, userID)
}

func (s *service) ValidateAPIKey(ctx context.Context, apiKey string) (*User, error) {
	// Get a valid API key
	apiKeyObj, err := s.repo.GetAPIKeyByHash(ctx, apiKey)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidAPIKey
		}
		return nil, fmt.Errorf("getting API key: %w", err)
	}

	// Update last used timestamp
	if err := s.repo.UpdateAPIKeyLastUsed(ctx, apiKeyObj.ID); err != nil {
		return nil, fmt.Errorf("updating API key last used timestamp: %w", err)
	}

	// Fetch the user associated with the valid API key
	return s.Get(ctx, apiKeyObj.UserID)
}
