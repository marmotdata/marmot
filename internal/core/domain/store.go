package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Domain, error)
	Get(ctx context.Context, id string) (*Domain, error)
	Children(ctx context.Context, parentID *string) ([]*Domain, error)
	Subtree(ctx context.Context, id string) ([]*Domain, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Domain, error)
	Delete(ctx context.Context, id string) error
	Move(ctx context.Context, id string, parentID *string) (*Domain, error)
	Assign(ctx context.Context, kind Kind, entityIDs []string, domainID string) error
	DomainOf(ctx context.Context, kind Kind, entityID string) (string, error)
	PipelineDomain(ctx context.Context, pipelineName string) (string, bool, error)
}

type membership struct {
	table, column, columnType, entityTable string
}

var memberships = map[Kind]membership{
	KindAsset:             {"asset_domains", "asset_id", "varchar", "assets"},
	KindDataProduct:       {"data_product_domains", "data_product_id", "uuid", "data_products"},
	KindGlossaryTerm:      {"glossary_term_domains", "glossary_term_id", "uuid", "glossary_terms"},
	KindIngestionSchedule: {"ingestion_schedule_domains", "schedule_id", "uuid", "ingestion_schedules"},
}

// treeLock serializes structural changes. A move rewrites the paths of a
// subtree in one statement; a concurrent insert under that subtree would keep
// the old prefix, and two crossing moves could form a cycle.
const treeLock = 7_361_024_190_551

