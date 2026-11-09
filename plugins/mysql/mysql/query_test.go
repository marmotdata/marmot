package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyStatement(t *testing.T) {
	reads := map[string]statementKind{
		"SELECT 1":                                   readSelect,
		"  select * from orders":                     readSelect,
		"WITH t AS (SELECT 1) SELECT * FROM t":       readSelect,
		"(SELECT 1) UNION (SELECT 2)":                readSelect,
		"-- the latest orders\nSELECT * FROM orders": readSelect,
		"# a trailing comment style\nSELECT 1":       readSelect,
		"/* note */ VALUES ROW(1)":                   readSelect,
		"TABLE orders":                               readSelect,
		"SHOW TABLES":                                readShow,
		"DESC orders":                                readShow,
		"DESCRIBE orders":                            readShow,
	}
	for statement, want := range reads {
		assert.Equal(t, want, classifyStatement(statement), statement)
	}

	writes := []string{
		"INSERT INTO orders VALUES (1)",
		"UPDATE orders SET amount = 1",
		"DELETE FROM orders",
		"DROP TABLE orders",
		"CALL some_procedure()",
		"-- SELECT\nINSERT INTO t VALUES (1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, statement := range writes {
		assert.Equal(t, notARead, classifyStatement(statement), statement)
	}
}

// A WITH clause can precede UPDATE or DELETE in MySQL, not just SELECT, so
// classifyStatement alone must not be trusted to prove a WITH-prefixed
// statement only reads; it is the READ ONLY transaction that actually
// refuses the write, and this documents that gap rather than hiding it.
func TestClassifyStatement_WithCanPrecedeAWrite(t *testing.T) {
	assert.Equal(t, readSelect, classifyStatement("WITH t AS (SELECT id FROM orders) UPDATE customers SET name = 'x'"))
}

func TestHasRefusedClause(t *testing.T) {
	refused := []string{
		"SELECT * FROM orders INTO OUTFILE '/tmp/x.csv'",
		"SELECT * FROM orders INTO DUMPFILE '/tmp/x.csv'",
		"SELECT * FROM orders FOR UPDATE",
		"SELECT * FROM orders LOCK IN SHARE MODE",
		"SELECT * FROM orders FOR SHARE",
		"select * from orders for update",
	}
	for _, statement := range refused {
		assert.True(t, hasRefusedClause(statement), statement)
	}

	allowed := []string{
		"SELECT * FROM orders",
		"SELECT 'send this FOR UPDATE later' FROM orders",
		"-- FOR UPDATE\nSELECT * FROM orders",
		"SELECT * FROM orders WHERE note = 'into outfile'",
		"SELECT * FROM `FOR UPDATE` AS t",
	}
	for _, statement := range allowed {
		assert.False(t, hasRefusedClause(statement), statement)
	}
}

func TestReferencesUserVariable(t *testing.T) {
	refused := []string{
		"SELECT @leak := data FROM secret LIMIT 1",
		"SELECT @leak",
		"SELECT @leak FROM orders",
		"SELECT @`leak`",
		"select @Leak",
	}
	for _, statement := range refused {
		assert.True(t, referencesUserVariable(statement), statement)
	}

	allowed := []string{
		"SELECT * FROM orders",
		"SELECT @@version",
		"SELECT @@session.sql_mode",
		"SELECT 'user@example.com' FROM orders",
		"SELECT * FROM orders WHERE note = '@leak'",
		"SELECT * FROM `my@column`",
		"-- @leak\nSELECT * FROM orders",
		"/* @leak */ SELECT * FROM orders",
	}
	for _, statement := range allowed {
		assert.False(t, referencesUserVariable(statement), statement)
	}
}

