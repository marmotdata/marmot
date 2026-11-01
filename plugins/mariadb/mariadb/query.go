package mariadb

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
// never reads the Source's stored config (which the SDK rewrites on every
// call) and instead parses the raw config afresh and keeps one connection
// pool per config, keyed by a hash of it.
//
// MariaDB's READ ONLY transaction blocks writes to transactional tables, but
// not DDL (which commits implicitly) and not SELECT ... INTO OUTFILE, so the
// hard boundary here is classifyRead, checked before a statement ever
// reaches the server; the read-only transaction is a second line of defence
// rather than the mechanism itself. The DSN never sets MultiStatements
// either, so the server itself refuses a second statement tacked onto one
// call.
//
// classifyRead also refuses any reference to a user variable (@name), the
// one piece of session state a statement can both set and have read back:
// it survives a rolled-back transaction for as long as the physical
// connection does, so a pooled connection handed to a second, unrelated
// caller could otherwise answer with whatever the first left behind.
// openQueryDB additionally never keeps a connection idle for reuse, which
// removes that whole channel rather than relying on classifyRead alone
// having thought of every way into it.
//
// PlanQuery does not parse the statement itself to find the tables it
// reads: the statement's own text promises nothing about where an alias
// sits relative to an index hint, a PARTITION clause or a comment, and an
// alias that happens to name a different real table would then resolve to
// that table instead of the one actually read. It instead asks MariaDB to
// plan the statement and reads back the optimiser's own rewritten form of
// it from SHOW WARNINGS, which is fully qualified, consistently quoted and
// has already replaced anything the statement's own text could get wrong.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// connIdleTTL is how long a connection pool can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection pool per source config. Several sources served by the same plugin process are queried at once, so each keeps its own pool rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	db       *sql.DB
	lastUsed time.Time
}

// connect returns the connection pool for the config, plus the config itself
// so callers can read QueryDatabase, opening one if this is the first call
// for it.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*sql.DB, *Config, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, nil, err
	}
	hash := configHash(config)

	if db := s.query.get(hash); db != nil {
		return db, config, nil
	}

	// Opening can take seconds, so it happens outside the lock and never holds up queries to other sources.
	db, err := openQueryDB(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to MariaDB: %w", err)
	}
	return s.query.put(hash, db), config, nil
}

func (q *queryConns) get(hash string) *sql.DB {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.closeIdle(now)
	if c, ok := q.conns[hash]; ok {
		c.lastUsed = now
		return c.db
	}
	return nil
}

// put caches a newly opened pool, or keeps the one another call opened for the same config meanwhile and closes this one.
func (q *queryConns) put(hash string, fresh *sql.DB) *sql.DB {
	q.mu.Lock()
	defer q.mu.Unlock()
	if c, ok := q.conns[hash]; ok {
		go func() { _ = fresh.Close() }()
		c.lastUsed = time.Now()
		return c.db
	}
	if q.conns == nil {
		q.conns = make(map[string]*cachedConn)
	}
	q.conns[hash] = &cachedConn{db: fresh, lastUsed: time.Now()}
	return fresh
}

// closeIdle closes pools unused for connIdleTTL. Close waits for queries still running, so it runs in the background.
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

