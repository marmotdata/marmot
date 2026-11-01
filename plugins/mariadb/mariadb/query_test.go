package mariadb

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SHOW WARNINGS fixtures below are copied from a MariaDB 11.8 server,
// the same convention parse_test.go uses for information_schema fixtures.

func TestClassifyRead_Reads(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from orders",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"-- the latest orders\nSELECT * FROM orders",
		"/* note */ VALUES (1)",
		"SELECT 1;",
		"SELECT 1;  ",
		"SELECT @@version",
		"SELECT @@session.time_zone",
	}
	for _, s := range reads {
		_, err := classifyRead(s)
		assert.NoError(t, err, s)
	}
}

func TestClassifyRead_RefusesWrites(t *testing.T) {
	writes := []string{
		"INSERT INTO orders VALUES (1, 1, 1.0)",
		"UPDATE orders SET total = 0",
		"DELETE FROM orders",
		"REPLACE INTO orders VALUES (1, 1, 1.0)",
		"CREATE TABLE stolen (id int)",
		"DROP TABLE orders",
		"ALTER TABLE orders ADD COLUMN x int",
		"TRUNCATE TABLE orders",
		"CALL some_procedure()",
		"LOAD DATA INFILE 'x' INTO TABLE orders",
		"-- SELECT\nINSERT INTO t VALUES (1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, s := range writes {
		_, err := classifyRead(s)
		assert.ErrorIs(t, err, errReadOnly, s)
	}
}

func TestClassifyRead_RefusesForbiddenModifiers(t *testing.T) {
	forbidden := []string{
		"SELECT * FROM orders FOR UPDATE",
		"SELECT * FROM orders LOCK IN SHARE MODE",
		"SELECT * FROM orders INTO OUTFILE '/tmp/x.csv'",
		"SELECT * FROM orders INTO DUMPFILE '/tmp/x.csv'",
		"select * from orders for update",
	}
	for _, s := range forbidden {
		_, err := classifyRead(s)
		assert.ErrorIs(t, err, errReadOnly, s)
	}
}

func TestClassifyRead_RefusesUserVariableReadsAndWrites(t *testing.T) {
	// A user variable survives a rolled-back transaction for as long as
	// the pooled connection stays open, so both setting and reading one
	// are refused: together they are a side channel between two callers
	// who happen to share a connection.
	variableStatements := []string{
		"SELECT @leak := (SELECT ssn FROM secret LIMIT 1)",
		"SELECT @leak",
		"SELECT @x, id FROM orders",
		"SELECT id FROM orders WHERE id = @x",
		"SELECT id INTO @x FROM orders LIMIT 1",
	}
	for _, s := range variableStatements {
		_, err := classifyRead(s)
		assert.ErrorIs(t, err, errReadOnly, s)
	}
}

func TestClassifyRead_AllowsSystemVariableReads(t *testing.T) {
	// "@@" reads a system variable, session or global, which nothing a
	// single statement run through Marmot can set, so it carries nothing
	// from one caller to the next.
	reads := []string{
		"SELECT @@version",
		"SELECT @@session.time_zone",
		"SELECT @@GLOBAL.max_connections",
	}
	for _, s := range reads {
		_, err := classifyRead(s)
		assert.NoError(t, err, s)
	}
}

func TestClassifyRead_IgnoresModifierTextInsideLiteralsAndIdentifiers(t *testing.T) {
	// "FOR UPDATE" inside a string, a comment or a backtick-quoted column
	// name is not the clause, and must not cause a false refusal. The same
	// goes for a literal '@' inside a string or a quoted identifier: it is
	// not a reference to a user variable.
	reads := []string{
		`SELECT * FROM orders WHERE note = 'pending for update'`,
		"SELECT * FROM orders -- FOR UPDATE\nWHERE id = 1",
		"SELECT `for update` FROM orders",
		`SELECT * FROM orders WHERE email = 'alice@example.com'`,
		"SELECT `@weird` FROM orders",
	}
	for _, s := range reads {
		_, err := classifyRead(s)
		assert.NoError(t, err, s)
	}
}

