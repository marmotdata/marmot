package sqlite_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Importing the plugin package registers the modernc.org/sqlite driver,
	// so the fixture below can open a database in write mode.
	_ "github.com/marmotdata/marmot/plugins/sqlite/sqlite"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses: plugintest.Build compiles the main package
// and every call spawns the process, runs one RPC and kills it again.

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func newE2ETestDB(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	stmts := []string{
		`CREATE TABLE users (
			id    INTEGER PRIMARY KEY AUTOINCREMENT,
			name  TEXT NOT NULL,
			email TEXT
		)`,
		`CREATE TABLE orders (
			id      INTEGER PRIMARY KEY,
			user_id INTEGER,
			total   REAL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE VIEW active_users AS SELECT id, name FROM users`,
		`INSERT INTO users (name, email) VALUES ('alice', 'alice@example.com')`,
		`INSERT INTO users (name, email) VALUES ('bob', NULL)`,
		`INSERT INTO orders (user_id, total) VALUES (1, 42.5)`,
	}
	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	return path
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "sqlite", meta.ID)
	assert.Equal(t, "SQLite", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingPathFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_DiscoverOverTheWire(t *testing.T) {
	bin := buildBinary(t)
	path := newE2ETestDB(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"path": path})
	require.NoError(t, err)
	require.NotNil(t, result)

	names := make(map[string]string) // name -> type
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		names[*a.Name] = a.Type
	}
	assert.Equal(t, "Table", names["users"])
	assert.Equal(t, "Table", names["orders"])
	assert.Equal(t, "View", names["active_users"])

	// The foreign key survives the round trip as a lineage edge.
	var foundFK bool
	for _, e := range result.Lineage {
		if e.Type == "FOREIGN_KEY" &&
			e.Source == "mrn://table/sqlite/orders" &&
			e.Target == "mrn://table/sqlite/users" {
			foundFK = true
		}
	}
	assert.True(t, foundFK, "expected orders->users foreign key edge")

	assert.NotEmpty(t, result.Statistics, "expected row/column count statistics")
}
