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

// ErrSavedQueryNotFound is returned for a saved query that does not exist or belongs to someone else, so ids cannot be probed.
var ErrSavedQueryNotFound = errors.New("saved query not found")

// Limits on what a saved query may hold, generous enough for real statements and small enough that one person cannot fill the table.
const (
	maxSavedQueryName      = 255
	maxSavedQueryStatement = 100_000
	maxSavedQueriesPerUser = 500
	defaultSavedQueryLimit = 50
)

// SavedQuery is a statement a person keeps to run again.
type SavedQuery struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	SQL       string     `json:"sql"`
	Source    string     `json:"source"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
} // @name SavedQuery

// SavedQueryInput creates or replaces a saved query.
type SavedQueryInput struct {
	Name   string `json:"name"`
	SQL    string `json:"sql"`
	Source string `json:"source"`
} // @name SavedQueryInput

func (in *SavedQueryInput) normalise() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Source = strings.TrimSpace(in.Source)
	switch {
	case in.Name == "":
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	case len(in.Name) > maxSavedQueryName:
		return fmt.Errorf("%w: name is longer than %d characters", ErrInvalidInput, maxSavedQueryName)
	case strings.TrimSpace(in.SQL) == "":
		return fmt.Errorf("%w: sql is required", ErrInvalidInput)
	case len(in.SQL) > maxSavedQueryStatement:
		return fmt.Errorf("%w: sql is longer than %d characters", ErrInvalidInput, maxSavedQueryStatement)
	case len(in.Source) > maxSavedQueryName:
		return fmt.Errorf("%w: source is longer than %d characters", ErrInvalidInput, maxSavedQueryName)
	}
	return nil
}

// SavedQueryStore keeps each person's saved queries. Every method is scoped to the owner, so one person can never read or change another's.
type SavedQueryStore struct {
	db *pgxpool.Pool
}

func NewSavedQueryStore(db *pgxpool.Pool) *SavedQueryStore {
	return &SavedQueryStore{db: db}
}

const savedQueryColumns = `id, name, statement, source_name, last_run_at, created_at, updated_at`

func scanSavedQuery(row pgx.Row) (*SavedQuery, error) {
	var q SavedQuery
	if err := row.Scan(&q.ID, &q.Name, &q.SQL, &q.Source, &q.LastRunAt, &q.CreatedAt, &q.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSavedQueryNotFound
		}
		return nil, fmt.Errorf("scanning saved query: %w", err)
	}
	return &q, nil
}

// List returns the owner's saved queries, the most recently run or edited first, which is the order Home and the workspace offer them in.
func (s *SavedQueryStore) List(ctx context.Context, userID string, limit, offset int) ([]*SavedQuery, int, error) {
	if limit <= 0 || limit > maxSavedQueriesPerUser {
		limit = defaultSavedQueryLimit
	}
	offset = max(offset, 0)

	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM saved_queries WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting saved queries: %w", err)
	}
	rows, err := s.db.Query(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries WHERE user_id = $1
		ORDER BY GREATEST(updated_at, COALESCE(last_run_at, updated_at)) DESC, id
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing saved queries: %w", err)
	}
	defer rows.Close()

	out := []*SavedQuery{}
	for rows.Next() {
		q, err := scanSavedQuery(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, q)
	}
	return out, total, rows.Err()
}

func (s *SavedQueryStore) Get(ctx context.Context, userID, id string) (*SavedQuery, error) {
	return scanSavedQuery(s.db.QueryRow(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries WHERE id = $1 AND user_id = $2`, id, userID))
}

func (s *SavedQueryStore) Create(ctx context.Context, userID string, in SavedQueryInput) (*SavedQuery, error) {
	if err := in.normalise(); err != nil {
		return nil, err
	}
	// The cap is checked in the insert itself so two saves at once cannot both slip past it.
	q, err := scanSavedQuery(s.db.QueryRow(ctx, `
		INSERT INTO saved_queries (user_id, name, statement, source_name)
		SELECT $1, $2, $3, $4
		WHERE (SELECT count(*) FROM saved_queries WHERE user_id = $1) < $5
		RETURNING `+savedQueryColumns, userID, in.Name, in.SQL, in.Source, maxSavedQueriesPerUser))
	if errors.Is(err, ErrSavedQueryNotFound) {
		return nil, fmt.Errorf("%w: at most %d saved queries are kept per person", ErrInvalidInput, maxSavedQueriesPerUser)
	}
	return q, err
}

func (s *SavedQueryStore) Update(ctx context.Context, userID, id string, in SavedQueryInput) (*SavedQuery, error) {
	if err := in.normalise(); err != nil {
		return nil, err
	}
	return scanSavedQuery(s.db.QueryRow(ctx, `
		UPDATE saved_queries SET name = $3, statement = $4, source_name = $5, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING `+savedQueryColumns, id, userID, in.Name, in.SQL, in.Source))
}

// MarkRun records that the owner ran the saved query, which moves it up the list.
func (s *SavedQueryStore) MarkRun(ctx context.Context, userID, id string) (*SavedQuery, error) {
	return scanSavedQuery(s.db.QueryRow(ctx, `
		UPDATE saved_queries SET last_run_at = NOW() WHERE id = $1 AND user_id = $2
		RETURNING `+savedQueryColumns, id, userID))
}

func (s *SavedQueryStore) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM saved_queries WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting saved query: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSavedQueryNotFound
	}
	return nil
}
