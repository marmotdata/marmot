package cockroachdb_test

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_Query_MetaDeclaresSupport(t *testing.T) {
	e2e(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)
	assert.True(t, meta.SupportsQuery, "Source implements Querier")
}

func TestE2E_Query_PlanMatchesDiscoveryMRNs(t *testing.T) {
	env := e2e(t)

	plan, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT c.name, o.total FROM public.customers c JOIN public.orders o ON o.customer_id = c.id",
	})
	require.NoError(t, err)
	assert.False(t, plan.Incomplete)
	assert.ElementsMatch(t, []string{customersMRN, ordersMRN}, plan.ReferencedMRNs)
}

func TestE2E_Query_ViewPlansToItsBaseTables(t *testing.T) {
	env := e2e(t)

	plan, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT * FROM public.customer_totals",
	})
	require.NoError(t, err)
	assert.False(t, plan.Incomplete)
	// The optimiser expands a plain view into its base tables; the view's
	// own MRN never names anything the plan actually scans.
	assert.ElementsMatch(t, []string{customersMRN, ordersMRN}, plan.ReferencedMRNs)
	assert.NotContains(t, plan.ReferencedMRNs, customerTotalsMRN)
}

func TestE2E_Query_StreamsRowsWithColumnTypes(t *testing.T) {
	env := e2e(t)

	_, chunks, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT id, name FROM public.customers ORDER BY id",
	})
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	require.Len(t, chunks[0].Columns, 2)
	assert.Equal(t, "id", chunks[0].Columns[0].Name)
	assert.Equal(t, "int8", chunks[0].Columns[0].Type)
	assert.Equal(t, "name", chunks[0].Columns[1].Name)
	assert.Equal(t, "text", chunks[0].Columns[1].Type)

	var rows [][]interface{}
	for _, c := range chunks {
		rows = append(rows, c.Rows...)
	}
	require.Len(t, rows, 3)
	assert.Equal(t, "alice", rows[0][1])
}

func TestE2E_Query_MaxRowsIsHonoured(t *testing.T) {
	env := e2e(t)

	_, chunks, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT * FROM public.orders",
		MaxRows:   1,
	})
	require.NoError(t, err)

	var rows [][]interface{}
	for _, c := range chunks {
		rows = append(rows, c.Rows...)
	}
	assert.Len(t, rows, 1)
}

func TestE2E_Query_EmptyResultStillCarriesColumns(t *testing.T) {
	env := e2e(t)

	_, chunks, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT id FROM public.customers WHERE false",
	})
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	assert.Equal(t, "id", chunks[0].Columns[0].Name)
	for _, c := range chunks {
		assert.Empty(t, c.Rows)
	}
}

func TestE2E_Query_AmbiguousTableNameMarksIncomplete(t *testing.T) {
	env := e2e(t)

	// "widgets" names a table in both public and sales; EXPLAIN (VERBOSE)
	// never qualifies the scanned table with its schema, so the plugin
	// cannot place it in exactly one.
	plan, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT a.id, b.region FROM public.widgets a JOIN sales.widgets b ON a.id = b.id",
	})
	require.NoError(t, err)
	assert.True(t, plan.Incomplete)
}

func TestE2E_Query_OpaqueFunctionCallMarksIncomplete(t *testing.T) {
	env := e2e(t)

	plan, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT public.order_count(1)",
	})
	require.NoError(t, err)
	assert.True(t, plan.Incomplete, "order_count reads orders itself, which the plan never shows")
}

func TestE2E_Query_RefusesInsert(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "INSERT INTO public.customers (id, name) VALUES (9001, 'intruder')",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesUpdate(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "UPDATE public.customers SET name = 'changed' WHERE id = 1",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesUpsert(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "UPSERT INTO public.customers (id, name) VALUES (1, 'changed')",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesDelete(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "DELETE FROM public.customers WHERE id = 1",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesDDL(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "CREATE TABLE public.intrusion (id INT)",
	})
	require.Error(t, err)

	// autocommit_before_ddl defaults to on and would otherwise let this
	// through despite the error above: confirm the table was never created.
	_, chunks, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'intrusion'",
	})
	require.NoError(t, err)
	for _, c := range chunks {
		assert.Empty(t, c.Rows, "CREATE TABLE must not have auto-committed outside the read-only transaction")
	}
}

func TestE2E_Query_RefusesMultipleStatements(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT 1; SELECT 2",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesDataModifyingCTE(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "WITH d AS (DELETE FROM public.customers WHERE id = 1 RETURNING id) SELECT * FROM d",
	})
	require.Error(t, err)
}

func TestE2E_Query_RefusesSquareBracketWrite(t *testing.T) {
	env := e2e(t)

	_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{
		Statement: "SELECT * FROM [INSERT INTO public.customers (id, name) VALUES (9010, 'bracket') RETURNING id]",
	})
	require.Error(t, err)
}

// On CockroachDB, a plain SET or set_config(..., is_local=false) can survive
// a ROLLBACK, unlike on PostgreSQL. A statement calling one would otherwise
// pass PlanQuery and ExecuteQuery's read check (it reads via EXPLAIN just
// fine) while actually mutating session or cluster state, so each of these
// must be refused outright rather than merely marked incomplete.
func TestE2E_Query_RefusesSessionMutatingBuiltins(t *testing.T) {
	env := e2e(t)

	for _, statement := range []string{
		"SELECT set_config('autocommit_before_ddl', 'true', false)",
		"SELECT pg_catalog.set_config('search_path', 'sales', false)",
		"SELECT setval('public.leak_seq', 1000)",
		"SELECT setseed(0.5)",
		"SELECT crdb_internal.set_vmodule('sql=2')",
		"SELECT crdb_internal.reset_sql_stats()",
	} {
		_, _, err := buildBinary(t).Query(t.Context(), env.queryConfig(), pluginsdk.QueryRequest{Statement: statement})
		assert.Error(t, err, statement)
	}
}
