package dataquery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSourceNotFound = errors.New("queryable source not found")
	ErrQueryNotFound  = errors.New("query not found")
	ErrInvalidInput   = errors.New("invalid input")
)

// Query statuses and decisions recorded in the query log.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusDenied    = "denied"
	// StatusCancelled is a query stopped by its caller, through the cancel endpoint or by going away mid-query.
	StatusCancelled = "cancelled"

	DecisionAllowed = "allowed"
	DecisionDenied  = "denied"
)

// Outcome reasons say why a query ended the way it did, finer than its decision and status, so the audit can be filtered by them. An authorizer may name its own (the cloud distribution adds grant), and these are the ones OSS decides.
const (
	ReasonRole        = "role"
	ReasonPermission  = "permission"
	ReasonPolicy      = "policy"
	ReasonSourceError = "source_error"
	ReasonTimeout     = "timeout"
	ReasonUnreachable = "unreachable"
	ReasonCancelled   = "cancelled"
)

// Outcomes are the audit's top-level filter: allowed means it ran to completion, so a query that failed or was cancelled after being allowed is counted under those instead.
const (
	OutcomeAllowed   = "allowed"
	OutcomeDenied    = "denied"
	OutcomeFailed    = "failed"
	OutcomeCancelled = "cancelled"
	OutcomeRunning   = "running"
)

// reasonExpr reads a record's outcome reason, deriving one for rows recorded before reasons were stored so old and new rows filter alike.
const reasonExpr = `COALESCE(outcome_reason, CASE
	WHEN decision = 'denied' THEN 'permission'
	WHEN status = 'failed' AND error = 'the query timed out' THEN 'timeout'
	WHEN status = 'failed' THEN 'source_error'
	WHEN status = 'completed' THEN 'role'
END)`

// AssetContext is the catalog's view of one asset a query referenced, as it
// stood when the query ran. It is returned with the rows so agents get meaning
// alongside data, and stored on the query record for later receipts.
type AssetContext struct {
	MRN         string     `json:"mrn"`
	AssetID     string     `json:"asset_id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Type        string     `json:"type,omitempty"`
	Providers   []string   `json:"providers,omitempty"`
	Description string     `json:"description,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`

	// What the catalog says about the table as data: who owns it, how big it is, which of its columns hold personal data and whether it is certified. ColumnsRead counts the table's catalogued columns among the result's columns, matched by name, so it is nil when the catalog has no columns for the table.
	Owners       []string             `json:"owners,omitempty"`
	RowCount     *int64               `json:"row_count,omitempty"`
	ColumnsRead  *int                 `json:"columns_read,omitempty"`
	PersonalData []PersonalDataColumn `json:"personal_data,omitempty"`
	Certified    bool                 `json:"certified"`
} // @name QueryAssetContext

// PersonalDataColumn is a column the catalog marks as personal data, and whether the query returned it.
type PersonalDataColumn struct {
	Name string `json:"name"`
	Read bool   `json:"read"`
} // @name QueryPersonalDataColumn

