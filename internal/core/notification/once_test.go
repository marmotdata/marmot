package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestCreateOnceSurvivesDismissal(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()

	var userID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('reader', 'Reader') RETURNING id`).Scan(&userID))

	svc := NewService(NewPostgresRepository(db), nil, WithDB(db))
	input := CreateNotificationInput{
		Recipients: []Recipient{{Type: RecipientTypeUser, ID: userID}},
		Type:       TypeAccessExpiry,
		Title:      "API key expired",
		Message:    "Your API key has expired.",
	}

	sent, err := svc.CreateOnce(ctx, "api-key:k1:1:expired", input)
	require.NoError(t, err)
	assert.True(t, sent)

	countFor := func() int {
		var n int
		require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1`, userID).Scan(&n))
		return n
	}
	assert.Equal(t, 1, countFor())

	// The reader clears it; the next check must not bring it back
	_, err = db.Exec(ctx, `DELETE FROM notifications WHERE user_id = $1`, userID)
	require.NoError(t, err)
	sent, err = svc.CreateOnce(ctx, "api-key:k1:1:expired", input)
	require.NoError(t, err)
	assert.False(t, sent)
	assert.Equal(t, 0, countFor())

	// A different occurrence is its own notice
	sent, err = svc.CreateOnce(ctx, "api-key:k1:2:expired", input)
	require.NoError(t, err)
	assert.True(t, sent)
	assert.Equal(t, 1, countFor())
}
