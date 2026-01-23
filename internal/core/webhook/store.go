package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the webhook data access interface.
type Repository interface {
	Create(ctx context.Context, webhook *Webhook) error
	Get(ctx context.Context, id string) (*Webhook, error)
	Update(ctx context.Context, id string, input UpdateWebhookInput) (*Webhook, error)
	Delete(ctx context.Context, id string) error
	ListByTeam(ctx context.Context, teamID string) ([]*Webhook, error)
	GetEnabledForNotificationType(ctx context.Context, teamID string, notificationType string) ([]*Webhook, error)
	UpdateLastTriggered(ctx context.Context, id string, lastError *string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, webhook *Webhook) error {
	typesJSON, err := json.Marshal(webhook.NotificationTypes)
	if err != nil {
		return fmt.Errorf("marshaling notification types: %w", err)
	}

	now := time.Now()
	err = r.db.QueryRow(ctx, `
		INSERT INTO team_webhooks (team_id, name, provider, webhook_url, notification_types, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		webhook.TeamID, webhook.Name, webhook.Provider, webhook.WebhookURL, typesJSON, webhook.Enabled, now, now,
	).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)

	if err != nil {
		return fmt.Errorf("creating webhook: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Webhook, error) {
	var webhook Webhook
	var typesRaw []byte

	err := r.db.QueryRow(ctx, `
		SELECT id, team_id, name, provider, webhook_url, notification_types, enabled,
		       last_triggered_at, last_error, created_at, updated_at
		FROM team_webhooks WHERE id = $1`, id,
	).Scan(
		&webhook.ID, &webhook.TeamID, &webhook.Name, &webhook.Provider,
		&webhook.WebhookURL, &typesRaw, &webhook.Enabled,
		&webhook.LastTriggeredAt, &webhook.LastError,
		&webhook.CreatedAt, &webhook.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting webhook: %w", err)
	}

	if err := json.Unmarshal(typesRaw, &webhook.NotificationTypes); err != nil {
		return nil, fmt.Errorf("unmarshaling notification types: %w", err)
	}

	return &webhook, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id string, input UpdateWebhookInput) (*Webhook, error) {
	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.WebhookURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("webhook_url = $%d", argIdx))
		args = append(args, *input.WebhookURL)
		argIdx++
	}
	if input.NotificationTypes != nil {
		typesJSON, err := json.Marshal(input.NotificationTypes)
		if err != nil {
			return nil, fmt.Errorf("marshaling notification types: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("notification_types = $%d", argIdx))
		args = append(args, typesJSON)
		argIdx++
	}
	if input.Enabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *input.Enabled)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.Get(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := "UPDATE team_webhooks SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)
	query += " RETURNING id, team_id, name, provider, webhook_url, notification_types, enabled, last_triggered_at, last_error, created_at, updated_at"

	var webhook Webhook
	var typesRaw []byte

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&webhook.ID, &webhook.TeamID, &webhook.Name, &webhook.Provider,
		&webhook.WebhookURL, &typesRaw, &webhook.Enabled,
		&webhook.LastTriggeredAt, &webhook.LastError,
		&webhook.CreatedAt, &webhook.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("updating webhook: %w", err)
	}

	if err := json.Unmarshal(typesRaw, &webhook.NotificationTypes); err != nil {
		return nil, fmt.Errorf("unmarshaling notification types: %w", err)
	}

	return &webhook, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM team_webhooks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListByTeam(ctx context.Context, teamID string) ([]*Webhook, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, team_id, name, provider, webhook_url, notification_types, enabled,
		       last_triggered_at, last_error, created_at, updated_at
		FROM team_webhooks
		WHERE team_id = $1
		ORDER BY created_at DESC
		LIMIT 50`, teamID)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var webhook Webhook
		var typesRaw []byte

		if err := rows.Scan(
			&webhook.ID, &webhook.TeamID, &webhook.Name, &webhook.Provider,
			&webhook.WebhookURL, &typesRaw, &webhook.Enabled,
			&webhook.LastTriggeredAt, &webhook.LastError,
			&webhook.CreatedAt, &webhook.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}

		if err := json.Unmarshal(typesRaw, &webhook.NotificationTypes); err != nil {
			return nil, fmt.Errorf("unmarshaling notification types: %w", err)
		}

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating webhooks: %w", err)
	}

	return webhooks, nil
}

func (r *PostgresRepository) GetEnabledForNotificationType(ctx context.Context, teamID string, notificationType string) ([]*Webhook, error) {
	typeFilter, err := json.Marshal([]string{notificationType})
	if err != nil {
		return nil, fmt.Errorf("marshaling type filter: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, team_id, name, provider, webhook_url, notification_types, enabled,
		       last_triggered_at, last_error, created_at, updated_at
		FROM team_webhooks
		WHERE team_id = $1 AND enabled = TRUE AND notification_types @> $2::jsonb`,
		teamID, typeFilter)
	if err != nil {
		return nil, fmt.Errorf("querying enabled webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var webhook Webhook
		var typesRaw []byte

		if err := rows.Scan(
			&webhook.ID, &webhook.TeamID, &webhook.Name, &webhook.Provider,
			&webhook.WebhookURL, &typesRaw, &webhook.Enabled,
			&webhook.LastTriggeredAt, &webhook.LastError,
			&webhook.CreatedAt, &webhook.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}

		if err := json.Unmarshal(typesRaw, &webhook.NotificationTypes); err != nil {
			return nil, fmt.Errorf("unmarshaling notification types: %w", err)
		}

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating webhooks: %w", err)
	}

	return webhooks, nil
}

func (r *PostgresRepository) UpdateLastTriggered(ctx context.Context, id string, lastError *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE team_webhooks
		SET last_triggered_at = NOW(), last_error = $2, updated_at = NOW()
		WHERE id = $1`, id, lastError)
	if err != nil {
		return fmt.Errorf("updating last triggered: %w", err)
	}
	return nil
}
