package bigquery

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
)

// Marmot keeps this plugin's process long-running and may call it
// concurrently, once for each source that uses the plugin, so the query path
// never reads the Source's config (which the SDK rewrites on every call) and
// instead keeps one client per config, keyed by a hash of it.
//
// BigQuery has no read-only session or transaction to run a statement under,
// so the hard boundary is the service account's IAM: grant it
// roles/bigquery.dataViewer on the datasets it may read and
// roles/bigquery.jobUser on the billing project, and nothing more. Marmot
// still gates every statement through a free, side-effect-free dry run before
// it reaches that boundary, and caps what a job may bill for with
// max_bytes_billed.

const executeChunkRows = 500

var errReadOnly = errors.New("only read queries can run through Marmot")

// defaultMaxBytesBilled caps a data query at 10GiB of billed bytes unless the
// config says otherwise, generous enough for ad hoc analysis without risking
// a runaway scan.
const defaultMaxBytesBilled = 10 * 1024 * 1024 * 1024

// clientIdleTTL is how long a client can go unused before it is closed.
const clientIdleTTL = 10 * time.Minute

// queryClients holds one BigQuery client per source config. Several sources
// served by the same plugin process are queried at once, so each keeps its
// own client rather than closing another source's.
type queryClients struct {
	mu      sync.Mutex
	clients map[string]*cachedClient
}

type cachedClient struct {
	client   *bigquery.Client
	config   *Config
	lastUsed time.Time
}

// connect returns a live client for the config, reusing the cached one when
// there is one.
func (s *Source) connect(ctx context.Context, raw pluginsdk.RawConfig) (*bigquery.Client, *Config, error) {
	config, err := parseConfig(raw)
	if err != nil {
		return nil, nil, err
	}
	hash := configHash(config)

	if c := s.query.get(hash); c != nil {
		return c.client, c.config, nil
	}

	// Opening can take seconds, so it happens outside the lock and never
	// holds up queries to other sources.
	client, err := newClient(ctx, config, bigquery.Scope)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to BigQuery: %w", err)
	}
	c := s.query.put(hash, &cachedClient{client: client, config: config})
	return c.client, c.config, nil
}

func (q *queryClients) get(hash string) *cachedClient {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.closeIdle(now)
	if c, ok := q.clients[hash]; ok {
		c.lastUsed = now
		return c
	}
	return nil
}

// put caches a newly opened client, or keeps the one another call opened for
// the same config meanwhile and closes this one.
func (q *queryClients) put(hash string, fresh *cachedClient) *cachedClient {
	q.mu.Lock()
	defer q.mu.Unlock()
	if c, ok := q.clients[hash]; ok {
		go func() { _ = fresh.client.Close() }()
		c.lastUsed = time.Now()
		return c
	}
	if q.clients == nil {
		q.clients = make(map[string]*cachedClient)
	}
	fresh.lastUsed = time.Now()
	q.clients[hash] = fresh
	return fresh
}

