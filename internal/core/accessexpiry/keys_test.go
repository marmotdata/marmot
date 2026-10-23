package accessexpiry

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/notification"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestKeyNoticeWarnsOnlyForLongLivedKeys(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

	long := keyRow{ID: "k1", Name: "ci", CreatedAt: now.Add(-90 * 24 * time.Hour), ExpiresAt: now.Add(3 * 24 * time.Hour)}
	n, ok := keyNotice(long, now)
	require.True(t, ok)
	assert.Equal(t, notification.TypeAccessExpiry, n.Input.Type)
	assert.Equal(t, StateExpiring, n.Input.Data["state"])
	assert.Equal(t, KindAPIKey, n.Input.Data["kind"])
	assert.Equal(t, "/account/api-keys", n.Input.Data["link"])

	// A key made to last two days was always going to expire soon, so warning about it is noise
	short := keyRow{ID: "k2", Name: "temp", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour)}
	_, ok = keyNotice(short, now)
	assert.False(t, ok)

	// Once it has expired it is announced like any other
	short.ExpiresAt = now.Add(-time.Minute)
	n, ok = keyNotice(short, now)
	require.True(t, ok)
	assert.Equal(t, StateExpired, n.Input.Data["state"])
}

func TestKeyNoticeKeyChangesWithExpiryAndState(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	k := keyRow{ID: "k1", Name: "ci", AccountID: "sa1", AccountName: "loader", CreatedAt: now.Add(-60 * 24 * time.Hour), ExpiresAt: now.Add(2 * 24 * time.Hour)}

	expiring, _ := keyNotice(k, now)
	assert.Equal(t, KindServiceAccountKey, expiring.Input.Data["kind"])
	assert.Equal(t, "/settings/service-accounts/sa1", expiring.Input.Data["link"])

	expired, _ := keyNotice(k, k.ExpiresAt.Add(time.Hour))
	assert.NotEqual(t, expiring.Key, expired.Key, "expiring and expired are separate notices")

	k.ExpiresAt = k.ExpiresAt.Add(30 * 24 * time.Hour)
	extended, _ := keyNotice(k, k.ExpiresAt.Add(-24*time.Hour))
	assert.NotEqual(t, expiring.Key, extended.Key, "a new expiry arms a new warning")
}

type recordingSender struct {
	sent map[string]notification.CreateNotificationInput
}

func (r *recordingSender) CreateOnce(_ context.Context, key string, input notification.CreateNotificationInput) (bool, error) {
	if _, ok := r.sent[key]; ok {
		return false, nil
	}
	r.sent[key] = input
	return true, nil
}

func seedUser(t *testing.T, db *pgxpool.Pool, username string, active bool) string {
	t.Helper()
	var id string
	require.NoError(t, db.QueryRow(context.Background(),
		`INSERT INTO users (username, name, active) VALUES ($1, $1, $2) RETURNING id`, username, active).Scan(&id))
	return id
}

func TestKeySourceRecipients(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	now := time.Now()

	owner := seedUser(t, db, "owner", true)
	gone := seedUser(t, db, "gone", false)
	admin := seedUser(t, db, "keeper", true)
	_, err := db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'admin'`, admin)
	require.NoError(t, err)

	created := now.Add(-60 * 24 * time.Hour)
	soon := now.Add(2 * 24 * time.Hour)
	_, err = db.Exec(ctx, `INSERT INTO api_keys (user_id, name, key_hash, expires_at, created_at) VALUES ($1, 'mine', 'h1', $2, $3)`, owner, soon, created)
	require.NoError(t, err)
	// Far off, and long expired, keys are not due
	_, err = db.Exec(ctx, `INSERT INTO api_keys (user_id, name, key_hash, expires_at, created_at) VALUES ($1, 'later', 'h2', $2, $3), ($1, 'ancient', 'h3', $4, $3)`,
		owner, now.Add(60*24*time.Hour), created, now.Add(-30*24*time.Hour))
	require.NoError(t, err)

	var ownedSA, orphanSA string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO service_accounts (name, created_by) VALUES ('loader', $1) RETURNING id`, owner).Scan(&ownedSA))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO service_accounts (name, created_by) VALUES ('orphan', $1) RETURNING id`, gone).Scan(&orphanSA))
	_, err = db.Exec(ctx, `INSERT INTO service_account_api_keys (service_account_id, name, key_hash, expires_at, created_at) VALUES ($1, 'k', 'h4', $3, $4), ($2, 'k', 'h5', $5, $4)`,
		ownedSA, orphanSA, soon, created, now.Add(-time.Hour))
	require.NoError(t, err)

	// Editions may respell the resource type, so read it rather than assume it
	var saResource string
	require.NoError(t, db.QueryRow(ctx, `SELECT resource_type FROM permissions WHERE name = 'service_accounts_manage'`).Scan(&saResource))
	source := NewKeySource(db, RoleHolders(db, saResource, "manage"))
	sender := &recordingSender{sent: map[string]notification.CreateNotificationInput{}}
	runner := &Runner{sender: sender, sources: []Source{source}}
	require.NoError(t, runner.Run(ctx, now))
	require.Len(t, sender.sent, 3)

	byName := map[string]notification.CreateNotificationInput{}
	for _, in := range sender.sent {
		name := in.Data["key_name"].(string)
		if account, ok := in.Data["account_name"].(string); ok {
			name = account + "/" + name
		}
		byName[name] = in
	}
	user := func(id string) []notification.Recipient {
		return []notification.Recipient{{Type: notification.RecipientTypeUser, ID: id}}
	}
	assert.Equal(t, user(owner), byName["mine"].Recipients)
	assert.Equal(t, user(owner), byName["loader/k"].Recipients)
	// An account whose creator has gone goes to whoever manages service accounts, which includes the admin the migrations seed
	orphanTo := byName["orphan/k"].Recipients
	assert.Contains(t, orphanTo, user(admin)[0])
	assert.NotContains(t, orphanTo, user(gone)[0])
	assert.NotContains(t, orphanTo, user(owner)[0])
	assert.Equal(t, StateExpired, byName["orphan/k"].Data["state"])

	// A second run sends nothing new
	require.NoError(t, runner.Run(ctx, now.Add(time.Hour)))
	assert.Len(t, sender.sent, 3)
}
