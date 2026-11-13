package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/filesource"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it concurrently, once for each source that uses the plugin, so the query path never reads the Source's config (which the SDK rewrites on every call) and instead keeps one read-only connection per config, keyed by a hash of it.
//
// The database file is opened with mode=ro, which stops SQLite creating or writing to it at the pager level, and _query_only=1 adds a PRAGMA-level guard on top. Those two are the hard boundary; everything else in this file (the statement checks and the bytecode scan) is a second line of defence against things mode=ro alone does not cover: ATTACH widens what a connection can read without writing anything, some PRAGMAs take effect the moment a statement is prepared rather than when it runs, and a crafted multi-statement call can make modernc.org/sqlite step straight through a later statement that was never checked at all.

const executeChunkRows = 500

var (
	errReadOnly           = errors.New("only read statements can run through Marmot")
	errMultipleStatements = errors.New("only one statement can run per call")
)

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served by the same plugin process are queried at once, so each keeps its own connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	db       *sql.DB
	cleanup  func() // removes the local copy filesource may have downloaded
	lastUsed time.Time
}

func (c *cachedConn) close() {
	_ = c.db.Close()
	c.cleanup()
}

// connect returns a live read-only connection for the config, reusing the cached one when it still answers a ping.
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
		log.Warn().Err(err).Msg("Cached SQLite query connection failed ping, reconnecting")
		s.query.drop(hash, c)
	}

	// Resolving an S3 or git path can take seconds, so it happens outside the lock and never holds up queries to other sources. The local copy is kept, not cleaned up here, because the connection reads from it for as long as it stays cached.
	localPath, cleanup, err := filesource.ResolveFilePath(ctx, config.FileSourceConfig, config.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving file path: %w", err)
	}

	db, err := sql.Open("sqlite", queryDSN(localPath))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		cleanup()
		return nil, fmt.Errorf("connecting to SQLite: %w", err)
	}

	c := s.query.put(hash, &cachedConn{db: db, cleanup: cleanup})
	return c.db, nil
}

// queryDSN builds a read-only connection string for the data query path. mode=ro is SQLite's own URI option and is validated by SQLite itself against the open flags, not by modernc.org/sqlite's DSN parsing; _query_only is that driver's shorthand for PRAGMA query_only, applied to every connection this DSN opens, including ones the pool opens later to replace one that failed its ping.
func queryDSN(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = "mode=ro&_query_only=1&_pragma=busy_timeout(5000)"
	return u.String()
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
		go fresh.close()
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
		go stale.close()
	}
}

// closeIdle closes connections unused for connIdleTTL. Close waits for queries still running, so it runs in the background.
func (q *queryConns) closeIdle(now time.Time) {
	for hash, c := range q.conns {
		if now.Sub(c.lastUsed) > connIdleTTL {
			delete(q.conns, hash)
			go c.close()
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

// readKeywords are the leading keywords of a statement that can only read. SQLite also allows a bare compound-select element of the form TABLE t, equivalent to SELECT * FROM t, so it is accepted alongside the usual three. Anything else, including PRAGMA, is refused before SQLite ever sees it: a PRAGMA's effect can take hold the moment the statement is prepared, which happens during planning too, well before Marmot has decided whether the caller may run it.
var readKeywords = map[string]bool{"select": true, "with": true, "values": true, "table": true}

// loadExtensionCall matches a call to load_extension, the one SQL function that can pull in arbitrary native code. modernc.org/sqlite refuses it at the engine level already (extension loading is off unless a caller enables it through an API this driver does not expose), but a statement that calls it is refused here too rather than relying on that.
var loadExtensionCall = regexp.MustCompile(`(?i)\bload_extension\s*\(`)

// maskStatement blanks out every comment and quoted literal in statement with spaces, leaving code elsewhere untouched and the string the same length, so a semicolon or keyword found in what is left is never one that only appears inside a string or a comment. ok is false when a string, a quoted identifier or a comment is left unterminated, which callers treat as a statement they cannot read safely.
func maskStatement(statement string) (masked string, ok bool) {
	var b strings.Builder
	b.Grow(len(statement))

	fail := func(from int) (string, bool) {
		b.WriteString(strings.Repeat(" ", len(statement)-from))
		return b.String(), false
	}

	i := 0
	for i < len(statement) {
		switch {
		case strings.HasPrefix(statement[i:], "--"):
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				// A line comment with no newline after it runs to the end of the statement, which is ordinary at the end of a query and not a sign of anything unterminated.
				b.WriteString(strings.Repeat(" ", len(statement)-i))
				i = len(statement)
				continue
			}
			b.WriteString(strings.Repeat(" ", end))
			b.WriteByte('\n')
			i += end + 1
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return fail(i)
			}
			b.WriteString(strings.Repeat(" ", end+4))
			i += end + 4
		case statement[i] == '\'' || statement[i] == '"' || statement[i] == '`':
			quote := statement[i]
			j := i + 1
			closed := false
			for j < len(statement) {
				if statement[j] == quote {
					if j+1 < len(statement) && statement[j+1] == quote {
						j += 2
						continue
					}
					closed = true
					j++
					break
				}
				j++
			}
			if !closed {
				return fail(i)
			}
			b.WriteString(strings.Repeat(" ", j-i))
			i = j
		case statement[i] == '[':
			end := strings.IndexByte(statement[i:], ']')
			if end < 0 {
				return fail(i)
			}
			b.WriteString(strings.Repeat(" ", end+1))
			i += end + 1
		default:
			b.WriteByte(statement[i])
			i++
		}
	}
	return b.String(), true
}

// leadingKeyword returns the lowercased first word of masked, after any leading whitespace and opening parentheses, such as the "(" in a parenthesised SELECT that starts a compound query.
func leadingKeyword(masked string) string {
	rest := strings.TrimLeft(masked, " \t\r\n(")
	end := strings.IndexFunc(rest, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	})
	if end < 0 {
		end = len(rest)
	}
	return strings.ToLower(rest[:end])
}

