package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound      = errors.New("subscription not found")
	ErrUnauthorized  = errors.New("unauthorized to modify subscription")
	ErrAlreadyExists = errors.New("subscription already exists for this asset")
)

// ValidationError represents a user-facing validation failure.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// IsValidationError reports whether err is a user-facing validation error.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

var ValidNotificationTypes = map[string]bool{
	"asset_change":             true,
	"schema_change":            true,
	"upstream_schema_change":   true,
	"downstream_schema_change": true,
	"lineage_change":           true,
}

// Subscription represents a user's subscription to notifications for a specific asset.
type Subscription struct {
	ID                string    `json:"id"`
	AssetID           string    `json:"asset_id"`
	UserID            string    `json:"user_id"`
	NotificationTypes []string  `json:"notification_types"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SubscriptionWithAsset extends Subscription with denormalized asset metadata.
type SubscriptionWithAsset struct {
	Subscription
	AssetName string `json:"asset_name"`
	AssetMRN  string `json:"asset_mrn"`
	AssetType string `json:"asset_type"`
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, assetID, userID string, types []string) (*Subscription, error) {
	if err := validateNotificationTypes(types); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByAssetAndUser(ctx, assetID, userID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyExists
	}

	sub := &Subscription{
		AssetID:           assetID,
		UserID:            userID,
		NotificationTypes: types,
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Service) Update(ctx context.Context, id, userID string, types []string) (*Subscription, error) {
	if err := validateNotificationTypes(types); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, ErrUnauthorized
	}

	return s.repo.Update(ctx, id, types)
}

func (s *Service) Delete(ctx context.Context, id, userID string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByAssetAndUser(ctx context.Context, assetID, userID string) (*Subscription, error) {
	return s.repo.GetByAssetAndUser(ctx, assetID, userID)
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]*SubscriptionWithAsset, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) GetSubscribersForAsset(ctx context.Context, assetID string, notificationType string) ([]string, error) {
	return s.repo.GetSubscribersForAsset(ctx, assetID, notificationType)
}

func validateNotificationTypes(types []string) error {
	if len(types) == 0 {
		return &ValidationError{Message: "at least one notification type is required"}
	}
	for _, t := range types {
		if !ValidNotificationTypes[t] {
			return &ValidationError{Message: fmt.Sprintf("invalid notification type: %q", t)}
		}
	}
	return nil
}
