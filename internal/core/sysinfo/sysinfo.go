// Package sysinfo reads facts about the running instance for administrators: what it is built from, the database it runs on and the size of its search index.
package sysinfo

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Build is the product version and the commit the binary was built from. Version comes from the release build flags; the commit comes from the VCS stamp Go embeds, so a source build reports one too.
type Build struct {
	Version   string     `json:"version"`
	Commit    string     `json:"commit,omitempty"`
	BuiltAt   *time.Time `json:"built_at,omitempty"`
	Modified  bool       `json:"modified,omitempty"`
	GoVersion string     `json:"go_version"`
}

// ReadBuild returns the build of the running binary.
func ReadBuild(version string) Build {
	b := Build{Version: version}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	b.GoVersion = info.GoVersion
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Commit = s.Value
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				b.BuiltAt = &t
			}
		case "vcs.modified":
			b.Modified = s.Value == "true"
		}
	}
	return b
}

// Database is the Postgres server the instance runs on.
type Database struct {
	Version string `json:"version"`
	// SizeBytes is the size of the whole Marmot database.
	SizeBytes int64 `json:"size_bytes"`
}

// SearchIndex is the search_index table every search reads, whether or not Elasticsearch mirrors it.
type SearchIndex struct {
	Documents int64 `json:"documents"`
	SizeBytes int64 `json:"size_bytes"`
}

// ReadDatabase returns the Postgres version and the database size.
func ReadDatabase(ctx context.Context, db *pgxpool.Pool) (Database, error) {
	var d Database
	err := db.QueryRow(ctx, `SELECT current_setting('server_version'), pg_database_size(current_database())`).Scan(&d.Version, &d.SizeBytes)
	if err != nil {
		return d, fmt.Errorf("reading database version: %w", err)
	}
	// Packagers append their own build, as in "18.3 (Debian 18.3-1.pgdg13+1)", which says nothing about the features available.
	if f := strings.Fields(d.Version); len(f) > 0 {
		d.Version = f[0]
	}
	return d, nil
}

// ReadSearchIndex counts the search documents and measures the table holding them, indexes included.
func ReadSearchIndex(ctx context.Context, db *pgxpool.Pool) (SearchIndex, error) {
	var s SearchIndex
	err := db.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM search_index), pg_total_relation_size('search_index')`).Scan(&s.Documents, &s.SizeBytes)
	if err != nil {
		return s, fmt.Errorf("reading search index size: %w", err)
	}
	return s, nil
}
