package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetSigningKey(ctx context.Context, key string) (string, error)
	StoreSigningKey(ctx context.Context, key, value string) error
}

type repository struct {
	db *pgxpool.Pool
}

var (
	ErrNotFound = errors.New("key not found")
	ErrDB       = errors.New("database error")
)

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) GetSigningKey(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRow(ctx,
		"SELECT value FROM system_secrets WHERE key = $1",
		key).Scan(&value)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", ErrDB
	}

	return value, nil
}

func (r *repository) StoreSigningKey(ctx context.Context, key, value string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`INSERT INTO system_secrets (key, value, created_at, updated_at) 
		 VALUES ($1, $2, $3, $3)
		 ON CONFLICT (key) DO UPDATE SET
			 value = EXCLUDED.value,
			 updated_at = EXCLUDED.updated_at`,
		key, value, now)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return ErrDB
		}
		return ErrDB
	}

	return nil
}
