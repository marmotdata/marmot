package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newQueryTestDB writes a SQLite database with a table, an indexed table and a view, for the query-path tests to plan and run statements against. It is kept separate from discover_test.go's newTestDB so query tests can add fixtures (the index, in particular) without touching discovery's.
func newQueryTestDB(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "query.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, amount REAL)`,
		`CREATE INDEX idx_orders_user_id ON orders(user_id)`,
		`CREATE VIEW active_users AS SELECT id, name FROM users WHERE id > 0`,
		`INSERT INTO users (id, name) VALUES (1, 'alice'), (2, 'bob')`,
		`INSERT INTO orders (id, user_id, amount) VALUES (1, 1, 10.5), (2, 2, 20.0)`,
	}
	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	return path
}

func collectRows(t *testing.T, s *Source, config pluginsdk.RawConfig, req pluginsdk.QueryRequest) []pluginsdk.QueryResultChunk {
	t.Helper()

	var chunks []pluginsdk.QueryResultChunk
	err := s.ExecuteQuery(t.Context(), config, req, func(chunk pluginsdk.QueryResultChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	require.NoError(t, err)
	return chunks
}

// Statement checking.

func TestCheckStatement_AllowsReads(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from users",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"-- the latest orders\nSELECT * FROM orders",
		"/* note */ VALUES (1)",
		"TABLE users",
		"SELECT * FROM users WHERE name = 'o''brien'",
		"SELECT * FROM users; -- trailing comment only",
		"SELECT * FROM users ;  ",
	}
	for _, stmt := range reads {
		assert.NoError(t, checkStatement(stmt), stmt)
	}
}

func TestCheckStatement_RefusesWritesAndEscapes(t *testing.T) {
	writes := []string{
		"INSERT INTO users VALUES (3, 'carol')",
		"UPDATE users SET name = 'x'",
		"DELETE FROM users",
		"CREATE TABLE copy AS SELECT * FROM users",
		"DROP TABLE users",
		"ALTER TABLE users RENAME TO users2",
		"ATTACH DATABASE 'other.db' AS other",
		"DETACH DATABASE other",
		"PRAGMA writable_schema = ON",
		"PRAGMA table_info(users)",
		"VACUUM",
		"VACUUM INTO 'copy.db'",
		"REINDEX",
		"ANALYZE",
		"BEGIN",
		"SELECT load_extension('evil')",
		"SELECT * FROM users; DROP TABLE users",
		"",
		"SELECTED",
		"/* unterminated",
		"SELECT * FROM users WHERE name = 'unterminated",
	}
	for _, stmt := range writes {
		assert.Error(t, checkStatement(stmt), stmt)
	}
}

func TestCheckStatement_RefusesSemicolonHiddenInsideAStringLiteral(t *testing.T) {
	// The ';' here is data, not a statement separator, so this must read as one statement and be allowed.
	assert.NoError(t, checkStatement("SELECT 'a;b' FROM users"))
}

func TestCheckStatement_RefusesSemicolonHiddenInsideALineComment(t *testing.T) {
	assert.Error(t, checkStatement("SELECT 1 -- ; not a real separator\n; SELECT 2"))
}

func TestMaskStatement_BlanksCommentsAndLiteralsButKeepsLength(t *testing.T) {
	masked, ok := maskStatement("SELECT 'x' FROM t -- trailing\n WHERE 1=1")
	require.True(t, ok)
	assert.Equal(t, len("SELECT 'x' FROM t -- trailing\n WHERE 1=1"), len(masked))
	assert.NotContains(t, masked, "trailing")
}

func TestMaskStatement_FailsClosedOnUnterminatedQuote(t *testing.T) {
	_, ok := maskStatement("SELECT 'unterminated")
	assert.False(t, ok)
}

func TestLeadingKeyword_SkipsParensAndWhitespace(t *testing.T) {
	assert.Equal(t, "select", leadingKeyword("  (( SELECT 1"))
	assert.Equal(t, "pragma", leadingKeyword("PRAGMA foo"))
}

// Connection caching.

func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	open := func() *cachedConn {
		db, err := sql.Open("sqlite", ":memory:")
		require.NoError(t, err)
		return &cachedConn{db: db, cleanup: func() {}}
	}
	var q queryConns
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.Same(t, a, q.put("a", open()), "a connection opened concurrently for the same config is discarded")

	q.drop("a", b)
	assert.Same(t, a, q.get("a"), "dropping a stale entry never removes its replacement")
	q.drop("a", a)
	assert.Nil(t, q.get("a"))

	q.conns["b"].lastUsed = q.conns["b"].lastUsed.Add(-connIdleTTL - 1)
	assert.Nil(t, q.get("b"), "an idle connection is closed and forgotten")
}

// Planning.

func TestPlanQuery_TableMRNMatchesDiscovery(t *testing.T) {
	path := newQueryTestDB(t)
	config := pluginsdk.RawConfig{"path": path}

	discovered := discover(t, config)
	usersAsset := findAsset(discovered, "users")
	require.NotNil(t, usersAsset)

	s := &Source{}
	plan, err := s.PlanQuery(t.Context(), config, pluginsdk.QueryRequest{Statement: "SELECT * FROM users"})
	require.NoError(t, err)

	assert.Equal(t, []string{*usersAsset.MRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestPlanQuery_ViewResolvesToItsBaseTable(t *testing.T) {
	// SQLite reads a view by reading the base tables behind it; it never opens a root page for the view itself, so the plan names "users" here, not "active_users", the same way the PostgreSQL and Trino plugins report a view's reads against its base tables.
	path := newQueryTestDB(t)
	s := &Source{}

	plan, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "SELECT * FROM active_users"})
	require.NoError(t, err)

	assert.Equal(t, []string{assetMRN("Table", "users")}, plan.ReferencedMRNs)
}

func TestPlanQuery_JoinThroughAnIndexListsBothTablesOnce(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	plan, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT u.name, o.amount FROM users u JOIN orders o ON o.user_id = u.id WHERE o.user_id = 1",
	})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{assetMRN("Table", "users"), assetMRN("Table", "orders")}, plan.ReferencedMRNs)
}

