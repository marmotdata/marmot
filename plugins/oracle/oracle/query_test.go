package oracle

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadStatement(t *testing.T) {
	reads := []string{
		"SELECT 1 FROM dual",
		"  select * from hr.employees",
		"WITH t AS (SELECT 1 FROM dual) SELECT * FROM t",
		"(SELECT 1 FROM dual) UNION (SELECT 2 FROM dual)",
		"-- the latest employees\nSELECT * FROM hr.employees",
		"/* note */ SELECT * FROM hr.employees",
		"SELECT * FROM hr.employees WHERE first_name = 'FOR UPDATE'",
		"SELECT * FROM hr.employees -- FOR UPDATE\nWHERE employee_id = 1",
		`SELECT "FOR", "UPDATE" FROM hr.employees`,
	}
	for _, s := range reads {
		assert.True(t, isReadStatement(s), s)
	}

	writes := []string{
		"INSERT INTO hr.employees (employee_id) VALUES (1)",
		"UPDATE hr.employees SET salary = 1",
		"DELETE FROM hr.employees",
		"MERGE INTO hr.employees USING x ON (1=1) WHEN MATCHED THEN UPDATE SET salary = 1",
		"CREATE TABLE stolen AS SELECT * FROM hr.employees",
		"ALTER SESSION SET current_schema = HR",
		"DROP TABLE hr.employees",
		"LOCK TABLE hr.employees IN EXCLUSIVE MODE",
		"CALL raise_salary(1, 1)",
		"EXEC raise_salary(1, 1)",
		"BEGIN NULL; END;",
		"DECLARE x NUMBER; BEGIN NULL; END;",
		"SELECT * FROM hr.employees FOR UPDATE",
		"SELECT * FROM hr.employees FOR  UPDATE OF salary",
		"SELECT * FROM hr.employees; DROP TABLE hr.employees",
		"SELECT * FROM hr.employees;",
		"",
		"   ",
		"SELECTED FROM dual",
		"WITH FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; SELECT f FROM dual",
		"with procedure p is begin null; end; select 1 from dual",
		"WITH /* sneaky */ FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; SELECT f FROM dual",
	}
	for _, s := range writes {
		assert.False(t, isReadStatement(s), s)
	}
}

// Oracle 12c and later let a WITH clause declare an inline function or
// procedure before its subquery factoring list; one marked PRAGMA
// AUTONOMOUS_TRANSACTION commits on its own, independently of the read-only
// transaction ExecuteQuery wraps the statement in, so it must be refused
// even though it still starts with WITH.
func TestIsReadStatement_RefusesInlinePLSQLDeclaredInAWithClause(t *testing.T) {
	statement := `WITH FUNCTION f RETURN NUMBER IS PRAGMA AUTONOMOUS_TRANSACTION; BEGIN DELETE FROM hr.employees; COMMIT; RETURN 1; END; SELECT f FROM dual`
	assert.False(t, isReadStatement(statement))
}

// FUNCTION and PROCEDURE are reserved words, so Oracle only accepts them
// unquoted as a WITH-clause PL/SQL declaration; quoted, they are ordinary
// identifiers and must not trip the refusal.
func TestIsReadStatement_AllowsAQuotedFunctionAsACTEName(t *testing.T) {
	statement := `WITH "FUNCTION" AS (SELECT 1 AS x FROM dual) SELECT * FROM "FUNCTION"`
	assert.True(t, isReadStatement(statement))
}

func TestConvertQueryValue(t *testing.T) {
	assert.Nil(t, convertQueryValue(nil, "NUMBER"))

	// A NUMBER that fits an int64 converts exactly.
	assert.Equal(t, int64(42), convertQueryValue("42", "NUMBER"))
	assert.Equal(t, int64(-7), convertQueryValue("-7", "NUMBER"))

	// A NUMBER with a fractional part, or wider than int64, stays a string so no precision is lost.
	assert.Equal(t, "5000.5", convertQueryValue("5000.5", "NUMBER"))
	assert.Equal(t, "99999999999999999999999999999999999999", convertQueryValue("99999999999999999999999999999999999999", "NUMBER"))

	// A plain string column is untouched.
	assert.Equal(t, "Alice", convertQueryValue("Alice", "VARCHAR2"))

	// RAW and BLOB arrive as []byte, which the caller leaves for encoding/json to render as base64.
	raw := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	assert.Equal(t, raw, convertQueryValue(raw, "RAW"))

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.Equal(t, "2026-01-02T03:04:05Z", convertQueryValue(ts, "DATE"))
}

func TestConvertQueryValueCapsLOBSize(t *testing.T) {
	big := make([]byte, maxLOBBytes+10)
	for i := range big {
		big[i] = 'a'
	}

	gotBytes, ok := convertQueryValue(big, "BLOB").([]byte)
	require.True(t, ok)
	assert.Len(t, gotBytes, maxLOBBytes)

	gotString, ok := convertQueryValue(string(big), "CLOB").(string)
	require.True(t, ok)
	assert.Len(t, gotString, maxLOBBytes)
}

func TestTruncateUTF8KeepsValidEncoding(t *testing.T) {
	// "é" is two bytes; cutting at an odd length must drop the whole rune, not half of it.
	s := "é"
	assert.Equal(t, "", truncateUTF8(s, 1))
	assert.Equal(t, "é", truncateUTF8(s, 2))
}

// Two sources served by one process must each keep their own connection; asking for one never closes the other.
func TestQueryConnsKeepOneConnectionPerConfig(t *testing.T) {
	// sql.Open connects lazily, so these never touch the network.
	open := func() *cachedConn {
		db, err := sql.Open("oracle", "oracle://user:pass@127.0.0.1:1/x")
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

func TestConfigHashIsStableAndDistinguishesConfigs(t *testing.T) {
	a := &Config{Host: "db1", Port: 1521, User: "marmot", ServiceName: "FREEPDB1"}
	b := &Config{Host: "db2", Port: 1521, User: "marmot", ServiceName: "FREEPDB1"}

	assert.Equal(t, configHash(a), configHash(a))
	assert.NotEqual(t, configHash(a), configHash(b))
}
