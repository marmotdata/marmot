package postgresql

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpaqueCalls(t *testing.T) {
	for expr, want := range map[string][]string{
		`query_to_xml('select * from payroll'::text, true, false, ''::text)`: {"query_to_xml"},
		`table_to_xml('payroll'::regclass, true, false, ''::text)`:           {"table_to_xml"},
		`public.read_payroll(o.id)`:                                          {"public.read_payroll"},
		`"Sneaky"(o.id)`:                                                     {`"Sneaky"`},
		`pg_catalog.lower((o.name)::text)`:                                   nil,
		`lower((o.name)::text)`:                                              nil,
		`COALESCE(o.total, '0'::numeric)`:                                    nil,
		`count(*)`:                                                           nil,
		`'x'::character varying(10)`:                                         nil,
		`(o.id = ANY ('{1,2}'::integer[]))`:                                  nil,
		`'query_to_xml(''x'')'::text`:                                        nil,
		`E'it\'s query_to_xml(x)'::text`:                                     nil,
		`'unterminated`:                                                      {"<unreadable expression>"},
	} {
		assert.Equal(t, want, opaqueCalls(expr), expr)
	}
}

// Plans come from a real PostgreSQL so the check is exercised against how PostgreSQL actually deparses each attack. Without MARMOT_TEST_PG_QUERY_DSN the test is skipped.
func TestParsePlanFindsCallsThePlanCannotSee(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_PG_QUERY_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_PG_QUERY_DSN to plan against a real PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })

	_, err = conn.Exec(ctx, `
		DROP SCHEMA IF EXISTS plantest CASCADE;
		CREATE SCHEMA plantest;
		CREATE TABLE plantest.orders (id int, name text, total numeric);
		CREATE TABLE plantest.payroll (id int, salary numeric);
		CREATE FUNCTION plantest.read_payroll(int) RETURNS numeric LANGUAGE sql AS 'SELECT salary FROM plantest.payroll WHERE id = $1';
		CREATE FUNCTION plantest.payroll_rows() RETURNS SETOF plantest.payroll LANGUAGE sql AS 'SELECT * FROM plantest.payroll';
		CREATE VIEW plantest.leaky AS SELECT o.id, query_to_xml('select * from plantest.payroll', true, false, '') AS x FROM plantest.orders o;
		CREATE VIEW plantest.plain AS SELECT id, lower(name) AS name FROM plantest.orders;`)
	require.NoError(t, err)

	plan := func(statement string) *planSummary {
		t.Helper()
		var planJSON []byte
		require.NoError(t, conn.QueryRow(ctx, "EXPLAIN (FORMAT JSON, VERBOSE) "+statement).Scan(&planJSON))
		summary, err := parsePlan(planJSON)
		require.NoError(t, err)
		return summary
	}

	for _, statement := range []string{
		`SELECT o.id, query_to_xml('select * from plantest.payroll', true, false, '') FROM plantest.orders o LIMIT 1`,
		`SELECT table_to_xml('plantest.payroll'::regclass, true, false, '') FROM plantest.orders LIMIT 1`,
		`SELECT id FROM plantest.orders WHERE length(query_to_xml('select * from plantest.payroll', true, false, '')::text) > 0`,
		`SELECT plantest.read_payroll(id) FROM plantest.orders`,
		`SELECT * FROM plantest.orders o, plantest.payroll_rows() p`,
		`WITH x AS (SELECT query_to_xml('select * from plantest.payroll', true, false, '') AS v) SELECT o.id, x.v FROM plantest.orders o, x`,
		`SELECT id, (SELECT cursor_to_xml('c', 1, false, false, '')) FROM plantest.orders`,
		`SELECT * FROM plantest.leaky`,
	} {
		assert.NotEmpty(t, plan(statement).opaqueCalls, "must be marked incomplete: %s", statement)
	}

	for _, statement := range []string{
		`SELECT id, lower(name), coalesce(total, 0) FROM plantest.orders`,
		`SELECT count(*), sum(total), date_trunc('day', now()) FROM plantest.orders GROUP BY 3`,
		`SELECT id, row_number() OVER (ORDER BY total DESC) FROM plantest.orders WHERE name = 'query_to_xml(x)'`,
		`SELECT name::varchar(10), total::numeric(10,2) FROM plantest.orders WHERE id = ANY ('{1,2}')`,
		`SELECT * FROM plantest.plain`,
		`SELECT jsonb_build_object('id', id) FROM plantest.orders, generate_series(1, 3)`,
	} {
		summary := plan(statement)
		assert.Empty(t, summary.opaqueCalls, "a plain read stays coverable by table grants: %s", statement)
		assert.NotEmpty(t, summary.relations, statement)
	}
}

func TestPlanQueryMarksOpaqueCallsIncomplete(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_PG_QUERY_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_PG_QUERY_DSN to plan against a real PostgreSQL")
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	raw := map[string]any{
		"host": cfg.Host, "port": int(cfg.Port), "user": cfg.User, "password": cfg.Password,
		"query_database": cfg.Database, "ssl_mode": "disable",
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.plan_orders (id int); CREATE TABLE IF NOT EXISTS public.plan_payroll (salary int)`)
	require.NoError(t, err)

	s := &Source{}
	leaky, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT query_to_xml('select * from public.plan_payroll', true, false, '') FROM public.plan_orders`})
	require.NoError(t, err)
	assert.True(t, leaky.Incomplete)
	assert.Equal(t, []string{"mrn://table/postgresql/plan_orders"}, leaky.ReferencedMRNs)

	plain, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT count(*) FROM public.plan_orders`})
	require.NoError(t, err)
	assert.False(t, plain.Incomplete)
}