// openQueryDB opens a connection pool against the configured query database.
// MaxIdleConns is 0, so a connection is never kept around for a later query
// to reuse: MariaDB keeps user variables, and any other session state, on
// the physical connection for as long as it stays open, surviving a rolled
// back transaction, so a connection handed to a second, unrelated caller
// could otherwise answer with whatever the first left behind. MariaDB has
// no equivalent of MySQL's lightweight RESET CONNECTION, and
// go-sql-driver/mysql's ResetSession is a liveness check, not a reset, so
// closing the connection is the only way to guarantee the next query starts
// from a clean session. The cost is a fresh TCP handshake and
// authentication on every query rather than a warm connection; data queries
// run far less often than a discovery crawl, so that trade is worth making.
// buildDSN never turns MultiStatements on either, which is what stops a
// second statement from riding in on the back of a read.
func openQueryDB(ctx context.Context, config *Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", buildDSN(config, config.QueryDatabase))
	if err != nil {
		return nil, fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(2 * time.Minute)

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().Str("host", config.Host).Str("database", config.QueryDatabase).Msg("Opened MariaDB connection pool for data queries")
	return db, nil
}

// readOnlyTx runs fn inside a READ ONLY transaction that is always rolled
// back. It is a second line of defence, not the enforcement mechanism:
// classifyRead has already refused anything the transaction's access mode
// would not actually stop.
func readOnlyTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("starting read-only transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return fn(tx)
}

// readKeywords are the statements that only read. MariaDB has no shorthand
// "TABLE tbl_name" statement the way some engines do, so it is left out.
var readKeywords = map[string]bool{"select": true, "with": true, "values": true}

// forbiddenReadModifier matches clauses that turn an otherwise read-only
// statement into one that writes, either to the database (FOR UPDATE, LOCK
// IN SHARE MODE take locks a read-only transaction refuses) or to the
// filesystem (INTO OUTFILE/DUMPFILE, which a read-only transaction does not
// refuse at all).
var forbiddenReadModifier = regexp.MustCompile(`(?i)\bFOR\s+UPDATE\b|\bLOCK\s+IN\s+SHARE\s+MODE\b|\bINTO\s+(?:OUTFILE|DUMPFILE)\b`)

// classifyRead refuses anything but a single plain read, before the
// statement ever reaches MariaDB, and returns the statement with its
// literals and comments blanked out so a caller that needs to look at its
// structure again, such as for a function call, does not have to reparse
// it. PlanQuery and ExecuteQuery must have no side effects for statements
// they refuse, and MariaDB's READ ONLY transaction does not cover DDL,
// SELECT ... INTO OUTFILE or a user variable surviving onto the next query
// on the same pooled connection, so this check is the actual boundary
// rather than the transaction.
func classifyRead(statement string) (code string, err error) {
	code, ok := stripLiteralsAndComments(statement)
	if !ok {
		return "", errReadOnly
	}
	if hasExtraStatements(code) {
		return "", errReadOnly
	}

	rest := strings.TrimLeft(code, " \t\r\n(")
	end := strings.IndexFunc(rest, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	})
	if end < 0 {
		end = len(rest)
	}
	if !readKeywords[strings.ToLower(rest[:end])] {
		return "", errReadOnly
	}

	if forbiddenReadModifier.MatchString(code) {
		return "", errReadOnly
	}
	if referencesUserVariable(code) {
		return "", errReadOnly
	}
	return code, nil
}

// referencesUserVariable reports whether code reads or assigns a user
// variable (@name): "SELECT @leak := (...)" and a later "SELECT @leak" are
// both plain reads as far as classifyRead's keyword check is concerned, but
// together they are a side channel between two callers sharing a pooled
// connection, since the variable outlives a rolled-back transaction. "@@"
// reads a system variable instead, which nothing a single statement run
// through Marmot can set, so it is left alone.
func referencesUserVariable(code string) bool {
	for i := 0; i < len(code); i++ {
		if code[i] != '@' {
			continue
		}
		if i+1 < len(code) && code[i+1] == '@' {
			i++
			continue
		}
		return true
	}
	return false
}

// stripLiteralsAndComments blanks out every string literal, quoted
// identifier and comment in a statement, keeping the surrounding structure
// so a keyword or punctuation check never trips on what is inside one of
// them. ok is false when a literal or comment is left open, and the
// statement is then treated as unsafe rather than guessed at.
func stripLiteralsAndComments(statement string) (code string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(statement); i++ {
		switch {
		case strings.HasPrefix(statement[i:], "--"):
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return "", false
			}
			i += end
			b.WriteByte('\n')
		case statement[i] == '#':
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return "", false
			}
			i += end
			b.WriteByte('\n')
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return "", false
			}
			i += end + 3
			b.WriteByte(' ')
		case statement[i] == '\'' || statement[i] == '"':
			quote := statement[i]
			closed := false
			for i++; i < len(statement); i++ {
				if statement[i] == '\\' && i+1 < len(statement) {
					i++
					continue
				}
				if statement[i] == quote {
					if i+1 < len(statement) && statement[i+1] == quote {
						i++
						continue
					}
					closed = true
					break
				}
			}
			if !closed {
				return "", false
			}
			b.WriteByte(quote)
			b.WriteByte(quote)
		case statement[i] == '`':
			closed := false
			for i++; i < len(statement); i++ {
				if statement[i] == '`' {
					if i+1 < len(statement) && statement[i+1] == '`' {
						i++
						continue
					}
					closed = true
					break
				}
			}
			if !closed {
				return "", false
			}
			b.WriteString("``")
		default:
			b.WriteByte(statement[i])
		}
	}
	return b.String(), true
}

