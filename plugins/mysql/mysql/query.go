package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one connection pool per config, keyed by a hash of it. The
// pool is shared by every caller of that source, whatever their own grants,
// so nothing one call leaves on a connection may still be there for another
// call that happens to be handed the same one; see openQueryDB.
//
// Every statement is planned and run inside a START TRANSACTION READ ONLY
// transaction that is always rolled back, and the DSN this plugin opens for
// queries leaves multiStatements unset, so the server accepts at most one
// statement per call. MySQL itself then refuses a write inside that
// transaction, but it does not refuse SELECT ... INTO OUTFILE or DUMPFILE,
// which write a file rather than a table, nor a locking read such as FOR
// UPDATE, FOR SHARE or LOCK IN SHARE MODE, so those are refused here before
// anything reaches the engine. A user variable (@name) is refused outright
// for the same reason: unlike the transaction, it survives a rollback for as
// long as the session does, so a value one call stashes in one is still
// there for a later call to read back, on whatever connection the pool hands
// it. The connecting user's own privileges remain the hard boundary on what
// it can read.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served
// by the same plugin process are queried at once, so each keeps its own
// connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	db       *sql.DB
	lastUsed time.Time
}

// connect returns the connection pool for the config, opening and caching
// one if none is cached yet. go-sql-driver's ResetSession only checks that a
// pooled connection is still live; it does not clear session state such as
// user variables, so that is left to openQueryDB's pool settings instead.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*sql.DB, *Config, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, nil, err
	}
	hash := configHash(config)

	if c := s.query.get(hash); c != nil {
		return c.db, config, nil
	}

	// Opening can take seconds, so it happens outside the lock and never
	// holds up queries to other sources.
	db, err := openQueryDB(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to MySQL: %w", err)
	}
	c := s.query.put(hash, db)
	return c.db, config, nil
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

// put caches a newly opened connection, or keeps the one another call opened
// for the same config meanwhile and closes this one.
func (q *queryConns) put(hash string, db *sql.DB) *cachedConn {
	q.mu.Lock()
	defer q.mu.Unlock()
	if c, ok := q.conns[hash]; ok {
		go func() { _ = db.Close() }()
		c.lastUsed = time.Now()
		return c
	}
	if q.conns == nil {
		q.conns = make(map[string]*cachedConn)
	}
	c := &cachedConn{db: db, lastUsed: time.Now()}
	q.conns[hash] = c
	return c
}

// closeIdle closes connections unused for connIdleTTL. Close waits for
// queries still running, so it runs in the background.
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

// openQueryDB opens the connection pool for data queries. multiStatements is
// deliberately left unset, so the server accepts at most one statement per
// call. MaxIdleConns is 0, so database/sql closes a connection the moment a
// call gives it back rather than keeping it ready for the next one: MySQL
// has no statement that clears a session's user variables, and the only
// protocol command that does, COM_RESET_CONNECTION, is not exposed by
// go-sql-driver, so refusing to reuse the session outlives any one call is
// what actually guarantees the next call starts clean. This trades a fresh
// handshake on every call for that guarantee, which is cheap next to a
// query's own round trip and cheap next to what a leftover session could
// leak between two callers of the same source with different grants.
func openQueryDB(ctx context.Context, config *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&parseTime=true&timeout=15s",
		config.User, config.Password, config.Host, config.Port, config.Database, config.TLS)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening connection: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(2 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().Str("host", config.Host).Str("database", config.Database).Msg("Opened MySQL connection for data queries")
	return db, nil
}

