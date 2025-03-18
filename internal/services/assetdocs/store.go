package assetdocs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("asset not found")
	ErrConflict = errors.New("asset already exists")
)

type Repository interface {
	GetDocumentation(ctx context.Context, mrn string) ([]Documentation, error)
	CreateDocumentation(ctx context.Context, doc Documentation) error
	CreateGlobalDocumentation(ctx context.Context, doc GlobalDocumentation) error
	GetGlobalDocumentation(ctx context.Context, source string) (string, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetDocumentation(ctx context.Context, mrn string) ([]Documentation, error) {
	query := `
        SELECT id, mrn, content, source, global_docs, created_at, updated_at
        FROM documentation
        WHERE mrn = $1
        ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, mrn)
	if err != nil {
		return nil, fmt.Errorf("querying documentation: %w", err)
	}
	defer rows.Close()

	var docs []Documentation
	for rows.Next() {
		var doc Documentation
		err := rows.Scan(
			&doc.ID,
			&doc.MRN,
			&doc.Content,
			&doc.Source,
			&doc.GlobalDocs,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning documentation: %w", err)
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating documentation rows: %w", err)
	}

	return docs, nil
}

func (r *PostgresRepository) CreateDocumentation(ctx context.Context, doc Documentation) error {
	query := `
        INSERT INTO documentation (mrn, content, source, global_docs, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $5)
        ON CONFLICT (mrn, source) 
        DO UPDATE SET 
            content = EXCLUDED.content,
            global_docs = EXCLUDED.global_docs,
            updated_at = EXCLUDED.updated_at`

	_, err := r.db.Exec(ctx, query,
		doc.MRN,
		doc.Content,
		doc.Source,
		doc.GlobalDocs,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("creating documentation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateGlobalDocumentation(ctx context.Context, doc GlobalDocumentation) error {
	query := `
        INSERT INTO global_documentation (content, source, created_at, updated_at)
        VALUES ($1, $2, $3, $3)
        ON CONFLICT (source) 
        DO UPDATE SET 
            content = EXCLUDED.content,
            updated_at = EXCLUDED.updated_at`

	_, err := r.db.Exec(ctx, query,
		doc.Content,
		doc.Source,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("creating global documentation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetGlobalDocumentation(ctx context.Context, source string) (string, error) {
	var content string
	err := r.db.QueryRow(ctx,
		`SELECT content FROM global_documentation WHERE source = $1`,
		source,
	).Scan(&content)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("getting global documentation: %w", err)
	}

	return content, nil
}
