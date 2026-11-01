package postgresql

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlanCollectsRelations(t *testing.T) {
	relations, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Hash Join",
		"Plans": [
			{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"},
			{"Node Type": "Hash", "Plans": [
				{"Node Type": "Index Scan", "Relation Name": "customers", "Schema": "public"},
				{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"}
			]}
		]
	}}]`))
	require.NoError(t, err)
	assert.Equal(t, []relation{{schema: "public", name: "orders"}, {schema: "public", name: "customers"}}, relations.relations)
	assert.Empty(t, relations.opaqueCalls)
}

func TestParsePlanRefusesWrites(t *testing.T) {
	// A data-modifying CTE inside a SELECT still plans a ModifyTable node.
	_, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "CTE Scan",
		"Plans": [{"Node Type": "ModifyTable", "Operation": "Delete", "Relation Name": "orders", "Schema": "public"}]
	}}]`))
	require.ErrorIs(t, err, errReadOnly)

	_, err = parsePlan([]byte(`[{"Plan": {"Node Type": "LockRows", "Plans": [{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"}]}}]`))
	require.ErrorIs(t, err, errReadOnly)
}

func TestParsePlanWithNoRelations(t *testing.T) {
	relations, err := parsePlan([]byte(`[{"Plan": {"Node Type": "Result"}}]`))
	require.NoError(t, err)
	assert.Empty(t, relations.relations, "the host then falls back to a whole-source grant")
}

func TestParseConfigDefaultsQueryDatabase(t *testing.T) {
	config, err := parseConfig(map[string]any{"host": "db", "user": "marmot"})
	require.NoError(t, err)
	assert.Equal(t, "postgres", config.QueryDatabase)
	assert.Equal(t, 5432, config.Port)
}

func TestParseConfigDefaultsTrueOptions(t *testing.T) {
	config, err := parseConfig(map[string]any{"host": "db", "user": "marmot"})
	require.NoError(t, err)
	assert.True(t, config.ExcludeSystemSchemas, "system schemas are left out unless asked for")
	assert.True(t, config.IncludeColumns)

	config, err = parseConfig(map[string]any{"host": "db", "user": "marmot", "exclude_system_schemas": false})
	require.NoError(t, err)
	assert.False(t, config.ExcludeSystemSchemas, "an explicit false is kept")
}

// Two sources served by one process must each keep their own pool; asking for one never closes the other.
func TestQueryPoolsKeepOnePoolPerConfig(t *testing.T) {
	// pgxpool connects lazily, so these pools never touch the network.
	open := func() *pgxpool.Pool {
		pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/db")
		require.NoError(t, err)
		return pool
	}
	var q queryPools
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.NotSame(t, a, b)

	duplicate := open()
	assert.Same(t, a, q.put("a", duplicate), "a pool opened concurrently for the same config is discarded")

	q.pools["a"].lastUsed = time.Now().Add(-poolIdleTTL - time.Second)
	assert.Nil(t, q.get("a"), "an idle pool is closed and forgotten")
	assert.Same(t, b, q.get("b"))
}