// Query is one query run through Marmot, allowed or denied.
type Query struct {
	ID            string  `json:"id"`
	PrincipalType string  `json:"principal_type"`
	PrincipalID   string  `json:"principal_id"`
	AuditSubject  string  `json:"audit_subject"`
	Client        string  `json:"client"`
	SourceID      *string `json:"source_id,omitempty"`
	SourceName    string  `json:"source_name"`
	// SourceProvider is the plugin behind the source, such as snowflake, so the UI can show its logo; empty once the source is deleted.
	SourceProvider string         `json:"source_provider,omitempty"`
	Statement      string         `json:"statement"`
	ReferencedMRNs []string       `json:"referenced_mrns"`
	Decision       string         `json:"decision"`
	DecisionDetail map[string]any `json:"decision_detail,omitempty"`
	Status         string         `json:"status"`
	// OutcomeReason refines the decision and status, such as role, permission, policy, source_error, timeout, unreachable or cancelled.
	OutcomeReason string `json:"outcome_reason,omitempty"`
	// ServiceAccount names the service account a machine caller ran as, Agent the client that identified itself on its behalf (an MCP client's name) and Credential how the request authenticated.
	ServiceAccount *string        `json:"service_account,omitempty"`
	Agent          *string        `json:"agent,omitempty"`
	Credential     *string        `json:"credential,omitempty"`
	RowsReturned   *int64         `json:"rows_returned,omitempty"`
	Truncated      bool           `json:"truncated"`
	Error          *string        `json:"error,omitempty"`
	Context        []AssetContext `json:"context,omitempty"`
	StartedAt      time.Time      `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
} // @name QueryRecord

// QueryFilter narrows query log listings. MRN matches queries that referenced
// that exact MRN.
type QueryFilter struct {
	// ID keeps the one record with this id, so a link can open the audit on a single query.
	ID            string
	PrincipalType string
	PrincipalID   string
	SourceName    string
	Decision      string
	Status        string
	MRN           string
	// Caller matches a principal as principal_type:principal_id or as its audit subject, such as user:alice.
	Caller string
	// Outcome is allowed, denied, failed, cancelled or running (see OutcomeAllowed), and Reason an outcome reason within it.
	Outcome string
	Reason  string
	// Clients keeps only queries from these clients, such as mcp and cli
	Clients []string
	// From and To bound when the query started; either may be zero
	From time.Time
	To   time.Time
	// Search matches the caller, the source, the statement or a referenced MRN, ignoring case
	Search string
	Limit  int
	Offset int
}

// QueryOutcome is what is written back to a query record once it finishes.
type QueryOutcome struct {
	Status       string
	Reason       string
	RowsReturned *int64
	Truncated    bool
	Error        *string
	Context      []AssetContext
}

// Repository is the data query persistence boundary. Queryable sources are not
// stored here: they are ingestion sources, resolved through a SourceProvider.
type Repository interface {
	CreateQuery(ctx context.Context, q *Query) error
	CompleteQuery(ctx context.Context, id string, outcome QueryOutcome) error
	GetQuery(ctx context.Context, id string) (*Query, error)
	ListQueries(ctx context.Context, filter QueryFilter) ([]*Query, int, error)
	// EachQuery calls fn for every query the filter matches, newest first, ignoring its limit and offset, reading from one cursor so an export never holds the log in memory.
	EachQuery(ctx context.Context, filter QueryFilter, fn func(*Query) error) error

	// TableFacts reads what the catalog knows about tables beyond the asset itself, keyed by asset id.
	TableFacts(ctx context.Context, assetIDs []string) (map[string]TableFacts, error)
	CreateAccessRequest(ctx context.Context, req *AccessRequest) error
	// RecentAccessRequest returns the caller's latest request for the same access since a time, or nil.
	RecentAccessRequest(ctx context.Context, principalType, principalID, sourceName, needed string, since time.Time) (*AccessRequest, error)

	Stats(ctx context.Context, filter StatsFilter) (*Stats, error)

	ListSourceTables(ctx context.Context, filter SourceTablesFilter) ([]SourceTable, map[string]map[string]any, error)
	SourceActivity(ctx context.Context, sources []*Source) error
	SourcesWithTables(ctx context.Context, ids []string, assets func(idExpr string, next int) (string, []any)) ([]string, error)
}

// PostgresRepository implements Repository on pgx.
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateQuery(ctx context.Context, q *Query) error {
	if q.ReferencedMRNs == nil {
		q.ReferencedMRNs = []string{}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO data_queries (principal_type, principal_id, audit_subject, client, source_id, source_name,
			statement, referenced_mrns, decision, decision_detail, status, outcome_reason, service_account, agent, credential)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), $13, $14, $15)
		RETURNING id, started_at`,
		q.PrincipalType, q.PrincipalID, q.AuditSubject, q.Client, q.SourceID, q.SourceName,
		q.Statement, q.ReferencedMRNs, q.Decision, q.DecisionDetail, q.Status, q.OutcomeReason, q.ServiceAccount, q.Agent, q.Credential,
	).Scan(&q.ID, &q.StartedAt)
	if err != nil {
		return fmt.Errorf("recording query: %w", err)
	}
	return nil
}

