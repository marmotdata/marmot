package cockroachdb

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadStatement(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from shop.public.orders",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"-- the latest orders\nSELECT * FROM orders",
		"/* note */ VALUES (1)",
		"TABLE shop.public.orders",
	}
	for _, s := range reads {
		assert.True(t, isReadStatement(s), s)
	}

	writes := []string{
		"INSERT INTO shop.public.customers VALUES (1, 'x')",
		"UPSERT INTO shop.public.customers VALUES (1, 'x')",
		"UPDATE shop.public.customers SET name = 'x'",
		"DELETE FROM shop.public.orders",
		"TRUNCATE shop.public.orders",
		"CREATE TABLE shop.public.stolen AS SELECT 1",
		"DROP TABLE shop.public.orders",
		"ALTER TABLE shop.public.orders ADD COLUMN x INT",
		"-- SELECT\nINSERT INTO t VALUES (1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, s := range writes {
		assert.False(t, isReadStatement(s), s)
	}
}

func TestCallsMutatingFunction(t *testing.T) {
	calls := []string{
		"SELECT set_config('autocommit_before_ddl', 'true', false)",
		"SELECT pg_catalog.set_config('search_path', 'other', false)",
		"select c.id, set_config('search_path', 'other', false) from shop.public.customers c",
		"SELECT setval('shop.public.orders_id_seq', 1000)",
		"SELECT setseed(0.5)",
		"SELECT crdb_internal.set_vmodule('sql=2')",
		"SELECT crdb_internal.reset_sql_stats()",
		"SELECT crdb_internal.set_trace_verbose(0, true)",
		"SELECT crdb_internal.reset_activity_tables()",
		"SELECT crdb_internal.reset_index_usage_stats()",
		"SELECT crdb_internal.reset_insights_tables()",
		"SELECT crdb_internal.reset_multi_region_zone_configs_for_database(1)",
		"SELECT crdb_internal.reset_multi_region_zone_configs_for_table(1)",
		"SELECT 1 /* hidden */ , set_config('x', 'y', false)",
		"-- SET_CONFIG\nSELECT set_config('x', 'y', false)",
		"SELECT 1 WHERE 'unterminated",
		"SELECT 1 /* unterminated",
	}
	for _, s := range calls {
		assert.True(t, callsMutatingFunction(s), s)
	}

	reads := []string{
		"SELECT * FROM shop.public.orders",
		"SELECT lower(name) FROM shop.public.customers",
		"SELECT current_setting('autocommit_before_ddl')",
		// The dangerous call only appears inside a comment or a string
		// literal, so it is never actually invoked.
		"SELECT 1 -- set_config('x', 'y', false)\nFROM shop.public.orders",
		"SELECT 1 /* set_config('x', 'y', false) */ FROM shop.public.orders",
		"SELECT note FROM shop.public.orders WHERE note = 'call set_config(x) if you dare'",
		"SELECT 'it''s set_config(x) in a literal' FROM shop.public.orders",
	}
	for _, s := range reads {
		assert.False(t, callsMutatingFunction(s), s)
	}
}

func TestRefuseWrite(t *testing.T) {
	require.NoError(t, refuseWrite("SELECT * FROM shop.public.orders"))

	require.ErrorIs(t, refuseWrite("INSERT INTO shop.public.orders VALUES (1)"), errReadOnly)
	require.ErrorIs(t, refuseWrite("SELECT set_config('autocommit_before_ddl', 'true', false)"), errReadOnly)
}

func TestParsePlan_CollectsRelations(t *testing.T) {
	summary, err := parsePlan(`distribution: local
vectorized: true

• project
│ columns: (id, total)
│
└── • hash join (inner)
    │ columns: (customer_id, total, id)
    │ estimated row count: 990 (missing stats)
    │ equality: (customer_id) = (id)
    │ right cols are key
    │
    ├── • scan
    │     columns: (customer_id, total)
    │     estimated row count: 1,000 (missing stats)
    │     table: orders@orders_pkey
    │     spans: FULL SCAN
    │
    └── • scan
          columns: (id)
          estimated row count: 1,000 (missing stats)
          table: customers@customers_pkey
          spans: FULL SCAN

index recommendations: 1
1. type: index creation
   SQL command: CREATE INDEX ON shop.public.orders (customer_id) STORING (total);`)
	require.NoError(t, err)
	assert.Equal(t, []string{"orders", "customers"}, summary.relations)
	assert.Empty(t, summary.opaqueCalls, "the recommended index's SQL text must not be read as a function call")
}

func TestParsePlan_DeduplicatesRepeatedScans(t *testing.T) {
	summary, err := parsePlan(`• lookup join (inner)
│ table: customers@customers_pkey
│
└── • scan
      table: customers@customers_pkey`)
	require.NoError(t, err)
	assert.Equal(t, []string{"customers"}, summary.relations)
}

