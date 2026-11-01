package oracle_test

import (
	"encoding/base64"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the query path over the same wire protocol the
// Marmot host uses, against a real Oracle database. They expect a
// marmot.qe_items table (id, name, price, big_number, created_at, notes
// CLOB, photo BLOB, token RAW(16)) with three rows and a marmot.qe_items_view
// view over it, owned by the connecting user.

// e2eQueryConfig is e2eConfig with SSL turned off, because the throwaway
// container these tests run against speaks plain TCP, not TCPS.
func e2eQueryConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()
	config := e2eConfig(t)
	config["ssl"] = false
	return config
}

func totalRows(chunks []pluginsdk.QueryResultChunk) int {
	n := 0
	for _, c := range chunks {
		n += len(c.Rows)
	}
	return n
}

func TestE2E_Query_MetaAdvertisesSupport(t *testing.T) {
	e2eConfig(t) // skips the suite when MARMOT_TEST_ORACLE_HOST is not set

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)
	assert.True(t, meta.SupportsQuery, "ExecuteQuery/PlanQuery should be advertised")
}

func TestE2E_Query_SelectStreamsRowsWithTypes(t *testing.T) {
	bin := buildBinary(t)
	req := pluginsdk.QueryRequest{
		Statement: "SELECT id, name, price, big_number, created_at, notes, photo, token FROM marmot.qe_items ORDER BY id",
		Identity:  "user:alice",
	}

	plan, chunks, err := bin.Query(t.Context(), e2eQueryConfig(t), req)
	require.NoError(t, err)
	assert.Nil(t, plan, "Oracle cannot plan a statement without a side effect, so the host must fall back to a whole-source grant")

	require.NotEmpty(t, chunks)
	require.Len(t, chunks[0].Columns, 8, "the first chunk carries the schema")
	assert.Equal(t, "ID", chunks[0].Columns[0].Name)

	require.Equal(t, 3, totalRows(chunks))

	var rows [][]any
	for _, c := range chunks {
		rows = append(rows, c.Rows...)
	}

	// Row 1: an integer NUMBER, a NUMBER with a fractional part and a NUMBER wider than int64.
	assert.EqualValues(t, 1, rows[0][0])
	assert.Equal(t, "Widget", rows[0][1])
	assert.Equal(t, "19.99", rows[0][2], "a NUMBER with a fractional part stays a string so it loses no precision")
	assert.Equal(t, "99999999999999999999999999999999999999", rows[0][3], "a NUMBER wider than int64 stays a string")
	assert.Contains(t, rows[0][4], "2026-01-02", "DATE renders as an RFC 3339 string")
	assert.Equal(t, "a widget", rows[0][5], "a CLOB arrives as plain text")

	photo, err := base64.StdEncoding.DecodeString(rows[0][6].(string))
	require.NoError(t, err, "a BLOB arrives as base64 text")
	assert.Equal(t, []byte{0x44, 0x45, 0x41, 0x44}, photo)

	token, err := base64.StdEncoding.DecodeString(rows[0][7].(string))
	require.NoError(t, err, "RAW arrives as base64 text")
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF}, token)

	// Row 2 and 3: NUMBER values that fit an int64 convert to a number.
	assert.EqualValues(t, 1, rows[1][3])
	assert.EqualValues(t, 2, rows[2][3])
}

func TestE2E_Query_SelectFromView(t *testing.T) {
	bin := buildBinary(t)
	req := pluginsdk.QueryRequest{Statement: "SELECT id, name FROM marmot.qe_items_view ORDER BY id"}

	plan, chunks, err := bin.Query(t.Context(), e2eQueryConfig(t), req)
	require.NoError(t, err)
	assert.Nil(t, plan)
	require.Equal(t, 3, totalRows(chunks))
}

func TestE2E_Query_MaxRowsIsHonoured(t *testing.T) {
	bin := buildBinary(t)
	req := pluginsdk.QueryRequest{Statement: "SELECT id FROM marmot.qe_items ORDER BY id", MaxRows: 2}

	_, chunks, err := bin.Query(t.Context(), e2eQueryConfig(t), req)
	require.NoError(t, err)
	assert.Equal(t, 2, totalRows(chunks))
}

func TestE2E_Query_ZeroRowsStillCarryColumns(t *testing.T) {
	bin := buildBinary(t)
	req := pluginsdk.QueryRequest{Statement: "SELECT id FROM marmot.qe_items WHERE id = -1"}

	_, chunks, err := bin.Query(t.Context(), e2eQueryConfig(t), req)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	assert.Len(t, chunks[0].Columns, 1)
	assert.Equal(t, 0, totalRows(chunks))
}

