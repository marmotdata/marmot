package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the notification data access interface.
type Repository interface {
	CreateBatch(ctx context.Context, userRecipients map[string]Recipient, input CreateNotificationInput, batchSize int) (int, error)
	Get(ctx context.Context, id string) (*Notification, error)
	List(ctx context.Context, filter NotificationFilter) ([]*Notification, int, error)
	ListWithCursor(ctx context.Context, filter NotificationFilter) (*ListResult, error)
	GetSummary(ctx context.Context, userID string) (*NotificationSummary, error)
	MarkAsRead(ctx context.Context, id string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	MarkAllAsReadChunked(ctx context.Context, userID string, chunkSize int) error
	Delete(ctx context.Context, id string) error
	DeleteAllRead(ctx context.Context, userID string) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// PostgresRepository implements Repository for PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL notification repository.
func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// CreateBatch creates notifications for multiple users in batches.
func (r *PostgresRepository) CreateBatch(ctx context.Context, userRecipients map[string]Recipient, input CreateNotificationInput, batchSize int) (int, error) {
	if len(userRecipients) == 0 {
		return 0, nil
	}

	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	if batchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	}

	dataJSON, err := json.Marshal(input.Data)
	if err != nil {
		return 0, fmt.Errorf("marshaling data: %w", err)
	}

	recipients := make([]userRecipient, 0, len(userRecipients))
	for userID, recipient := range userRecipients {
		recipients = append(recipients, userRecipient{userID: userID, recipient: recipient})
	}

	totalInserted := 0
	for i := 0; i < len(recipients); i += batchSize {
		end := i + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}
		chunk := recipients[i:end]

		count, err := r.insertChunk(ctx, chunk, input, dataJSON)
		if err != nil {
			return totalInserted, fmt.Errorf("inserting chunk %d: %w", i/batchSize, err)
		}
		totalInserted += count
	}

	return totalInserted, nil
}

type userRecipient struct {
	userID    string
	recipient Recipient
}

func (r *PostgresRepository) insertChunk(ctx context.Context, chunk []userRecipient, input CreateNotificationInput, dataJSON []byte) (int, error) {
	valueStrings := make([]string, 0, len(chunk))
	valueArgs := make([]interface{}, 0, len(chunk)*7)
	argIndex := 1

	for _, ur := range chunk {
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6))
		valueArgs = append(valueArgs,
			ur.userID,
			ur.recipient.Type,
			ur.recipient.ID,
			input.Type,
			input.Title,
			input.Message,
			dataJSON,
		)
		argIndex += 7
	}

	query := fmt.Sprintf(`
		INSERT INTO notifications (user_id, recipient_type, recipient_id, type, title, message, data)
		VALUES %s`, strings.Join(valueStrings, ", "))

	result, err := r.db.Exec(ctx, query, valueArgs...)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// Get retrieves a single notification by ID.
func (r *PostgresRepository) Get(ctx context.Context, id string) (*Notification, error) {
	query := `
		SELECT id, user_id, recipient_type, recipient_id, type, title, message, data, read, read_at, created_at
		FROM notifications
		WHERE id = $1`

	notification := &Notification{}
	var dataJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.RecipientType,
		&notification.RecipientID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&dataJSON,
		&notification.Read,
		&notification.ReadAt,
		&notification.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("getting notification: %w", err)
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
			return nil, fmt.Errorf("unmarshaling data: %w", err)
		}
	}

	return notification, nil
}

// List retrieves notifications for a user with offset-based pagination.
func (r *PostgresRepository) List(ctx context.Context, filter NotificationFilter) ([]*Notification, int, error) {
	whereClauses := []string{"user_id = $1"}
	args := []interface{}{filter.UserID}
	argIndex := 2

	if filter.Type != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, filter.Type)
		argIndex++
	}

	if filter.ReadOnly != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("read = $%d", argIndex))
		args = append(args, *filter.ReadOnly)
		argIndex++
	}

	whereClause := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notifications WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting notifications: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, recipient_type, recipient_id, type, title, message, data, read, read_at, created_at
		FROM notifications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		whereClause, argIndex, argIndex+1)

	args = append(args, filter.Limit, filter.Offset)

	return r.queryNotifications(ctx, query, args, total)
}

