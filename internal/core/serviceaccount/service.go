package serviceaccount

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

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

	ValidateAPIKey(ctx context.Context, apiKey string) (*ServiceAccount, error)
}

type service struct {
	repo        Repository
	maxAPIKeys  int
}

func NewService(repo Repository, maxAPIKeys int) Service {
	if maxAPIKeys <= 0 {
		maxAPIKeys = DefaultMaxAPIKeysPerAccount
	}
	return &service{repo: repo, maxAPIKeys: maxAPIKeys}
}

func (s *service) Create(ctx context.Context, input CreateInput, createdBy *string) (*ServiceAccount, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
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

func (s *service) CreateAPIKey(ctx context.Context, saID string, name string, expiresIn *time.Duration) (*APIKey, error) {
	count, err := s.repo.CountAPIKeys(ctx, saID)
	if err != nil {
		return nil, fmt.Errorf("counting api keys: %w", err)
	}
	if count >= s.maxAPIKeys {
		return nil, fmt.Errorf("%w: limit is %d", ErrAPIKeyLimitReached, s.maxAPIKeys)
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}

	key := base64.URLEncoding.EncodeToString(keyBytes)
	keyHash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing api key: %w", err)
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
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now(),
	}

	if err := s.repo.CreateAPIKey(ctx, saID, apiKey, string(keyHash)); err != nil {
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