const columns = `id, parent_id, path, depth, name, description, metadata, tags, restricted, created_by, created_at, updated_at`

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func scanDomain(row pgx.Row) (*Domain, error) {
	var d Domain
	err := row.Scan(&d.ID, &d.ParentID, &d.Path, &d.Depth, &d.Name, &d.Description,
		&d.Metadata, &d.Tags, &d.Restricted, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func scanDomains(rows pgx.Rows) ([]*Domain, error) {
	defer rows.Close()
	out := []*Domain{}
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// notFound maps a missing row, and an id that is not a valid UUID, to target.
func notFound(err error, target error) error {
	if errors.Is(err, pgx.ErrNoRows) || pgCode(err) == "22P02" {
		return target
	}
	return err
}

func lockTree(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", treeLock)
	return err
}

func nodePosition(ctx context.Context, tx pgx.Tx, id string) (path string, depth int, err error) {
	err = tx.QueryRow(ctx, "SELECT path, depth FROM domains WHERE id = $1 FOR UPDATE", id).Scan(&path, &depth)
	return path, depth, notFound(err, ErrNotFound)
}

func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return nil, err
	}

	parentPath, parentDepth := "/", 0
	if in.ParentID != nil {
		if parentPath, parentDepth, err = nodePosition(ctx, tx, *in.ParentID); err != nil {
			return nil, err
		}
	}
	if parentDepth+1 > MaxDepth {
		return nil, ErrTooDeep
	}
	metadata, tags := in.Metadata, in.Tags
	if metadata == nil {
		metadata = map[string]any{}
	}
	if tags == nil {
		tags = []string{}
	}

	d, err := scanDomain(tx.QueryRow(ctx, `
		WITH g AS (SELECT gen_random_uuid() AS id)
		INSERT INTO domains (id, parent_id, path, depth, name, description, metadata, tags, created_by)
		SELECT g.id, $1::uuid, $2::text || g.id::text || '/', $3::int, $4::text, $5::text, $6::jsonb, $7::text[], NULLIF($8::text, '') FROM g
		RETURNING `+columns,
		in.ParentID, parentPath, parentDepth+1, in.Name, in.Description, metadata, tags, in.CreatedBy))
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("inserting domain: %w", err)
	}
	return d, tx.Commit(ctx)
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Domain, error) {
	d, err := scanDomain(r.db.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1", id))
	return d, notFound(err, ErrNotFound)
}

func (r *PostgresRepository) Children(ctx context.Context, parentID *string) ([]*Domain, error) {
	var rows pgx.Rows
	var err error
	if parentID == nil {
		rows, err = r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE parent_id IS NULL ORDER BY lower(name)")
	} else {
		rows, err = r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE parent_id = $1 ORDER BY lower(name)", *parentID)
	}
	if err != nil {
		return nil, notFound(err, ErrNotFound)
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) Subtree(ctx context.Context, id string) ([]*Domain, error) {
	root, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE path LIKE $1::text || '%' ORDER BY depth, lower(name)", root.Path)
	if err != nil {
		return nil, err
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) Update(ctx context.Context, id string, in UpdateInput) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanDomain(tx.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1 FOR UPDATE", id))
	if err != nil {
		return nil, notFound(err, ErrNotFound)
	}
	if in.Name != nil {
		current.Name = *in.Name
	}
	if in.Description != nil {
		current.Description = in.Description
	}
	if in.Metadata != nil {
		current.Metadata = in.Metadata
	}
	if in.Tags != nil {
		current.Tags = in.Tags
	}

	d, err := scanDomain(tx.QueryRow(ctx, `
		UPDATE domains SET name = $2, description = $3, metadata = $4, tags = $5, updated_at = now()
		WHERE id = $1 RETURNING `+columns,
		id, current.Name, current.Description, current.Metadata, current.Tags))
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("updating domain: %w", err)
	}
	return d, tx.Commit(ctx)
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return err
	}
	if _, _, err := nodePosition(ctx, tx, id); err != nil {
		return err
	}

	var hasChildren bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM domains WHERE parent_id = $1)", id).Scan(&hasChildren); err != nil {
		return err
	}
	if hasChildren {
		return ErrHasChildren
	}
	for _, m := range memberships {
		var hasMembers bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+m.table+" WHERE domain_id = $1)", id).Scan(&hasMembers); err != nil {
			return err
		}
		if hasMembers {
			return ErrNotEmpty
		}
	}

	if _, err := tx.Exec(ctx, "DELETE FROM domains WHERE id = $1", id); err != nil {
		if pgCode(err) == "23503" {
			return ErrNotEmpty
		}
		return fmt.Errorf("deleting domain: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Move(ctx context.Context, id string, parentID *string) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return nil, err
	}

	oldPath, oldDepth, err := nodePosition(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	parentPath, parentDepth := "/", 0
	if parentID != nil {
		if parentPath, parentDepth, err = nodePosition(ctx, tx, *parentID); err != nil {
			return nil, err
		}
		if len(parentPath) >= len(oldPath) && parentPath[:len(oldPath)] == oldPath {
			return nil, ErrCycle
		}
	}

	var deepest int
	if err := tx.QueryRow(ctx, "SELECT max(depth) FROM domains WHERE path LIKE $1::text || '%'", oldPath).Scan(&deepest); err != nil {
		return nil, err
	}
	shift := parentDepth + 1 - oldDepth
	if deepest+shift > MaxDepth {
		return nil, ErrTooDeep
	}

	_, err = tx.Exec(ctx, `
		UPDATE domains
		   SET path = $3::text || substr(path, $4::int),
		       depth = depth + $5::int,
		       parent_id = CASE WHEN id = $1 THEN $2::uuid ELSE parent_id END,
		       updated_at = CASE WHEN id = $1 THEN now() ELSE updated_at END
		 WHERE path LIKE $6::text || '%'`,
		id, parentID, parentPath+id+"/", len(oldPath)+1, shift, oldPath)
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("moving domain: %w", err)
	}

	d, err := scanDomain(tx.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1", id))
	if err != nil {
		return nil, err
	}
	return d, tx.Commit(ctx)
}

// Assign sets the owning domain of every listed entity, all or none.
// Assigning Unassigned removes membership rows, since a missing row already
// means Unassigned.
func (r *PostgresRepository) Assign(ctx context.Context, kind Kind, entityIDs []string, domainID string) error {
	m, ok := memberships[kind]
	if !ok {
		return ErrInvalidInput
	}
	ids := unique(entityIDs)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Comparing as text keeps a malformed UUID a plain miss, not a cast error.
	var found int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+m.entityTable+" WHERE id::text = ANY($1::text[])", ids).Scan(&found); err != nil {
		return err
	}
	if found != len(ids) {
		return ErrEntityNotFound
	}

	if domainID == UnassignedID {
		_, err = tx.Exec(ctx, "DELETE FROM "+m.table+" WHERE "+m.column+"::text = ANY($1::text[])", ids)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO `+m.table+` (`+m.column+`, domain_id)
			SELECT unnest($1::text[])::`+m.columnType+`, $2::uuid
			ON CONFLICT (`+m.column+`) DO UPDATE SET domain_id = EXCLUDED.domain_id, assigned_at = now()`,
			ids, domainID)
	}
	switch pgCode(err) {
	case "23503", "22P02":
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("assigning %s to domain: %w", kind, err)
	}
	return tx.Commit(ctx)
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func (r *PostgresRepository) DomainOf(ctx context.Context, kind Kind, entityID string) (string, error) {
	m, ok := memberships[kind]
	if !ok {
		return "", ErrInvalidInput
	}
	var id string
	err := r.db.QueryRow(ctx, "SELECT domain_id FROM "+m.table+" WHERE "+m.column+" = $1", entityID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) || pgCode(err) == "22P02" {
		return UnassignedID, nil
	}
	return id, err
}

// PipelineDomain returns the domain assigned to the ingestion schedule named
// pipelineName. Runs started by the scheduler use the schedule name as their
// pipeline name; other pipelines have no schedule and report found=false.
func (r *PostgresRepository) PipelineDomain(ctx context.Context, pipelineName string) (string, bool, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT sd.domain_id
		  FROM ingestion_schedule_domains sd
		  JOIN ingestion_schedules s ON s.id = sd.schedule_id
		 WHERE s.name = $1`, pipelineName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
