package pgtest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempDBAppliesTheMigrations(t *testing.T) {
	pool := TempDB(t)

	// The users migration seeds the admin user, so its presence means the
	// chain ran rather than just the database existing.
	var admins int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM users WHERE username = 'admin'`).Scan(&admins)
	require.NoError(t, err)
	assert.Equal(t, 1, admins)
}

func TestTempDBGivesEachCallItsOwnDatabase(t *testing.T) {
	ctx := context.Background()
	first := TempDB(t)
	second := TempDB(t)

	var a, b string
	require.NoError(t, first.QueryRow(ctx, `SELECT current_database()`).Scan(&a))
	require.NoError(t, second.QueryRow(ctx, `SELECT current_database()`).Scan(&b))
	assert.NotEqual(t, a, b)
}

func TestTempDBDropsTheDatabaseAfterwards(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("MARMOT_TEST_POSTGRES_DSN not set")
	}
	ctx := context.Background()

	// The subtest's cleanups have run by the time t.Run returns.
	var name string
	t.Run("use a database", func(t *testing.T) {
		pool := TempDB(t)
		require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&name))
	})
	require.NotEmpty(t, name)

	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close(ctx)

	var remaining int
	err = admin.QueryRow(ctx,
		`SELECT count(*) FROM pg_database WHERE datname = $1`, name).Scan(&remaining)
	require.NoError(t, err)
	assert.Zero(t, remaining)
}

func TestSeedAssetInsertsARow(t *testing.T) {
	pool := TempDB(t)
	id := SeedAsset(t, pool)

	var rows int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM assets WHERE id = $1`, id).Scan(&rows)
	require.NoError(t, err)
	assert.Equal(t, 1, rows)
}
