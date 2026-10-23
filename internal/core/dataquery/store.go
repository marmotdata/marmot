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

	DecisionAllowed = "allowed"
	DecisionDenied  = "denied"
)

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
} // @name QueryAssetContext

// Query is one query run through Marmot, allowed or denied.
type Query struct {
	ID             string         `json:"id"`
	PrincipalType  string         `json:"principal_type"`
	PrincipalID    string         `json:"principal_id"`
	AuditSubject   string         `json:"audit_subject"`
	Client         string         `json:"client"`
	SourceID       *string        `json:"source_id,omitempty"`
	SourceName     string         `json:"source_name"`
	Statement      string         `json:"statement"`
	ReferencedMRNs []string       `json:"referenced_mrns"`
	Decision       string         `json:"decision"`
	DecisionDetail map[string]any `json:"decision_detail,omitempty"`
	Status         string         `json:"status"`
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
	PrincipalType string
	PrincipalID   string
	SourceName    string
	Decision      string
	Status        string
	MRN           string
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
			statement, referenced_mrns, decision, decision_detail, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, started_at`,
		q.PrincipalType, q.PrincipalID, q.AuditSubject, q.Client, q.SourceID, q.SourceName,
		q.Statement, q.ReferencedMRNs, q.Decision, q.DecisionDetail, q.Status,
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
			SET status = $2, rows_returned = $3, truncated = $4, error = $5, context = $6, completed_at = NOW()
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
		id, outcome.Status, outcome.RowsReturned, outcome.Truncated, outcome.Error, contextArg)
	if err != nil {
		return fmt.Errorf("completing query: %w", err)
	}
	return nil
}

const queryColumns = `id, principal_type, principal_id, audit_subject, client, source_id, source_name,
	statement, referenced_mrns, decision, decision_detail, status, rows_returned, truncated, error,
	context, started_at, completed_at`

func scanQuery(row pgx.Row) (*Query, error) {
	var q Query
	if err := row.Scan(&q.ID, &q.PrincipalType, &q.PrincipalID, &q.AuditSubject, &q.Client, &q.SourceID, &q.SourceName,
		&q.Statement, &q.ReferencedMRNs, &q.Decision, &q.DecisionDetail, &q.Status, &q.RowsReturned, &q.Truncated, &q.Error,
		&q.Context, &q.StartedAt, &q.CompletedAt); err != nil {
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

func (r *PostgresRepository) ListQueries(ctx context.Context, filter QueryFilter) ([]*Query, int, error) {
	where := ` WHERE 1=1`
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.PrincipalType != "" {
		where += ` AND principal_type = ` + arg(filter.PrincipalType)
	}
	if filter.PrincipalID != "" {
		where += ` AND principal_id = ` + arg(filter.PrincipalID)
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
			` OR statement ILIKE ` + pattern + ` OR array_to_string(referenced_mrns, ' ') ILIKE ` + pattern + `)`
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM data_queries`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting queries: %w", err)
	}

	listQuery := `SELECT ` + queryColumns + ` FROM data_queries` + where +
		` ORDER BY started_at DESC LIMIT ` + arg(filter.Limit) + ` OFFSET ` + arg(filter.Offset)
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