// hasExtraStatements reports whether code, once blanked of literals and
// comments, holds anything but whitespace after its first semicolon. A
// single trailing semicolon is fine; anything beyond it is a second
// statement, which the engine would refuse anyway since the connection
// never sets MultiStatements.
func hasExtraStatements(code string) bool {
	i := strings.IndexByte(code, ';')
	if i < 0 {
		return false
	}
	return strings.TrimSpace(code[i+1:]) != ""
}

// loadObjects lists every table, view and sequence of the query database, so
// a canonical table reference can be checked against what is actually
// catalogued rather than guessed at.
func loadObjects(ctx context.Context, tx *sql.Tx, database string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?", database)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	objects := make(map[string]string)
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return nil, fmt.Errorf("scanning table row: %w", err)
		}
		objects[strings.ToLower(name)] = tableType
	}
	return objects, rows.Err()
}

// loadFunctionNames lists the query database's stored functions, the only
// routines MariaDB allows inside a SELECT's expression list.
func loadFunctionNames(ctx context.Context, tx *sql.Tx, database string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT ROUTINE_NAME FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? AND ROUTINE_TYPE = 'FUNCTION'", database)
	if err != nil {
		return nil, fmt.Errorf("listing functions: %w", err)
	}
	defer rows.Close()

	names := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning function row: %w", err)
		}
		names[strings.ToLower(name)] = true
	}
	return names, rows.Err()
}

// callsKnownFunction reports whether code calls any of the query database's
// stored functions. MariaDB's EXPLAIN never mentions a statement's
// expressions, so a call to a function that runs its own query never shows
// up in the plan at all; the statement text is the only place left to look
// for one.
func callsKnownFunction(code string, functions map[string]bool) bool {
	if len(functions) == 0 {
		return false
	}
	names := make([]string, 0, len(functions))
	for name := range functions {
		names = append(names, regexp.QuoteMeta(name))
	}
	pattern := regexp.MustCompile(`(?i)\b(?:` + strings.Join(names, "|") + `)\s*\(`)
	return pattern.MatchString(code)
}

// relation is a table, view or sequence a statement reads, named exactly as
// MariaDB's optimiser names it once every identifier is fully qualified.
type relation struct {
	database string
	table    string
}

// canonicalRelationPattern finds a table reference in the optimiser's
// rewritten statement: always `db`.`table`, immediately after FROM, JOIN or
// STRAIGHT_JOIN. The rewrite is machine-generated, so every table is
// consistently double-backtick-quoted and fully qualified, with nothing of
// the original statement's aliases, index hints, PARTITION clauses or
// comments surviving into it. A column reference is always three parts
// (db.table.column), never two, so it can never be mistaken for one of
// these.
var canonicalRelationPattern = regexp.MustCompile(
	"(?i)\\b(?:from|join|straight_join)\\s+`((?:[^`]|``)+)`\\.`((?:[^`]|``)+)`",
)

