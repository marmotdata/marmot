// Package dgumigrations applies the fork's own schema track.
//
// It keeps a separate version table so fork migrations never take a number
// in the upstream sequence: a shared number would make a deployed database
// skip the upstream migration that later claims it.
package dgumigrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// VersionTable records the fork track's applied version.
const VersionTable = "public.dgu_schema_version"

// Migrate applies every pending fork migration. It must run after the core
// migrations, since fork tables reference core tables.
func Migrate(ctx context.Context, conn *pgx.Conn) error {
	migrator, err := migrate.NewMigrator(ctx, conn, VersionTable)
	if err != nil {
		return fmt.Errorf("creating fork migrator: %w", err)
	}
	migrator.OnStart = func(sequence int32, name, direction, _ string) {
		log.Info().Int32("sequence", sequence).Str("name", name).Str("direction", direction).Msg("Running fork migration")
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("opening fork migrations: %w", err)
	}
	if err := migrator.LoadMigrations(sub); err != nil {
		return fmt.Errorf("loading fork migrations: %w", err)
	}
	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("running fork migrations: %w", err)
	}

	version, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting fork schema version: %w", err)
	}
	log.Info().Int32("version", version).Msg("Fork schema is up to date")
	return nil
}
