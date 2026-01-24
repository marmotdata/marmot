package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const versionTable = "public.schema_version"

type Setup struct {
	db *pgxpool.Pool
}

func NewSetup(db *pgxpool.Pool) *Setup {
	return &Setup{db: db}
}

func (s *Setup) Initialize(ctx context.Context) error {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), versionTable)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	migrator.OnStart = func(sequence int32, name, direction, sql string) {
		log.Info().
			Int32("sequence", sequence).
			Str("name", name).
			Str("direction", direction).
			Msg("Running migration")
	}

	migrationsSubFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migrations sub filesystem: %w", err)
	}

	if err := migrator.LoadMigrations(migrationsSubFS); err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	if err := s.seedVersionFromLegacy(ctx, conn.Conn(), migrator); err != nil {
		return fmt.Errorf("seeding version from legacy table: %w", err)
	}

	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	currentVersion, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current version: %w", err)
	}
	log.Info().Int32("version", currentVersion).Msg("Database schema is up to date")

	return nil
}

// seedVersionFromLegacy checks for the old schema_migrations table, parses the highest
// applied version number, and seeds tern's schema_version table so already-applied
// migrations are not re-run.
func (s *Setup) seedVersionFromLegacy(ctx context.Context, conn *pgx.Conn, migrator *migrate.Migrator) error {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking for legacy table: %w", err)
	}
	if !exists {
		return nil
	}

	currentVersion, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current tern version: %w", err)
	}
	if currentVersion > 0 {
		return nil
	}

	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("querying legacy migrations: %w", err)
	}
	defer rows.Close()

	var maxVersion int32
	for rows.Next() {
		var versionStr string
		if err := rows.Scan(&versionStr); err != nil {
			return fmt.Errorf("scanning version: %w", err)
		}

		numStr := versionStr
		if idx := strings.Index(versionStr, "_"); idx > 0 {
			numStr = versionStr[:idx]
		}
		numStr = strings.TrimLeft(numStr, "0")
		if numStr == "" {
			numStr = "0"
		}

		num, err := strconv.ParseInt(numStr, 10, 32)
		if err != nil {
			return fmt.Errorf("parsing version number from %q: %w", versionStr, err)
		}
		if int32(num) > maxVersion {
			maxVersion = int32(num)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating legacy migrations: %w", err)
	}

	if maxVersion == 0 {
		return nil
	}

	log.Info().
		Int32("version", maxVersion).
		Msg("Seeding tern schema_version from legacy schema_migrations table")

	_, err = conn.Exec(ctx, fmt.Sprintf("UPDATE %s SET version=$1", versionTable), maxVersion)
	if err != nil {
		return fmt.Errorf("updating tern version: %w", err)
	}

	_, err = conn.Exec(ctx, "ALTER TABLE schema_migrations RENAME TO schema_migrations_legacy")
	if err != nil {
		return fmt.Errorf("renaming legacy table: %w", err)
	}

	log.Info().Msg("Legacy schema_migrations table renamed to schema_migrations_legacy")
	return nil
}
