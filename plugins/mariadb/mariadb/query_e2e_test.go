package mariadb_test

import (
	"context"
	"database/sql"
	"net"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/go-sql-driver/mysql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise PlanQuery and ExecuteQuery over the same gRPC wire
// protocol the Marmot host uses, against a real MariaDB server. Set
// MARMOT_TEST_MARIADB_DSN to a go-sql-driver DSN (for example
// "root:root@tcp(127.0.0.1:55540)/shop") to run them; the test creates and
// drops its own objects in that database, so it can share a server with
// other tests.

func queryDSNConfig(t *testing.T) (pluginsdk.RawConfig, string) {
	t.Helper()

	dsn := os.Getenv("MARMOT_TEST_MARIADB_DSN")
	if dsn == "" {
		t.Skip("MARMOT_TEST_MARIADB_DSN is not set, skipping MariaDB query e2e tests")
	}

	cfg, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)

	host, portStr, err := net.SplitHostPort(cfg.Addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return pluginsdk.RawConfig{
		"host":     host,
		"port":     port,
		"user":     cfg.User,
		"password": cfg.Passwd,
		"database": cfg.DBName,
	}, dsn
}

// setUpQuerySchema creates the objects the query tests read: two tables, a
// FOREIGN KEY, a view that aliases both tables internally (the case plain
// EXPLAIN output cannot resolve on its own), a sequence and a stored
// function, then seeds them. Objects are dropped and recreated so the test
// is safe to run against a server more than once.
func setUpQuerySchema(t *testing.T, dsn string) {
	t.Helper()

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	for _, stmt := range []string{
		"DROP VIEW IF EXISTS qcustomer_totals",
		"DROP VIEW IF EXISTS qcustomer_orders",
		"DROP TABLE IF EXISTS qorders",
		"DROP TABLE IF EXISTS qcustomers",
		"DROP SEQUENCE IF EXISTS qorder_seq",
		"DROP FUNCTION IF EXISTS qget_total",
		`CREATE TABLE qcustomers (id INT PRIMARY KEY, name VARCHAR(100))`,
		`CREATE TABLE qorders (id INT PRIMARY KEY, customer_id INT, total DECIMAL(10,2), FOREIGN KEY (customer_id) REFERENCES qcustomers(id))`,
		// qcustomer_totals aggregates, which forces materialisation
		// instead of merging; qcustomer_orders does not, so it merges.
		`CREATE VIEW qcustomer_totals AS
			SELECT c.id AS customer_id, SUM(o.total) AS total
			FROM qcustomers c JOIN qorders o ON o.customer_id = c.id
			GROUP BY c.id`,
		`CREATE VIEW qcustomer_orders AS
			SELECT c.id AS customer_id, o.total
			FROM qcustomers c JOIN qorders o ON o.customer_id = c.id`,
		`CREATE SEQUENCE qorder_seq START WITH 1000`,
		`INSERT INTO qcustomers VALUES (1, 'alice'), (2, 'bob')`,
		`INSERT INTO qorders VALUES (1, 1, 10.50), (2, 1, 20.00), (3, 2, 5.00)`,
	} {
		_, err := db.ExecContext(ctx, stmt)
		require.NoErrorf(t, err, "running %q", stmt)
	}

	// A function lives in its own statement because of the DELIMITER-free
	// body; go-sql-driver sends it as one statement regardless.
	_, err = db.ExecContext(ctx, `
		CREATE FUNCTION qget_total(cid INT) RETURNS DECIMAL(10,2) DETERMINISTIC READS SQL DATA
		BEGIN
			DECLARE t DECIMAL(10,2);
			SELECT SUM(total) INTO t FROM qorders WHERE customer_id = cid;
			RETURN t;
		END`)
	require.NoError(t, err)
}

func rowCount(t *testing.T, dsn, table string) int {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer db.Close()

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
	return count
}

func tableExists(t *testing.T, dsn, table string) bool {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer db.Close()

	var count int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table,
	).Scan(&count))
	return count > 0
}

