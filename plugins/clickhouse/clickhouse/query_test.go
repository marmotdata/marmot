package clickhouse

import (
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlanCollectsMergeTreeRelations(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Join",
		"Plans": [
			{"Node Type": "Expression", "Plans": [{"Node Type": "ReadFromMergeTree", "Description": "qtest.orders"}]},
			{"Node Type": "Expression", "Plans": [{"Node Type": "ReadFromMergeTree", "Description": "qtest.customers"}]},
			{"Node Type": "Expression", "Plans": [{"Node Type": "ReadFromMergeTree", "Description": "qtest.orders"}]}
		]
	}}]`))
	require.NoError(t, err)
	assert.Equal(t, []relation{{database: "qtest", table: "orders"}, {database: "qtest", table: "customers"}}, summary.relations, "a table read twice is only named once")
	assert.Empty(t, summary.unresolvedNodeTypes)
}

func TestParsePlanRecordsUnresolvedReads(t *testing.T) {
	// A Memory or Log engine table, or a table function the keyword and
	// function checks did not already refuse, shows up as some other
	// ReadFrom* node without the db.table name ReadFromMergeTree carries.
	summary, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Expression",
		"Plans": [{"Node Type": "ReadFromMemoryStorage"}]
	}}]`))
	require.NoError(t, err)
	assert.Empty(t, summary.relations, "the host then falls back to a whole-source grant")
	assert.Equal(t, []string{"ReadFromMemoryStorage"}, summary.unresolvedNodeTypes)
}

func TestParsePlanWithNoReads(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {"Node Type": "Expression"}}]`))
	require.NoError(t, err)
	assert.Empty(t, summary.relations)
	assert.Empty(t, summary.unresolvedNodeTypes)
}

func TestRelationMRNMatchesDiscovery(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/clickhouse/qtest.orders", relationMRN(&Config{}, relation{database: "qtest", table: "orders"}))
}

func TestIsReadStatement(t *testing.T) {
	reads := []string{
		"SELECT 1",
		"  select * from qtest.orders",
		"WITH cte AS (SELECT 1) SELECT * FROM cte",
		"(SELECT 1) UNION ALL (SELECT 2)",
		"-- the latest orders\nSELECT * FROM qtest.orders",
		"/* note */ SELECT 1",
		"# shebang-style comment\nSELECT 1",
	}
	for _, s := range reads {
		assert.True(t, isReadStatement(s), s)
	}

	writes := []string{
		"INSERT INTO qtest.orders VALUES (1, 1, 1)",
		"CREATE TABLE qtest.stolen ENGINE = Memory AS SELECT 1",
		"ALTER TABLE qtest.orders DELETE WHERE id = 1",
		"DROP TABLE qtest.orders",
		"SET max_threads = 1",
		"OPTIMIZE TABLE qtest.orders",
		"-- SELECT\nINSERT INTO qtest.orders VALUES (1, 1, 1)",
		"/* unterminated SELECT",
		"",
		"SELECTED",
	}
	for _, s := range writes {
		assert.False(t, isReadStatement(s), s)
	}
}

func TestCallsExternalTableFunction(t *testing.T) {
	for statement, want := range map[string]bool{
		`SELECT * FROM url('http://evil.example/x.csv', CSV, 'a String')`:                 true,
		`SELECT * FROM s3('https://bucket/x.csv', CSV, 'a String')`:                       true,
		`SELECT * FROM file('/etc/passwd', CSV, 'a String')`:                              true,
		`SELECT * FROM remote('127.0.0.1:9000', system.one)`:                              true,
		`SELECT * FROM remoteSecure('127.0.0.1:9440', system.one)`:                        true,
		`SELECT * FROM mysql('host:3306', 'db', 'tbl', 'user', 'pass')`:                   true,
		`SELECT * FROM postgresql('host:5432', 'db', 'tbl', 'user', 'pass')`:              true,
		`SELECT * FROM jdbc('ds', 'q')`:                                                   true,
		`select o.* from orders o, url (   'http://x', CSV, 'a String') u`:                true,
		"SELECT * FROM orders /* hidden */ CROSS JOIN url(\n'http://x', CSV, 'a String')": true,
		`SELECT * FROM orders WHERE note = 'url(http://x)'`:                               false,
		"SELECT * FROM orders -- url(http://x)\nWHERE id = 1":                             false,
		`SELECT * FROM orders`:                            false,
		`SELECT * FROM qtest.orders_view`:                 false,
		`SELECT * FROM orders WHERE note = 'unterminated`: true,
		`SELECT * FROM orders /* unterminated`:            true,
	} {
		assert.Equal(t, want, callsExternalTableFunction(statement), statement)
	}
}

func TestCallsDictionaryFunction(t *testing.T) {
	for statement, want := range map[string]bool{
		`SELECT dictGet('ips', 'region', ip) FROM qtest.orders`:         true,
		`SELECT dictGetOrDefault('ips', 'region', ip, 'x') FROM orders`: true,
		`SELECT dictHas('ips', ip) FROM orders`:                         true,
		`SELECT * FROM orders WHERE note = 'dictGet(x)'`:                false,
		`SELECT * FROM orders`:                                          false,
	} {
		assert.Equal(t, want, callsDictionaryFunction(statement), statement)
	}
}

// Two sources served by one process must each keep their own connection; asking for one never closes the other.
func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	// clickhouse.Open connects lazily, so these never touch the network.
	open := func() *cachedConn {
		conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{"127.0.0.1:1"}})
		require.NoError(t, err)
		return &cachedConn{conn: conn}
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
