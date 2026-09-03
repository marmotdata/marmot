package sqlite

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB writes a small SQLite database to a temp file and returns its
// path. It exercises the features discovery reports: tables, a view, a
// foreign key, an internal sqlite_* table (via AUTOINCREMENT) and a couple of
// rows to count.
func newTestDB(t *testing.T) string {
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

func discover(t *testing.T, config pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	s := &Source{}
	result, err := s.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].Name != nil && *result.Assets[i].Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func TestDiscover_FindsTablesAndViews(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	users := findAsset(result, "users")
	require.NotNil(t, users)
	assert.Equal(t, "Table", users.Type)
	assert.Equal(t, []string{"SQLite"}, users.Providers)

	orders := findAsset(result, "orders")
	require.NotNil(t, orders)
	assert.Equal(t, "Table", orders.Type)

	view := findAsset(result, "active_users")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
}

func TestDiscover_CarriesTableMetadata(t *testing.T) {
	path := newTestDB(t)
	result := discover(t, pluginsdk.RawConfig{"path": path})

	users := findAsset(result, "users")
	require.NotNil(t, users)
	assert.Equal(t, path, users.Metadata["path"])
	assert.Equal(t, "users", users.Metadata["table_name"])
	assert.Equal(t, "table", users.Metadata["object_type"])
}

func TestDiscover_ExcludesSystemTablesByDefault(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		assert.Falsef(t, strings.HasPrefix(*a.Name, "sqlite_"),
			"internal table leaked into results: %s", *a.Name)
	}
}

func TestDiscover_IncludesSystemTablesWhenConfigured(t *testing.T) {
	// AUTOINCREMENT forces SQLite to create the internal sqlite_sequence
	// table, which must surface once the exclusion is turned off.
	result := discover(t, pluginsdk.RawConfig{
		"path":                  newTestDB(t),
		"exclude_system_tables": false,
	})

	assert.NotNil(t, findAsset(result, "sqlite_sequence"))
}

func TestDiscover_IncludesColumnsInSchema(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	users := findAsset(result, "users")
	require.NotNil(t, users)

	raw, ok := users.Schema["columns"]
	require.True(t, ok, "expected column schema on the users asset")

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}

	require.Contains(t, byName, "id")
	require.Contains(t, byName, "name")
	require.Contains(t, byName, "email")

	assert.Equal(t, true, byName["id"]["is_primary_key"])
	assert.Equal(t, false, byName["name"]["is_nullable"])
	assert.Equal(t, true, byName["email"]["is_nullable"])
}

func TestDiscover_DiscoversForeignKeys(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	want := pluginsdk.LineageEdge{
		Source: assetMRN("Table", "orders"),
		Target: assetMRN("Table", "users"),
		Type:   "FOREIGN_KEY",
	}
	assert.Contains(t, result.Lineage, want)
}

func TestDiscover_EmitsRowAndColumnCountStatistics(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	usersMRN := assetMRN("Table", "users")

	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == usersMRN {
			stats[st.MetricName] = st.Value
		}
	}

	assert.Equal(t, float64(2), stats["asset.row_count"])
	assert.Equal(t, float64(3), stats["asset.column_count"])
}

func TestDiscover_SkipsRowCountForViews(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"path": newTestDB(t)})

	viewMRN := assetMRN("View", "active_users")
	for _, st := range result.Statistics {
		if st.AssetMRN == viewMRN {
			assert.NotEqual(t, "asset.row_count", st.MetricName,
				"views should not report a row count")
		}
	}
}

func TestDiscover_HandlesPathWithSpaces(t *testing.T) {
	// A file: DSN built by naive string interpolation breaks on a path
	// containing a space; readOnlyDSN percent-encodes it. Put the space in
	// both the directory and the filename to exercise the escaping.
	dir := filepath.Join(t.TempDir(), "my data")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "app store.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	result := discover(t, pluginsdk.RawConfig{"path": path})
	assert.NotNil(t, findAsset(result, "widgets"))
}

func TestDiscover_MissingFileFails(t *testing.T) {
	s := &Source{}
	_, err := s.Discover(t.Context(), pluginsdk.RawConfig{
		"path": filepath.Join(t.TempDir(), "does-not-exist.db"),
	})
	require.Error(t, err)
}
