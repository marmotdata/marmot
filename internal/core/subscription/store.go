package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the subscription data access interface.
type Repository interface {
	Create(ctx context.Context, sub *Subscription) error
	Update(ctx context.Context, id string, types []string) (*Subscription, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*Subscription, error)
	GetByAssetAndUser(ctx context.Context, assetID, userID string) (*Subscription, error)
	ListByUser(ctx context.Context, userID string) ([]*SubscriptionWithAsset, error)
	GetSubscribersForAsset(ctx context.Context, assetID string, notificationType string) ([]string, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, sub *Subscription) error {
	typesJSON, err := json.Marshal(sub.NotificationTypes)
	if err != nil {
		return fmt.Errorf("marshaling notification types: %w", err)
	}

	now := time.Now()
	err = r.db.QueryRow(ctx, `
		INSERT INTO asset_subscriptions (asset_id, user_id, notification_types, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		sub.AssetID, sub.UserID, typesJSON, now, now,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		return fmt.Errorf("creating subscription: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, id string, types []string) (*Subscription, error) {
	typesJSON, err := json.Marshal(types)
	if err != nil {
		return nil, fmt.Errorf("marshaling notification types: %w", err)
	}

	var sub Subscription
	var typesRaw []byte
	err = r.db.QueryRow(ctx, `
		UPDATE asset_subscriptions
		SET notification_types = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, asset_id, user_id, notification_types, created_at, updated_at`,
		typesJSON, id,
	).Scan(&sub.ID, &sub.AssetID, &sub.UserID, &typesRaw, &sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("updating subscription: %w", err)
	}

	if err := json.Unmarshal(typesRaw, &sub.NotificationTypes); err != nil {
		return nil, fmt.Errorf("unmarshaling notification types: %w", err)
	}

	return &sub, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM asset_subscriptions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*Subscription, error) {
	var sub Subscription
	var typesRaw []byte

	err := r.db.QueryRow(ctx, `
		SELECT id, asset_id, user_id, notification_types, created_at, updated_at
		FROM asset_subscriptions WHERE id = $1`, id,
	).Scan(&sub.ID, &sub.AssetID, &sub.UserID, &typesRaw, &sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting subscription: %w", err)
	}

	if err := json.Unmarshal(typesRaw, &sub.NotificationTypes); err != nil {
		return nil, fmt.Errorf("unmarshaling notification types: %w", err)
	}

	return &sub, nil
}

func (r *PostgresRepository) GetByAssetAndUser(ctx context.Context, assetID, userID string) (*Subscription, error) {
	var sub Subscription
	var typesRaw []byte

	err := r.db.QueryRow(ctx, `
		SELECT id, asset_id, user_id, notification_types, created_at, updated_at
		FROM asset_subscriptions WHERE asset_id = $1 AND user_id = $2`, assetID, userID,
	).Scan(&sub.ID, &sub.AssetID, &sub.UserID, &typesRaw, &sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting subscription by asset and user: %w", err)
	}

	if err := json.Unmarshal(typesRaw, &sub.NotificationTypes); err != nil {
		return nil, fmt.Errorf("unmarshaling notification types: %w", err)
	}

	return &sub, nil
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID string) ([]*SubscriptionWithAsset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.asset_id, s.user_id, s.notification_types, s.created_at, s.updated_at,
		       COALESCE(a.name, ''), COALESCE(a.mrn, ''), COALESCE(a.type, '')
		FROM asset_subscriptions s
		LEFT JOIN assets a ON s.asset_id = a.id
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC
		LIMIT 500`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*SubscriptionWithAsset
	for rows.Next() {
		var sub SubscriptionWithAsset
		var typesRaw []byte

		if err := rows.Scan(
			&sub.ID, &sub.AssetID, &sub.UserID, &typesRaw, &sub.CreatedAt, &sub.UpdatedAt,
			&sub.AssetName, &sub.AssetMRN, &sub.AssetType,
		); err != nil {
			return nil, fmt.Errorf("scanning subscription: %w", err)
		}

		if err := json.Unmarshal(typesRaw, &sub.NotificationTypes); err != nil {
			return nil, fmt.Errorf("unmarshaling notification types: %w", err)
		}

		subs = append(subs, &sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subscriptions: %w", err)
	}

	return subs, nil
}

func (r *PostgresRepository) GetSubscribersForAsset(ctx context.Context, assetID string, notificationType string) ([]string, error) {
	typeFilter, err := json.Marshal([]string{notificationType})
	if err != nil {
		return nil, fmt.Errorf("marshaling type filter: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT user_id FROM asset_subscriptions
		WHERE asset_id = $1 AND notification_types @> $2::jsonb
		LIMIT 1000`, assetID, typeFilter)
	if err != nil {
		return nil, fmt.Errorf("querying subscribers: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scanning subscriber: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subscribers: %w", err)
	}

	return userIDs, nil
}