func TestPlanQuery_IncompleteForATableValuedFunction(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	plan, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "SELECT * FROM pragma_table_info('users')"})
	require.NoError(t, err)

	assert.True(t, plan.Incomplete)
}

func TestPlanQuery_RefusesAWriteHiddenBehindAWith(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	_, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "WITH t AS (SELECT 1) UPDATE users SET name = 'x'",
	})
	require.ErrorIs(t, err, errReadOnly)
}

func TestPlanQuery_RefusesAnOrdinaryWrite(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	_, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "DELETE FROM users"})
	require.ErrorIs(t, err, errReadOnly)
}

func TestPlanQuery_RefusesMultipleStatements(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	_, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "SELECT 1; DELETE FROM users"})
	require.ErrorIs(t, err, errMultipleStatements)
}

func TestPlanQuery_RefusesAttach(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	_, err := s.PlanQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "ATTACH DATABASE 'other.db' AS other"})
	require.ErrorIs(t, err, errReadOnly)
}

// Executing.

func TestExecuteQuery_StreamsRowsWithColumnsAndTypes(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	chunks := collectRows(t, s, pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "SELECT id, name FROM users ORDER BY id"})
	require.Len(t, chunks, 1)

	require.Equal(t, []pluginsdk.QueryColumn{
		{Name: "id", Type: "integer"},
		{Name: "name", Type: "text"},
	}, chunks[0].Columns)
	require.Len(t, chunks[0].Rows, 2)
	assert.EqualValues(t, 1, chunks[0].Rows[0][0])
	assert.Equal(t, "alice", chunks[0].Rows[0][1])
	assert.EqualValues(t, 2, chunks[0].Rows[1][0])
	assert.Equal(t, "bob", chunks[0].Rows[1][1])
}

func TestExecuteQuery_HonoursMaxRows(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	chunks := collectRows(t, s, pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT id FROM users ORDER BY id",
		MaxRows:   1,
	})

	var total int
	for _, c := range chunks {
		total += len(c.Rows)
	}
	assert.Equal(t, 1, total)
}

func TestExecuteQuery_EmitsSchemaEvenForZeroRows(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	chunks := collectRows(t, s, pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "SELECT id, name FROM users WHERE id = -1"})

	require.Len(t, chunks, 1)
	assert.Empty(t, chunks[0].Rows)
	assert.Equal(t, []pluginsdk.QueryColumn{
		{Name: "id", Type: "integer"},
		{Name: "name", Type: "text"},
	}, chunks[0].Columns)
}

func TestExecuteQuery_RefusesWrites(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	err := s.ExecuteQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "DELETE FROM users"}, func(pluginsdk.QueryResultChunk) error { return nil })
	require.ErrorIs(t, err, errReadOnly)

	// The refused statement never reached the engine, so nothing changed.
	result := discover(t, pluginsdk.RawConfig{"path": path})
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN("Table", "users") {
			stats[st.MetricName] = st.Value
		}
	}
	assert.Equal(t, float64(2), stats["asset.row_count"])
}

func TestExecuteQuery_RefusesMultipleStatements(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	err := s.ExecuteQuery(t.Context(), pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT 1; DELETE FROM users",
	}, func(pluginsdk.QueryResultChunk) error { return nil })
	require.ErrorIs(t, err, errMultipleStatements)

	result := discover(t, pluginsdk.RawConfig{"path": path})
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN("Table", "users") {
			stats[st.MetricName] = st.Value
		}
	}
	assert.Equal(t, float64(2), stats["asset.row_count"], "a hidden second statement must never run")
}

func TestExecuteQuery_RefusesPragmaWritableSchema(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	err := s.ExecuteQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "PRAGMA writable_schema = ON"}, func(pluginsdk.QueryResultChunk) error { return nil })
	require.Error(t, err)
}

func TestExecuteQuery_RefusesVacuumInto(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}

	err := s.ExecuteQuery(t.Context(), pluginsdk.RawConfig{"path": path},
		pluginsdk.QueryRequest{Statement: "VACUUM INTO 'copy.db'"}, func(pluginsdk.QueryResultChunk) error { return nil })
	require.Error(t, err)
}

func TestExecuteQuery_ReusesConnectionAcrossCalls(t *testing.T) {
	path := newQueryTestDB(t)
	s := &Source{}
	config := pluginsdk.RawConfig{"path": path}

	collectRows(t, s, config, pluginsdk.QueryRequest{Statement: "SELECT 1"})
	first := s.query.get(configHash(mustParseConfig(t, config)))
	require.NotNil(t, first)

	collectRows(t, s, config, pluginsdk.QueryRequest{Statement: "SELECT 1"})
	second := s.query.get(configHash(mustParseConfig(t, config)))

	assert.Same(t, first, second)
}

func mustParseConfig(t *testing.T, raw pluginsdk.RawConfig) *Config {
	t.Helper()
	config, err := parseConfig(raw)
	require.NoError(t, err)
	return config
}