func TestClassifyRead_RefusesMultipleStatements(t *testing.T) {
	multi := []string{
		"SELECT 1; DROP TABLE orders",
		"SELECT 1; SELECT 2",
	}
	for _, s := range multi {
		_, err := classifyRead(s)
		assert.ErrorIs(t, err, errReadOnly, s)
	}
}

func TestReferencesUserVariable(t *testing.T) {
	assert.True(t, referencesUserVariable("select @leak"))
	assert.True(t, referencesUserVariable("select @leak := 1"))
	assert.True(t, referencesUserVariable("select 1 into @x"))
	assert.False(t, referencesUserVariable("select @@version"))
	assert.False(t, referencesUserVariable("select @@session.time_zone"))
	assert.False(t, referencesUserVariable("select 1"))
}

func TestStripLiteralsAndComments_BlanksLiteralsAndComments(t *testing.T) {
	code, ok := stripLiteralsAndComments("SELECT 'a;b', \"c;d\", `e;f` FROM t -- g;h\n/* i;j */ WHERE x = 1")
	require.True(t, ok)
	assert.NotContains(t, code, "a;b")
	assert.NotContains(t, code, "c;d")
	assert.NotContains(t, code, "e;f")
	assert.NotContains(t, code, "g;h")
	assert.NotContains(t, code, "i;j")
	assert.Contains(t, code, "FROM t")
	assert.Contains(t, code, "WHERE x = 1")
}

func TestStripLiteralsAndComments_HandlesEscapesAndDoubling(t *testing.T) {
	code, ok := stripLiteralsAndComments(`SELECT 'it''s fine', 'back\'slash' FROM t`)
	require.True(t, ok)
	assert.Contains(t, code, "FROM t")
}

func TestStripLiteralsAndComments_FailsClosedOnUnterminated(t *testing.T) {
	for _, s := range []string{
		"SELECT 'unterminated",
		"SELECT /* unterminated",
		"SELECT `unterminated",
		"SELECT 1 -- unterminated",
	} {
		_, ok := stripLiteralsAndComments(s)
		assert.False(t, ok, s)
	}
}

func TestHasExtraStatements(t *testing.T) {
	assert.False(t, hasExtraStatements("SELECT 1"))
	assert.False(t, hasExtraStatements("SELECT 1;"))
	assert.False(t, hasExtraStatements("SELECT 1;  "))
	assert.True(t, hasExtraStatements("SELECT 1; SELECT 2"))
	assert.True(t, hasExtraStatements("SELECT 1; DROP TABLE orders"))
}

func TestCanonicalRelationPattern_FindsPlainTables(t *testing.T) {
	rewritten := "select `shop`.`secret`.`id` AS `id` from `shop`.`secret`"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 1)
	assert.Equal(t, "shop", matches[0][1])
	assert.Equal(t, "secret", matches[0][2])
}

func TestCanonicalRelationPattern_FindsAJoinRegardlessOfTheAliasChosen(t *testing.T) {
	// The alias "public_t" here names a different real table; the pattern
	// must not care, since it never looks at the alias at all.
	rewritten := "select `shop`.`x`.`id` AS `id` from `shop`.`secret` `x` join `shop`.`public_t` `y` where `shop`.`y`.`id` = `shop`.`x`.`id`"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 2)
	assert.Equal(t, [2]string{"shop", "secret"}, [2]string{matches[0][1], matches[0][2]})
	assert.Equal(t, [2]string{"shop", "public_t"}, [2]string{matches[1][1], matches[1][2]})
}

func TestCanonicalRelationPattern_SurvivesAPartitionClauseBetweenTableAndAlias(t *testing.T) {
	rewritten := "select `shop`.`public_t`.`id` AS `id` from `shop`.`secret2` PARTITION (`p0`) `public_t`"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 1)
	assert.Equal(t, "secret2", matches[0][2])
}