// CompleteQuery records a query's outcome and folds it into the hourly
// rollups in one statement, so the dashboards can never drift from the log.
func (r *PostgresRepository) CompleteQuery(ctx context.Context, id string, outcome QueryOutcome) error {
	var contextArg any
	if len(outcome.Context) > 0 {
		contextArg = outcome.Context
	}
	_, err := r.db.Exec(ctx, `
		WITH q AS (
			UPDATE data_queries
			SET status = $2, rows_returned = $3, truncated = $4, error = $5, context = $6, completed_at = NOW(),
			    outcome_reason = COALESCE(NULLIF($7, ''), outcome_reason)
			WHERE id = $1
			RETURNING date_trunc('hour', started_at) AS bucket, principal_type, principal_id, audit_subject,
			          source_name, decision, status, referenced_mrns, COALESCE(rows_returned, 0) AS rows_returned
		), callers AS (
			INSERT INTO data_query_stats_hourly (bucket, principal_type, principal_id, audit_subject, source_name, decision, status, queries, rows_returned)
			SELECT bucket, principal_type, principal_id, audit_subject, source_name, decision, status, 1, rows_returned FROM q
			ON CONFLICT (bucket, principal_type, principal_id, source_name, decision, status) DO UPDATE
			SET queries = data_query_stats_hourly.queries + 1,
			    rows_returned = data_query_stats_hourly.rows_returned + EXCLUDED.rows_returned,
			    audit_subject = EXCLUDED.audit_subject
		)
		INSERT INTO data_query_table_stats_hourly (bucket, mrn, principal_type, principal_id, audit_subject, decision, queries)
		SELECT DISTINCT q.bucket, m, q.principal_type, q.principal_id, q.audit_subject, q.decision, 1
		FROM q, unnest(q.referenced_mrns) AS m
		WHERE m NOT LIKE 'mrn://source/%'
		ON CONFLICT (bucket, mrn, principal_type, principal_id, decision) DO UPDATE
		SET queries = data_query_table_stats_hourly.queries + 1,
		    audit_subject = EXCLUDED.audit_subject`,
		id, outcome.Status, outcome.RowsReturned, outcome.Truncated, outcome.Error, contextArg, outcome.Reason)
	if err != nil {
		return fmt.Errorf("completing query: %w", err)
	}
	return nil
}

var queryColumns = `id, principal_type, principal_id, audit_subject, client, source_id, source_name,
	statement, referenced_mrns, decision, decision_detail, status, COALESCE(` + reasonExpr + `, ''), service_account, agent, credential,
	rows_returned, truncated, error, context, started_at, completed_at,
	COALESCE((SELECT sc.plugin_id FROM ingestion_schedules sc WHERE sc.name = data_queries.source_name LIMIT 1), '')`

func scanQuery(row pgx.Row) (*Query, error) {
	var q Query
	if err := row.Scan(&q.ID, &q.PrincipalType, &q.PrincipalID, &q.AuditSubject, &q.Client, &q.SourceID, &q.SourceName,
		&q.Statement, &q.ReferencedMRNs, &q.Decision, &q.DecisionDetail, &q.Status, &q.OutcomeReason, &q.ServiceAccount, &q.Agent, &q.Credential,
		&q.RowsReturned, &q.Truncated, &q.Error,
		&q.Context, &q.StartedAt, &q.CompletedAt, &q.SourceProvider); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQueryNotFound
		}
		return nil, fmt.Errorf("scanning query: %w", err)
	}
	return &q, nil
}

