package cockroachdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one pool per config, keyed by a hash of it.
//
// Every statement is planned and run inside a READ ONLY transaction over the
// extended protocol, which refuses more than one statement in a call. That
// alone is not the whole story on CockroachDB: by default it auto-commits an
// explicit transaction before running DDL inside it, which would let a
// CREATE TABLE or DROP TABLE slip past ReadOnly, so the query pool turns
// that off. A statement is also classified before it reaches the engine at
// all, refusing anything that is not a SELECT, WITH, VALUES or TABLE
// statement outright; the database user's own privileges remain the hard
// boundary underneath all of it.

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

// connectQuery returns a pool for the config, reusing a cached one when
// there is one. config comes from parseConfig rather than the Source, so it
// is this call's own copy and never races with a concurrent Validate or
// Discover. It is named apart from the Source.connect discovery uses, which
// opens a single plain connection to one database rather than a pool.
func (s *Source) connectQuery(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
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
	poolConfig, err := pgxpool.ParseConfig(connString(config, config.QueryDatabase))
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}
	poolConfig.MaxConns = 5
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "marmot-query"
	// autocommit_before_ddl defaults to on, which commits an explicit transaction before running DDL inside it rather than inside the transaction; turning it off keeps DDL inside the read-only transaction this plugin always opens, where CockroachDB itself refuses it.
	poolConfig.ConnConfig.RuntimeParams["autocommit_before_ddl"] = "false"
	poolConfig.AfterRelease = resetSessionAfterRelease

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeoutCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to CockroachDB: %w", err)
	}

	log.Debug().Str("host", config.Host).Str("database", config.QueryDatabase).Msg("Opened CockroachDB pool for data queries")
	return pool, nil
}

// resetSessionAfterRelease scrubs a connection's session state before it goes
// back into the pool for another caller to acquire. refuseWrite keeps a
// known session-mutating builtin away from the engine, but a rolled-back
// transaction does not always undo a session-level change on CockroachDB (a
// plain SET or set_config(..., is_local=false) survives ROLLBACK, unlike on
// PostgreSQL), and a builtin outside the denylist could have the same
// effect. RESET ALL also happens to restore every setting CockroachDB
// carries over from the connection's startup parameters, including
// autocommit_before_ddl, but that is reapplied explicitly rather than relied
// on, in case a future CockroachDB version stops doing that.
func resetSessionAfterRelease(conn *pgx.Conn) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := conn.Exec(ctx, "RESET ALL"); err != nil {
		log.Warn().Err(err).Msg("Failed to reset session state on a query connection; discarding it")
		return false
	}
	if _, err := conn.Exec(ctx, "SET autocommit_before_ddl = false"); err != nil {
		log.Warn().Err(err).Msg("Failed to reapply autocommit_before_ddl on a query connection; discarding it")
		return false
	}
	return true
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

// readKeywords are the statements that only read. Anything else is refused before CockroachDB sees it: EXPLAIN of a write does not itself write, but ReadOnly is the only thing stopping the statement underneath, and autocommit_before_ddl shows that a CockroachDB default can quietly undermine that for a whole class of statement, so a write is never handed to the engine in the first place.
var readKeywords = toSet("select", "with", "values", "table")

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
			end := strings.IndexFunc(rest, isNotASCIILetter)
			if end < 0 {
				end = len(rest)
			}
			return readKeywords[strings.ToLower(rest[:end])]
		}
	}
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNotASCIILetter(r rune) bool {
	return !isASCIILetter(r)
}

// refuseWrite reports why a statement cannot run as a Marmot data query, or nil when it can proceed to the engine for planning or execution.
func refuseWrite(statement string) error {
	if !isReadStatement(statement) {
		return errReadOnly
	}
	if callsMutatingFunction(statement) {
		return errReadOnly
	}
	return nil
}

// mutatingFunctions are built-in functions that mutate session or cluster state rather than compute a value from their arguments, keyed by their bare, unqualified name. A call to one is refused outright rather than merely marked Incomplete: Incomplete only widens the grant a plan needs, it does not stop the call from running, and a rolled-back transaction does not always undo a session-level change on CockroachDB, which a pooled connection then carries into the next caller's query.
var mutatingFunctions = toSet("set_config", "setval", "setseed")

// mutatingSchemaQualifiedFunctions are mutatingFunctions that only live outside pg_catalog. Unlike pg_catalog, crdb_internal is never implicitly searched, so these only ever appear schema-qualified and are matched on their full name rather than isMutatingFunctionCall's pg_catalog-stripping rule.
var mutatingSchemaQualifiedFunctions = toSet(
	"crdb_internal.set_vmodule", "crdb_internal.set_trace_verbose",
	"crdb_internal.reset_sql_stats", "crdb_internal.reset_activity_tables",
	"crdb_internal.reset_index_usage_stats", "crdb_internal.reset_insights_tables",
	"crdb_internal.reset_multi_region_zone_configs_for_database",
	"crdb_internal.reset_multi_region_zone_configs_for_table",
)

