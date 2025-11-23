package glossary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/metrics"
)

var (
	ErrNotFound = errors.New("glossary term not found")
	ErrConflict = errors.New("glossary term with this name already exists")
)

type Repository interface {
	Create(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error
	Get(ctx context.Context, id string) (*GlossaryTerm, error)
	Update(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)
	GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error)
}

type PostgresRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

func NewPostgresRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresRepository {
	return &PostgresRepository{
		db:       db,
		recorder: recorder,
	}
}

func (r *PostgresRepository) loadOwners(ctx context.Context, termID string) ([]Owner, error) {
	query := `
		SELECT
			COALESCE(u.id::text, t.id::text) as id,
			u.username,
			COALESCE(u.name, t.name) as name,
			CASE WHEN u.id IS NOT NULL THEN 'user' ELSE 'team' END as type
		FROM glossary_term_owners gto
		LEFT JOIN users u ON gto.user_id = u.id
		LEFT JOIN teams t ON gto.team_id = t.id
		WHERE gto.glossary_term_id = $1
		ORDER BY type, COALESCE(u.username, t.name)`

	rows, err := r.db.Query(ctx, query, termID)
	if err != nil {
		return nil, fmt.Errorf("loading owners: %w", err)
	}
	defer rows.Close()

	owners := []Owner{}
	for rows.Next() {
		var owner Owner
		if err := rows.Scan(&owner.ID, &owner.Username, &owner.Name, &owner.Type); err != nil {
			return nil, fmt.Errorf("scanning owner: %w", err)
		}
		owners = append(owners, owner)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating owners: %w", err)
	}

	return owners, nil
}

func (r *PostgresRepository) setOwners(ctx context.Context, tx pgx.Tx, termID string, owners []OwnerInput) error {
	_, err := tx.Exec(ctx, "DELETE FROM glossary_term_owners WHERE glossary_term_id = $1", termID)
	if err != nil {
		return fmt.Errorf("deleting existing owners: %w", err)
	}

	for _, owner := range owners {
		var query string
		if owner.Type == "user" {
			query = "INSERT INTO glossary_term_owners (glossary_term_id, user_id) VALUES ($1, $2)"
		} else {
			query = "INSERT INTO glossary_term_owners (glossary_term_id, team_id) VALUES ($1, $2)"
		}

		_, err := tx.Exec(ctx, query, termID, owner.ID)
		if err != nil {
			return fmt.Errorf("inserting owner: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) Create(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error {
	start := time.Now()

	metadataJSON, err := json.Marshal(term.Metadata)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_create", time.Since(start), false)
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_create", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO glossary_terms (
			name, definition, description, parent_term_id,
			metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	err = tx.QueryRow(ctx, query,
		term.Name, term.Definition, term.Description,
		term.ParentTermID, metadataJSON,
		term.CreatedAt, term.UpdatedAt,
	).Scan(&term.ID)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_create", time.Since(start), false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("creating glossary term: %w", err)
	}

	if err := r.setOwners(ctx, tx, term.ID, owners); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_create", time.Since(start), false)
		return fmt.Errorf("setting owners: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_create", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	duration := time.Since(start)
	r.recorder.RecordDBQuery(ctx, "glossary_create", duration, true)
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*GlossaryTerm, error) {
	start := time.Now()

	query := `
		SELECT id, name, definition, description, parent_term_id,
			   metadata, created_at, updated_at, deleted_at
		FROM glossary_terms
		WHERE id = $1`

	var term GlossaryTerm
	var metadataJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&term.ID, &term.Name, &term.Definition,
		&term.Description, &term.ParentTermID,
		&metadataJSON, &term.CreatedAt, &term.UpdatedAt, &term.DeletedAt,
	)

	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.recorder.RecordDBQuery(ctx, "glossary_get", duration, true)
			return nil, ErrNotFound
		}
		r.recorder.RecordDBQuery(ctx, "glossary_get", duration, false)
		return nil, fmt.Errorf("getting glossary term: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &term.Metadata); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get", duration, false)
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	term.Owners, err = r.loadOwners(ctx, id)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get", duration, false)
		return nil, fmt.Errorf("loading owners: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_get", duration, true)
	return &term, nil
}

func (r *PostgresRepository) Update(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error {
	start := time.Now()

	metadataJSON, err := json.Marshal(term.Metadata)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_update", time.Since(start), false)
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_update", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE glossary_terms
		SET name = $1, definition = $2, description = $3, parent_term_id = $4,
			metadata = $5, updated_at = $6, deleted_at = $7
		WHERE id = $8`

	result, err := tx.Exec(ctx, query,
		term.Name, term.Definition, term.Description, term.ParentTermID,
		metadataJSON, term.UpdatedAt, term.DeletedAt, term.ID,
	)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_update", duration, false)
		return fmt.Errorf("updating glossary term: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "glossary_update", duration, true)
		return ErrNotFound
	}

	if owners != nil {
		if err := r.setOwners(ctx, tx, term.ID, owners); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_update", duration, false)
			return fmt.Errorf("setting owners: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_update", duration, false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_update", duration, true)
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	start := time.Now()

	countQuery := `SELECT COUNT(*) FROM glossary_terms WHERE deleted_at IS NULL`

	var total int
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_list_count", time.Since(start), false)
		return nil, fmt.Errorf("counting glossary terms: %w", err)
	}

	query := `
		SELECT id, name, definition, description, parent_term_id,
			   metadata, created_at, updated_at, deleted_at
		FROM glossary_terms
		WHERE deleted_at IS NULL
		ORDER BY name ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), false)
		return nil, fmt.Errorf("listing glossary terms: %w", err)
	}
	defer rows.Close()

	var terms []*GlossaryTerm
	for rows.Next() {
		var term GlossaryTerm
		var metadataJSON []byte

		if err := rows.Scan(
			&term.ID, &term.Name, &term.Definition,
			&term.Description, &term.ParentTermID,
			&metadataJSON, &term.CreatedAt, &term.UpdatedAt, &term.DeletedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), false)
			return nil, fmt.Errorf("scanning glossary term: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &term.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		term.Owners, err = r.loadOwners(ctx, term.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for term %s: %w", term.ID, err)
		}

		terms = append(terms, &term)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), false)
		return nil, fmt.Errorf("iterating glossary terms: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_list", time.Since(start), true)
	return &ListResult{Terms: terms, Total: total}, nil
}