func TestCanonicalRelationPattern_SurvivesAnIndexHintAfterTheAlias(t *testing.T) {
	rewritten := "select `shop`.`public_t`.`id` AS `id` from `shop`.`secret` `public_t` USE INDEX (`idx_secret_id`)"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 1)
	assert.Equal(t, "secret", matches[0][2])
}

func TestCanonicalRelationPattern_IgnoresThreePartColumnReferences(t *testing.T) {
	// A column is always three backtick-quoted parts, db.table.column,
	// never the two the pattern requires, so it is never mistaken for a
	// table reference even though it shares the same db.table prefix.
	rewritten := "select `shop`.`secret`.`id` AS `id` from `shop`.`secret`"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 1, "only the FROM clause's reference must match")
}

func TestCanonicalRelationPattern_IgnoresADerivedTablesOwnAlias(t *testing.T) {
	// A derived table that the optimiser keeps (rather than merging away)
	// is never a `db`.`table` pair; its alias, "public_t" here, is not
	// adjacent to one either, so it is never captured as if it named a
	// real table. Its own FROM clause, nested in the parentheses, is found
	// as its own separate match.
	rewritten := "select `public_t`.`customer_id` AS `customer_id` from " +
		"(select `shop`.`orders3`.`customer_id` AS `customer_id` from `shop`.`orders3` group by `shop`.`orders3`.`customer_id`) `public_t`"
	matches := canonicalRelationPattern.FindAllStringSubmatch(rewritten, -1)
	require.Len(t, matches, 1)
	assert.Equal(t, "orders3", matches[0][2])
}