func TestE2E_Query_PlanQueryReturnsNilPlanForAnOrdinaryRead(t *testing.T) {
	bin := buildBinary(t)
	req := pluginsdk.QueryRequest{Statement: "SELECT id FROM marmot.qe_items"}

	plan, chunks, err := bin.Query(t.Context(), e2eQueryConfig(t), req)
	require.NoError(t, err)
	assert.Nil(t, plan, "no side-effect-free way exists to plan an Oracle statement, so the host must require a grant on the whole source")
	assert.NotEmpty(t, chunks)
}

func TestE2E_Query_RefusesEverythingThatIsNotAPlainRead(t *testing.T) {
	bin := buildBinary(t)
	config := e2eQueryConfig(t)

	statements := map[string]string{
		"insert":              "INSERT INTO marmot.qe_items (id, name) VALUES (99, 'x')",
		"update":              "UPDATE marmot.qe_items SET name = 'x' WHERE id = 1",
		"delete":              "DELETE FROM marmot.qe_items WHERE id = 1",
		"merge":               "MERGE INTO marmot.qe_items t USING (SELECT 1 AS id FROM dual) s ON (t.id = s.id) WHEN MATCHED THEN UPDATE SET t.name = 'x'",
		"create":              "CREATE TABLE marmot.qe_scratch (id NUMBER)",
		"drop":                "DROP TABLE marmot.qe_items_view",
		"alter session":       "ALTER SESSION SET current_schema = MARMOT",
		"plsql block":         "BEGIN NULL; END;",
		"declare block":       "DECLARE x NUMBER; BEGIN x := 1; END;",
		"call":                "CALL marmot.qe_items_view()",
		"select for update":   "SELECT * FROM marmot.qe_items FOR UPDATE",
		"trailing semicolon":  "SELECT * FROM marmot.qe_items;",
		"multiple statements": "SELECT * FROM marmot.qe_items; SELECT * FROM marmot.qe_items",
		"with clause function": `WITH FUNCTION f RETURN NUMBER IS PRAGMA AUTONOMOUS_TRANSACTION;
			BEGIN DELETE FROM marmot.qe_items; COMMIT; RETURN 1; END;
			SELECT f FROM dual`,
		"with clause procedure": `WITH PROCEDURE p IS PRAGMA AUTONOMOUS_TRANSACTION;
			BEGIN DELETE FROM marmot.qe_items; COMMIT; END;
			SELECT 1 FROM dual`,
	}

	for name, statement := range statements {
		t.Run(name, func(t *testing.T) {
			_, _, err := bin.Query(t.Context(), config, pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err, "%q must be refused", statement)
		})
	}
}

// A WITH clause can declare an inline PL/SQL function marked PRAGMA
// AUTONOMOUS_TRANSACTION, which commits independently of the read-only
// transaction ExecuteQuery wraps the statement in; it starts with WITH like
// an ordinary read, so this is refused on its own, not just caught by the
// semicolon check that also happens to fire on it.
func TestE2E_Query_RefusesAutonomousTransactionInWithClause(t *testing.T) {
	bin := buildBinary(t)
	config := e2eQueryConfig(t)

	statement := `WITH FUNCTION f RETURN NUMBER IS PRAGMA AUTONOMOUS_TRANSACTION;
		BEGIN DELETE FROM marmot.qe_items; COMMIT; RETURN 1; END;
		SELECT f FROM dual`

	_, _, err := bin.Query(t.Context(), config, pluginsdk.QueryRequest{Statement: statement})
	require.Error(t, err)

	_, chunks, err := bin.Query(t.Context(), config, pluginsdk.QueryRequest{Statement: "SELECT id FROM marmot.qe_items"})
	require.NoError(t, err)
	assert.Equal(t, 3, totalRows(chunks), "the autonomous transaction's DELETE must never have run")
}

func TestE2E_Query_UnaffectedByWriteRefusal(t *testing.T) {
	// A refused write must never reach Oracle, so the table it would have touched is unaffected.
	bin := buildBinary(t)
	config := e2eQueryConfig(t)

	_, _, err := bin.Query(t.Context(), config, pluginsdk.QueryRequest{Statement: "DELETE FROM marmot.qe_items"})
	require.Error(t, err)

	_, chunks, err := bin.Query(t.Context(), config, pluginsdk.QueryRequest{Statement: "SELECT id FROM marmot.qe_items"})
	require.NoError(t, err)
	assert.Equal(t, 3, totalRows(chunks), "the refused DELETE must not have run")
}