func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	start := time.Now()

	args := []interface{}{}
	argCount := 1

	baseWhere := "WHERE deleted_at IS NULL"
	conditions := []string{}

	if filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf("search_text @@ plainto_tsquery('english', $%d)", argCount))
		args = append(args, filter.Query)
		argCount++
	}

	if filter.ParentTermID != nil {
		if *filter.ParentTermID == "" {
			conditions = append(conditions, "parent_term_id IS NULL")
		} else {
			conditions = append(conditions, fmt.Sprintf("parent_term_id = $%d", argCount))
			args = append(args, *filter.ParentTermID)
			argCount++
		}
	}

	if len(filter.OwnerIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("owner_id = ANY($%d)", argCount))
		args = append(args, filter.OwnerIDs)
		argCount++
	}

	where := baseWhere
	if len(conditions) > 0 {
		where = fmt.Sprintf("%s AND %s", baseWhere, joinConditions(conditions, " AND "))
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM glossary_terms %s", where)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_search_count", time.Since(start), false)
		return nil, fmt.Errorf("counting search results: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, name, definition, description, parent_term_id,
			   metadata, created_at, updated_at, deleted_at
		FROM glossary_terms
		%s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d`, where, argCount, argCount+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), false)
		return nil, fmt.Errorf("searching glossary terms: %w", err)
	}
	defer rows.Close()

	var terms []*GlossaryTerm
	for rows.Next() {
		var term GlossaryTerm
		var metadataJSON []byte

		if err := rows.Scan(
			&term.ID, &term.Name, &term.Definition,
			&term.Description, &term.ParentTermID,
			&metadataJSON, &term.CreatedAt, &term.UpdatedAt, &term.DeletedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), false)
			return nil, fmt.Errorf("scanning search result: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &term.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		// Load owners from join table
		term.Owners, err = r.loadOwners(ctx, term.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for term %s: %w", term.ID, err)
		}

		terms = append(terms, &term)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), false)
		return nil, fmt.Errorf("iterating search results: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_search", time.Since(start), true)
	return &ListResult{Terms: terms, Total: total}, nil
}

func (r *PostgresRepository) GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error) {
	start := time.Now()

	query := `
		SELECT id, name, definition, description, parent_term_id,
			   metadata, created_at, updated_at, deleted_at
		FROM glossary_terms
		WHERE parent_term_id = $1 AND deleted_at IS NULL
		ORDER BY name ASC`

	rows, err := r.db.Query(ctx, query, parentID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), false)
		return nil, fmt.Errorf("getting children: %w", err)
	}
	defer rows.Close()

	var terms []*GlossaryTerm
	for rows.Next() {
		var term GlossaryTerm
		var metadataJSON []byte

		if err := rows.Scan(
			&term.ID, &term.Name, &term.Definition,
			&term.Description, &term.ParentTermID,
			&metadataJSON, &term.CreatedAt, &term.UpdatedAt, &term.DeletedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), false)
			return nil, fmt.Errorf("scanning child term: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &term.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		// Load owners from join table
		term.Owners, err = r.loadOwners(ctx, term.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for term %s: %w", term.ID, err)
		}

		terms = append(terms, &term)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), false)
		return nil, fmt.Errorf("iterating child terms: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_get_children", time.Since(start), true)
	return terms, nil
}

func joinConditions(conditions []string, separator string) string {
	result := ""
	for i, cond := range conditions {
		if i > 0 {
			result += separator
		}
		result += cond
	}
	return result
}
