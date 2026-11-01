package trino

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIOPlanReadsInputs(t *testing.T) {
	plan, err := parseIOPlan(`{
		"inputTableColumnInfos": [
			{"table": {"catalog": "postgresql", "schemaTable": {"schema": "public", "table": "orders"}}}
		],
		"estimate": {"outputSizeInBytes": 1024}
	}`)
	require.NoError(t, err)
	require.Len(t, plan.InputTableColumnInfos, 1)
	assert.Equal(t, "orders", plan.InputTableColumnInfos[0].Table.SchemaTable.Table)
	assert.Equal(t, int64(1024), plan.estimatedBytes())
}

func TestParseIOPlanToleratesNaNEstimates(t *testing.T) {
	plan, err := parseIOPlan(`{"inputTableColumnInfos": [], "estimate": {"outputSizeInBytes": "NaN"}}`)
	require.NoError(t, err)
	assert.Zero(t, plan.estimatedBytes())
}

func TestParseIOPlanRefusesStatementsWithoutAPlan(t *testing.T) {
	// EXPLAIN of DDL comes back as the statement text.
	_, err := parseIOPlan("DROP TABLE postgresql.public.salaries")
	require.ErrorIs(t, err, errReadOnly)
}

func TestParseIOPlanRefusesWrites(t *testing.T) {
	_, err := parseIOPlan(`{
		"inputTableColumnInfos": [],
		"outputTable": {"catalog": "postgresql", "schemaTable": {"schema": "public", "table": "orders"}}
	}`)
	require.ErrorIs(t, err, errReadOnly)
}

func TestParseIOPlanAllowsExplicitNullOutput(t *testing.T) {
	_, err := parseIOPlan(`{"inputTableColumnInfos": [], "outputTable": null}`)
	require.NoError(t, err)
}

func TestIsReadStatement(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from postgresql.public.orders",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"-- the latest orders\nSELECT * FROM orders",
		"/* note */ VALUES (1)",
		"TABLE postgresql.public.orders",
	}
	for _, s := range reads {
		assert.True(t, isReadStatement(s), s)
	}

	writes := []string{
		"INSERT INTO postgresql.public.salaries VALUES ('x', 1)",
		"CREATE TABLE postgresql.public.stolen AS SELECT 1",
		"DELETE FROM orders",
		"MERGE INTO orders USING x ON true WHEN MATCHED THEN DELETE",
		"DROP TABLE orders",
		"CALL system.flush_metadata_cache()",
		"-- SELECT\nINSERT INTO t VALUES (1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, s := range writes {
		assert.False(t, isReadStatement(s), s)
	}
}

func TestCallsTableFunction(t *testing.T) {
	for statement, want := range map[string]bool{
		`SELECT * FROM TABLE(pg.system.query(query => 'select * from payroll'))`:               true,
		`select o.id, q.* from orders o, table (pg.system.query(query => 'select 1')) q`:       true,
		"SELECT * FROM orders /* hidden */ CROSS JOIN TABLE(\n pg.system.query(query => 'x'))": true,
		`SELECT * FROM orders WHERE note = 'TABLE(x)'`:                                         false,
		"SELECT * FROM orders -- TABLE(x)\nWHERE id = 1":                                       false,
		`TABLE orders`: false,
		`SELECT table_name FROM information_schema.tables`: false,
		`SELECT * FROM orders WHERE note = 'unterminated`:  true,
		`SELECT * FROM orders /* unterminated`:             true,
	} {
		assert.Equal(t, want, callsTableFunction(statement), statement)
	}
}

// Two sources served by one process must each keep their own connection; asking for one never closes the other.
func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	// sql.Open connects lazily, so these never touch the network.
	open := func() *cachedConn {
		db, err := sql.Open("trino", "http://user@127.0.0.1:1")
		require.NoError(t, err)
		return &cachedConn{db: db}
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