// hasTrailingStatement reports whether masked holds a second statement after its first top-level semicolon. Marmot validates and runs one statement per call, and some modernc.org/sqlite code paths step straight through every statement in a script once the first one is read, so a hidden second statement is refused here rather than ever reaching the driver.
func hasTrailingStatement(masked string) bool {
	semi := strings.IndexByte(masked, ';')
	if semi < 0 {
		return false
	}
	return strings.TrimSpace(masked[semi+1:]) != ""
}

// checkStatement refuses anything that is not a single read statement, before the statement is prepared for real: a second statement hidden after a semicolon, a write, ATTACH or DETACH (ATTACH can bring in another file under a connection that was opened read-only, widening what the caller can read), PRAGMA, VACUUM and a call to load_extension anywhere in the text.
func checkStatement(statement string) error {
	masked, ok := maskStatement(statement)
	if !ok {
		return errReadOnly
	}
	if hasTrailingStatement(masked) {
		return errMultipleStatements
	}
	if !readKeywords[leadingKeyword(masked)] {
		return errReadOnly
	}
	if loadExtensionCall.MatchString(masked) {
		return errReadOnly
	}
	return nil
}

// writeOpcodes are the SQLite VDBE opcodes that change a database. They catch what checkStatement's keyword check cannot: a write hidden behind a leading WITH, such as "WITH t AS (SELECT 1) UPDATE users SET ...", which reads as a read until the engine's own compiled plan is inspected.
var writeOpcodes = map[string]bool{
	"OpenWrite": true, "Insert": true, "IdxInsert": true,
	"Delete": true, "IdxDelete": true, "Clear": true,
	"CreateBtree": true, "DropTable": true, "DropIndex": true,
	"DropTrigger": true, "Destroy": true, "VUpdate": true,
}

// explainRow is one instruction of EXPLAIN <statement>'s bytecode listing.
type explainRow struct {
	Addr       int64
	Opcode     string
	P1, P2, P3 int64
	P4         sql.NullString
	P5         int64
	Comment    sql.NullString
}

// explainProgram runs EXPLAIN on statement and returns the bytecode program SQLite compiled for it. EXPLAIN only compiles a statement; the program it returns is never stepped, so this has no side effect beyond whatever a PRAGMA already had at prepare time, which checkStatement has refused by the time this is called.
func explainProgram(ctx context.Context, db *sql.DB, statement string) ([]explainRow, error) {
	rows, err := db.QueryContext(ctx, "EXPLAIN "+statement)
	if err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}
	defer rows.Close()

	var program []explainRow
	for rows.Next() {
		var r explainRow
		if err := rows.Scan(&r.Addr, &r.Opcode, &r.P1, &r.P2, &r.P3, &r.P4, &r.P5, &r.Comment); err != nil {
			return nil, fmt.Errorf("scanning bytecode: %w", err)
		}
		program = append(program, r)
	}
	return program, rows.Err()
}