// closeIdle closes clients unused for clientIdleTTL.
func (q *queryClients) closeIdle(now time.Time) {
	for hash, c := range q.clients {
		if now.Sub(c.lastUsed) > clientIdleTTL {
			delete(q.clients, hash)
			go func() { _ = c.client.Close() }()
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

// dryRun asks BigQuery to validate and plan statement without running it. A
// dry run never creates a job that does anything beyond returning
// statistics, so it is free and has no side effects whatever the statement
// says, which is why both PlanQuery and ExecuteQuery can run one before
// deciding whether the engine may go further.
func dryRun(ctx context.Context, client *bigquery.Client, statement string) (*bigquery.QueryStatistics, error) {
	q := client.Query(statement)
	q.DryRun = true
	q.UseLegacySQL = false

	job, err := q.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}
	status := job.LastStatus()
	if status == nil || status.Statistics == nil {
		return nil, errors.New("planning statement: BigQuery returned no statistics")
	}
	stats, ok := status.Statistics.Details.(*bigquery.QueryStatistics)
	if !ok {
		return nil, errors.New("planning statement: BigQuery returned no query statistics")
	}
	return stats, nil
}

// classifyStatement refuses anything a dry run shows is not a plain read.
// BigQuery reports a script or a multi-statement batch as StatementType
// SCRIPT, and every DDL, DML or EXPORT DATA statement names itself by its
// own statement type, so requiring exactly SELECT rejects all of them in one
// check. DDLTargetTable, DDLOperationPerformed and ExportDataStatistics are
// checked too, in case some statement type BigQuery reports as SELECT still
// names a destination.
func classifyStatement(stats *bigquery.QueryStatistics) error {
	if stats.StatementType != "SELECT" {
		return fmt.Errorf("statement type %q is not a read: %w", stats.StatementType, errReadOnly)
	}
	if stats.DDLTargetTable != nil || stats.DDLOperationPerformed != "" || stats.ExportDataStatistics != nil {
		return errReadOnly
	}
	return nil
}

// externalQueryCall matches BigQuery's federated query function, the
// standard way a statement reads from an external database at execution
// time without that database's tables ever showing up as referenced tables.
var externalQueryCall = regexp.MustCompile(`(?i)\bEXTERNAL_QUERY\s*\(`)

// callsExternalQuery reports whether a statement invokes EXTERNAL_QUERY,
// ignoring comments and string literals, so a mention inside a literal or a
// comment does not trigger it. A statement it cannot read, such as one with
// an unterminated literal or comment, counts as calling it, so the check
// fails closed.
func callsExternalQuery(statement string) bool {
	var code strings.Builder
	for i := 0; i < len(statement); i++ {
		switch {
		case strings.HasPrefix(statement[i:], "--"), strings.HasPrefix(statement[i:], "#"):
			end := strings.IndexByte(statement[i:], '\n')
			if end < 0 {
				return true
			}
			i += end
		case strings.HasPrefix(statement[i:], "/*"):
			end := strings.Index(statement[i+2:], "*/")
			if end < 0 {
				return true
			}
			i += end + 3
			code.WriteByte(' ')
		case strings.HasPrefix(statement[i:], "'''"), strings.HasPrefix(statement[i:], `"""`):
			quote := statement[i : i+3]
			end := strings.Index(statement[i+3:], quote)
			if end < 0 {
				return true
			}
			i += 3 + end + 2
			code.WriteString("''")
		case statement[i] == '\'' || statement[i] == '"':
			quote := statement[i]
			closed := false
			for i++; i < len(statement); i++ {
				if statement[i] == '\\' {
					i++
					continue
				}
				if statement[i] == quote {
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
	return externalQueryCall.MatchString(code.String())
}

// classifyMetadata maps a table's metadata to the asset type discovery gives
// it, mirroring getTableType and discoverTables so a query's plan names the
// same MRN discovery would.
func classifyMetadata(md *bigquery.TableMetadata) string {
	switch {
	case md.ViewQuery != "":
		return "View"
	case md.ExternalDataConfig != nil:
		return "ExternalTable"
	default:
		return "Table"
	}
}

// classifyTable fetches a referenced table's metadata and classifies it. A
// table whose metadata cannot be read, such as one in a project this
// source's credentials cannot see, is treated as a plain table, the default
// discovery itself falls back to when it has nothing better to go on.
func classifyTable(ctx context.Context, t *bigquery.Table) string {
	md, err := t.Metadata(ctx)
	if err != nil {
		log.Warn().Err(err).Str("project", t.ProjectID).Str("dataset", t.DatasetID).Str("table", t.TableID).
			Msg("Could not read referenced table's metadata for query planning, treating it as a table")
		return "Table"
	}
	return classifyMetadata(md)
}

// referencedMRNs maps a dry run's referenced tables to the MRNs discovery
// would give them, deduplicating repeats. classify is a parameter, rather
// than a direct call to Table.Metadata, so the mapping can be tested without
// a live project: standard SQL lists a referenced view by itself rather than
// expanding it to the tables it queries, so classify sees the view and the
// plan names only the view, matching how BigQuery's own authorized views
// work.
func referencedMRNs(ctx context.Context, tables []*bigquery.Table, classify func(ctx context.Context, t *bigquery.Table) string) []string {
	var mrns []string
	seen := map[string]bool{}
	for _, t := range tables {
		m := mrn.New(classify(ctx, t), "BigQuery", t.TableID)
		if !seen[m] {
			seen[m] = true
			mrns = append(mrns, m)
		}
	}
	return mrns
}

// PlanQuery asks BigQuery what a statement would read via a dry run. A dry
// run is free and never creates a job that does anything beyond returning
// statistics, so it is safe to run before Marmot decides whether to refuse
// the statement, unlike engines where even planning a write has side
// effects.
func (s *Source) PlanQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	client, _, err := s.connect(ctx, raw)
	if err != nil {
		return nil, err
	}

	stats, err := dryRun(ctx, client, req.Statement)
	if err != nil {
		return nil, err
	}
	if err := classifyStatement(stats); err != nil {
		return nil, err
	}

	return &pluginsdk.QueryPlan{
		ReferencedMRNs: referencedMRNs(ctx, stats.ReferencedTables, classifyTable),
		EstimatedBytes: stats.TotalBytesProcessed,
		// EXTERNAL_QUERY reads from a connection to an external database at
		// execution time, so the tables it touches there never appear in
		// ReferencedTables and only access to the whole source can cover it.
		Incomplete: callsExternalQuery(req.Statement),
	}, nil
}

// labelValuePattern matches what a BigQuery label value cannot hold.
var labelValuePattern = regexp.MustCompile(`[^a-z0-9_-]+`)

// sanitizeLabelValue lowercases identity and replaces anything a BigQuery
// label value cannot hold with a dash, then trims to the 63 characters
// BigQuery allows. An identity that sanitises to nothing, such as one made
// entirely of characters the pattern strips, comes back empty so the caller
// can leave the label off rather than send an invalid one.
func sanitizeLabelValue(identity string) string {
	value := strings.Trim(labelValuePattern.ReplaceAllString(strings.ToLower(identity), "-"), "-")
	if len(value) > 63 {
		value = strings.TrimRight(value[:63], "-")
	}
	return value
}

// queryLabels tags a job with the fact that Marmot ran it and, when known,
// the identity data queries ran it on behalf of, so both show up in Cloud
// Logging and billing exports.
func queryLabels(identity string) map[string]string {
	labels := map[string]string{"marmot": "data-query"}
	if value := sanitizeLabelValue(identity); value != "" {
		labels["marmot-identity"] = value
	}
	return labels
}

// ExecuteQuery re-checks the statement with its own dry run, because Marmot
// plans a statement before checking the caller's grants but calls
// ExecuteQuery independently afterwards, then runs it and streams rows back
// in chunks, the first carrying the column schema. BigQuery has no
// read-only session to run it under, so the service account's IAM role and
// the MaxBytesBilled cap are what actually bound what the job reads, bills
// or touches; see the README.
func (s *Source) ExecuteQuery(ctx context.Context, raw pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	client, config, err := s.connect(ctx, raw)
	if err != nil {
		return err
	}

	stats, err := dryRun(ctx, client, req.Statement)
	if err != nil {
		return err
	}
	if err := classifyStatement(stats); err != nil {
		return err
	}

	q := client.Query(req.Statement)
	q.UseLegacySQL = false
	q.MaxBytesBilled = config.MaxBytesBilled
	q.Labels = queryLabels(req.Identity)

	it, err := q.Read(ctx)
	if err != nil {
		return fmt.Errorf("executing statement: %w", err)
	}
	return streamRows(it, req.MaxRows, emit)
}

// columnType names a column's BigQuery type for the result schema,
// lowercased to match the other plugins' columns and wrapped in array<...>
// when the field repeats.
func columnType(f *bigquery.FieldSchema) string {
	t := strings.ToLower(string(f.Type))
	if f.Repeated {
		return "array<" + t + ">"
	}
	return t
}

func columnsFromSchema(schema bigquery.Schema) []pluginsdk.QueryColumn {
	columns := make([]pluginsdk.QueryColumn, len(schema))
	for i, f := range schema {
		columns[i] = pluginsdk.QueryColumn{Name: f.Name, Type: columnType(f)}
	}
	return columns
}

// convertValue turns a row's value into something that serialises cleanly.
// BigQuery returns a RECORD as a slice of values positional to the field's
// own nested schema rather than a map, so one is rebuilt here keyed by
// field name, and a REPEATED field becomes a slice of its element type
// converted the same way. NUMERIC and BIGNUMERIC arrive as *big.Rat, which
// would otherwise only round-trip through JSON as a float64, so they are
// rendered as the decimal strings BigQuery itself uses.
func convertValue(v bigquery.Value, f *bigquery.FieldSchema) any {
	if v == nil {
		return nil
	}
	if f.Repeated {
		items, ok := v.([]bigquery.Value)
		if !ok {
			return v
		}
		elem := *f
		elem.Repeated = false
		values := make([]any, len(items))
		for i, item := range items {
			values[i] = convertValue(item, &elem)
		}
		return values
	}

	switch f.Type {
	case bigquery.RecordFieldType:
		fields, ok := v.([]bigquery.Value)
		if !ok {
			return v
		}
		record := make(map[string]any, len(fields))
		for i, nested := range f.Schema {
			if i < len(fields) {
				record[nested.Name] = convertValue(fields[i], nested)
			}
		}
		return record
	case bigquery.NumericFieldType:
		r, ok := v.(*big.Rat)
		if !ok {
			return v
		}
		return bigquery.NumericString(r)
	case bigquery.BigNumericFieldType:
		r, ok := v.(*big.Rat)
		if !ok {
			return v
		}
		return bigquery.BigNumericString(r)
	case bigquery.TimestampFieldType:
		t, ok := v.(time.Time)
		if !ok {
			return v
		}
		return t.Format(time.RFC3339)
	case bigquery.BytesFieldType:
		b, ok := v.([]byte)
		if !ok {
			return v
		}
		return base64.StdEncoding.EncodeToString(b)
	default:
		// civil.Date, civil.Time, civil.DateTime and *IntervalValue each
		// carry a String method that already matches BigQuery's own text
		// representation for the type.
		if str, ok := v.(fmt.Stringer); ok {
			return str.String()
		}
		return v
	}
}

// streamRows sends rows to emit in chunks of executeChunkRows, stopping at
// maxRows when it is set. Fetching the iterator's first page, even an empty
// one, always populates its Schema, so the columns are read only once Next
// has been called at least once, and the final chunk carries them even for
// a zero-row result.
func streamRows(it *bigquery.RowIterator, maxRows int64, emit func(pluginsdk.QueryResultChunk) error) error {
	var chunk pluginsdk.QueryResultChunk
	var sent int64
	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("reading row: %w", err)
		}
		if chunk.Columns == nil {
			chunk.Columns = columnsFromSchema(it.Schema)
		}

		values := make([]any, len(row))
		for i, v := range row {
			values[i] = convertValue(v, it.Schema[i])
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
	if chunk.Columns == nil {
		chunk.Columns = columnsFromSchema(it.Schema)
	}

	// By now the schema is always known, even for a zero-row result, so the
	// final chunk always carries it.
	return emit(chunk)
}
