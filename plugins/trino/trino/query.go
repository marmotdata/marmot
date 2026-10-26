package trino

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one connection, plus its catalog map, per config, keyed by a
// hash of it.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// connIdleTTL is how long a connection can go unused before it is closed.
const connIdleTTL = 10 * time.Minute

// queryConns holds one connection per source config. Several sources served by the same plugin process are queried at once, so each keeps its own connection rather than closing another source's.
type queryConns struct {
	mu    sync.Mutex
	conns map[string]*cachedConn
}

type cachedConn struct {
	db         *sql.DB
	connectors map[string]string // catalog name → connector name
	lastUsed   time.Time
}

// connect returns a live connection for the config, reusing the cached one
// when it still answers a ping. The connector map is loaded once per
// connection; unlike discoverCatalogs it applies no exclusions, because
// planning must map any catalog a statement can name.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*sql.DB, map[string]string, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, nil, err
	}
	hash := configHash(config)

	if c := s.query.get(hash); c != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return c.db, c.connectors, nil
		}
		log.Warn().Err(err).Msg("Cached Trino connection failed ping, reconnecting")
		s.query.drop(hash, c)
	}

	// Connecting can take seconds, so it happens outside the lock and never holds up queries to other sources.
	db, err := openDB(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to Trino: %w", err)
	}
	connectors, err := loadCatalogConnectors(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("loading catalog connectors: %w", err)
	}
	c := s.query.put(hash, &cachedConn{db: db, connectors: connectors})
	return c.db, c.connectors, nil
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

func loadCatalogConnectors(ctx context.Context, db *sql.DB) (map[string]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(queryCtx, "SELECT catalog_name, connector_name FROM system.metadata.catalogs")
	if err != nil {
		return nil, fmt.Errorf("querying catalogs: %w", err)
	}
	defer rows.Close()

	connectors := make(map[string]string)
	for rows.Next() {
		var name, connector string
		if err := rows.Scan(&name, &connector); err != nil {
			continue
		}
		connectors[name] = connector
	}
	return connectors, rows.Err()
}

// ioPlan mirrors the parts of EXPLAIN (TYPE IO, FORMAT JSON) output the
// planner needs.
type ioPlan struct {
	InputTableColumnInfos []struct {
		Table struct {
			Catalog     string `json:"catalog"`
			SchemaTable struct {
				Schema string `json:"schema"`
				Table  string `json:"table"`
			} `json:"schemaTable"`
		} `json:"table"`
	} `json:"inputTableColumnInfos"`
	// OutputTable is set when the statement writes (INSERT, CTAS, ...).
	OutputTable json.RawMessage `json:"outputTable"`
	Estimate    struct {
		// Trino reports unknown estimates as the string "NaN", so the
		// size is decoded leniently.
		OutputSizeInBytes json.RawMessage `json:"outputSizeInBytes"`
	} `json:"estimate"`
}

// estimatedBytes returns the plan's output size estimate, or 0 when Trino
// does not know it.
func (p *ioPlan) estimatedBytes() int64 {
	var size float64
	if err := json.Unmarshal(p.Estimate.OutputSizeInBytes, &size); err != nil || math.IsNaN(size) || math.IsInf(size, 0) {
		return 0
	}
	return int64(size)
}

// readKeywords are the statements that only read. Anything else is refused
// before Trino sees it, because even EXPLAIN of a write is not free of side
// effects: the JDBC connectors create a temporary table while planning an
// INSERT, and Marmot plans every statement before checking the caller's
// grants.
var readKeywords = map[string]bool{"select": true, "with": true, "values": true, "table": true}

// isReadStatement reports whether a statement's first keyword, after any
// comments and opening parentheses, is a read.
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

// parseIOPlan decodes EXPLAIN (TYPE IO, FORMAT JSON) output and refuses
// plans that write. Trino answers EXPLAIN of a statement it cannot plan,
// such as DDL, with the statement text instead of a JSON plan, so anything
// that is not a JSON object is refused too.
func parseIOPlan(planJSON string) (*ioPlan, error) {
	if !strings.HasPrefix(strings.TrimSpace(planJSON), "{") {
		return nil, errReadOnly
	}
	var plan ioPlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return nil, fmt.Errorf("parsing IO plan: %w", err)
	}
	if len(plan.OutputTable) > 0 && string(plan.OutputTable) != "null" {
		return nil, errReadOnly
	}
	return &plan, nil
}

