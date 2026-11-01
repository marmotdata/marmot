package duckdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	duckdbdriver "github.com/duckdb/duckdb-go/v2"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/filesource"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it concurrently, once for each source that uses the plugin, so the query path never reads the Source's config (which the SDK rewrites on every call) and instead keeps one connection per config, keyed by a hash of it.
//
// A DuckDB file is opened read-only with external access disabled (see readOnlyDSN), which is the hard boundary: a statement cannot write, and cannot reach outside the file through a table function, ATTACH, COPY or an extension. DuckDB's own driver can still run more than one statement for a single call, silently executing every statement but the last while preparing it, so Marmot refuses anything but a single statement itself, before the engine ever sees it.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served by the same plugin process are queried at once, so each keeps its own connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

// cachedConn is one source config's open database. cleanup removes the local file filesource.ResolveFilePath downloaded for a remote path (S3 or Git); it is a no-op when the configured path was already local.
type cachedConn struct {
	db       *sql.DB
	cleanup  func()
	lastUsed time.Time
}

// connect returns a live connection for the config, reusing the cached one when it still answers a ping.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*sql.DB, error) {
	config, err := parseQueryConfig(raw)
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
		log.Warn().Err(err).Msg("Cached DuckDB connection failed ping, reconnecting")
		s.query.drop(hash, c)
	}

	// Resolving a remote path and opening the file can take time, so both happen outside the lock and never hold up queries against other sources.
	localPath, cleanup, err := filesource.ResolveFilePath(ctx, config.FileSourceConfig, config.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving file path: %w", err)
	}

	db, err := openQueryDB(ctx, localPath)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("opening database: %w", err)
	}

	c := s.query.put(hash, &cachedConn{db: db, cleanup: cleanup})
	return c.db, nil
}

// parseQueryConfig unmarshals and validates raw the same way Validate does, independent of whatever the Source's own config currently holds.
func parseQueryConfig(raw pluginsdk.RawConfig) (*Config, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}
	return config, nil
}

// openQueryDB opens path with the exact DSN discovery uses (see readOnlyDSN), and allows a handful of concurrent read-only connections since DuckDB permits several readers against the same file.
func openQueryDB(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("duckdb", readOnlyDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to DuckDB: %w", err)
	}
	return db, nil
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

func (c *cachedConn) close() {
	_ = c.db.Close()
	c.cleanup()
}

func configHash(c *Config) string {
	data, err := json.Marshal(c) //nolint:gosec // G117: hashed in memory as a cache key, never stored or logged
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readKeywords are the statement forms DuckDB treats as reads with no side effects of their own. Anything else is refused before DuckDB ever sees it, because the read-only attachment only stops a statement from writing; it does not stop one from running at all.
var readKeywords = map[string]bool{"select": true, "with": true, "values": true, "table": true, "from": true}

// isReadStatement reports whether a statement's first keyword, after any comments and opening parentheses, is a read.
func isReadStatement(statement string) bool {
	rest := statement
	for {
		rest = strings.TrimLeft(rest, " \t\r\n(")
		switch {
		case strings.HasPrefix(rest, "--"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				return false
			}
			rest = rest[end+1:]
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest, "*/")
			if end < 0 {
				return false
			}
			rest = rest[end+2:]
		default:
			end := strings.IndexFunc(rest, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
			})
			if end < 0 {
				end = len(rest)
			}
			return readKeywords[strings.ToLower(rest[:end])]
		}
	}
}

// singleStatement reports whether sql is exactly one statement. DuckDB's driver extracts every statement in a semicolon-separated batch and, while preparing it, executes every one but the last for real; a second statement hidden after the one Marmot planned would then run during ExecuteQuery without ever having been planned or granted. Marmot sends one statement per call, so anything else here is refused rather than analysed, including a statement left inside an unterminated comment or quote, since what follows it cannot be classified safely either.
func singleStatement(sql string) bool {
	redacted, ok := redactLiterals(sql)
	if !ok {
		return false
	}
	statements := 0
	for _, part := range strings.Split(redacted, ";") {
		if strings.TrimSpace(part) != "" {
			statements++
		}
	}
	return statements <= 1
}

// redactLiterals blanks out the contents of string and quoted identifier literals and strips comments, so that a semicolon or keyword inside one is never mistaken for a statement boundary or a keyword. It reports false when a quote or block comment is left open, because what follows cannot be classified safely.
func redactLiterals(s string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], "--"):
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				// The comment runs to the end of the string with no closing newline, which is ordinary SQL; nothing can follow it.
				return out.String(), true
			}
			i += end
			out.WriteByte(' ')
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return out.String(), false
			}
			i += end + 3
			out.WriteByte(' ')
		case s[i] == '\'' || s[i] == '"':
			quote := s[i]
			closed := false
			for i++; i < len(s); i++ {
				if s[i] == quote {
					if i+1 < len(s) && s[i+1] == quote {
						i++
						continue
					}
					closed = true
					break
				}
			}
			if !closed {
				return out.String(), false
			}
			out.WriteByte('x')
		default:
			out.WriteByte(s[i])
		}
	}
	return out.String(), true
}