// readOnly runs fn inside a START TRANSACTION READ ONLY transaction that is
// always rolled back. go-sql-driver maps BeginTx's ReadOnly option to that
// statement.
func readOnly(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("starting read-only transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return fn(tx)
}

// statementKind classifies the first keyword of a statement.
type statementKind int

const (
	notARead statementKind = iota
	// readSelect statements can be planned with EXPLAIN FORMAT=JSON.
	readSelect
	// readShow statements are SHOW and DESCRIBE style reads; MySQL's
	// EXPLAIN does not accept them, so PlanQuery cannot name what they
	// touch and the host falls back to a whole-source grant.
	readShow
)

var selectLikeKeywords = map[string]bool{"select": true, "with": true, "table": true, "values": true}
var showLikeKeywords = map[string]bool{"show": true, "desc": true, "describe": true}

// classifyStatement reports whether a statement's first keyword, after any
// comments and opening parentheses, is a read, and if so which kind. A
// WITH clause can precede UPDATE or DELETE as well as SELECT in MySQL, so
// a leading WITH alone does not prove the statement reads only; the READ
// ONLY transaction is what actually refuses that write, and this check is
// a fast, clear refusal for the ordinary case rather than the last line of
// defence.
func classifyStatement(statement string) statementKind {
	rest := statement
	for {
		rest = strings.TrimLeft(rest, " \t\r\n(")
		switch {
		case strings.HasPrefix(rest, "--"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				return notARead
			}
			rest = rest[end+1:]
		case strings.HasPrefix(rest, "#"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				return notARead
			}
			rest = rest[end+1:]
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest, "*/")
			if end < 0 {
				return notARead
			}
			rest = rest[end+2:]
		default:
			end := strings.IndexFunc(rest, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
			})
			if end < 0 {
				end = len(rest)
			}
			word := strings.ToLower(rest[:end])
			switch {
			case selectLikeKeywords[word]:
				return readSelect
			case showLikeKeywords[word]:
				return readShow
			default:
				return notARead
			}
		}
	}
}

// refusedClausePattern matches constructs that reach outside what the READ
// ONLY transaction stops. INTO OUTFILE and INTO DUMPFILE write a file
// rather than a table, which a read-only transaction does not prevent, and
// MySQL only refuses FOR UPDATE in such a transaction, not FOR SHARE or
// LOCK IN SHARE MODE.
var refusedClausePattern = regexp.MustCompile(`(?i)\bINTO\s+(OUTFILE|DUMPFILE)\b|\bFOR\s+UPDATE\b|\bLOCK\s+IN\s+SHARE\s+MODE\b|\bFOR\s+SHARE\b`)

func hasRefusedClause(statement string) bool {
	return refusedClausePattern.MatchString(maskLiteralsAndComments(statement))
}

// referencesUserVariable reports whether a statement reads or assigns a
// user variable (@name), anywhere in the statement, which is refused
// outright: a user variable lives on the session for as long as the
// connection does, not just for the transaction that set it, so a value one
// call stashes in one, such as SELECT @leak := secret_col FROM secret, is
// still there for a later, unrelated call to read back with a bare SELECT
// @leak, on whatever pooled connection it happens to be handed. A system
// variable (@@name) carries no caller data, so reading one is left alone;
// setting one is a SET statement, already refused by classifyStatement
// before this is ever reached. @ inside a comment, a string or a quoted
// identifier is not a reference, so this scans the same masked statement
// hasRefusedClause does.
func referencesUserVariable(statement string) bool {
	masked := maskLiteralsAndComments(statement)
	for i := 0; i < len(masked); i++ {
		if masked[i] != '@' {
			continue
		}
		if i+1 < len(masked) && masked[i+1] == '@' {
			i++ // a system variable's second '@': skip past the pair
			continue
		}
		return true
	}
	return false
}

// maskLiteralsAndComments blanks the content of comments, string literals
// and quoted identifiers in a statement, keeping its overall shape, so a
// scan for a clause such as INTO OUTFILE or a call to a particular function
// name never matches text that is only there because it sits inside a
// comment, a quoted name or a string value.
func maskLiteralsAndComments(statement string) string {
	var out strings.Builder
	out.Grow(len(statement))
	for i := 0; i < len(statement); {
		switch {
		case strings.HasPrefix(statement[i:], "--"):
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return out.String()
			}
			out.WriteByte('\n')
			i += end + 1
		case statement[i] == '#':
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return out.String()
			}
			out.WriteByte('\n')
			i += end + 1
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return out.String()
			}
			out.WriteByte(' ')
			i += end + 4
		case statement[i] == '\'' || statement[i] == '"' || statement[i] == '`':
			quote := statement[i]
			out.WriteByte(quote)
			i++
			for i < len(statement) {
				if quote != '`' && statement[i] == '\\' && i+1 < len(statement) {
					i += 2
					continue
				}
				if statement[i] == quote {
					if i+1 < len(statement) && statement[i+1] == quote {
						i += 2
						continue
					}
					break
				}
				i++
			}
			if i < len(statement) {
				out.WriteByte(quote)
				i++
			}
		default:
			out.WriteByte(statement[i])
			i++
		}
	}
	return out.String()
}

