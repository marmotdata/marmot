// Package pgtest gives database-backed tests a real, fully migrated Postgres.
//
// Each call to TempDB creates a database of its own on the server named by
// MARMOT_TEST_POSTGRES_DSN, applies every migration to it, and drops it when
// the test ends. A test therefore starts from exactly the schema the server
// would, never sees another test's rows, and packages can run in parallel
// against one server. Without MARMOT_TEST_POSTGRES_DSN the test is skipped.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marmotdata/marmot/internal/store/postgres"
)

// Setup applies migrations to a fresh database. TempDB runs the shared
// migrations and then every Setup passed to RegisterSetup, which is how a
// distribution that ships migrations of its own gets them applied too.
type Setup func(ctx context.Context, pool *pgxpool.Pool) error

var extra []Setup

// RegisterSetup adds a Setup for TempDB to run after the shared migrations.
// Call it from an init function in this package.
func RegisterSetup(s Setup) {
	extra = append(extra, s)
}

// TempDB returns a pool on a temporary database with every migration applied.
// Like t.TempDir, what it hands out belongs to this test alone and is dropped
// when the test ends. The user in MARMOT_TEST_POSTGRES_DSN needs CREATEDB; the
// database the DSN names is only used to issue CREATE DATABASE and DROP
// DATABASE.
func TempDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("MARMOT_TEST_POSTGRES_DSN not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to MARMOT_TEST_POSTGRES_DSN: %v", err)
	}

	name := "marmot_test_" + randomHex(4)
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil { //nolint:gosec // G202: the name is a quoted identifier, not input
		_ = admin.Close(ctx)
		t.Fatalf("creating database %s: %v", name, err)
	}
	t.Cleanup(func() {
		// FORCE terminates any connection still open on the database. The pool
		// is closed first, so this only matters if a test leaked a connection.
		if _, err := admin.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil { //nolint:gosec // G202: as above
			t.Errorf("dropping database %s: %v", name, err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Errorf("closing the admin connection: %v", err)
		}
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing MARMOT_TEST_POSTGRES_DSN: %v", err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	// Registered after the drop so it runs before it: cleanups run last-in,
	// first-out.
	t.Cleanup(pool.Close)

	if err := postgres.NewSetup(pool).Initialize(ctx); err != nil {
		t.Fatalf("applying migrations to %s: %v", name, err)
	}
	for _, s := range extra {
		if err := s(ctx, pool); err != nil {
			t.Fatalf("applying registered migrations to %s: %v", name, err)
		}
	}
	return pool
}

// SeedAsset inserts one asset and returns its id. It sets only the columns
// without a default: a test that needs an asset needs a row to hang things
// off, not a realistic one. It is attributed to the admin user the migrations
// create, as an asset the server created would be.
func SeedAsset(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := randomHex(4)
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO assets (id, name, mrn, type, created_by)
		SELECT gen_random_uuid()::text, $1, $2, 'Table', id
		  FROM users WHERE username = 'admin'
		RETURNING id`,
		"pgtest_"+suffix, "mrn://table/pgtest/pgtest_"+suffix).Scan(&id)
	if err != nil {
		t.Fatalf("seeding an asset: %v", err)
	}
	return id
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