// isMutatingFunctionCall reports whether a called name is one of mutatingFunctions or mutatingSchemaQualifiedFunctions, stripping a pg_catalog qualifier the same way isSafeFunction does before matching a bare name.
func isMutatingFunctionCall(name string) bool {
	lower := strings.ToLower(name)
	if mutatingSchemaQualifiedFunctions[lower] {
		return true
	}
	if rest, ok := strings.CutPrefix(lower, "pg_catalog."); ok {
		lower = rest
	}
	if strings.Contains(lower, ".") {
		return false
	}
	return mutatingFunctions[lower]
}

// callsMutatingFunction reports whether a statement calls a mutating function, ignoring comments and string literals so neither hides a call nor is mistaken for one. A statement it cannot read cleanly, such as one with an unterminated literal or comment, counts as calling one, so the check fails closed.
func callsMutatingFunction(statement string) bool {
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

	for _, m := range callPattern.FindAllStringSubmatch(code.String(), -1) {
		if isMutatingFunctionCall(m[1]) {
			return true
		}
	}
	return false
}

// mutationNodes are the first words of an EXPLAIN (VERBOSE) node name that change data. DDL cannot appear inside a CTE or a square-bracket subquery, so isReadStatement already keeps it away from this plan, but these are checked unconditionally in case that screen is ever bypassed. The statement forms this really exists to catch are a writable CTE and the square-bracket form, such as "SELECT * FROM [INSERT INTO t ... RETURNING ...]", both of which plan as a normal read at the top with the write nested underneath.
var mutationNodes = toSet(
	"insert", "upsert", "update", "delete", "truncate", "create", "drop", "alter",
	"grant", "revoke", "import", "export", "backup", "restore", "comment", "rename",
	"split", "unsplit", "relocate", "scatter", "refresh", "reassign", "configure",
	"set", "reset", "use", "copy", "cancel", "pause", "resume", "discard",
)

// tableLinePattern matches the "table: name@index" line EXPLAIN (VERBOSE)
// prints for every relation a plan scans. CockroachDB never qualifies the
// name with a schema here, which resolveRelation resolves afterwards.
var tableLinePattern = regexp.MustCompile(`^table:\s*([^@\s]+)`)

// rowExprKeyPattern matches the key of a VALUES node's "row N, expr N: ..." line.
var rowExprKeyPattern = regexp.MustCompile(`^row\s+\d+,\s*expr\s+\d+$`)

// planSummary is what a statement's EXPLAIN (VERBOSE) output says it reads.
type planSummary struct {
	// relations are the bare table names the plan scans, in the exact case
	// CockroachDB stores them, deduplicated in the order first seen.
	relations []string
	// opaqueCalls are the functions the statement calls that may read data the plan cannot name, such as a user-defined function that runs a query of its own at execution time.
	opaqueCalls []string
}

// parsePlan walks EXPLAIN (VERBOSE) output, line by line, and returns every
// table the statement scans and every function it calls that is not known
// to be safe. It refuses plans that contain a mutation node, which catches a
// data-modifying CTE or square-bracket subquery inside an otherwise
// read-only statement. Views are expanded by the optimiser, so a statement
// over a view reports its base tables instead; a materialized view is its
// own stored relation and reports itself.
func parsePlan(planText string) (*planSummary, error) {
	summary := &planSummary{}
	seenRelation := map[string]bool{}
	seenCall := map[string]bool{}

	for _, raw := range strings.Split(planText, "\n") {
		line := strings.TrimLeft(raw, " \t│├└─")
		if line == "" {
			continue
		}

		if name, ok := strings.CutPrefix(line, "• "); ok {
			verb, _, _ := strings.Cut(name, " ")
			if mutationNodes[strings.ToLower(verb)] {
				return nil, errReadOnly
			}
			continue
		}

		if m := tableLinePattern.FindStringSubmatch(line); m != nil {
			name := m[1]
			if !seenRelation[name] {
				seenRelation[name] = true
				summary.relations = append(summary.relations, name)
			}
			continue
		}

		expr, ok := expressionValue(line)
		if !ok {
			continue
		}
		for _, call := range opaqueCalls(expr) {
			if !seenCall[call] {
				seenCall[call] = true
				summary.opaqueCalls = append(summary.opaqueCalls, call)
			}
		}
	}
	return summary, nil
}