// tableRef is a table MySQL actually read, as the optimizer trace names it.
type tableRef struct {
	db    string
	table string
}

// parseTraceTableName parses the optimizer trace's own form for a table,
// [`db`.]`table`, reporting false for the placeholder MySQL substitutes for
// a derived table or a UNION's own result, neither of which has a real
// table name of its own. Only the leading [db.]table is read; whatever
// follows, an alias, a PARTITION clause, an index hint, or several of
// these together, is deliberately left unparsed. That ordering is not an
// assumption: EXPLAIN FORMAT=JSON's own table_name field reports a table's
// alias rather than its name once one is given, which is unsafe to resolve
// by parsing the statement's own alias back out, because an alias chosen
// to equal another real table's name would then read as that table.
// Checked directly against MySQL 8, [db.]table always comes first in this
// trace field regardless of what follows it, including every one of those
// constructs and combinations of them, which is what lets this function
// stop after the first one or two identifiers rather than needing to
// understand everything that can follow a table reference.
func parseTraceTableName(s string) (db, table string, ok bool) {
	i := 0
	readIdent := func() (string, bool) {
		if i >= len(s) || s[i] != '`' {
			return "", false
		}
		i++
		var b strings.Builder
		for i < len(s) {
			if s[i] == '`' {
				if i+1 < len(s) && s[i+1] == '`' {
					b.WriteByte('`')
					i += 2
					continue
				}
				i++
				return b.String(), true
			}
			b.WriteByte(s[i])
			i++
		}
		return "", false
	}

	name, ok := readIdent()
	if !ok {
		return "", "", false
	}
	if i < len(s) && s[i] == '.' {
		i++
		second, ok := readIdent()
		if !ok {
			return "", "", false
		}
		db, name = name, second
	}
	if name == "" {
		return "", "", false
	}
	return db, name, true
}

// walkTraceTables recursively collects every table_dependencies entry in an
// optimizer trace, at any nesting level. A view or a mergeable derived table
// is folded into the surrounding query before EXPLAIN ever sees it, taking
// its own alias with it, and MySQL's plan then names that alias rather than
// the table it stands for; join_preparation and join_optimization nest once
// per merged subquery, each with its own table_dependencies, which is the
// only place MySQL keeps the table's own name once that happens. A view
// MySQL cannot merge, because its own definition needs a temporary table,
// still carries its own name here, and is resolved like any other table
// rather than expanded, so the result may name both a view and the table
// behind it; that is a broader plan than the statement strictly needs, not
// a wrong one.
func walkTraceTables(v any, into map[tableRef]bool) {
	switch v := v.(type) {
	case map[string]any:
		if deps, ok := v["table_dependencies"].([]any); ok {
			for _, d := range deps {
				dep, ok := d.(map[string]any)
				if !ok {
					continue
				}
				raw, _ := dep["table"].(string)
				if db, table, ok := parseTraceTableName(raw); ok {
					into[tableRef{db: db, table: table}] = true
				}
			}
		}
		for _, child := range v {
			walkTraceTables(child, into)
		}
	case []any:
		for _, child := range v {
			walkTraceTables(child, into)
		}
	}
}

