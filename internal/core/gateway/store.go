package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTargetNotFound  = errors.New("query target not found")
	ErrGrantNotFound   = errors.New("grant not found")
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionClosed   = errors.New("session is revoked or expired")
	ErrInvalidInput    = errors.New("invalid input")
)

// Target is a queryable engine attachment: a plugin id plus the connection
// config its long-running instance is driven with. Config holds sensitive
// fields encrypted at rest and is redacted in API responses.
type Target struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	PluginID  string         `json:"plugin_id"`
	Modes     []string       `json:"modes"`
	Config    map[string]any `json:"config,omitempty"`
	Enabled   bool           `json:"enabled"`
	// IngestOnQuery asks the gateway to refresh this source's catalog
	// (asynchronously, rate-limited) after a successful query.
	IngestOnQuery bool      `json:"ingest_on_query"`
	CreatedBy *string        `json:"created_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
} // @name GatewayTarget

// Grant allows a principal to query resources matching an MRN selector.
// Deny is the default: queries are allowed only where every referenced
// resource matches at least one live grant.
type Grant struct {
	ID               string     `json:"id"`
	PrincipalType    string     `json:"principal_type"`
	PrincipalID      string     `json:"principal_id"`
	ResourceSelector string     `json:"resource_selector"`
	Actions          []string   `json:"actions"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedBy        *string    `json:"created_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevokedBy        *string    `json:"revoked_by,omitempty"`
	Reason           *string    `json:"reason,omitempty"`
} // @name GatewayGrant

// Session is what an agent holds instead of a database credential. Every
// query names a session and revoking the session kills its access.
type Session struct {
	ID             string     `json:"id"`
	PrincipalType  string     `json:"principal_type"`
	PrincipalID    string     `json:"principal_id"`
	Purpose        *string    `json:"purpose,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
} // @name GatewaySession

// AuditEntry is one query attempt, allowed or denied, on any path.
type AuditEntry struct {
	ID             string         `json:"id"`
	SessionID      *string        `json:"session_id,omitempty"`
	PrincipalType  string         `json:"principal_type"`
	PrincipalID    string         `json:"principal_id"`
	AuditSubject   string         `json:"audit_subject"`
	TargetID       *string        `json:"target_id,omitempty"`
	TargetName     string         `json:"target_name"`
	QueryText      string         `json:"query_text"`
	ReferencedMRNs []string       `json:"referenced_mrns,omitempty"`
	Decision       string         `json:"decision"`
	DecisionDetail map[string]any `json:"decision_detail,omitempty"`
	Status         string         `json:"status"`
	RowsReturned   *int64         `json:"rows_returned,omitempty"`
	Error          *string        `json:"error,omitempty"`
	Source         string         `json:"source"`
	StartedAt      time.Time      `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
} // @name GatewayAuditEntry

// AuditFilter narrows audit log listings.
type AuditFilter struct {
	PrincipalID string
	SessionID   string
	TargetName  string
	Decision    string
	Limit       int
	Offset      int
}

// Repository is the query gateway's persistence boundary. Query targets are
// not stored here — they are queryable ingestion sources, resolved through a
// TargetProvider (see service.go).
type Repository interface {
	CreateGrant(ctx context.Context, g *Grant) error
	GetGrant(ctx context.Context, id string) (*Grant, error)
	ListGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error)
	ListLiveGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error)
	RevokeGrant(ctx context.Context, id, revokedBy, reason string) error

	CreateSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, limit, offset int) ([]*Session, error)
	TouchSession(ctx context.Context, id string) error
	RevokeSession(ctx context.Context, id, revokedBy string) error

	CreateAuditEntry(ctx context.Context, e *AuditEntry) error
	CompleteAuditEntry(ctx context.Context, id, status string, rowsReturned *int64, queryErr *string) error
	ListAuditEntries(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)
}

// PostgresRepository implements Repository on pgx.
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateGrant(ctx context.Context, g *Grant) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO gateway_grants (principal_type, principal_id, resource_selector, actions, expires_at, created_by, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		g.PrincipalType, g.PrincipalID, g.ResourceSelector, g.Actions, g.ExpiresAt, g.CreatedBy, g.Reason,
	).Scan(&g.ID, &g.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating grant: %w", err)
	}
	return nil
}

const grantColumns = `id, principal_type, principal_id, resource_selector, actions, expires_at, created_by, created_at, revoked_at, revoked_by, reason`

func scanGrant(row pgx.Row) (*Grant, error) {
	var g Grant
	if err := row.Scan(&g.ID, &g.PrincipalType, &g.PrincipalID, &g.ResourceSelector, &g.Actions, &g.ExpiresAt, &g.CreatedBy, &g.CreatedAt, &g.RevokedAt, &g.RevokedBy, &g.Reason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGrantNotFound
		}
		return nil, fmt.Errorf("scanning grant: %w", err)
	}
	return &g, nil
}

func (r *PostgresRepository) GetGrant(ctx context.Context, id string) (*Grant, error) {
	return scanGrant(r.db.QueryRow(ctx, `SELECT `+grantColumns+` FROM gateway_grants WHERE id = $1`, id))
}

func (r *PostgresRepository) listGrants(ctx context.Context, query string, args ...any) ([]*Grant, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing grants: %w", err)
	}
	defer rows.Close()

	var grants []*Grant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (r *PostgresRepository) ListGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error) {
	if principalType == "" {
		return r.listGrants(ctx, `SELECT `+grantColumns+` FROM gateway_grants ORDER BY created_at DESC`)
	}
	return r.listGrants(ctx, `
		SELECT `+grantColumns+` FROM gateway_grants
		WHERE principal_type = $1 AND principal_id = $2
		ORDER BY created_at DESC`, principalType, principalID)
}

func (r *PostgresRepository) ListLiveGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error) {
	return r.listGrants(ctx, `
		SELECT `+grantColumns+` FROM gateway_grants
		WHERE principal_type = $1 AND principal_id = $2
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		principalType, principalID)
}

