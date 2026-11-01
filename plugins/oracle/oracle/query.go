package oracle

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one connection per config, keyed by a hash of it.
//
// Oracle has no driver-level read-only transaction: go-ora's BeginTx refuses
// sql.TxOptions{ReadOnly: true} outright, and DDL commits itself and ends a
// transaction regardless of the mode it started in. Read-only enforcement is
// therefore two layered checks, not one: every statement is restricted to a
// single SELECT or WITH before it reaches Oracle at all, and the one that
// gets through still runs inside a transaction that issues Oracle's own SET
// TRANSACTION READ ONLY as its first statement and is always rolled back.

const executeChunkRows = 500

// maxLOBBytes caps how much of a single LOB value a query result carries, so
// one huge CLOB or BLOB cannot blow up a result chunk. go-ora fetches LOBs
// inline rather than by cursor, so the value already sits fully in memory by
// the time this runs; the cap only bounds what gets sent back over the wire.
const maxLOBBytes = 1 << 20

var errReadOnly = errors.New("only a single select or with statement can run through Marmot")

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served by the same plugin process are queried at once, so each keeps its own connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	db       *sql.DB
	lastUsed time.Time
}

// connect returns a live connection for the config, reusing the cached one
// when it still answers a ping.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*sql.DB, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	hash := configHash(config)

	if c := s.query.get(hash); c != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return c.db, nil
		}
		log.Warn().Err(err).Msg("Cached Oracle connection failed ping, reconnecting")
		s.query.drop(hash, c)
	}

	// Connecting can take seconds, so it happens outside the lock and never holds up queries to other sources.
	db, err := openConnection(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connecting to Oracle: %w", err)
	}
	c := s.query.put(hash, &cachedConn{db: db})
	return c.db, nil
}

func (q *queryConns) get(hash string) *cachedConn {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.closeIdle(now)
	if c, ok := q.conns[hash]; ok {
		c.lastUsed = now
		return c
	}
	return nil
}

// put caches a newly opened connection, or keeps the one another call opened for the same config meanwhile and closes this one.
func (q *queryConns) put(hash string, fresh *cachedConn) *cachedConn {
	q.mu.Lock()
	defer q.mu.Unlock()
	if c, ok := q.conns[hash]; ok {
		go func() { _ = fresh.db.Close() }()
		c.lastUsed = time.Now()
		return c
	}
	if q.conns == nil {
		q.conns = make(map[string]*cachedConn)
	}
	fresh.lastUsed = time.Now()
	q.conns[hash] = fresh
	return fresh
}

// drop removes a connection that failed its ping, unless it has already been replaced.
func (q *queryConns) drop(hash string, stale *cachedConn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conns[hash] == stale {
		delete(q.conns, hash)
		go func() { _ = stale.db.Close() }()
	}
}

// closeIdle closes connections unused for connIdleTTL. Close waits for queries still running, so it runs in the background.
func (q *queryConns) closeIdle(now time.Time) {
	for hash, c := range q.conns {
		if now.Sub(c.lastUsed) > connIdleTTL {
			delete(q.conns, hash)
			go func() { _ = c.db.Close() }()
		}
	}
}

func configHash(c *Config) string {
	data, err := json.Marshal(c) //nolint:gosec // G117: hashed in memory as a cache key, never stored or logged
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readStatementKeywords are the only statement kinds Marmot ever runs through
// this path. Everything else, including PL/SQL blocks, CALL, EXEC, ALTER
// SESSION, LOCK TABLE and every DML and DDL statement, is refused before it
// reaches Oracle, because DDL auto-commits and would end the read-only
// transaction before it could protect anything.
var readStatementKeywords = map[string]bool{"SELECT": true, "WITH": true}

// isReadStatement reports whether a statement is a single read with no
// locking clause and no inline PL/SQL declared in a WITH clause. It walks
// the same tokenizer view reference extraction uses, so a semicolon, a FOR
// UPDATE clause or a FUNCTION/PROCEDURE keyword hidden inside a comment, a
// string literal or a quoted identifier is never mistaken for a real one;
// conversely, anything genuinely hidden behind an unterminated comment or
// string is left for Oracle's own parser to reject, since go-ora sends the
// whole statement text as a single call and Oracle accepts only one
// statement per call.
func isReadStatement(statement string) bool {
	tokens := tokenize(statement)

	i := 0
	for i < len(tokens) && !tokens[i].quoted && tokens[i].text == "(" {
		i++
	}
	if i >= len(tokens) || tokens[i].quoted || !readStatementKeywords[strings.ToUpper(tokens[i].text)] {
		return false
	}
	if strings.EqualFold(tokens[i].text, "WITH") && withClauseDeclaresPLSQL(tokens, i) {
		return false
	}

	for j, t := range tokens {
		if t.quoted {
			continue
		}
		if t.text == ";" {
			// A trailing semicolon or a second statement. This also catches
			// every other way to declare inline PL/SQL in a WITH clause,
			// since a function or procedure body cannot be written without
			// at least one PL/SQL statement terminator of its own; the
			// explicit check above exists so that refusal does not depend
			// on this incidentally catching it too.
			return false
		}
		if strings.EqualFold(t.text, "FOR") && j+1 < len(tokens) && !tokens[j+1].quoted && strings.EqualFold(tokens[j+1].text, "UPDATE") {
			return false
		}
	}
	return true
}

// withClauseDeclaresPLSQL reports whether the WITH clause starting at
// tokens[with] opens with an inline FUNCTION or PROCEDURE declaration.
// Oracle 12c and later let a WITH clause declare PL/SQL ahead of its
// subquery factoring list, and a function or procedure declared that way can
// mark itself PRAGMA AUTONOMOUS_TRANSACTION, which commits independently of
// the read-only transaction this package starts around the statement. The
// keyword is only ever a declaration here, not a name, because FUNCTION and
// PROCEDURE are reserved words Oracle will not accept as an unquoted CTE
// name.
func withClauseDeclaresPLSQL(tokens []token, with int) bool {
	i := with + 1
	if i >= len(tokens) || tokens[i].quoted {
		return false
	}
	switch strings.ToUpper(tokens[i].text) {
	case "FUNCTION", "PROCEDURE":
		return true
	default:
		return false
	}
}

// readOnly runs fn inside a transaction that issues Oracle's own SET
// TRANSACTION READ ONLY as its first statement, and always rolls the
// transaction back afterwards; a data query never has anything to commit.
// go-ora's BeginTx refuses sql.TxOptions{ReadOnly: true} outright, so the
// read-only mode is set with Oracle's statement instead of the driver
// option, which only binds because nothing else runs on the transaction
// first.
func readOnly(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		return fmt.Errorf("starting read-only transaction: %w", err)
	}
	return fn(tx)
}