func TestParseTraceTableName(t *testing.T) {
	cases := []struct {
		raw    string
		db     string
		table  string
		wantOK bool
	}{
		{raw: "`orders`", table: "orders", wantOK: true},
		{raw: "`orders` `o`", table: "orders", wantOK: true},
		{raw: "`otherdb`.`widgets`", db: "otherdb", table: "widgets", wantOK: true},
		{raw: "`otherdb`.`widgets` `w`", db: "otherdb", table: "widgets", wantOK: true},
		{raw: " `dt`", wantOK: false},
		{raw: "``", wantOK: false},
		{raw: "", wantOK: false},
		// Adversarial: an alias equal to another real table's name, with
		// text MySQL inserts between the real name and that alias. The
		// real table must win regardless of what follows it.
		{raw: "`secret` `public_t`", table: "secret", wantOK: true},
		{raw: "`secret` PARTITION (`p0`) `public_t`", table: "secret", wantOK: true},
		{raw: "`secret` `public_t` USE INDEX (PRIMARY)", table: "secret", wantOK: true},
		{raw: "`otherdb`.`secret` PARTITION (`p0`) `public_t`", db: "otherdb", table: "secret", wantOK: true},
	}
	for _, c := range cases {
		db, table, ok := parseTraceTableName(c.raw)
		assert.Equal(t, c.wantOK, ok, c.raw)
		if c.wantOK {
			assert.Equal(t, c.db, db, c.raw)
			assert.Equal(t, c.table, table, c.raw)
		}
	}
}

// Captured from a real MySQL 8 optimizer trace for
// "SELECT o.id, c.name FROM orders o JOIN customers c ON o.customer_id = c.id",
// where both tables are referenced by an alias EXPLAIN's own table_name
// would have shown instead of the real table. Go raw strings cannot contain
// a backtick, so this uses an ordinary quoted string instead, where a
// backtick needs no escaping.
const joinTraceJSON = "{\"steps\":[{\"join_optimization\":{\"select#\":1,\"steps\":[" +
	"{\"table_dependencies\":[{\"table\":\"`orders` `o`\"},{\"table\":\"`customers` `c`\"}]}" +
	"]}}]}"

func TestWalkTraceTables_ResolvesAliasedTablesByTheirRealName(t *testing.T) {
	var traced any
	require.NoError(t, json.Unmarshal([]byte(joinTraceJSON), &traced))

	refs := map[tableRef]bool{}
	walkTraceTables(traced, refs)
	assert.Equal(t, map[tableRef]bool{
		{table: "orders"}:    true,
		{table: "customers"}: true,
	}, refs)
}

// A materialised derived table's table_dependencies entry carries no real
// name at all, only its alias, and must not be mistaken for a table.
func TestWalkTraceTables_SkipsMaterializedDerivedTables(t *testing.T) {
	traceJSON := "{\"join_optimization\":{\"select#\":1,\"steps\":[" +
		"{\"join_optimization\":{\"select#\":2,\"steps\":[{\"table_dependencies\":[{\"table\":\"`orders`\"}]}]}}," +
		"{\"table_dependencies\":[{\"table\":\" `dt`\"}]}" +
		"]}}"
	var traced any
	require.NoError(t, json.Unmarshal([]byte(traceJSON), &traced))

	refs := map[tableRef]bool{}
	walkTraceTables(traced, refs)
	assert.Equal(t, map[tableRef]bool{{table: "orders"}: true}, refs)
}

func TestExplainReferencesAnyTable(t *testing.T) {
	withTable := map[string]any{"query_block": map[string]any{"table": map[string]any{"table_name": "orders"}}}
	assert.True(t, explainReferencesAnyTable(withTable))

	noTables := map[string]any{"query_block": map[string]any{"message": "No tables used"}}
	assert.False(t, explainReferencesAnyTable(noTables))

	onlyPseudo := map[string]any{"query_block": map[string]any{"union_result": map[string]any{"table_name": "<union1,2>"}}}
	assert.False(t, explainReferencesAnyTable(onlyPseudo))
}

