package serviceaccount

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/limits"
	"golang.org/x/crypto/bcrypt"
)

const DefaultMaxAPIKeysPerAccount = 5

var ErrAPIKeyLimitReached = errors.New("api key limit reached for this service account")

type Service interface {
	Create(ctx context.Context, input CreateInput, createdBy *string) (*ServiceAccount, error)
	Get(ctx context.Context, id string) (*ServiceAccount, error)
	List(ctx context.Context) ([]*ServiceAccount, error)
	Update(ctx context.Context, id string, input UpdateInput) (*ServiceAccount, error)
	Delete(ctx context.Context, id string) error

	CreateAPIKey(ctx context.Context, saID string, name string, expiresIn *time.Duration) (*APIKey, error)
	ListAPIKeys(ctx context.Context, saID string) ([]*APIKey, error)
	DeleteAPIKey(ctx context.Context, saID string, keyID string) error
	RotateAPIKey(ctx context.Context, saID string, keyID string) (*APIKey, error)

	ValidateAPIKey(ctx context.Context, apiKey string) (*ServiceAccount, error)
}

type service struct {
	repo       Repository
	maxAPIKeys int
	guard      limits.Guard
}

// ServiceOption configures a serviceaccount service.
type ServiceOption func(*service)

// WithMaxAPIKeys overrides the per-account API key cap. Non-positive values
// fall back to DefaultMaxAPIKeysPerAccount.
func WithMaxAPIKeys(n int) ServiceOption {
	return func(s *service) {
		if n > 0 {
			s.maxAPIKeys = n
		}
	}
}

// WithGuard installs a resource-count Guard consulted before each Create.
// Passing NoopGuard (the default) disables enforcement.
func WithGuard(g limits.Guard) ServiceOption {
	return func(s *service) {
		if g != nil {
			s.guard = g
		}
	}
}

func NewService(repo Repository, opts ...ServiceOption) Service {
	s := &service{
		repo:       repo,
		maxAPIKeys: DefaultMaxAPIKeysPerAccount,
		guard:      limits.NoopGuard{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) Create(ctx context.Context, input CreateInput, createdBy *string) (*ServiceAccount, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := s.guard.CheckCreate(ctx, limits.ResourceServiceAccounts); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, input, createdBy)
}

func (s *service) Get(ctx context.Context, id string) (*ServiceAccount, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) List(ctx context.Context) ([]*ServiceAccount, error) {
	return s.repo.List(ctx)
}

func (s *service) Update(ctx context.Context, id string, input UpdateInput) (*ServiceAccount, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}

const APIKeyPrefix = "mrmt_s_"

// visibleKeyChars is how many random characters after the scheme prefix stay readable: enough to tell keys apart, a negligible part of the 256 random bits.
const visibleKeyChars = 4

func displayPrefix(key string) string {
	n := len(APIKeyPrefix) + visibleKeyChars
	if len(key) < n {
		return ""
	}
	return key[:n]
}

// newAPIKeySecret mints a key and its bcrypt hash, shared by create and rotate so both issue keys the same way.
func newAPIKeySecret() (string, string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("generating api key: %w", err)
	}

	key := APIKeyPrefix + base64.URLEncoding.EncodeToString(keyBytes)
	keyHash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hashing api key: %w", err)
	}
	return key, string(keyHash), nil
}

func (s *service) CreateAPIKey(ctx context.Context, saID string, name string, expiresIn *time.Duration) (*APIKey, error) {
	count, err := s.repo.CountAPIKeys(ctx, saID)
	if err != nil {
		return nil, fmt.Errorf("counting api keys: %w", err)
	}
	if count >= s.maxAPIKeys {
		return nil, fmt.Errorf("%w: limit is %d", ErrAPIKeyLimitReached, s.maxAPIKeys)
	}

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
		ServiceAccountID: saID,
		Name:             name,
		Key:              key,
		Prefix:           displayPrefix(key),
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now(),
	}

	if err := s.repo.CreateAPIKey(ctx, saID, apiKey, keyHash); err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (s *service) ListAPIKeys(ctx context.Context, saID string) ([]*APIKey, error) {
	return s.repo.ListAPIKeys(ctx, saID)
}

func (s *service) DeleteAPIKey(ctx context.Context, saID string, keyID string) error {
	key, err := s.repo.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return ErrKeyNotFound
		}
		return err
	}

	if key.ServiceAccountID != saID {
		return ErrKeyNotFound
	}

	return s.repo.DeleteAPIKey(ctx, keyID)
}

// RotateAPIKey replaces a key's secret in place, keeping its name and expiry, so the old secret stops at once and the key count limit is not consumed by a replacement.
func (s *service) RotateAPIKey(ctx context.Context, saID string, keyID string) (*APIKey, error) {
	existing, err := s.repo.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if existing.ServiceAccountID != saID {
		return nil, ErrKeyNotFound
	}

	key, keyHash, err := newAPIKeySecret()
	if err != nil {
		return nil, err
	}
	prefix := displayPrefix(key)
	if err := s.repo.RotateAPIKey(ctx, keyID, keyHash, APIKeyLookup(key), prefix); err != nil {
		return nil, err
	}

	existing.Key = key
	existing.Prefix = prefix
	existing.LastUsedAt = nil
	return existing, nil
}

func (s *service) ValidateAPIKey(ctx context.Context, apiKey string) (*ServiceAccount, error) {
	keyObj, err := s.repo.GetAPIKeyByHash(ctx, apiKey)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("getting api key: %w", err)
	}

	if err := s.repo.UpdateAPIKeyLastUsed(ctx, keyObj.ID); err != nil {
		return nil, fmt.Errorf("updating last used: %w", err)
	}

	sa, err := s.repo.Get(ctx, keyObj.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	if !sa.Active {
		return nil, fmt.Errorf("service account is inactive")
	}

	return sa, nil
}