func TestParsePlan_WithNoRelations(t *testing.T) {
	summary, err := parsePlan("• values\n  size: 1 column, 1 row\n  row 0, expr 0: 1")
	require.NoError(t, err)
	assert.Empty(t, summary.relations, "the host then falls back to a whole-source grant")
}

func TestParsePlan_RefusesTopLevelMutationNode(t *testing.T) {
	// isReadStatement keeps a bare INSERT away from this, but the plan walk
	// refuses it too in case that screen is ever bypassed.
	_, err := parsePlan("• insert fast path\n  into: customers(id, name)")
	require.ErrorIs(t, err, errReadOnly)
}

func TestParsePlan_RefusesWriteNestedInASelect(t *testing.T) {
	// "SELECT * FROM [INSERT INTO t ... RETURNING ...]" and a writable CTE
	// both plan as a read at the top with the write nested underneath.
	_, err := parsePlan(`• root
│ columns: (id)
│
├── • scan buffer
│     columns: (id)
│
└── • subquery
    │ original sql: INSERT INTO public.customers(id, name) VALUES (1, 'x') RETURNING id
    │
    └── • buffer
        │
        └── • insert
            │ into: customers(id, name)
            │
            └── • values
                  row 0, expr 0: 1`)
	require.ErrorIs(t, err, errReadOnly)

	_, err = parsePlan("• delete range\n  from: orders\n  spans: FULL SCAN")
	require.ErrorIs(t, err, errReadOnly)

	_, err = parsePlan("• upsert\n  into: customers(id, name)")
	require.ErrorIs(t, err, errReadOnly)
}

func TestParsePlan_DetectsOpaqueFunctionCalls(t *testing.T) {
	summary, err := parsePlan("• values\n  columns: (order_count)\n  row 0, expr 0: order_count(1)")
	require.NoError(t, err)
	assert.Equal(t, []string{"order_count"}, summary.opaqueCalls)
}

func TestParsePlan_SafeFunctionsAreNotOpaque(t *testing.T) {
	summary, err := parsePlan(`• render
│ render lower: lower(name)
│ render id: id
│
└── • filter
      filter: name = 'alice'`)
	require.NoError(t, err)
	assert.Empty(t, summary.opaqueCalls)
}

func TestExpressionValue(t *testing.T) {
	for line, want := range map[string]string{
		"render order_count: order_count(id)":    "order_count(id)",
		"render name_new: 'y'":                   "'y'",
		"filter: name = 'alice'":                 "name = 'alice'",
		"equality: (customer_id) = (id)":         "(customer_id) = (id)",
		"aggregate 0: sum(total)":                "sum(total)",
		"window 0: row_number() OVER (ORDER BY)": "row_number() OVER (ORDER BY)",
		"distinct on: customer_id":               "customer_id",
		"group by: id":                           "id",
		"row 0, expr 0: 9010":                    "9010",
	} {
		got, ok := expressionValue(line)
		assert.True(t, ok, line)
		assert.Equal(t, want, got, line)
	}

	for _, line := range []string{
		"columns: (id, total)",
		"table: orders@orders_pkey",
		"spans: FULL SCAN",
		"estimated row count: 990 (missing stats)",
		"into: customers(id, name)",
		"SQL command: CREATE INDEX ON shop.public.orders (customer_id) STORING (total);",
		"index recommendations: 1",
		"1. type: index creation",
		"distribution: local",
		"auto commit",
		"parallel",
	} {
		_, ok := expressionValue(line)
		assert.False(t, ok, line)
	}
}

func TestParseConfig_DefaultsQueryDatabaseToDefaultdbWhenEveryDatabaseIsDiscovered(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{"host": "localhost", "user": "root"})
	require.NoError(t, err)
	assert.Equal(t, "defaultdb", config.QueryDatabase)
}

func TestParseConfig_DefaultsQueryDatabaseToTheConfiguredDatabase(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{"host": "localhost", "user": "root", "database": "shop"})
	require.NoError(t, err)
	assert.Equal(t, "shop", config.QueryDatabase)
}

func TestParseConfig_KeepsAnExplicitQueryDatabase(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{
		"host": "localhost", "user": "root", "database": "shop", "query_database": "analytics",
	})
	require.NoError(t, err)
	assert.Equal(t, "analytics", config.QueryDatabase)
}

// Two sources served by one process must each keep their own pool; asking for one never closes the other.
func TestQueryPoolsKeepOnePoolPerConfig(t *testing.T) {
	// pgxpool connects lazily, so these pools never touch the network.
	open := func() *pgxpool.Pool {
		pool, err := pgxpool.New(context.Background(), "postgres://u@127.0.0.1:1/db")
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