// explainReferencesAnyTable reports whether EXPLAIN's plan touches at least
// one table. It is used only as a cross-check against the optimizer trace:
// if EXPLAIN finds a table but the trace resolves none, the trace's shape
// has not matched what this plugin expects, and the statement needs a
// whole-source grant rather than an empty, and wrong, plan.
func explainReferencesAnyTable(v any) bool {
	switch v := v.(type) {
	case map[string]any:
		if name, _ := v["table_name"].(string); name != "" && !strings.HasPrefix(name, "<") {
			return true
		}
		for _, child := range v {
			if explainReferencesAnyTable(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if explainReferencesAnyTable(child) {
				return true
			}
		}
	}
	return false
}

// isView reports whether db.table is a view, so the plan files it as a View
// asset the way discovery does rather than as a Table.
func isView(ctx context.Context, tx *sql.Tx, db, table string) bool {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var tableType string
	err := tx.QueryRowContext(queryCtx,
		"SELECT TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?",
		db, table).Scan(&tableType)
	return err == nil && strings.EqualFold(tableType, "VIEW")
}

// PlanQuery asks MySQL what a statement would read via EXPLAIN FORMAT=JSON
// plus the optimizer trace that planning it leaves behind, and maps each
// table to the same MRN discovery gives it, so grants written against
// catalog assets bind here too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	kind := classifyStatement(req.Statement)
	if kind == notARead {
		return nil, errReadOnly
	}
	if hasRefusedClause(req.Statement) || referencesUserVariable(req.Statement) {
		return nil, errReadOnly
	}
	if kind == readShow {
		return nil, nil
	}

	db, config, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	plan := &pluginsdk.QueryPlan{}
	err = readOnly(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET optimizer_trace = "enabled=on"`); err != nil {
			return fmt.Errorf("enabling optimizer trace: %w", err)
		}
		defer func() { _, _ = tx.ExecContext(ctx, `SET optimizer_trace = "enabled=off"`) }()

		var planJSON []byte
		if err := tx.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+req.Statement).Scan(&planJSON); err != nil {
			return fmt.Errorf("planning statement: %w", err)
		}
		var explained any
		if err := json.Unmarshal(planJSON, &explained); err != nil {
			return fmt.Errorf("parsing plan: %w", err)
		}

		var traceJSON string
		if err := tx.QueryRowContext(ctx, "SELECT TRACE FROM information_schema.OPTIMIZER_TRACE").Scan(&traceJSON); err != nil {
			return fmt.Errorf("reading optimizer trace: %w", err)
		}
		var traced any
		if err := json.Unmarshal([]byte(traceJSON), &traced); err != nil {
			return fmt.Errorf("parsing optimizer trace: %w", err)
		}

		refs := map[tableRef]bool{}
		walkTraceTables(traced, refs)

		if len(refs) == 0 {
			if explainReferencesAnyTable(explained) {
				plan.Incomplete = true
				log.Warn().Str("statement", req.Statement).Msg("EXPLAIN found a table the optimizer trace did not resolve; requiring access to the whole source")
			}
			return nil
		}

		seen := map[string]bool{}
		for ref := range refs {
			schema := ref.db
			if schema == "" {
				schema = config.Database
			}
			assetType := "Table"
			if isView(ctx, tx, schema, ref.table) {
				assetType = "View"
			}
			tableMRN := assetMRN(config, assetType, schema, ref.table)
			if !seen[tableMRN] {
				seen[tableMRN] = true
				plan.ReferencedMRNs = append(plan.ReferencedMRNs, tableMRN)
			}
		}

		// A stored function can run a query of its own that reads tables
		// this plan never sees, so a table grant only means something if
		// the plan names everything the statement reads.
		if calls := opaqueCalls(req.Statement); len(calls) > 0 {
			plan.Incomplete = true
			log.Info().Strs("functions", calls).Msg("Statement calls functions that may read unlisted tables; it needs access to the whole source")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// ExecuteQuery runs the statement in a read-only transaction and streams
// rows back in chunks, the first carrying the column schema.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if classifyStatement(req.Statement) == notARead || hasRefusedClause(req.Statement) || referencesUserVariable(req.Statement) {
		return errReadOnly
	}

	db, _, err := s.connect(ctx, raw)
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
			values[i] = convertMySQLValue(v)
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
