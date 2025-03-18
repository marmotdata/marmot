package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Setup struct {
	db *pgxpool.Pool
}

func NewSetup(db *pgxpool.Pool) *Setup {
	return &Setup{db: db}
}

func (s *Setup) Initialize(ctx context.Context) error {
	migrations, err := s.loadMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	if err := s.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	applied, err := s.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		if err := s.runMigrationSafely(ctx, m); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.Version, err)
		}
	}

	return nil
}

type migration struct {
	Version string
	UpSQL   string
}

func (s *Setup) loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, err
		}

		version := strings.TrimSuffix(entry.Name(), ".up.sql")
		migrations = append(migrations, migration{
			Version: version,
			UpSQL:   string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func (s *Setup) createMigrationsTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (s *Setup) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func (s *Setup) runMigrationSafely(ctx context.Context, m migration) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	log.Info().Str("version", m.Version).Msg("Running migration")

	if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}

	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1)",
		m.Version); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	log.Info().Str("version", m.Version).Msg("Migration completed")
	return nil
}
