package cockroachdb

import (
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file exercises the query pool directly against a real CockroachDB,
// the same one the package's e2e_test.go uses:
//
//	docker run -d --name qtest-cockroach -p 55520:26257 \
//	  cockroachdb/cockroach:latest-v25.2 start-single-node --insecure
//	MARMOT_TEST_COCKROACHDB_HOST=localhost MARMOT_TEST_COCKROACHDB_PORT=55520 go test ./...
//
// It lives in this package rather than cockroachdb_test because it needs the
// pool itself, something plugintest's black-box harness cannot give: every
// call there spawns a fresh plugin process with its own fresh pool, so it
// can never show whether one call's session change reaches another's.

func queryPoolEnv(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_COCKROACHDB_HOST")
	if host == "" {
		t.Skip("set MARMOT_TEST_COCKROACHDB_HOST (and MARMOT_TEST_COCKROACHDB_PORT) to run the CockroachDB e2e tests")
	}
	port := 26257
	if raw := os.Getenv("MARMOT_TEST_COCKROACHDB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_COCKROACHDB_PORT must be a number")
		port = parsed
	}
	return pluginsdk.RawConfig{"host": host, "port": port, "user": "root", "query_database": "defaultdb"}
}

// TestQueryPool_ScrubsSessionStateBetweenCallers proves the pool's
// AfterRelease hook, not just refuseWrite, is what actually stops a session
// change from reaching the next caller. It bypasses refuseWrite entirely
// and calls set_config straight over the pool, the way a builtin refuseWrite
// does not yet know about would: on CockroachDB, unlike PostgreSQL, a plain
// SET or set_config(..., is_local=false) is not undone when the transaction
// that ran it is rolled back, so without AfterRelease a later caller reusing
// the same pooled connection would inherit the change.
func TestQueryPool_ScrubsSessionStateBetweenCallers(t *testing.T) {
	raw := queryPoolEnv(t)
	config, err := parseConfig(raw)
	require.NoError(t, err)

	poolConfig, err := pgxpool.ParseConfig(connString(config, config.QueryDatabase))
	require.NoError(t, err)
	// Capped at one connection, so caller B is forced to reuse exactly the
	// connection caller A mutated rather than a fresh one, which would pass
	// the test for the wrong reason.
	poolConfig.MaxConns = 1
	poolConfig.ConnConfig.RuntimeParams["autocommit_before_ddl"] = "false"
	poolConfig.AfterRelease = resetSessionAfterRelease

	pool, err := pgxpool.NewWithConfig(t.Context(), poolConfig)
	require.NoError(t, err)
	defer pool.Close()

	// Caller A: the same read-only, always-rolled-back shape every call in
	// this plugin runs a statement under, but calling set_config directly
	// rather than through PlanQuery or ExecuteQuery, so refuseWrite never
	// sees it.
	err = readOnly(t.Context(), pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), "SELECT set_config('autocommit_before_ddl', 'true', false)", pgx.QueryExecModeExec)
		return err
	})
	require.NoError(t, err, "set_config is not itself refused by CockroachDB's read-only transaction")

	// Caller B: an unrelated later call, necessarily on the same connection
	// since MaxConns is 1, which NewConnsCount confirms below.
	err = readOnly(t.Context(), pool, func(tx pgx.Tx) error {
		var value string
		if err := tx.QueryRow(t.Context(), "SHOW autocommit_before_ddl").Scan(&value); err != nil {
			return err
		}
		assert.Equal(t, "off", value, "a later caller must not see the previous caller's session change")

		_, ddlErr := tx.Exec(t.Context(), "CREATE TABLE defaultdb.public.query_pool_leak_check (id INT)", pgx.QueryExecModeExec)
		assert.Error(t, ddlErr, "DDL must still be refused for the next caller on the same connection")
		return nil
	})
	require.NoError(t, err)

	assert.EqualValues(t, 1, pool.Stat().NewConnsCount(), "caller B must have reused caller A's connection, not opened a fresh one")
}

// TestQueryPool_RefuseWriteCatchesTheSameBuiltinUpFront is the companion to
// the pool-level test above: refuseWrite is the first, cheaper line of
// defence, and PlanQuery and ExecuteQuery both reach it before a statement
// could ever touch the pool.
func TestQueryPool_RefuseWriteCatchesTheSameBuiltinUpFront(t *testing.T) {
	require.ErrorIs(t, refuseWrite("SELECT set_config('autocommit_before_ddl', 'true', false)"), errReadOnly)
}