func TestQueryE2E_PlanQueryMatchesDiscoveryMRNs(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	result, err := bin.Discover(ctx, config)
	require.NoError(t, err)
	orders := requireAsset(t, result, "Table", "qorders")
	customers := requireAsset(t, result, "Table", "qcustomers")

	plan, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT * FROM qorders"})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, []string{*orders.MRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)

	plan, _, err = bin.Query(ctx, config, pluginsdk.QueryRequest{
		Statement: "SELECT c.id, o.total FROM qcustomers c JOIN qorders o ON o.customer_id = c.id",
	})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.ElementsMatch(t, []string{*orders.MRN, *customers.MRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestQueryE2E_PlanQueryThroughAMergeableViewResolvesToItsBaseTables(t *testing.T) {
	// qcustomer_orders has no GROUP BY, so MariaDB's optimiser merges it
	// into the outer query; the canonical rewrite then names its base
	// tables directly rather than the view.
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	result, err := bin.Discover(ctx, config)
	require.NoError(t, err)
	orders := requireAsset(t, result, "Table", "qorders")
	customers := requireAsset(t, result, "Table", "qcustomers")

	plan, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT * FROM qcustomer_orders"})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.ElementsMatch(t, []string{*orders.MRN, *customers.MRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestQueryE2E_PlanQueryThroughAMaterialisedViewResolvesToTheViewItself(t *testing.T) {
	// qcustomer_totals has a GROUP BY, which forces MariaDB to materialise
	// it as a derived table instead of merging it; the canonical rewrite
	// then names the view itself, not its base tables, so the plan must
	// resolve to the view's own MRN, which is exactly the asset a grant
	// would be written against.
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	result, err := bin.Discover(ctx, config)
	require.NoError(t, err)
	view := requireAsset(t, result, "View", "qcustomer_totals")

	plan, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT * FROM qcustomer_totals"})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, []string{*view.MRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)
}

func TestQueryE2E_PlanQueryMarksIncompleteForAStoredFunctionCall(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	plan, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT id, qget_total(id) FROM qcustomers"})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Incomplete, "qget_total runs its own query that the plan cannot see")
}

func TestQueryE2E_ExecuteQueryStreamsRowsWithTypesAndHonoursMaxRows(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	_, chunks, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT id, total FROM qorders ORDER BY id"})
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	require.Len(t, chunks[0].Columns, 2)
	assert.Equal(t, "id", chunks[0].Columns[0].Name)
	assert.Equal(t, "total", chunks[0].Columns[1].Name)
	assert.NotEmpty(t, chunks[0].Columns[1].Type)

	var rows [][]any
	for _, c := range chunks {
		rows = append(rows, c.Rows...)
	}
	require.Len(t, rows, 3)
	assert.EqualValues(t, 1, rows[0][0])
	// DECIMAL comes back over the wire as a string, not a float.
	assert.Equal(t, "10.50", rows[0][1])

	_, limited, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT id FROM qorders ORDER BY id", MaxRows: 1})
	require.NoError(t, err)
	var limitedRows [][]any
	for _, c := range limited {
		limitedRows = append(limitedRows, c.Rows...)
	}
	assert.Len(t, limitedRows, 1, "MaxRows must cap the rows returned")
}

func TestQueryE2E_ExecuteQueryCarriesSchemaOnAZeroRowResult(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	_, chunks, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT * FROM qorders WHERE 1 = 0"})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.NotEmpty(t, chunks[0].Columns)
	assert.Empty(t, chunks[0].Rows)
}

func TestQueryE2E_RefusesWritesWithoutRunningThem(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	before := rowCount(t, dsn, "qorders")

	writes := []string{
		"INSERT INTO qorders VALUES (99, 1, 1.00)",
		"UPDATE qorders SET total = 0",
		"DELETE FROM qorders",
		"REPLACE INTO qorders VALUES (1, 1, 1.00)",
	}
	for _, stmt := range writes {
		_, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: stmt})
		assert.Error(t, err, stmt)
	}

	assert.Equal(t, before, rowCount(t, dsn, "qorders"), "none of the refused writes actually ran")
}

func TestQueryE2E_RefusesDDLWithoutRunningIt(t *testing.T) {
	// MariaDB's READ ONLY transaction does not stop DDL, which commits on
	// its own, so this specifically proves classifyRead is doing the work.
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	_, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "CREATE TABLE qevil (id INT)"})
	assert.Error(t, err)
	assert.False(t, tableExists(t, dsn, "qevil"), "the refused CREATE TABLE must never have run")

	_, _, err = bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "DROP TABLE qorders"})
	assert.Error(t, err)
	assert.True(t, tableExists(t, dsn, "qorders"), "the refused DROP TABLE must never have run")

	_, _, err = bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "TRUNCATE TABLE qorders"})
	assert.Error(t, err)
	assert.Equal(t, 3, rowCount(t, dsn, "qorders"), "the refused TRUNCATE must never have run")
}

