package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestMarkAsUnread(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()

	var userID, otherID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('reader', 'Reader') RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('other', 'Other') RETURNING id`).Scan(&otherID))

	svc := NewService(NewPostgresRepository(db), nil)
	_, err := svc.CreateSync(ctx, CreateNotificationInput{
		Recipients: []Recipient{{Type: RecipientTypeUser, ID: userID}},
		Type:       TypeSystem,
		Title:      "Hello",
		Message:    "World",
	})
	require.NoError(t, err)

	var id string
	require.NoError(t, db.QueryRow(ctx, `SELECT id FROM notifications WHERE user_id = $1`, userID).Scan(&id))
	require.NoError(t, svc.MarkAsRead(ctx, id, userID))

	assert.ErrorIs(t, svc.MarkAsUnread(ctx, id, otherID), ErrUnauthorized)
	require.NoError(t, svc.MarkAsUnread(ctx, id, userID))

	n, err := svc.Get(ctx, id)
	require.NoError(t, err)
	assert.False(t, n.Read)
	assert.Nil(t, n.ReadAt)

	summary, err := svc.GetSummary(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.UnreadCount)
	assert.Equal(t, 14, summary.ReadRetentionDays)
	assert.Equal(t, 90, summary.RetentionDays)
}
