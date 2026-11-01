package clickhouse

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one connection per config, keyed by a hash of it.
//
// Every statement runs with the readonly setting, which ClickHouse enforces
// server-side: it refuses writes, DDL and changes to settings, and the
// native protocol accepts one statement per call, so a statement can
// neither write nor smuggle a second statement past it. readonly does not,
// however, stop table functions that read data outside ClickHouse (url,
// s3, remote, mysql, and the like), so this plugin refuses those outright
// before a statement ever reaches the server. The ClickHouse user's own
// grants remain the hard boundary underneath all of this.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// queryExecutionTimeoutSeconds bounds how long a planned or executed statement may run on the server, matching the limit Discover's own connection already uses.
const queryExecutionTimeoutSeconds = 60

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served by the same plugin process are queried at once, so each keeps its own connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	conn     clickhouse.Conn
	lastUsed time.Time
}

// connect returns a live connection for the config, reusing the cached one when it still answers a ping.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (clickhouse.Conn, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	hash := configHash(config)

	if c := s.query.get(hash); c != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.conn.Ping(pingCtx)
		cancel()
		if err == nil {
			return c.conn, nil
		}
		log.Warn().Err(err).Msg("Cached ClickHouse connection failed ping, reconnecting")
		s.query.drop(hash, c)
	}

	// Opening can take seconds, so it happens outside the lock and never holds up queries to other sources.
	conn, err := openQueryConn(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connecting to ClickHouse: %w", err)
	}
	return s.query.put(hash, &cachedConn{conn: conn}).conn, nil
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
		go func() { _ = fresh.conn.Close() }()
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
		go func() { _ = stale.conn.Close() }()
	}
}

// closeIdle closes connections unused for connIdleTTL. Close waits for queries still running, so it runs in the background.
func (q *queryConns) closeIdle(now time.Time) {
	for hash, c := range q.conns {
		if now.Sub(c.lastUsed) > connIdleTTL {
			delete(q.conns, hash)
			go func() { _ = c.conn.Close() }()
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

func openQueryConn(ctx context.Context, config *Config) (clickhouse.Conn, error) {
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.User,
			Password: config.Password,
		},
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 2 * time.Minute,
	}
	if config.Secure {
		options.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("opening connection: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("pinging ClickHouse: %w", err)
	}
	return conn, nil
}

// readOnlyContext returns ctx carrying the settings that make ClickHouse
// refuse anything but a read. readonly=1 blocks writes, DDL and setting
// changes server-side; max_result_rows with result_overflow_mode=break
// stops the server once maxRows is reached rather than raising an error,
// backing up the limit the caller already applies as it streams rows.
func readOnlyContext(ctx context.Context, maxRows int64) context.Context {
	settings := clickhouse.Settings{
		"readonly":           1,
		"max_execution_time": queryExecutionTimeoutSeconds,
	}
	if maxRows > 0 {
		settings["max_result_rows"] = maxRows
		settings["result_overflow_mode"] = "break"
	}
	return clickhouse.Context(ctx, clickhouse.WithSettings(settings))
}

// readKeywords are the statements that only read. Anything else is refused before a statement reaches ClickHouse, because PlanQuery must have no side effects and EXPLAIN itself only accepts SELECT.
var readKeywords = map[string]bool{"select": true, "with": true}

// sqlCode returns statement with every comment and string literal blanked out, so pattern matching never mistakes commented-out or quoted SQL for the real thing. The second return value is false when a comment or literal is never closed, which callers treat as a statement they cannot vouch for.
func sqlCode(statement string) (string, bool) {
	var code strings.Builder
	for i := 0; i < len(statement); i++ {
		switch {
		case strings.HasPrefix(statement[i:], "--"), statement[i] == '#':
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return code.String(), false
			}
			i += end
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return code.String(), false
			}
			i += end + 3
			code.WriteByte(' ')
		case statement[i] == '\'':
			closed := false
			for i++; i < len(statement); i++ {
				if statement[i] == '\\' {
					i++
					continue
				}
				if statement[i] == '\'' {
					if i+1 < len(statement) && statement[i+1] == '\'' {
						i++
						continue
					}
					closed = true
					break
				}
			}
			if !closed {
				return code.String(), false
			}
			code.WriteString("''")
		default:
			code.WriteByte(statement[i])
		}
	}
	return code.String(), true
}