func TestQueryE2E_RefusesMultipleStatements(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	_, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT 1; DROP TABLE qorders"})
	assert.Error(t, err)
	assert.True(t, tableExists(t, dsn, "qorders"))
}

func TestQueryE2E_RefusesIntoOutfileDumpfileForUpdateAndLockInShareMode(t *testing.T) {
	config, _ := queryDSNConfig(t)
	bin := buildBinary(t)
	ctx := t.Context()

	for _, stmt := range []string{
		"SELECT * FROM qorders INTO OUTFILE '/tmp/marmot-query-e2e.csv'",
		"SELECT * FROM qorders INTO DUMPFILE '/tmp/marmot-query-e2e.dump'",
		"SELECT * FROM qorders FOR UPDATE",
		"SELECT * FROM qorders LOCK IN SHARE MODE",
	} {
		_, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: stmt})
		assert.Error(t, err, stmt)
	}
}

// setUpAdversarialSchema creates a table whose name, qpublic, is designed
// to be picked as an alias for a different table, qsecret, whose contents
// a grant on qpublic alone must never cover. qsecret_part is partitioned,
// so a statement can place PARTITION (...) between the table and its alias
// too.
func setUpAdversarialSchema(t *testing.T, dsn string) {
	t.Helper()

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS qsecret_part",
		"DROP TABLE IF EXISTS qsecret",
		"DROP TABLE IF EXISTS qpublic",
		`CREATE TABLE qsecret (id INT PRIMARY KEY, ssn VARCHAR(20))`,
		`CREATE INDEX idx_qsecret_id ON qsecret(id)`,
		`CREATE TABLE qpublic (id INT PRIMARY KEY, name VARCHAR(20))`,
		`CREATE TABLE qsecret_part (id INT PRIMARY KEY, ssn VARCHAR(20))
			PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN MAXVALUE)`,
		`INSERT INTO qsecret VALUES (1, '111-22-3333')`,
		`INSERT INTO qpublic VALUES (1, 'alice')`,
		`INSERT INTO qsecret_part VALUES (1, '111-22-3333')`,
	} {
		_, err := db.ExecContext(ctx, stmt)
		require.NoErrorf(t, err, "running %q", stmt)
	}
}

