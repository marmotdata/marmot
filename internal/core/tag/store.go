package tag

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/metrics"
)

type PostgresRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

func NewPostgresRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresRepository {
	return &PostgresRepository{db: db, recorder: recorder}
}

func (r *PostgresRepository) GetTag(ctx context.Context, id string) (*Tag, error) {
	start := time.Now()
	var tag Tag
	err := r.db.QueryRow(ctx,
		"SELECT id, name, description, created_at, updated_at FROM tags WHERE id = $1",
		id,
	).Scan(&tag.ID, &tag.Name, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "tag_get", time.Since(start), false)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.recorder.RecordDBQuery(ctx, "tag_get", time.Since(start), true)
	return &tag, nil
}

func (r *PostgresRepository) ListTags(ctx context.Context) ([]Tag, error) {
	start := time.Now()
	rows, err := r.db.Query(ctx,
		"SELECT id, name, description, created_at, updated_at FROM tags ORDER BY name ASC",
	)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "tag_list", time.Since(start), false)
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			r.recorder.RecordDBQuery(ctx, "tag_list", time.Since(start), false)
			return nil, err
		}
		tags = append(tags, tag)
	}
	r.recorder.RecordDBQuery(ctx, "tag_list", time.Since(start), true)
	return tags, rows.Err()
}

func (r *PostgresRepository) CreateTag(ctx context.Context, input CreateTagInput) (*Tag, error) {
	start := time.Now()
	var tag Tag
	err := r.db.QueryRow(ctx,
		`INSERT INTO tags (name, description)
		 VALUES ($1, $2)
		 RETURNING id, name, description, created_at, updated_at`,
		input.Name, input.Description,
	).Scan(&tag.ID, &tag.Name, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "tag_create", time.Since(start), false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, err
	}
	r.recorder.RecordDBQuery(ctx, "tag_create", time.Since(start), true)
	return &tag, nil
}

func (r *PostgresRepository) UpdateTag(ctx context.Context, id string, input UpdateTagInput) (*Tag, error) {
	start := time.Now()
	query := "UPDATE tags SET updated_at = NOW()"
	args := []any{}
	paramCount := 1

	if input.Name != "" {
		query += fmt.Sprintf(", name = $%d", paramCount)
		args = append(args, input.Name)
		paramCount++
	}
	if input.Description != nil {
		query += fmt.Sprintf(", description = $%d", paramCount)
		args = append(args, input.Description)
		paramCount++
	}

	query += fmt.Sprintf(" WHERE id = $%d RETURNING id, name, description, created_at, updated_at", paramCount)
	args = append(args, id)

	var tag Tag
	err := r.db.QueryRow(ctx, query, args...).Scan(&tag.ID, &tag.Name, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "tag_update", time.Since(start), false)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, err
	}
	r.recorder.RecordDBQuery(ctx, "tag_update", time.Since(start), true)
	return &tag, nil
}

func (r *PostgresRepository) ResolveNames(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return []string{}, nil
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO tags (name)
		SELECT unnest($1::text[])
		ON CONFLICT (name) DO NOTHING`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("auto-creating tags: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, name FROM tags WHERE name = ANY($1)`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()

	nameToID := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		nameToID[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}

	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = nameToID[name]
	}
	return ids, nil
}

func (r *PostgresRepository) DeleteTag(ctx context.Context, id string) error {
	start := time.Now()
	res, err := r.db.Exec(ctx, "DELETE FROM tags WHERE id = $1", id)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "tag_delete", time.Since(start), false)
		return err
	}
	if res.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "tag_delete", time.Since(start), false)
		return ErrNotFound
	}
	r.recorder.RecordDBQuery(ctx, "tag_delete", time.Since(start), true)
	return nil
}
