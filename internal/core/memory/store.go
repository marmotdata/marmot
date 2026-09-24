package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository scopes every call to one entity. An id belonging to a different
// entity does not match.
type Repository interface {
	Remember(ctx context.Context, e Entity, in RememberInput) (*Memory, error)
	Get(ctx context.Context, e Entity, id string) (*Memory, error)
	Update(ctx context.Context, e Entity, id string, in UpdateInput) (*Memory, error)
	Delete(ctx context.Context, e Entity, id string) error
	List(ctx context.Context, e Entity, filter ListFilter) (*ListResult, error)
	// Search ranks by full-text relevance.
	Search(ctx context.Context, e Entity, q SearchQuery) ([]*Memory, error)
	// SearchAll is Search over every entity.
	SearchAll(ctx context.Context, q SearchQuery) ([]*Memory, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// entityColumn is the column holding an entity type's id. Each type has its
// own foreign key, so an entity's memory is deleted with it.
func entityColumn(t EntityType) string {
	switch t {
	case EntityAsset:
		return "asset_id"
	case EntityDataProduct:
		return "data_product_id"
	}
	panic(fmt.Sprintf("memory: unknown entity type %q", t))
}

const memoryColumns = `id,
	CASE WHEN asset_id IS NOT NULL THEN 'asset' ELSE 'data_product' END,
	COALESCE(asset_id, data_product_id::text), content,
	created_by_type, created_by_id, created_by_name, COALESCE(session_id, ''),
	updated_by_type, updated_by_id, updated_by_name, COALESCE(updated_session_id, ''),
	created_at, updated_at`

func scanMemory(row pgx.Row, extra ...any) (*Memory, error) {
	var m Memory
	dest := make([]any, 0, 14+len(extra))
	dest = append(dest,
		&m.ID, &m.EntityType, &m.EntityID, &m.Content,
		&m.CreatedBy.Type, &m.CreatedBy.ID, &m.CreatedBy.Name, &m.SessionID,
		&m.UpdatedBy.Type, &m.UpdatedBy.ID, &m.UpdatedBy.Name, &m.UpdatedSessionID,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err := row.Scan(append(dest, extra...)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *PostgresRepository) Remember(ctx context.Context, e Entity, in RememberInput) (*Memory, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO memories (
			`+entityColumn(e.Type)+`, content,
			created_by_type, created_by_id, created_by_name, session_id,
			updated_by_type, updated_by_id, updated_by_name, updated_session_id
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $3, $4, $5, NULLIF($6, ''))
		RETURNING `+memoryColumns,
		e.ID, in.Content,
		in.Author.Type, in.Author.ID, in.Author.Name, in.SessionID,
	)
	m, err := scanMemory(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("writing memory: %w", err)
	}
	return m, nil
}

func (r *PostgresRepository) Get(ctx context.Context, e Entity, id string) (*Memory, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+memoryColumns+` FROM memories
		WHERE `+entityColumn(e.Type)+` = $1 AND id = $2`,
		e.ID, id)
	return scanMemory(row)
}

func (r *PostgresRepository) Update(ctx context.Context, e Entity, id string, in UpdateInput) (*Memory, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE memories SET
			content = $3,
			updated_by_type = $4, updated_by_id = $5, updated_by_name = $6,
			updated_session_id = NULLIF($7, ''),
			updated_at = NOW()
		WHERE `+entityColumn(e.Type)+` = $1 AND id = $2
		RETURNING `+memoryColumns,
		e.ID, id, in.Content,
		in.Author.Type, in.Author.ID, in.Author.Name, in.SessionID)
	return scanMemory(row)
}

func (r *PostgresRepository) Delete(ctx context.Context, e Entity, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM memories WHERE `+entityColumn(e.Type)+` = $1 AND id = $2`, e.ID, id)
	if err != nil {
		return fmt.Errorf("deleting memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scope is the condition choosing which entities' entries a query reads.
// Placeholder numbering continues after the args passed in.
type scope func(args []any) (string, []any)

func entityScope(e Entity) scope {
	return func(args []any) (string, []any) {
		args = append(args, e.ID)
		return fmt.Sprintf("%s = $%d", entityColumn(e.Type), len(args)), args
	}
}

func allEntities(args []any) (string, []any) {
	return "TRUE", args
}

// where builds the filter shared by list and search. Placeholder numbering
// continues after the args passed in.
func where(sc scope, f Filter, args []any) (string, []any) {
	cond, args := sc(args)
	conds := []string{cond}
	if f.SessionID != "" {
		args = append(args, f.SessionID)
		conds = append(conds, fmt.Sprintf("(session_id = $%d OR updated_session_id = $%d)", len(args), len(args)))
	}
	return strings.Join(conds, " AND "), args
}

// orderBy is the ORDER BY clause for a sort.
func orderBy(s Sort) string {
	switch s {
	case SortCreated:
		return "created_at DESC"
	}
	return "updated_at DESC"
}

// List returns entries in the filter's sort order.
func (r *PostgresRepository) List(ctx context.Context, e Entity, f ListFilter) (*ListResult, error) {
	cond, args := where(entityScope(e), f.Filter, nil)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM memories WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting memories: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT `+memoryColumns+` FROM memories
		WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		cond, orderBy(f.Sort), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("listing memories: %w", err)
	}
	defer rows.Close()

	out := &ListResult{Memories: make([]*Memory, 0, f.Limit), Total: total}
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out.Memories = append(out.Memories, m)
	}
	return out, rows.Err()
}

// searchQuery builds the full-text ranking query.
func searchQuery(sc scope, q SearchQuery) (string, []any) {
	cond, args := where(sc, q.Filter, []any{q.Query})
	args = append(args, q.Limit)
	return fmt.Sprintf(`
		SELECT `+memoryColumns+`, ts_rank(search_text, websearch_to_tsquery('english', $1))::float8 AS score
		FROM memories
		WHERE %s AND search_text @@ websearch_to_tsquery('english', $1)
		ORDER BY score DESC, updated_at DESC
		LIMIT $%d`, cond, len(args)), args
}

func (r *PostgresRepository) Search(ctx context.Context, e Entity, q SearchQuery) ([]*Memory, error) {
	return r.search(ctx, entityScope(e), q)
}

func (r *PostgresRepository) SearchAll(ctx context.Context, q SearchQuery) ([]*Memory, error) {
	return r.search(ctx, allEntities, q)
}

func (r *PostgresRepository) search(ctx context.Context, sc scope, q SearchQuery) ([]*Memory, error) {
	sql, args := searchQuery(sc, q)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("searching memories: %w", err)
	}
	defer rows.Close()

	out := make([]*Memory, 0, q.Limit)
	for rows.Next() {
		var score float64
		m, err := scanMemory(rows, &score)
		if err != nil {
			return nil, err
		}
		m.Score = &score
		out = append(out, m)
	}
	return out, rows.Err()
}