// ListWithCursor retrieves notifications using cursor-based pagination.
func (r *PostgresRepository) ListWithCursor(ctx context.Context, filter NotificationFilter) (*ListResult, error) {
	var cursorTime time.Time
	var err error
	if filter.Cursor != "" {
		cursorTime, err = time.Parse(time.RFC3339Nano, filter.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
	}

	whereClauses := []string{"user_id = $1"}
	args := []interface{}{filter.UserID}
	argIndex := 2

	if !cursorTime.IsZero() {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at < $%d", argIndex))
		args = append(args, cursorTime)
		argIndex++
	}

	if filter.Type != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, filter.Type)
		argIndex++
	}

	if filter.ReadOnly != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("read = $%d", argIndex))
		args = append(args, *filter.ReadOnly)
		argIndex++
	}

	whereClause := strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT id, user_id, recipient_type, recipient_id, type, title, message, data, read, read_at, created_at
		FROM notifications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d`,
		whereClause, argIndex)

	args = append(args, filter.Limit+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]*Notification, 0, filter.Limit)
	for rows.Next() {
		n := &Notification{}
		var dataJSON []byte
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.RecipientType, &n.RecipientID,
			&n.Type, &n.Title, &n.Message, &dataJSON,
			&n.Read, &n.ReadAt, &n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning notification: %w", err)
		}
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
				return nil, fmt.Errorf("unmarshaling data: %w", err)
			}
		}
		notifications = append(notifications, n)
	}

	result := &ListResult{
		Notifications: notifications,
		Total:         -1,
	}

	if len(notifications) > filter.Limit {
		result.Notifications = notifications[:filter.Limit]
		lastNotification := result.Notifications[len(result.Notifications)-1]
		result.NextCursor = lastNotification.CreatedAt.Format(time.RFC3339Nano)
	}

	return result, nil
}

func (r *PostgresRepository) queryNotifications(ctx context.Context, query string, args []interface{}, total int) ([]*Notification, int, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]*Notification, 0)
	for rows.Next() {
		n := &Notification{}
		var dataJSON []byte
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.RecipientType, &n.RecipientID,
			&n.Type, &n.Title, &n.Message, &dataJSON,
			&n.Read, &n.ReadAt, &n.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning notification: %w", err)
		}
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
				return nil, 0, fmt.Errorf("unmarshaling data: %w", err)
			}
		}
		notifications = append(notifications, n)
	}

	return notifications, total, nil
}

// GetSummary returns unread/total count for a user.
func (r *PostgresRepository) GetSummary(ctx context.Context, userID string) (*NotificationSummary, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE read = FALSE) as unread_count,
			COUNT(*) as total_count
		FROM notifications
		WHERE user_id = $1`

	summary := &NotificationSummary{}
	err := r.db.QueryRow(ctx, query, userID).Scan(&summary.UnreadCount, &summary.TotalCount)
	if err != nil {
		return nil, fmt.Errorf("getting summary: %w", err)
	}

	return summary, nil
}

// MarkAsRead marks a single notification as read.
func (r *PostgresRepository) MarkAsRead(ctx context.Context, id string) error {
	query := `UPDATE notifications SET read = TRUE, read_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all notifications for a user as read.
func (r *PostgresRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET read = TRUE, read_at = NOW() WHERE user_id = $1 AND read = FALSE`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("marking all notifications as read: %w", err)
	}
	return nil
}

// MarkAllAsReadChunked marks notifications as read in chunks to avoid long locks.
func (r *PostgresRepository) MarkAllAsReadChunked(ctx context.Context, userID string, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = 1000
	}

	for {
		query := `
			UPDATE notifications
			SET read = TRUE, read_at = NOW()
			WHERE id IN (
				SELECT id FROM notifications
				WHERE user_id = $1 AND read = FALSE
				LIMIT $2
				FOR UPDATE SKIP LOCKED
			)`

		result, err := r.db.Exec(ctx, query, userID, chunkSize)
		if err != nil {
			return fmt.Errorf("marking notifications as read: %w", err)
		}

		if result.RowsAffected() == 0 {
			break
		}
	}

	return nil
}

// Delete deletes a single notification.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM notifications WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting notification: %w", err)
	}
	return nil
}

// DeleteAllRead deletes all read notifications for a user.
func (r *PostgresRepository) DeleteAllRead(ctx context.Context, userID string) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND read = TRUE`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("deleting read notifications: %w", err)
	}
	return nil
}

// DeleteOlderThan deletes notifications older than the specified time.
func (r *PostgresRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM notifications WHERE created_at < $1`
	result, err := r.db.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("deleting old notifications: %w", err)
	}
	return result.RowsAffected(), nil
}