// schemaObjectAt maps a root page a statement opens for reading back to the table that owns it, the same way discovery names it: a table's own sqlite_master row gives its name directly, an index's row gives its owning table's name through tbl_name. A view never needs resolving this way, because it has no root page of its own; SQLite always reads a view by reading the base tables underneath it, which is what this function sees instead.
func schemaObjectAt(ctx context.Context, db *sql.DB, rootPage int64) (table string, ok bool) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var objType, name, tblName string
	err := db.QueryRowContext(queryCtx,
		"SELECT type, name, tbl_name FROM sqlite_master WHERE rootpage = ?", rootPage,
	).Scan(&objType, &name, &tblName)
	if err != nil {
		return "", false
	}

	switch objType {
	case "table":
		return name, true
	case "index":
		return tblName, true
	default:
		return "", false
	}
}

// PlanQuery asks SQLite what a statement's compiled bytecode would touch, via EXPLAIN, which returns that bytecode as rows rather than running it, so planning stays free of side effects as long as the statement reached EXPLAIN in the first place: checkStatement refuses anything that could still act on preparation alone before that happens. Each table the bytecode opens for reading is mapped to the MRN discovery would give it, so grants written against catalogued assets bind on the query path too. Because SQLite always reads a view by reading its base tables, a statement over a view is reported against those tables rather than the view, which matches how the PostgreSQL and Trino plugins report a view's query-time reads too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if err := checkStatement(req.Statement); err != nil {
		return nil, err
	}
	db, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}
	config, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}

	program, err := explainProgram(ctx, db, req.Statement)
	if err != nil {
		return nil, err
	}

	rootPages := map[int64]bool{}
	var incomplete bool
	for _, instr := range program {
		if writeOpcodes[instr.Opcode] {
			return nil, errReadOnly
		}
		if instr.Opcode == "VOpen" {
			// A table-valued function such as pragma_table_info or json_each can be pointed at any table through an argument the bytecode never names, so the plan cannot promise it lists everything the statement reads.
			incomplete = true
		}
		// p3 is the database the cursor opens in; 0 is main. ATTACH is refused by checkStatement, so no OpenRead ever names anything else, but ignoring iDb 1 keeps a temp b-tree the engine builds for sorting or grouping from ever being looked up as if it were a catalogued table.
		if instr.Opcode == "OpenRead" && instr.P3 == 0 {
			rootPages[instr.P2] = true
		}
	}

	plan := &pluginsdk.QueryPlan{Incomplete: incomplete}
	seen := map[string]bool{}
	for page := range rootPages {
		table, ok := schemaObjectAt(ctx, db, page)
		if !ok {
			continue
		}
		tableMRN := assetMRN(config, "Table", table)
		if !seen[tableMRN] {
			seen[tableMRN] = true
			plan.ReferencedMRNs = append(plan.ReferencedMRNs, tableMRN)
		}
	}
	sort.Strings(plan.ReferencedMRNs)
	return plan, nil
}

// ExecuteQuery runs the statement against the same read-only connection PlanQuery would use and streams rows back in chunks, the first carrying the column schema. Marmot always plans a statement before executing it, so a write or a hidden second statement has already been refused by PlanQuery; checkStatement still runs again here rather than trusting that call order, because the connection's mode=ro and query_only, not Marmot's sequencing, are what actually keep a statement from changing the database.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if err := checkStatement(req.Statement); err != nil {
		return err
	}
	db, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	rows, err := db.QueryContext(ctx, req.Statement)
	if err != nil {
		return fmt.Errorf("executing statement: %w", err)
	}
	defer rows.Close()

	return streamRows(rows, req.MaxRows, emit)
}

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at
// maxRows when it is set.
func streamRows(rows *sql.Rows, maxRows int64, emit func(pluginsdk.QueryResultChunk) error) error {
	columnNames, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("reading columns: %w", err)
	}
	columns := make([]pluginsdk.QueryColumn, len(columnNames))
	for i, name := range columnNames {
		columns[i] = pluginsdk.QueryColumn{Name: name}
	}
	if columnTypes, err := rows.ColumnTypes(); err == nil {
		for i, ct := range columnTypes {
			columns[i].Type = strings.ToLower(ct.DatabaseTypeName())
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
			// A BLOB column scans as []byte, which json.Marshal would otherwise turn into base64 with nothing downstream aware it needs decoding back.
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
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