// TestQueryE2E_PlanQueryNeverUnderReportsAnAliasThatNamesARealTable covers
// the ways a statement's own text could defeat an alias-based reading of
// what it touches: an alias equal to a different real table's name,
// combined with a PARTITION clause, an index hint, a comment and
// backtick-quoting between the table and its alias, plus a derived table
// aliased as a real table's name. Each must either name the table it
// actually reads, qsecret or qorders, or mark the plan incomplete; none
// may ever resolve to qpublic, the table whose name was borrowed as bait.
func TestQueryE2E_PlanQueryNeverUnderReportsAnAliasThatNamesARealTable(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	setUpAdversarialSchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	result, err := bin.Discover(ctx, config)
	require.NoError(t, err)
	secret := requireAsset(t, result, "Table", "qsecret")
	secretPart := requireAsset(t, result, "Table", "qsecret_part")
	publicT := requireAsset(t, result, "Table", "qpublic")
	orders := requireAsset(t, result, "Table", "qorders")

	cases := []struct {
		name      string
		statement string
	}{
		{"plain alias equal to another real table's name", "SELECT * FROM qsecret AS qpublic"},
		{"PARTITION clause between the table and its alias", "SELECT * FROM qsecret_part PARTITION (p0) AS qpublic"},
		{"USE INDEX between the alias and the rest of the clause", "SELECT * FROM qsecret AS qpublic USE INDEX (idx_qsecret_id)"},
		{"FORCE INDEX between the alias and the rest of the clause", "SELECT * FROM qsecret AS qpublic FORCE INDEX (idx_qsecret_id) WHERE id = 1"},
		{"IGNORE INDEX between the alias and the rest of the clause", "SELECT * FROM qsecret AS qpublic IGNORE INDEX (idx_qsecret_id) WHERE id = 1"},
		{"a comment between the table and AS", "SELECT * FROM qsecret /* comment */ AS qpublic"},
		{"a comment between AS and the alias", "SELECT * FROM qsecret AS /* comment */ qpublic"},
		{"backtick-quoted table and alias", "SELECT * FROM `qsecret` AS `qpublic`"},
		{"a derived table aliased as a real table's name", "SELECT * FROM (SELECT id, ssn FROM qsecret) AS qpublic"},
		{"a materialising derived table aliased as a real table's name", "SELECT * FROM (SELECT customer_id, SUM(total) s FROM qorders GROUP BY customer_id) AS qpublic"},
		{"old-style comma join, first table aliased as a real table's name", "SELECT 1 FROM qsecret AS qpublic, qorders WHERE qpublic.id = qorders.customer_id"},
		{"STRAIGHT_JOIN, first table aliased as a real table's name", "SELECT 1 FROM qsecret AS qpublic STRAIGHT_JOIN qorders ON qpublic.id = qorders.customer_id"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: c.statement})
			require.NoError(t, err)
			require.NotNil(t, plan)

			assert.NotContainsf(t, plan.ReferencedMRNs, *publicT.MRN,
				"qpublic was never read by %q but was reported as if it had been", c.statement)

			if !plan.Incomplete {
				assert.Truef(t,
					slices.Contains(plan.ReferencedMRNs, *secret.MRN) ||
						slices.Contains(plan.ReferencedMRNs, *secretPart.MRN) ||
						slices.Contains(plan.ReferencedMRNs, *orders.MRN),
					"a complete plan for %q must name the table it actually reads, got %v", c.statement, plan.ReferencedMRNs)
			}
		})
	}
}

// TestQueryE2E_RefusesUserVariableReadsAndWrites proves the primary defence
// against the session-state channel: a caller can neither set nor read a
// user variable through PlanQuery or ExecuteQuery, so there is nothing for
// a second caller sharing a pooled connection to read back.
func TestQueryE2E_RefusesUserVariableReadsAndWrites(t *testing.T) {
	config, dsn := queryDSNConfig(t)
	setUpQuerySchema(t, dsn)
	bin := buildBinary(t)
	ctx := t.Context()

	for _, stmt := range []string{
		"SELECT @leak := (SELECT total FROM qorders LIMIT 1)",
		"SELECT @leak",
		"SELECT id, @x FROM qorders",
		"SELECT id INTO @x FROM qorders LIMIT 1",
	} {
		_, _, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: stmt})
		assert.Error(t, err, stmt)
	}

	// A system variable read is harmless and must still work.
	_, chunks, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT @@version"})
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}