func (r *PostgresRepository) GetQuery(ctx context.Context, id string) (*Query, error) {
	return scanQuery(r.db.QueryRow(ctx, `SELECT `+queryColumns+` FROM data_queries WHERE id = $1`, id))
}

// queryWhere turns a filter into a WHERE clause and its arguments, shared by the listing and the export so both read the same rows.
func queryWhere(filter QueryFilter) (string, []any) {
	where := ` WHERE 1=1`
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.ID != "" {
		where += ` AND id::text = ` + arg(filter.ID)
	}
	if filter.PrincipalType != "" {
		where += ` AND principal_type = ` + arg(filter.PrincipalType)
	}
	if filter.PrincipalID != "" {
		where += ` AND principal_id = ` + arg(filter.PrincipalID)
	}
	if caller := strings.TrimSpace(filter.Caller); caller != "" {
		// Both spellings share the principal type before the colon, which keeps the principal index usable.
		kind, rest, _ := strings.Cut(caller, ":")
		where += ` AND principal_type = ` + arg(kind) + ` AND (principal_id = ` + arg(rest) + ` OR audit_subject = ` + arg(caller) + `)`
	}
	if filter.SourceName != "" {
		where += ` AND source_name = ` + arg(filter.SourceName)
	}
	if len(filter.Clients) > 0 {
		where += ` AND client = ANY(` + arg(filter.Clients) + `)`
	}
	if filter.Decision != "" {
		where += ` AND decision = ` + arg(filter.Decision)
	}
	if filter.Status != "" {
		where += ` AND status = ` + arg(filter.Status)
	}
	switch filter.Outcome {
	case OutcomeAllowed:
		where += ` AND decision = 'allowed' AND status = 'completed'`
	case OutcomeDenied:
		where += ` AND decision = 'denied'`
	case OutcomeFailed, OutcomeCancelled, OutcomeRunning:
		where += ` AND status = ` + arg(filter.Outcome)
	}
	if filter.Reason != "" {
		where += ` AND ` + reasonExpr + ` = ` + arg(filter.Reason)
	}
	if filter.MRN != "" {
		where += ` AND referenced_mrns @> ARRAY[` + arg(filter.MRN) + `]::text[]`
	}
	if !filter.From.IsZero() {
		where += ` AND started_at >= ` + arg(filter.From)
	}
	if !filter.To.IsZero() {
		where += ` AND started_at < ` + arg(filter.To)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := arg("%" + likeEscaper.Replace(search) + "%")
		where += ` AND (audit_subject ILIKE ` + pattern + ` OR source_name ILIKE ` + pattern +
			` OR statement ILIKE ` + pattern + ` OR array_to_string(referenced_mrns, ' ') ILIKE ` + pattern +
			` OR COALESCE(agent, '') ILIKE ` + pattern + ` OR COALESCE(service_account, '') ILIKE ` + pattern + `)`
	}
	return where, args
}

func (r *PostgresRepository) ListQueries(ctx context.Context, filter QueryFilter) ([]*Query, int, error) {
	where, args := queryWhere(filter)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM data_queries`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting queries: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	listQuery := `SELECT ` + queryColumns + ` FROM data_queries` + where +
		fmt.Sprintf(` ORDER BY started_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing queries: %w", err)
	}
	defer rows.Close()

	queries := []*Query{}
	for rows.Next() {
		q, err := scanQuery(rows)
		if err != nil {
			return nil, 0, err
		}
		queries = append(queries, q)
	}
	return queries, total, rows.Err()
}

func (r *PostgresRepository) EachQuery(ctx context.Context, filter QueryFilter, fn func(*Query) error) error {
	where, args := queryWhere(filter)
	rows, err := r.db.Query(ctx, `SELECT `+queryColumns+` FROM data_queries`+where+` ORDER BY started_at DESC`, args...)
	if err != nil {
		return fmt.Errorf("exporting queries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		q, err := scanQuery(rows)
		if err != nil {
			return err
		}
		if err := fn(q); err != nil {
			return err
		}
	}
	return rows.Err()
}
