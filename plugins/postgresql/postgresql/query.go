package postgresql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one pool per config, keyed by a hash of it.
//
// Every statement is planned and run inside a READ ONLY transaction over the
// extended protocol, which refuses multiple statements in one call, so a
// statement can neither write nor smuggle a COMMIT to escape the
// transaction. The database user's own privileges remain the hard boundary.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// poolIdleTTL is how long a pool can go unused before it is closed.
const poolIdleTTL = 10 * time.Minute

// queryPools holds one pool per source config. Several sources served by the same plugin process are queried at once, so each keeps its own pool rather than closing another source's.
type queryPools struct {
	mu    sync.Mutex
	pools map[string]*cachedPool
}

type cachedPool struct {
	pool     *pgxpool.Pool
	lastUsed time.Time
}

func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*pgxpool.Pool, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	hash := configHash(config)

	if pool := s.query.get(hash); pool != nil {
		return pool, nil
	}
	// Opening can take seconds, so it happens outside the lock and never holds up queries to other sources.
	pool, err := openQueryPool(ctx, config)
	if err != nil {
		return nil, err
	}
	return s.query.put(hash, pool), nil
}

func (q *queryPools) get(hash string) *pgxpool.Pool {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.closeIdle(now)
	if c, ok := q.pools[hash]; ok {
		c.lastUsed = now
		return c.pool
	}
	return nil
}

// put caches a newly opened pool, or keeps the one another call opened for the same config meanwhile and closes this one.
func (q *queryPools) put(hash string, pool *pgxpool.Pool) *pgxpool.Pool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if c, ok := q.pools[hash]; ok {
		go pool.Close()
		c.lastUsed = time.Now()
		return c.pool
	}
	if q.pools == nil {
		q.pools = make(map[string]*cachedPool)
	}
	q.pools[hash] = &cachedPool{pool: pool, lastUsed: time.Now()}
	return pool
}

// closeIdle closes pools unused for poolIdleTTL. Close waits for connections still in use, so it runs in the background and a long query on an idle-looking pool finishes first.
func (q *queryPools) closeIdle(now time.Time) {
	for hash, c := range q.pools {
		if now.Sub(c.lastUsed) > poolIdleTTL {
			delete(q.pools, hash)
			go c.pool.Close()
		}
	}
}

func openQueryPool(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	connURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(config.User, config.Password),
		Host:     config.Host + ":" + strconv.Itoa(config.Port),
		Path:     "/" + config.QueryDatabase,
		RawQuery: url.Values{"sslmode": {config.SSLMode}}.Encode(),
	}

	poolConfig, err := pgxpool.ParseConfig(connURL.String())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}
	poolConfig.MaxConns = 5
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "marmot-query"

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeoutCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to PostgreSQL: %w", err)
	}

	log.Debug().Str("host", config.Host).Str("database", config.QueryDatabase).Msg("Opened PostgreSQL pool for data queries")
	return pool, nil
}

func configHash(c *Config) string {
	data, err := json.Marshal(c) //nolint:gosec // G117: hashed in memory as a cache key, never stored or logged
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readOnly runs fn inside a READ ONLY transaction that is always rolled back.
func readOnly(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("starting read-only transaction: %w", err)
	}
	defer func() {
		// The caller's context may already be cancelled after a truncated
		// read, and the connection must still go back clean.
		rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rbCtx)
	}()
	return fn(tx)
}

type relation struct {
	schema string
	name   string
}

// planSummary is what a statement's plan says it reads.
type planSummary struct {
	relations []relation
	// opaqueCalls are the functions the statement calls that may read data the plan cannot name, such as query_to_xml, which runs a query of its own at execution time.
	opaqueCalls []string
}

// nameKeys are EXPLAIN fields that hold names rather than expressions, so they are not searched for function calls.
var nameKeys = map[string]bool{
	"Node Type": true, "Relation Name": true, "Schema": true, "Alias": true,
	"Index Name": true, "CTE Name": true, "Subplan Name": true,
}