// expressionValue returns the value half of an attribute line that carries a
// user expression, such as "render total: total * 1.21" or "filter: name =
// 'alice'". Most attribute lines are structural, such as "columns:" or
// "spans:", and are deliberately left out: scanning every colon-separated
// line for a call risks matching an identifier in an index recommendation's
// suggested SQL rather than anything the statement itself evaluates.
func expressionValue(line string) (string, bool) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		return "", false
	}
	key = strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.HasPrefix(key, "render "),
		strings.HasPrefix(key, "aggregate "),
		strings.HasPrefix(key, "window "),
		key == "filter",
		key == "having",
		key == "equality",
		key == "group by",
		key == "distinct on",
		rowExprKeyPattern.MatchString(key):
		return strings.TrimSpace(value), true
	default:
		return "", false
	}
}

// explainStatement asks CockroachDB to plan a statement without running it
// and returns its EXPLAIN (VERBOSE) output as one string, one source line
// per line of the plan.
func explainStatement(ctx context.Context, tx pgx.Tx, statement string) (string, error) {
	rows, err := tx.Query(ctx, "EXPLAIN (VERBOSE) "+statement, pgx.QueryExecModeExec)
	if err != nil {
		return "", fmt.Errorf("planning statement: %w", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", fmt.Errorf("reading plan: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("planning statement: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}

// resolveRelation maps a bare relation name from EXPLAIN output to the
// schema and asset type discovery would give it, by looking the name up in
// the connected database's own catalog the same way discovery lists
// relations. It reports ambiguous when the name does not resolve to exactly
// one schema: EXPLAIN (VERBOSE) never qualifies a scanned table with its
// schema, so two tables sharing a name in different schemas of the same
// database cannot be told apart, and a name discovery never saw, such as a
// system catalog object or a table in a different database that
// CockroachDB lets a statement read directly, cannot be placed at all.
func resolveRelation(ctx context.Context, tx pgx.Tx, name string) (schema, assetType string, ambiguous bool, err error) {
	rows, err := tx.Query(ctx, `
		SELECT n.nspname, c.relkind::text
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1`, name)
	if err != nil {
		return "", "", false, fmt.Errorf("resolving relation %q: %w", name, err)
	}
	defer rows.Close()

	var matches []relation
	for rows.Next() {
		var r relation
		if err := rows.Scan(&r.Schema, &r.Kind); err != nil {
			return "", "", false, fmt.Errorf("scanning relation %q: %w", name, err)
		}
		matches = append(matches, r)
	}
	if err := rows.Err(); err != nil {
		return "", "", false, fmt.Errorf("resolving relation %q: %w", name, err)
	}

	if len(matches) != 1 {
		return "", "", true, nil
	}
	return matches[0].Schema, matches[0].assetType(), false, nil
}

// PlanQuery asks CockroachDB what a statement would read via EXPLAIN
// (VERBOSE), so the engine does the SQL parsing, and maps each scanned
// table to the MRN discovery gives it so grants written against catalog
// assets bind here too.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if err := refuseWrite(req.Statement); err != nil {
		return nil, err
	}

	config, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	pool, err := s.connectQuery(ctx, config)
	if err != nil {
		return nil, err
	}

	plan := &pluginsdk.QueryPlan{}
	err = readOnly(ctx, pool, func(tx pgx.Tx) error {
		planText, err := explainStatement(ctx, tx, req.Statement)
		if err != nil {
			return err
		}

		summary, err := parsePlan(planText)
		if err != nil {
			return err
		}

		// A table grant only means something if the plan names everything the statement reads; a function that runs its own query reads tables the plan never shows, so access to the whole source is needed instead.
		if len(summary.opaqueCalls) > 0 {
			plan.Incomplete = true
			log.Info().Strs("functions", summary.opaqueCalls).Msg("Statement calls functions that may read unlisted tables; it needs access to the whole source")
		}

		for _, name := range summary.relations {
			schema, assetType, ambiguous, err := resolveRelation(ctx, tx, name)
			if err != nil {
				return err
			}
			if ambiguous {
				plan.Incomplete = true
				log.Info().Str("table", name).Msg("Statement reads a table this source cannot place in exactly one schema; it needs access to the whole source")
				continue
			}
			plan.ReferencedMRNs = append(plan.ReferencedMRNs, assetMRN(assetType, qualifiedName(config.QueryDatabase, schema, name)))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// ExecuteQuery runs the statement in a read-only transaction and streams rows
// back in chunks, the first carrying the column schema.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if err := refuseWrite(req.Statement); err != nil {
		return err
	}

	config, err := parseConfig(raw)
	if err != nil {
		return err
	}
	pool, err := s.connectQuery(ctx, config)
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
				values[i] = convertValue(v)
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