// isReadStatement reports whether a statement's first keyword, after any comments and opening parentheses, is a read.
func isReadStatement(statement string) bool {
	code, ok := sqlCode(statement)
	if !ok {
		return false
	}
	rest := strings.TrimLeft(code, " \t\r\n(")
	end := strings.IndexFunc(rest, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	})
	if end < 0 {
		end = len(rest)
	}
	return readKeywords[strings.ToLower(rest[:end])]
}

// externalTableFunctionCall matches ClickHouse table functions that read data from outside ClickHouse itself, such as an arbitrary URL, a file on disk, or another database entirely. ClickHouse function names are case sensitive, but the match is case insensitive anyway so a miscased call is refused rather than let through on a technicality.
var externalTableFunctionCall = regexp.MustCompile(`(?i)\b(url|s3|file|remote|remoteSecure|cluster|mysql|postgresql|odbc|jdbc|hdfs|azureBlobStorage|input)\s*\(`)

// callsExternalTableFunction reports whether a statement calls a table function that reads outside ClickHouse. These are refused outright rather than merely marked incomplete, because planning must never risk a side effect such as dialling a URL.
func callsExternalTableFunction(statement string) bool {
	code, ok := sqlCode(statement)
	if !ok {
		// An unresolved comment or literal might be hiding a call, so the check fails closed.
		return true
	}
	return externalTableFunctionCall.MatchString(code)
}

// dictionaryFunctionCall matches ClickHouse's dictGet family, which reads a dictionary by name rather than a table the plan can resolve.
var dictionaryFunctionCall = regexp.MustCompile(`(?i)\bdict[A-Za-z]*\s*\(`)

// callsDictionaryFunction reports whether a statement reads a dictionary, which PlanQuery cannot name as a table and so marks the plan incomplete instead of refusing it outright.
func callsDictionaryFunction(statement string) bool {
	code, ok := sqlCode(statement)
	if !ok {
		return true
	}
	return dictionaryFunctionCall.MatchString(code)
}

// relation is a database and table name as ClickHouse's query plan names it.
type relation struct {
	database string
	table    string
}

// planSummary is what a statement's plan says it reads.
type planSummary struct {
	relations []relation
	// unresolvedNodeTypes are plan node types that read storage the plan cannot name as a specific table, such as a non-MergeTree engine or a table function. The host then needs a grant on the whole source, because grants on the listed tables cannot cover what the statement really reads.
	unresolvedNodeTypes []string
}

// parsePlan walks ClickHouse's EXPLAIN PLAN (FORMAT JSON) output and
// collects every MergeTree-family table it scans. Views, normal and
// materialised, are expanded by the planner into the tables behind them, so
// a statement over a view names those tables rather than the view itself.
// Any other storage read, such as a non-MergeTree engine or a table
// function the planner did not already refuse, cannot be named as a table
// from this plan alone and is recorded as unresolved.
func parsePlan(planJSON []byte) (*planSummary, error) {
	var explained []struct {
		Plan map[string]any `json:"Plan"`
	}
	if err := json.Unmarshal(planJSON, &explained); err != nil {
		return nil, fmt.Errorf("parsing plan: %w", err)
	}

	summary := &planSummary{}
	seenRelation := map[relation]bool{}
	seenNodeType := map[string]bool{}
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		nodeType, _ := node["Node Type"].(string)
		switch {
		case nodeType == "ReadFromMergeTree":
			if desc, _ := node["Description"].(string); desc != "" {
				if db, table, ok := strings.Cut(desc, "."); ok {
					r := relation{database: db, table: table}
					if !seenRelation[r] {
						seenRelation[r] = true
						summary.relations = append(summary.relations, r)
					}
				}
			}
		case strings.HasPrefix(nodeType, "ReadFrom"):
			if !seenNodeType[nodeType] {
				seenNodeType[nodeType] = true
				summary.unresolvedNodeTypes = append(summary.unresolvedNodeTypes, nodeType)
			}
		}
		if children, ok := node["Plans"].([]any); ok {
			for _, child := range children {
				if childMap, ok := child.(map[string]any); ok {
					walk(childMap)
				}
			}
		}
	}
	for _, e := range explained {
		walk(e.Plan)
	}
	return summary, nil
}