// canonicalRelations asks MariaDB to plan the statement, then reads back
// the optimiser's own rewritten form of it from SHOW WARNINGS (Note 1003).
// That text is the one place a statement's real table references can be
// read with certainty: a view the optimiser can merge is already replaced
// by the base tables it reads, a comma join is already rewritten as an
// explicit join, and every table is fully qualified and quoted the same
// way, which the statement's own text, written and formatted however its
// author liked, cannot promise. EXPLAIN alone does not populate SHOW
// WARNINGS; EXTENDED is required.
func canonicalRelations(ctx context.Context, tx *sql.Tx, statement string) ([]relation, error) {
	if _, err := tx.ExecContext(ctx, "EXPLAIN EXTENDED "+statement); err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}

	rows, err := tx.QueryContext(ctx, "SHOW WARNINGS")
	if err != nil {
		return nil, fmt.Errorf("reading the optimiser's rewritten statement: %w", err)
	}
	defer rows.Close()

	var rewritten strings.Builder
	for rows.Next() {
		var level, message string
		var code int
		if err := rows.Scan(&level, &code, &message); err != nil {
			return nil, fmt.Errorf("scanning planner output: %w", err)
		}
		// 1003 is the rewritten statement; anything else is a genuine
		// warning about the statement, not SQL to parse.
		if code == 1003 {
			rewritten.WriteByte(' ')
			rewritten.WriteString(message)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating planner output: %w", err)
	}

	var relations []relation
	seen := map[relation]bool{}
	for _, m := range canonicalRelationPattern.FindAllStringSubmatch(rewritten.String(), -1) {
		r := relation{
			database: strings.ReplaceAll(m[1], "``", "`"),
			table:    strings.ReplaceAll(m[2], "``", "`"),
		}
		if !seen[r] {
			seen[r] = true
			relations = append(relations, r)
		}
	}
	return relations, nil
}

// resolveRelations maps a statement's canonical table references onto asset
// MRNs, the way discovery itself names them. A reference to a database
// other than the one being queried is not something this source's
// catalogue can vouch for, and a reference to a name the catalogue does not
// currently list cannot be guessed at either; either marks the plan
// incomplete, since naming the wrong table in a grant check would be worse
// than asking for a grant on the whole source.
func resolveRelations(relations []relation, database string, objects map[string]string) (mrns []string, incomplete bool) {
	seen := map[string]bool{}
	for _, r := range relations {
		if !strings.EqualFold(r.database, database) {
			incomplete = true
			continue
		}

		tableType, found := objects[strings.ToLower(r.table)]
		if !found {
			incomplete = true
			continue
		}
		assetType, _, _, found := objectKind(tableType)
		if !found {
			incomplete = true
			continue
		}

		m := assetMRN(assetType, r.table)
		if !seen[m] {
			seen[m] = true
			mrns = append(mrns, m)
		}
	}
	return mrns, incomplete
}

// PlanQuery asks MariaDB what a statement would read by planning it and
// reading back the optimiser's own rewritten form of it, and maps each
// table that names back to the MRN discovery gives it so grants written
// against catalogued assets bind here too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	code, err := classifyRead(req.Statement)
	if err != nil {
		return nil, err
	}

	db, config, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	var plan *pluginsdk.QueryPlan
	err = readOnlyTx(ctx, db, func(tx *sql.Tx) error {
		objects, err := loadObjects(ctx, tx, config.QueryDatabase)
		if err != nil {
			return err
		}
		functions, err := loadFunctionNames(ctx, tx, config.QueryDatabase)
		if err != nil {
			return err
		}

		relations, err := canonicalRelations(ctx, tx, req.Statement)
		if err != nil {
			return err
		}

		mrns, incomplete := resolveRelations(relations, config.QueryDatabase, objects)

		// A table grant only means something if the statement's referenced
		// tables cover everything it reads; a function that runs its own
		// query reads tables this plan never sees, so access to the whole
		// source is needed instead.
		if callsKnownFunction(code, functions) {
			incomplete = true
			log.Info().Str("database", config.QueryDatabase).Msg("Statement calls a stored function that may read unlisted tables; it needs access to the whole source")
		}

		plan = &pluginsdk.QueryPlan{ReferencedMRNs: mrns, Incomplete: incomplete}
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
	if _, err := classifyRead(req.Statement); err != nil {
		return err
	}

	db, _, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	return readOnlyTx(ctx, db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, req.Statement)
		if err != nil {
			return fmt.Errorf("executing statement: %w", err)
		}
		defer rows.Close()

		return streamRows(rows, req.MaxRows, emit)
	})
}

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at
// maxRows when it is set, and always emits a final chunk, even empty, so
// the schema reaches the host for zero-row results.
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
		values := make([]interface{}, len(columnNames))
		pointers := make([]interface{}, len(columnNames))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		for i, v := range values {
			values[i] = convertValue(v)
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

	if len(chunk.Rows) > 0 || chunk.Columns != nil {
		return emit(chunk)
	}
	return nil
}
