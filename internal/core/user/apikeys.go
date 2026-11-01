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
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Key    string `json:"key,omitempty"`
	// Prefix is the start of the key kept in clear so a key can be recognised in a list after its secret is gone; empty for keys issued before it was stored.
	Prefix     string     `json:"prefix,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
} // @name APIKey

const UserAPIKeyPrefix = "mrmt_u_"

// visibleKeyChars is how many random characters after the scheme prefix stay readable: enough to tell keys apart in a list, a negligible part of the 256 random bits.
const visibleKeyChars = 4

// displayPrefix returns the part of a key that is safe to keep and show after creation.
func displayPrefix(key, scheme string) string {
	n := len(scheme) + visibleKeyChars
	if len(key) < n {
		return ""
	}
	return key[:n]
}

// newAPIKeySecret mints a key and its bcrypt hash, shared by create and rotate so both issue keys the same way.
func newAPIKeySecret() (string, string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("generating API key: %w", err)
	}

	key := UserAPIKeyPrefix + base64.URLEncoding.EncodeToString(keyBytes)
	keyHash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hashing API key: %w", err)
	}
	return key, string(keyHash), nil
}

func (s *service) CreateAPIKey(ctx context.Context, userID string, name string, expiresIn *time.Duration) (*APIKey, error) {
	key, keyHash, err := newAPIKeySecret()
	if err != nil {
		return nil, err
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
		Prefix:    displayPrefix(key, UserAPIKeyPrefix),
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateAPIKey(ctx, apiKey, keyHash); err != nil {
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

// RotateAPIKey replaces a key's secret in place, keeping its name and expiry, so whatever references the key by name keeps working while the old secret stops at once.
func (s *service) RotateAPIKey(ctx context.Context, userID string, keyID string) (*APIKey, error) {
	existing, err := s.repo.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, ErrUnauthorized
	}

	key, keyHash, err := newAPIKeySecret()
	if err != nil {
		return nil, err
	}
	prefix := displayPrefix(key, UserAPIKeyPrefix)
	if err := s.repo.RotateAPIKey(ctx, keyID, keyHash, APIKeyLookup(key), prefix); err != nil {
		return nil, err
	}

	existing.Key = key
	existing.Prefix = prefix
	existing.LastUsedAt = nil
	return existing, nil
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

	// Fetch the user associated with the valid API key
	user, err := s.Get(ctx, apiKeyObj.UserID)
	if err != nil {
		return nil, err
	}

	// A deactivated user keeps their keys, so without this the key would outlive
	// the account: the JWT path refuses an inactive user and this has to agree.
	if !user.Active {
		return nil, ErrUserInactive
	}

	// Only a key that actually authenticated counts as used.
	if err := s.repo.UpdateAPIKeyLastUsed(ctx, apiKeyObj.ID); err != nil {
		return nil, fmt.Errorf("updating API key last used timestamp: %w", err)
	}

	return user, nil
}