// explainNode mirrors the parts of EXPLAIN (FORMAT JSON) output the planner needs. A scan of a real table or view names it under extra_info.Table as "<database>.<schema>.<name>"; DuckDB has no materialised views, so a statement over a view never reaches the planner as a view at all, it is rewritten into the view's defining query first and the plan names the view's own base tables instead. A table function such as range or read_csv names itself under extra_info.Function instead, with no asset behind it.
type explainNode struct {
	Children  []explainNode  `json:"children"`
	ExtraInfo map[string]any `json:"extra_info"`
}

// safeTableFunctions are table functions DuckDB can run entirely from their own arguments, generating rows rather than reading them from anywhere. enable_external_access already refuses every function that would touch the filesystem or the network, but any other table function Marmot has not vetted is treated as possibly reading data the plan cannot name.
var safeTableFunctions = map[string]bool{
	"RANGE": true, "GENERATE_SERIES": true, "UNNEST": true, "REPEAT_ROW": true,
}

// tableRef is a schema-qualified table or view name, in the same shape discovery addresses one by.
type tableRef struct {
	schema, table string
}

// PlanQuery asks DuckDB what a statement would read via EXPLAIN (FORMAT JSON), so the engine does the SQL parsing, then maps each scanned table to the same MRN discovery gives it so grants written against catalogued assets bind on the query path too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if !singleStatement(req.Statement) || !isReadStatement(req.Statement) {
		return nil, errReadOnly
	}

	db, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "EXPLAIN (FORMAT JSON) "+req.Statement)
	if err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}
	defer rows.Close()

	plan := &pluginsdk.QueryPlan{}
	seen := map[tableRef]bool{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("reading plan: %w", err)
		}
		var nodes []explainNode
		if err := json.Unmarshal([]byte(value), &nodes); err != nil {
			return nil, fmt.Errorf("parsing plan: %w", err)
		}
		for _, n := range nodes {
			walkPlan(n, plan, seen)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plan rows: %w", err)
	}
	return plan, nil
}

// walkPlan adds every table the plan scans and marks it incomplete if it calls a table function Marmot has not vetted as side-effect and data free.
func walkPlan(node explainNode, plan *pluginsdk.QueryPlan, seen map[tableRef]bool) {
	if qualified, _ := node.ExtraInfo["Table"].(string); qualified != "" {
		if ref, ok := parseTableRef(qualified); ok && !seen[ref] {
			seen[ref] = true
			plan.ReferencedMRNs = append(plan.ReferencedMRNs, assetMRN("Table", ref.schema, ref.table))
		}
	} else if fn, _ := node.ExtraInfo["Function"].(string); fn != "" && !safeTableFunctions[fn] {
		plan.Incomplete = true
	}
	for _, child := range node.Children {
		walkPlan(child, plan, seen)
	}
}

// parseTableRef splits a plan's "<database>.<schema>.<name>" into the schema and name discovery addresses the same table or view by; the database component is the file itself and plays no part in the MRN.
func parseTableRef(qualified string) (tableRef, bool) {
	parts := strings.Split(qualified, ".")
	if len(parts) < 2 {
		return tableRef{}, false
	}
	return tableRef{schema: parts[len(parts)-2], table: parts[len(parts)-1]}, true
}

// ExecuteQuery runs the statement and streams rows back in chunks, the first carrying the column schema. The database/sql driver refuses to run a write against a connection opened with access_mode=read_only, so the read-only DSN is the hard boundary; the keyword and single-statement checks below only keep an obvious misuse from ever reaching the engine.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if !singleStatement(req.Statement) || !isReadStatement(req.Statement) {
		return errReadOnly
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

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at maxRows when it is set.
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
			values[i] = convertValue(v, columns[i].Type)
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

	// Always emit the final chunk, even empty, so the schema reaches the host for zero-row results.
	if len(chunk.Rows) > 0 || chunk.Columns != nil {
		return emit(chunk)
	}
	return nil
}

// convertValue converts a DuckDB-specific Go type the driver returns into a form the host's JSON encoding carries without losing or misrepresenting it. A duckdbdriver.Decimal's own fields are the unscaled integer and the scale to divide it by, so encoding it directly would publish the wrong number; a *big.Int round-trips through JSON as a bare number, which a JavaScript client reads back as a float64 and loses precision on; and duckdbdriver.OrderedMap keeps its keys and values in unexported fields, so the default encoding would publish an empty object.
func convertValue(v any, columnType string) any {
	switch val := v.(type) {
	case nil:
		return nil
	case *big.Int:
		return val.String()
	case duckdbdriver.Decimal:
		return val.String()
	case duckdbdriver.OrderedMap:
		keys, values := val.Keys(), val.Values()
		converted := make(map[string]any, len(keys))
		for i, k := range keys {
			converted[fmt.Sprint(k)] = convertValue(values[i], "")
		}
		return converted
	case duckdbdriver.Union:
		return convertValue(val.Value, "")
	case map[string]any:
		converted := make(map[string]any, len(val))
		for k, item := range val {
			converted[k] = convertValue(item, "")
		}
		return converted
	case []any:
		converted := make([]any, len(val))
		for i, item := range val {
			converted[i] = convertValue(item, "")
		}
		return converted
	case []byte:
		if columnType == "uuid" && len(val) == 16 {
			return fmt.Sprintf("%x-%x-%x-%x-%x", val[0:4], val[4:6], val[6:8], val[8:10], val[10:16])
		}
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339Nano)
	default:
		return val
	}
}