// parsePlan walks EXPLAIN (FORMAT JSON, VERBOSE) output and returns every
// relation the statement scans and every function it calls that is not known
// to be safe. It refuses plans that modify data, which catches data-modifying
// CTEs inside an otherwise read-only SELECT. Views are expanded by the
// planner, so a statement over a view references its base tables and the
// functions the view calls.
func parsePlan(planJSON []byte) (*planSummary, error) {
	var explained []struct {
		Plan map[string]any `json:"Plan"`
	}
	if err := json.Unmarshal(planJSON, &explained); err != nil {
		return nil, fmt.Errorf("parsing plan: %w", err)
	}

	summary := &planSummary{}
	seenRelation := map[relation]bool{}
	seenCall := map[string]bool{}
	var walk func(v any, key string) error
	walk = func(v any, key string) error {
		switch v := v.(type) {
		case map[string]any:
			if nodeType, _ := v["Node Type"].(string); nodeType == "ModifyTable" || nodeType == "LockRows" {
				return errReadOnly
			}
			if name, _ := v["Relation Name"].(string); name != "" {
				schema, _ := v["Schema"].(string)
				r := relation{schema: schema, name: name}
				if !seenRelation[r] {
					seenRelation[r] = true
					summary.relations = append(summary.relations, r)
				}
			}
			for k, child := range v {
				if err := walk(child, k); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child, key); err != nil {
					return err
				}
			}
		case string:
			if nameKeys[key] {
				return nil
			}
			for _, call := range opaqueCalls(v) {
				if !seenCall[call] {
					seenCall[call] = true
					summary.opaqueCalls = append(summary.opaqueCalls, call)
				}
			}
		}
		return nil
	}
	for _, e := range explained {
		if err := walk(e.Plan, ""); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

// PlanQuery asks PostgreSQL what a statement would read via EXPLAIN, so the
// server does the SQL parsing, and maps each relation to the MRN discovery
// gives it so grants written against catalog assets bind here too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	pool, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	plan := &pluginsdk.QueryPlan{}
	err = readOnly(ctx, pool, func(tx pgx.Tx) error {
		var planJSON []byte
		if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON, VERBOSE) "+req.Statement, pgx.QueryExecModeExec).Scan(&planJSON); err != nil {
			return fmt.Errorf("planning statement: %w", err)
		}

		summary, err := parsePlan(planJSON)
		if err != nil {
			return err
		}

		// A table grant only means something if the plan names everything the statement reads; a function that runs its own query reads tables the plan never shows, so access to the whole source is needed instead.
		if len(summary.opaqueCalls) > 0 {
			plan.Incomplete = true
			log.Info().Strs("functions", summary.opaqueCalls).Msg("Statement calls functions that may read unlisted tables; it needs access to the whole source")
		}
		for _, r := range summary.relations {
			plan.ReferencedMRNs = append(plan.ReferencedMRNs, mrn.New(relationType(ctx, tx, r), "PostgreSQL", r.name))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// relationType maps a relation to the asset type discovery gives it:
// materialised views are catalogued as views, everything else as tables.
func relationType(ctx context.Context, tx pgx.Tx, r relation) string {
	var relkind string
	err := tx.QueryRow(ctx, `
		SELECT c.relkind::text
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2`, r.schema, r.name).Scan(&relkind)
	if err == nil && (relkind == "v" || relkind == "m") {
		return "View"
	}
	return "Table"
}

// ExecuteQuery runs the statement in a read-only transaction and streams rows
// back in chunks, the first carrying the column schema.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	pool, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	return readOnly(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, req.Statement, pgx.QueryExecModeExec)
		if err != nil {
			return fmt.Errorf("executing statement: %w", err)
		}
		defer rows.Close()

		typeMap := tx.Conn().TypeMap()
		fields := rows.FieldDescriptions()
		columns := make([]pluginsdk.QueryColumn, len(fields))
		for i, f := range fields {
			columns[i] = pluginsdk.QueryColumn{Name: f.Name}
			if t, ok := typeMap.TypeForOID(f.DataTypeOID); ok {
				columns[i].Type = t.Name
			}
		}

		chunk := pluginsdk.QueryResultChunk{Columns: columns}
		var sent int64
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return fmt.Errorf("reading row: %w", err)
			}
			for i, v := range values {
				values[i] = convertPgxValue(v)
			}

			chunk.Rows = append(chunk.Rows, values)
			sent++
			if req.MaxRows > 0 && sent >= req.MaxRows {
				break
			}
			if len(chunk.Rows) >= executeChunkRows {
				if err := emit(chunk); err != nil {
					return err
				}
				chunk = pluginsdk.QueryResultChunk{}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating rows: %w", err)
		}

		// Always emit the final chunk, even empty, so the schema reaches
		// the host for zero-row results.
		if len(chunk.Rows) > 0 || chunk.Columns != nil {
			return emit(chunk)
		}
		return nil
	})
}