func (r *PostgresRepository) RevokeGrant(ctx context.Context, id, revokedBy, reason string) error {
	var reasonArg *string
	if reason != "" {
		reasonArg = &reason
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE gateway_grants SET revoked_at = NOW(), revoked_by = $2, reason = COALESCE($3, reason)
		WHERE id = $1 AND revoked_at IS NULL`, id, revokedBy, reasonArg)
	if err != nil {
		return fmt.Errorf("revoking grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrGrantNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, s *Session) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO gateway_sessions (principal_type, principal_id, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		s.PrincipalType, s.PrincipalID, s.Purpose, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

const sessionColumns = `id, principal_type, principal_id, purpose, created_at, expires_at, last_activity_at, revoked_at`

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	if err := row.Scan(&s.ID, &s.PrincipalType, &s.PrincipalID, &s.Purpose, &s.CreatedAt, &s.ExpiresAt, &s.LastActivityAt, &s.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("scanning session: %w", err)
	}
	return &s, nil
}

func (r *PostgresRepository) GetSession(ctx context.Context, id string) (*Session, error) {
	return scanSession(r.db.QueryRow(ctx, `SELECT `+sessionColumns+` FROM gateway_sessions WHERE id = $1`, id))
}

func (r *PostgresRepository) ListSessions(ctx context.Context, limit, offset int) ([]*Session, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+sessionColumns+` FROM gateway_sessions ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *PostgresRepository) TouchSession(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE gateway_sessions SET last_activity_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, id, revokedBy string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE gateway_sessions SET revoked_at = NOW(), revoked_by = $2
		WHERE id = $1 AND revoked_at IS NULL`, id, revokedBy)
	if err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateAuditEntry(ctx context.Context, e *AuditEntry) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO gateway_audit_log (session_id, principal_type, principal_id, audit_subject, target_id, target_name,
			query_text, referenced_mrns, decision, decision_detail, status, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, started_at`,
		e.SessionID, e.PrincipalType, e.PrincipalID, e.AuditSubject, e.TargetID, e.TargetName,
		e.QueryText, e.ReferencedMRNs, e.Decision, e.DecisionDetail, e.Status, e.Source,
	).Scan(&e.ID, &e.StartedAt)
	if err != nil {
		return fmt.Errorf("creating audit entry: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CompleteAuditEntry(ctx context.Context, id, status string, rowsReturned *int64, queryErr *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE gateway_audit_log SET status = $2, rows_returned = $3, error = $4, completed_at = NOW()
		WHERE id = $1`, id, status, rowsReturned, queryErr)
	if err != nil {
		return fmt.Errorf("completing audit entry: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListAuditEntries(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error) {
	query := `
		SELECT id, session_id, principal_type, principal_id, audit_subject, target_id, target_name,
		       query_text, referenced_mrns, decision, decision_detail, status, rows_returned, error,
		       source, started_at, completed_at
		FROM gateway_audit_log WHERE 1=1`
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.PrincipalID != "" {
		query += ` AND principal_id = ` + arg(filter.PrincipalID)
	}
	if filter.SessionID != "" {
		query += ` AND session_id = ` + arg(filter.SessionID)
	}
	if filter.TargetName != "" {
		query += ` AND target_name = ` + arg(filter.TargetName)
	}
	if filter.Decision != "" {
		query += ` AND decision = ` + arg(filter.Decision)
	}
	query += ` ORDER BY started_at DESC LIMIT ` + arg(filter.Limit) + ` OFFSET ` + arg(filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.PrincipalType, &e.PrincipalID, &e.AuditSubject, &e.TargetID, &e.TargetName,
			&e.QueryText, &e.ReferencedMRNs, &e.Decision, &e.DecisionDetail, &e.Status, &e.RowsReturned, &e.Error,
			&e.Source, &e.StartedAt, &e.CompletedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