// PlanQuery asks Trino what a statement would read via EXPLAIN (TYPE IO),
// so the engine does the SQL parsing, then maps each referenced table to
// the same MRN the discovery path would give it so grants written against
// catalog assets bind on the query path too. Data queries are read-only, so a
// statement whose plan writes to a table is refused here, before it runs.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if !isReadStatement(req.Statement) {
		return nil, errReadOnly
	}
	db, connectors, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	var planJSON string
	if err := db.QueryRowContext(ctx, "EXPLAIN (TYPE IO, FORMAT JSON) "+req.Statement).Scan(&planJSON); err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}

	plan, err := parseIOPlan(planJSON)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	// A table function such as a connector's system.query passes its own query straight to the underlying database, so the tables it reads never appear in Trino's plan and only access to the whole source can cover it.
	result := &pluginsdk.QueryPlan{EstimatedBytes: plan.estimatedBytes(), Incomplete: callsTableFunction(req.Statement)}
	for _, input := range plan.InputTableColumnInfos {
		tableMRN := tableMRN(ctx, db, connectors, input.Table.Catalog, input.Table.SchemaTable.Schema, input.Table.SchemaTable.Table)
		if !seen[tableMRN] {
			seen[tableMRN] = true
			result.ReferencedMRNs = append(result.ReferencedMRNs, tableMRN)
		}
	}
	return result, nil
}

// tableMRN reproduces the discovery path's MRN for a referenced table.
// Internal connectors (memory, tpch, ...) are skipped by discovery, so
// their tables get a Trino-scoped MRN that grants can still target.
func tableMRN(ctx context.Context, db *sql.DB, connectors map[string]string, catalog, schema, table string) string {
	info, ok := connectorInfoForName(connectors[catalog])
	if !ok {
		info = connectorInfo{Provider: "Trino", MRNName: defaultMRNName}
	}

	assetType := "Table"
	if isView(ctx, db, catalog, schema, table) {
		assetType = "View"
	}
	return mrn.New(assetType, info.Provider, info.MRNName(catalog, schema, table))
}

func isView(ctx context.Context, db *sql.DB, catalog, schema, table string) bool {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitized via quoteIdentifier/escapeString
		"SELECT table_type FROM %s.information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'",
		quoteIdentifier(catalog), escapeString(schema), escapeString(table),
	)

	var tableType string
	if err := db.QueryRowContext(queryCtx, query).Scan(&tableType); err != nil {
		return false
	}
	return strings.EqualFold(tableType, "VIEW")
}

// ExecuteQuery runs the statement and streams rows back in chunks, the
// first carrying the column schema. Marmot always plans a statement before
// executing it, so writes have already been refused by PlanQuery.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if !isReadStatement(req.Statement) {
		return errReadOnly
	}
	db, _, err := s.connect(ctx, raw)
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

// tableFunctionCall matches Trino's table function invocation, TABLE(...), the only syntax that calls one.
var tableFunctionCall = regexp.MustCompile(`(?i)\bTABLE\s*\(`)

// callsTableFunction reports whether a statement invokes a table function, ignoring comments and string literals. A statement it cannot read, such as one with an unterminated literal or comment, counts as calling one, so the check fails closed.
func callsTableFunction(statement string) bool {
	var code strings.Builder
	for i := 0; i < len(statement); i++ {
		switch {
		case strings.HasPrefix(statement[i:], "--"):
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				i = len(statement)
				continue
			}
			i += end
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return true
			}
			i += end + 3
			code.WriteByte(' ')
		case statement[i] == '\'':
			closed := false
			for i++; i < len(statement); i++ {
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
				return true
			}
			code.WriteString("''")
		default:
			code.WriteByte(statement[i])
		}
	}
	return tableFunctionCall.MatchString(code.String())
}