// PlanQuery refuses anything other than a single read statement and
// otherwise returns a nil plan, because nothing in Oracle names the tables a
// statement reads without a side effect of its own: EXPLAIN PLAN FOR writes
// its result to PLAN_TABLE, and V$SQL_PLAN only knows a statement after it
// has already run. A nil plan with a nil error tells the host it must fall
// back to requiring a grant on the whole source for this plugin; the Oracle
// user the source connects as remains the real access boundary, so it
// should be granted only what callers are meant to read.
func (s *Source) PlanQuery(_ context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if !isReadStatement(req.Statement) {
		return nil, errReadOnly
	}
	return nil, nil
}

// ExecuteQuery runs the statement in a read-only transaction and streams
// rows back in chunks, the first carrying the column schema. Marmot always
// plans a statement before executing it, so PlanQuery has already refused a
// write, but the check is repeated here because ExecuteQuery must refuse one
// on its own too.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if !isReadStatement(req.Statement) {
		return errReadOnly
	}
	db, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	return readOnly(ctx, db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, req.Statement)
		if err != nil {
			return fmt.Errorf("executing statement: %w", err)
		}
		defer rows.Close()

		return streamRows(rows, req.MaxRows, emit)
	})
}

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at
// maxRows when it is set.
func streamRows(rows *sql.Rows, maxRows int64, emit func(pluginsdk.QueryResultChunk) error) error {
	columnNames, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("reading columns: %w", err)
	}
	columns := make([]pluginsdk.QueryColumn, len(columnNames))
	databaseTypes := make([]string, len(columnNames))
	for i, name := range columnNames {
		columns[i] = pluginsdk.QueryColumn{Name: name}
	}
	if columnTypes, err := rows.ColumnTypes(); err == nil {
		for i, ct := range columnTypes {
			databaseTypes[i] = ct.DatabaseTypeName()
			columns[i].Type = strings.ToLower(databaseTypes[i])
		}
	}

	chunk := pluginsdk.QueryResultChunk{Columns: columns}
	var sent int64
	for rows.Next() {
		values := make([]any, len(columnNames))
		pointers := make([]any, len(columnNames))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		for i, v := range values {
			values[i] = convertQueryValue(v, databaseTypes[i])
		}

		chunk.Rows = append(chunk.Rows, values)
		sent++
		if maxRows > 0 && sent >= maxRows {
			break
		}
		if len(chunk.Rows) >= executeChunkRows {
			if err := emit(chunk); err != nil {
				return err
			}
			chunk = pluginsdk.QueryResultChunk{}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating rows: %w", err)
	}

	// Always emit the final chunk, even empty, so the schema reaches the
	// host for zero-row results.
	if len(chunk.Rows) > 0 || chunk.Columns != nil {
		return emit(chunk)
	}
	return nil
}

// convertQueryValue turns a value go-ora handed back for a query result into
// one that serialises predictably as JSON. NUMBER arrives as a string so its
// precision survives Oracle's up to 38 digits; it is turned into an int64
// only when that is an exact conversion, and left as the original string
// otherwise rather than risk the precision loss a float64 parse could cause.
// RAW and BLOB columns already arrive as []byte, which encoding/json renders
// as base64 unchanged. DATE and TIMESTAMP columns arrive as time.Time.
func convertQueryValue(val any, databaseType string) any {
	switch v := val.(type) {
	case nil:
		return nil
	case string:
		if databaseType == "NUMBER" {
			return parseWholeNumber(v)
		}
		return truncateUTF8(v, maxLOBBytes)
	case []byte:
		return truncateBytes(v, maxLOBBytes)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return v.String()
	default:
		return v
	}
}

// parseWholeNumber returns a NUMBER column's decimal text as an int64 when
// that conversion is exact, and leaves it as the original string otherwise,
// so a value wider than int64 or with a fractional part never loses
// precision by going through a lossy float64 parse.
func parseWholeNumber(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
}

// truncateUTF8 caps s at max bytes, dropping a final rune cut in half by the
// boundary rather than emitting invalid UTF-8.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "")
}

// truncateBytes caps b at max bytes.
func truncateBytes(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}