func TestResolveRelations_ResolvesATableInTheQueryDatabase(t *testing.T) {
	objects := map[string]string{"orders": "BASE TABLE", "customers": "BASE TABLE"}
	relations := []relation{{database: "shop", table: "orders"}, {database: "shop", table: "customers"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.False(t, incomplete)
	assert.ElementsMatch(t, []string{"mrn://table/mariadb/orders", "mrn://table/mariadb/customers"}, mrns)
}

func TestResolveRelations_ResolvesViewsAndSequencesToTheirOwnType(t *testing.T) {
	objects := map[string]string{"customer_totals": "VIEW", "order_seq": "SEQUENCE"}
	relations := []relation{{database: "shop", table: "customer_totals"}, {database: "shop", table: "order_seq"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.False(t, incomplete)
	assert.ElementsMatch(t, []string{"mrn://view/mariadb/customer_totals", "mrn://sequence/mariadb/order_seq"}, mrns)
}

func TestResolveRelations_MarksIncompleteForADifferentDatabase(t *testing.T) {
	// This source's catalogue only covers the configured query database;
	// a table in a different one, even a real table, is not something it
	// can vouch for.
	objects := map[string]string{"orders": "BASE TABLE"}
	relations := []relation{{database: "otherdb", table: "orders"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.True(t, incomplete)
	assert.Empty(t, mrns)
}

func TestResolveRelations_MarksIncompleteForAnUncataloguedName(t *testing.T) {
	objects := map[string]string{"orders": "BASE TABLE"}
	relations := []relation{{database: "shop", table: "ghost"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.True(t, incomplete)
	assert.Empty(t, mrns)
}

func TestResolveRelations_MarksIncompleteForAnUnknownObjectType(t *testing.T) {
	objects := map[string]string{"tmp_session": "SYSTEM VIEW"}
	relations := []relation{{database: "shop", table: "tmp_session"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.True(t, incomplete)
	assert.Empty(t, mrns)
}

func TestResolveRelations_DedupesRepeatedTables(t *testing.T) {
	objects := map[string]string{"orders": "BASE TABLE"}
	relations := []relation{{database: "shop", table: "orders"}, {database: "shop", table: "orders"}}

	mrns, incomplete := resolveRelations(relations, "shop", objects)

	assert.False(t, incomplete)
	assert.Equal(t, []string{"mrn://table/mariadb/orders"}, mrns)
}

func TestCallsKnownFunction(t *testing.T) {
	functions := map[string]bool{"get_total": true}

	assert.True(t, callsKnownFunction("SELECT get_total(id) FROM customers", functions))
	assert.True(t, callsKnownFunction("SELECT shop.get_total(id) FROM customers", functions))
	assert.True(t, callsKnownFunction("SELECT GET_TOTAL(id) FROM customers", functions))
	assert.False(t, callsKnownFunction("SELECT id FROM customers", functions))
	assert.False(t, callsKnownFunction("SELECT get_total_other(id) FROM customers", functions))
	assert.False(t, callsKnownFunction("SELECT 1", map[string]bool{}))
}

func TestParseConfig_DefaultsQueryDatabaseToDatabase(t *testing.T) {
	config, err := parseConfig(map[string]any{"host": "db", "user": "marmot", "database": "shop"})
	require.NoError(t, err)
	assert.Equal(t, "shop", config.QueryDatabase)
}

func TestParseConfig_RespectsAnExplicitQueryDatabase(t *testing.T) {
	config, err := parseConfig(map[string]any{
		"host": "db", "user": "marmot", "database": "shop", "query_database": "reporting",
	})
	require.NoError(t, err)
	assert.Equal(t, "reporting", config.QueryDatabase)
}

// Two sources served by one process must each keep their own connection pool; asking for one never closes the other.
func TestQueryConns_KeepsOnePoolPerConfig(t *testing.T) {
	// sql.Open connects lazily, so these never touch the network.
	open := func() *sql.DB {
		db, err := sql.Open("mysql", "marmot:marmot@tcp(127.0.0.1:1)/shop")
		require.NoError(t, err)
		return db
	}
	var q queryConns
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.NotSame(t, a, b)

	duplicate := open()
	assert.Same(t, a, q.put("a", duplicate), "a pool opened concurrently for the same config is discarded")

	q.conns["a"].lastUsed = time.Now().Add(-connIdleTTL - time.Second)
	assert.Nil(t, q.get("a"), "an idle pool is closed and forgotten")
	assert.Same(t, b, q.get("b"))
}

func TestSource_ImplementsQuerier(t *testing.T) {
	var _ pluginsdk.Querier = &Source{}
}

// TestConnect_NeverKeepsASessionAliveForTheNextCaller is the belt-and-braces
// proof behind classifyRead's user variable refusal, not a test of it: it
// bypasses classifyRead entirely, running statements directly over the pool
// s.connect hands out, exactly like PlanQuery and ExecuteQuery do, so a
// future gap in classifyRead would not get credit it has not earned here.
// "Caller A" sets a user variable and "caller B" reads it, both over the
// same cached pool two unrelated callers through this Source would share;
// MaxIdleConns(0) means every borrow dials a fresh connection, so MariaDB's
// session-scoped state can never cross from one borrow to the next. Set
// MARMOT_TEST_MARIADB_DSN to run it.
func TestConnect_NeverKeepsASessionAliveForTheNextCaller(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_MARIADB_DSN")
	if dsn == "" {
		t.Skip("MARMOT_TEST_MARIADB_DSN is not set, skipping MariaDB session isolation test")
	}
	cfg, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)
	host, portStr, err := net.SplitHostPort(cfg.Addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	raw := pluginsdk.RawConfig{
		"host": host, "port": port, "user": cfg.User, "password": cfg.Passwd, "database": cfg.DBName,
	}

	s := &Source{}
	ctx := context.Background()
	db, _, err := s.connect(ctx, raw)
	require.NoError(t, err)

	// Caller A: assign a value to a user variable, then let its connection
	// go back to the pool.
	var assigned sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, "SELECT @leak := 'caller-a-secret'").Scan(&assigned))
	require.True(t, assigned.Valid)
	require.Equal(t, "caller-a-secret", assigned.String, "sanity check: the assignment itself must have worked")

	// Caller B: a second, unrelated borrow of the same cached pool reads
	// the same variable name back.
	var leaked sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, "SELECT @leak").Scan(&leaked))
	assert.Falsef(t, leaked.Valid, "caller B must never see caller A's session state, got %q", leaked.String)
}
