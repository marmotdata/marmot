package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadStatement(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from orders",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"-- the latest orders\nSELECT * FROM orders",
		"/* note */ VALUES (1)",
		"TABLE orders",
		"FROM orders",
		"FROM orders SELECT id",
	}
	for _, s := range reads {
		assert.True(t, isReadStatement(s), s)
	}

	writes := []string{
		"INSERT INTO orders VALUES (1)",
		"UPDATE orders SET id = 1",
		"DELETE FROM orders",
		"CREATE TABLE stolen AS SELECT 1",
		"DROP TABLE orders",
		"ATTACH '/tmp/evil.duckdb' AS evil",
		"COPY orders TO '/tmp/out.csv'",
		"INSTALL httpfs",
		"LOAD httpfs",
		"SET memory_limit='1GB'",
		"PRAGMA database_list",
		"CALL some_function()",
		"-- SELECT\nINSERT INTO t VALUES (1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, s := range writes {
		assert.False(t, isReadStatement(s), s)
	}
}

func TestSingleStatement(t *testing.T) {
	single := []string{
		"SELECT 1",
		"SELECT 1;",
		"SELECT 1;  ",
		"SELECT ';' FROM orders",
		`SELECT "weird;column" FROM orders`,
		"-- a comment with a ; inside\nSELECT 1",
		"/* a comment; with a semicolon */ SELECT 1",
		"SELECT 'it''s fine; really' FROM orders",
		"SELECT 1 -- trailing comment with no newline",
	}
	for _, s := range single {
		assert.True(t, singleStatement(s), s)
	}

	multiple := []string{
		"SELECT 1; SELECT 2",
		"SELECT * FROM orders; DROP TABLE orders",
		"DROP TABLE orders; SELECT * FROM orders",
		"SELECT 'unterminated",
		`SELECT "unterminated`,
	}
	for _, s := range multiple {
		assert.False(t, singleStatement(s), s)
	}
}

func TestParseTableRef(t *testing.T) {
	ref, ok := parseTableRef("mydb.main.orders")
	require.True(t, ok)
	assert.Equal(t, tableRef{schema: "main", table: "orders"}, ref)

	_, ok = parseTableRef("orders")
	assert.False(t, ok, "a bare name has no schema to split off")
}