func TestOpaqueCalls(t *testing.T) {
	assert.Empty(t, opaqueCalls("SELECT COUNT(*), UPPER(name) FROM customers"))
	assert.Equal(t, []string{"add_one"}, opaqueCalls("SELECT add_one(amount) FROM orders"))
	assert.Equal(t, []string{"otherdb.add_one"}, opaqueCalls("SELECT otherdb.add_one(amount) FROM orders"))
	assert.Empty(t, opaqueCalls("SELECT 'call add_one(1) in this string' FROM orders"), "a call inside a string literal is not a call")
}

// Two sources served by one process must each keep their own connection;
// asking for one never closes the other.
func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	// sql.Open connects lazily, so these never touch the network.
	open := func() *sql.DB {
		db, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/db")
		require.NoError(t, err)
		return db
	}
	var q queryConns
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.NotSame(t, a, b)

	assert.Same(t, a, q.put("a", open()), "a connection opened concurrently for the same config is discarded")

	q.conns["a"].lastUsed = time.Now().Add(-connIdleTTL - time.Second)
	assert.Nil(t, q.get("a"), "an idle connection is closed and forgotten")
	assert.Same(t, b, q.get("b"))
}

// TestQuery_Integration exercises PlanQuery and ExecuteQuery against a real
// MySQL server. Point MARMOT_TEST_MYSQL_DSN at a throwaway instance with
// enough privilege to create its own database, for example:
//
//	docker run -d --name qtest-mysql -p 55530:3306 -e MYSQL_ROOT_PASSWORD=rootpw mysql:8
//	MARMOT_TEST_MYSQL_DSN="root:rootpw@tcp(127.0.0.1:55530)/" go test ./...
//	docker rm -f qtest-mysql
//
// The test creates and drops its own database, so it is safe to run against
// a persistent instance too. The Source under test connects with the same
// credentials; README.md recommends a SELECT-only account for a real
// deployment, which this test does not need in order to exercise planning.
func TestQuery_Integration(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_MYSQL_DSN to run the data query tests against a real MySQL server")
	}

	connConfig, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)

	ctx := context.Background()
	admin, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	require.NoError(t, admin.PingContext(ctx))

	const testDB = "marmot_query_test"
	_, err = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+testDB)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+testDB)
	})

	for _, stmt := range []string{
		"CREATE DATABASE " + testDB,
		"CREATE TABLE " + testDB + ".orders (id INT PRIMARY KEY, customer_id INT, amount DECIMAL(10,2))",
		"CREATE TABLE " + testDB + ".customers (id INT PRIMARY KEY, name VARCHAR(100))",
		"CREATE VIEW " + testDB + ".order_summary AS SELECT o.id, c.name, o.amount FROM " + testDB + ".orders o JOIN " + testDB + ".customers c ON o.customer_id = c.id",
		"CREATE FUNCTION " + testDB + ".add_one(x INT) RETURNS INT DETERMINISTIC RETURN x + 1",
		"INSERT INTO " + testDB + ".customers VALUES (1, 'Alice'), (2, 'Bob')",
		"INSERT INTO " + testDB + ".orders VALUES (1, 1, 10.50), (2, 2, 20.00), (3, 1, 5.25)",
		// secret and public_t exist so that a plan which mistook an alias
		// for a real table, the way EXPLAIN's own table_name field does,
		// would name public_t while the statement actually reads secret.
		"CREATE TABLE " + testDB + ".secret (id INT PRIMARY KEY, data VARCHAR(50)) PARTITION BY HASH(id) PARTITIONS 2",
		"CREATE TABLE " + testDB + ".public_t (id INT PRIMARY KEY, data VARCHAR(50))",
		"INSERT INTO " + testDB + ".secret VALUES (1, 's1'), (2, 's2')",
		"INSERT INTO " + testDB + ".public_t VALUES (1, 'p1')",
	} {
		_, err := admin.ExecContext(ctx, stmt)
		require.NoError(t, err, stmt)
	}

	host, port, err := splitHostPort(connConfig.Addr)
	require.NoError(t, err)
	raw := pluginsdk.RawConfig{
		"host": host, "port": port,
		"user": connConfig.User, "password": connConfig.Passwd,
		"database": testDB,
	}

	s := &Source{}

	t.Run("plain table keeps its own MRN", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT * FROM orders"})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.Equal(t, []string{"mrn://asset/table/mysql/" + testDB + ".orders"}, plan.ReferencedMRNs)
		assert.False(t, plan.Incomplete)
	})

	t.Run("a joined view expands to both tables it reads, by their real names", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT * FROM order_summary"})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.ElementsMatch(t, []string{"mrn://asset/table/mysql/" + testDB + ".orders", "mrn://asset/table/mysql/" + testDB + ".customers"}, plan.ReferencedMRNs)
	})

	t.Run("aliased tables resolve to their real names, not the alias", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT o.id, c.name FROM orders o JOIN customers c ON o.customer_id = c.id"})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.ElementsMatch(t, []string{"mrn://asset/table/mysql/" + testDB + ".orders", "mrn://asset/table/mysql/" + testDB + ".customers"}, plan.ReferencedMRNs)
	})

	t.Run("a stored function marks the plan incomplete", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT add_one(amount) FROM orders"})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.True(t, plan.Incomplete)
	})

	// A value stashed in a user variable outlives the transaction that set
	// it, for as long as the session does, so one caller's
	// "SELECT @leak := data FROM secret" could otherwise still answer
	// a later, unrelated caller's bare "SELECT @leak" on the same pooled
	// connection. Both the write and the read are refused outright.
	userVariableStatements := []string{
		"SELECT @leak := data FROM secret LIMIT 1",
		"SELECT @leak",
	}
	for _, statement := range userVariableStatements {
		t.Run("PlanQuery refuses a user variable: "+statement, func(t *testing.T) {
			_, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err)
		})
		t.Run("ExecuteQuery refuses a user variable: "+statement, func(t *testing.T) {
			err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement}, func(pluginsdk.QueryResultChunk) error { return nil })
			require.Error(t, err)
		})
	}

	// Belt and braces, independent of the refusal above: even a form of
	// session state this plugin has not thought to refuse must not survive
	// from one call to the next on a pooled connection. This bypasses
	// ExecuteQuery and sets a user variable directly through the same pool
	// s.connect hands out, to prove the pool itself, not the statement
	// check, is what stops it from being seen again.
	t.Run("the connection pool never lets session state outlive the call that set it", func(t *testing.T) {
		db, _, err := s.connect(ctx, raw)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx, "SET @leak = 'top-secret'")
		require.NoError(t, err)

		var leak sql.NullString
		require.NoError(t, db.QueryRowContext(ctx, "SELECT @leak").Scan(&leak))
		assert.False(t, leak.Valid, "a fresh connection must not see a variable set on a previous one")
	})

	// public_t is a real table distinct from secret. Each of these statements
	// aliases secret as public_t, so a plan that mistook EXPLAIN's own
	// table_name field, which holds the alias, for the real table would wrongly
	// name public_t; a grant limited to public_t must never cover secret. Each
	// construct here is something that sits between the real table name and
	// the alias in the statement, confirmed against the plugin's own parsing
	// rather than assumed: a PARTITION clause, an index hint, redundant
	// parentheses, explicit backtick quoting, a comment, and a derived table
	// that happens to reuse the same alias.
	adversarialAliases := map[string]string{
		"plain alias collision":        "SELECT * FROM secret AS public_t WHERE id = 1",
		"partition clause":             "SELECT * FROM secret PARTITION (p0) AS public_t",
		"index hint":                   "SELECT * FROM secret AS public_t USE INDEX (PRIMARY) WHERE public_t.id = 1",
		"redundant parentheses":        "SELECT * FROM (secret AS public_t JOIN customers ON 1 = 1) LIMIT 1",
		"explicit backtick quoting":    "SELECT * FROM `secret` AS `public_t`",
		"a comment before the alias":   "SELECT * FROM secret /* hide */ AS public_t",
		"a derived table as the alias": "SELECT * FROM (SELECT id, data FROM secret) AS public_t",
	}
	for name, statement := range adversarialAliases {
		t.Run("alias collision ("+name+") names the real table, not the alias", func(t *testing.T) {
			plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement})
			require.NoError(t, err)
			require.NotNil(t, plan)
			if plan.Incomplete {
				return
			}
			assert.Contains(t, plan.ReferencedMRNs, "mrn://asset/table/mysql/"+testDB+".secret", statement)
			assert.NotContains(t, plan.ReferencedMRNs, "mrn://asset/table/mysql/"+testDB+".public_t", statement)
		})
	}

	t.Run("SHOW cannot be planned and falls back to a whole-source grant", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SHOW TABLES"})
		require.NoError(t, err)
		assert.Nil(t, plan)
	})

	refused := []string{
		"INSERT INTO customers VALUES (99, 'z')",
		"UPDATE customers SET name = 'z' WHERE id = 1",
		"DELETE FROM customers WHERE id = 1",
		"DROP TABLE customers",
		"SELECT 1; SELECT 2",
		"SELECT * FROM orders INTO OUTFILE '/tmp/marmot_query_test_leak.csv'",
		"SELECT * FROM orders FOR UPDATE",
		"SELECT * FROM orders LOCK IN SHARE MODE",
	}
	for _, statement := range refused {
		t.Run("PlanQuery refuses: "+statement, func(t *testing.T) {
			_, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err)
		})
		t.Run("ExecuteQuery refuses: "+statement, func(t *testing.T) {
			err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement}, func(pluginsdk.QueryResultChunk) error { return nil })
			require.Error(t, err)
		})
	}

	t.Run("execute streams rows with column types", func(t *testing.T) {
		var columns []pluginsdk.QueryColumn
		var rows [][]any
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT id, name FROM customers ORDER BY id"}, func(chunk pluginsdk.QueryResultChunk) error {
			if chunk.Columns != nil {
				columns = chunk.Columns
			}
			rows = append(rows, chunk.Rows...)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, columns, 2)
		assert.Equal(t, "id", columns[0].Name)
		assert.NotEmpty(t, columns[0].Type)
		require.Len(t, rows, 2)
		assert.Equal(t, "Alice", rows[0][1])
	})

	t.Run("a decimal column streams as a string", func(t *testing.T) {
		var rows [][]any
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT amount FROM orders WHERE id = 1"}, func(chunk pluginsdk.QueryResultChunk) error {
			rows = append(rows, chunk.Rows...)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "10.50", rows[0][0])
	})

	t.Run("a statement with no rows still carries the schema", func(t *testing.T) {
		var columns []pluginsdk.QueryColumn
		var chunks int
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT id FROM customers WHERE id < 0"}, func(chunk pluginsdk.QueryResultChunk) error {
			chunks++
			columns = chunk.Columns
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 1, chunks)
		require.Len(t, columns, 1)
	})

	t.Run("max rows is honoured", func(t *testing.T) {
		var rows [][]any
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: "SELECT * FROM orders", MaxRows: 2}, func(chunk pluginsdk.QueryResultChunk) error {
			rows = append(rows, chunk.Rows...)
			return nil
		})
		require.NoError(t, err)
		assert.Len(t, rows, 2)
	})
}

func splitHostPort(addr string) (string, int, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return "", 0, fmt.Errorf("invalid address %q", addr)
	}
	var port int
	if _, err := fmt.Sscanf(addr[i+1:], "%d", &port); err != nil {
		return "", 0, err
	}
	return addr[:i], port, nil
}