// relationMRN reproduces the MRN discovery gives a table, so grants written against catalog assets bind on the query path too. ReadFromMergeTree only ever backs a Table asset; a view never reaches this node because the planner expands it into its underlying tables first.
func relationMRN(r relation) string {
	return mrn.New("Table", "ClickHouse", r.database+"."+r.table)
}

// PlanQuery asks ClickHouse what a statement would read via EXPLAIN PLAN,
// so the server does the SQL parsing, and maps each table to the MRN
// discovery gives it so grants written against catalog assets bind here
// too. A statement is refused outright, before ClickHouse ever sees it, if
// its first keyword is not a read or it calls a table function that reads
// outside ClickHouse.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if !isReadStatement(req.Statement) {
		return nil, errReadOnly
	}
	if callsExternalTableFunction(req.Statement) {
		return nil, errReadOnly
	}

	conn, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	var planJSON string
	if err := conn.QueryRow(readOnlyContext(ctx, req.MaxRows), "EXPLAIN PLAN json=1 "+req.Statement).Scan(&planJSON); err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}

	summary, err := parsePlan([]byte(planJSON))
	if err != nil {
		return nil, err
	}

	plan := &pluginsdk.QueryPlan{Incomplete: len(summary.unresolvedNodeTypes) > 0 || callsDictionaryFunction(req.Statement)}
	if len(summary.unresolvedNodeTypes) > 0 {
		log.Info().Strs("node_types", summary.unresolvedNodeTypes).Msg("Statement reads storage the plan cannot name as a table; it needs access to the whole source")
	}
	for _, r := range summary.relations {
		plan.ReferencedMRNs = append(plan.ReferencedMRNs, relationMRN(r))
	}
	return plan, nil
}

// ExecuteQuery runs the statement read-only and streams rows back in
// chunks, the first carrying the column schema. Marmot always plans a
// statement before executing it, so writes and external table functions
// have already been refused by PlanQuery; the same checks run here too, so
// ExecuteQuery is never the only thing standing between a write and
// ClickHouse.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if !isReadStatement(req.Statement) {
		return errReadOnly
	}
	if callsExternalTableFunction(req.Statement) {
		return errReadOnly
	}

	conn, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	rows, err := conn.Query(readOnlyContext(ctx, req.MaxRows), req.Statement)
	if err != nil {
		return fmt.Errorf("executing statement: %w", err)
	}
	defer rows.Close()

	return streamRows(rows, req.MaxRows, emit)
}

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at maxRows when it is set.
func streamRows(rows driver.Rows, maxRows int64, emit func(pluginsdk.QueryResultChunk) error) error {
	colTypes := rows.ColumnTypes()
	columns := make([]pluginsdk.QueryColumn, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = pluginsdk.QueryColumn{Name: ct.Name(), Type: ct.DatabaseTypeName()}
	}

	chunk := pluginsdk.QueryResultChunk{Columns: columns}
	var sent int64
	for rows.Next() {
		scanDests := make([]any, len(colTypes))
		for i, ct := range colTypes {
			scanDests[i] = reflect.New(ct.ScanType()).Interface()
		}
		if err := rows.Scan(scanDests...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}

		row := make([]any, len(colTypes))
		for i, dest := range scanDests {
			row[i] = convertClickHouseValue(reflect.ValueOf(dest).Elem().Interface())
		}

		chunk.Rows = append(chunk.Rows, row)
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