func TestWalkPlanCollectsTablesFromJoin(t *testing.T) {
	var nodes []explainNode
	require.NoError(t, json.Unmarshal([]byte(`[{
		"name": "HASH_JOIN",
		"children": [
			{"name": "SEQ_SCAN", "children": [], "extra_info": {"Table": "db.main.orders"}},
			{"name": "SEQ_SCAN", "children": [], "extra_info": {"Table": "db.main.customers"}}
		],
		"extra_info": {}
	}]`), &nodes))

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for _, n := range nodes {
		walkPlan(&Config{}, "db", n, plan, seen)
	}

	assert.ElementsMatch(t, []string{
		assetMRN(&Config{}, "db", "Table", "main", "orders"),
		assetMRN(&Config{}, "db", "Table", "main", "customers"),
	}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestWalkPlanDeduplicatesRepeatedTables(t *testing.T) {
	var nodes []explainNode
	require.NoError(t, json.Unmarshal([]byte(`[{
		"name": "UNION",
		"children": [
			{"name": "SEQ_SCAN", "children": [], "extra_info": {"Table": "db.main.orders"}},
			{"name": "SEQ_SCAN", "children": [], "extra_info": {"Table": "db.main.orders"}}
		],
		"extra_info": {}
	}]`), &nodes))

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for _, n := range nodes {
		walkPlan(&Config{}, "db", n, plan, seen)
	}

	assert.Equal(t, []string{assetMRN(&Config{}, "db", "Table", "main", "orders")}, plan.ReferencedMRNs)
}

func TestWalkPlanIgnoresCTEScan(t *testing.T) {
	// A materialised CTE's second reference plans as a CTE_SCAN with no Table field; only the CTE node's own definition names a real table.
	var nodes []explainNode
	require.NoError(t, json.Unmarshal([]byte(`[{
		"name": "CTE",
		"children": [
			{"name": "SEQ_SCAN", "children": [], "extra_info": {"Table": "db.main.orders"}},
			{"name": "CTE_SCAN", "children": [], "extra_info": {"CTE Index": "0"}}
		],
		"extra_info": {"CTE Name": "cte"}
	}]`), &nodes))

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for _, n := range nodes {
		walkPlan(&Config{}, "db", n, plan, seen)
	}

	assert.Equal(t, []string{assetMRN(&Config{}, "db", "Table", "main", "orders")}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestWalkPlanKnownGeneratorIsNotIncomplete(t *testing.T) {
	var nodes []explainNode
	require.NoError(t, json.Unmarshal([]byte(`[{"name": "RANGE", "children": [], "extra_info": {"Function": "RANGE"}}]`), &nodes))

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for _, n := range nodes {
		walkPlan(&Config{}, "db", n, plan, seen)
	}

	assert.Empty(t, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestWalkPlanUnknownTableFunctionIsIncomplete(t *testing.T) {
	var nodes []explainNode
	require.NoError(t, json.Unmarshal([]byte(`[{"name": "DUCKDB_TABLES", "children": [], "extra_info": {"Function": "DUCKDB_TABLES"}}]`), &nodes))

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for _, n := range nodes {
		walkPlan(&Config{}, "db", n, plan, seen)
	}

	assert.Empty(t, plan.ReferencedMRNs)
	assert.True(t, plan.Incomplete, "the host then requires a grant on the whole source")
}

// Two sources served by one process must each keep their own connection; asking for one never closes the other.
func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	// sql.Open connects lazily, so these never touch the database.
	open := func() *cachedConn {
		db, err := sql.Open("duckdb", ":memory:")
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

	q.conns["b"].lastUsed = time.Now().Add(-connIdleTTL - time.Second)
	assert.Nil(t, q.get("b"), "an idle connection is closed and forgotten")
}

// setupTestDatabase creates a DuckDB file with tables, a view and some rows across two schemas, read-write, then closes it so the query and discovery paths below can open it read-only.
func setupTestDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.duckdb")

	db, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE orders(id INTEGER, amount DOUBLE);
		INSERT INTO orders VALUES (1, 10.5), (2, 20.25), (3, 30.0);

		CREATE SCHEMA analytics;
		CREATE TABLE analytics.events(id INTEGER, name VARCHAR);
		INSERT INTO analytics.events VALUES (1, 'signup'), (2, 'purchase');
		CREATE VIEW analytics.events_view AS SELECT * FROM analytics.events;
	`)
	require.NoError(t, err)
	return path
}

func TestIntegration_PlanQueryMRNsMatchDiscovery(t *testing.T) {
	path := setupTestDatabase(t)
	ctx := context.Background()

	discovered := &Source{}
	result, err := discovered.Discover(ctx, pluginsdk.RawConfig{"path": path})
	require.NoError(t, err)

	mrnByTable := map[string]string{}
	for _, a := range result.Assets {
		schema, _ := a.Metadata["schema"].(string)
		table, _ := a.Metadata["table_name"].(string)
		mrnByTable[schema+"."+table] = *a.MRN
	}
	require.Contains(t, mrnByTable, "main.orders")
	require.Contains(t, mrnByTable, "analytics.events")
	require.Contains(t, mrnByTable, "analytics.events_view")

	// A fresh Source, as the host would use for a query call unrelated to any previous Discover, must still plan to the identical MRNs.
	queried := &Source{}
	plan, err := queried.PlanQuery(ctx, pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT * FROM orders",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{mrnByTable["main.orders"]}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)

	// DuckDB has no materialised views: querying the view plans a scan of its base table, not the view itself.
	plan, err = queried.PlanQuery(ctx, pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT * FROM analytics.events_view",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{mrnByTable["analytics.events"]}, plan.ReferencedMRNs)

	plan, err = queried.PlanQuery(ctx, pluginsdk.RawConfig{"path": path}, pluginsdk.QueryRequest{
		Statement: "SELECT o.id, e.name FROM orders o JOIN analytics.events e ON o.id = e.id",
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{mrnByTable["main.orders"], mrnByTable["analytics.events"]}, plan.ReferencedMRNs)
}

func TestIntegration_RefusesWritesAndExternalAccess(t *testing.T) {
	path := setupTestDatabase(t)
	ctx := context.Background()
	s := &Source{}
	raw := pluginsdk.RawConfig{"path": path}

	refused := []string{
		"INSERT INTO orders VALUES (4, 40.0)",
		"UPDATE orders SET amount = 0",
		"DELETE FROM orders",
		"CREATE TABLE stolen AS SELECT * FROM orders",
		"ATTACH '" + filepath.Join(t.TempDir(), "evil.duckdb") + "' AS evil",
		"COPY orders TO '" + filepath.Join(t.TempDir(), "out.csv") + "'",
		"INSTALL httpfs",
		"LOAD httpfs",
		"SET memory_limit='1GB'",
		"SELECT * FROM read_csv('/etc/passwd')",
		"SELECT * FROM glob('/etc/*')",
		"SELECT 1; SELECT * FROM orders",
		"DROP TABLE orders; SELECT 1",
	}

	for _, stmt := range refused {
		_, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: stmt})
		assert.Error(t, err, "PlanQuery should refuse: %s", stmt)

		err = s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: stmt}, func(pluginsdk.QueryResultChunk) error {
			return nil
		})
		assert.Error(t, err, "ExecuteQuery should refuse: %s", stmt)
	}
}

func TestIntegration_ExecuteQueryStreamsRowsAndHonoursMaxRows(t *testing.T) {
	path := setupTestDatabase(t)
	ctx := context.Background()
	s := &Source{}
	raw := pluginsdk.RawConfig{"path": path}

	var chunks []pluginsdk.QueryResultChunk
	err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT id, amount FROM orders ORDER BY id"}, func(c pluginsdk.QueryResultChunk) error {
		chunks = append(chunks, c)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	require.Len(t, chunks[0].Columns, 2)
	assert.Equal(t, "id", chunks[0].Columns[0].Name)

	var allRows [][]any
	for _, c := range chunks {
		allRows = append(allRows, c.Rows...)
	}
	require.Len(t, allRows, 3)
	assert.EqualValues(t, 1, allRows[0][0])

	chunks = nil
	err = s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT id FROM orders ORDER BY id", MaxRows: 1}, func(c pluginsdk.QueryResultChunk) error {
		chunks = append(chunks, c)
		return nil
	})
	require.NoError(t, err)

	var total int
	for _, c := range chunks {
		total += len(c.Rows)
	}
	assert.Equal(t, 1, total, "MaxRows must cap the rows returned")
}

func TestIntegration_ExecuteQueryEmitsSchemaForZeroRows(t *testing.T) {
	path := setupTestDatabase(t)
	ctx := context.Background()
	s := &Source{}
	raw := pluginsdk.RawConfig{"path": path}

	var chunks []pluginsdk.QueryResultChunk
	err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT id FROM orders WHERE id = -1"}, func(c pluginsdk.QueryResultChunk) error {
		chunks = append(chunks, c)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Columns, 1)
	assert.Empty(t, chunks[0].Rows)
}
