package updates

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// notifyChannel is the Postgres channel a replica notifies when it changes a plugin's version, so the others switch without waiting for their next poll.
const notifyChannel = "core_plugin_versions"

// row is a plugin moved off the version the release pins.
type row struct {
	PluginID        string
	Version         string
	Digest          string
	Source          string
	Pinned          bool
	PreviousVersion string
	PreviousDigest  string
	ReleaseVersion  string
	UpdatedBy       string
	UpdatedAt       time.Time
}

// indexRow is the stored plugin index and how the last check of it went.
type indexRow struct {
	Version   int64
	Body      []byte
	Signature []byte
	ExpiresAt *time.Time
	FetchedAt *time.Time
	CheckedAt *time.Time
	LastError string
}

// errStaleIndex means a fetched index is older than one already stored, which is what replaying a captured index to hide newer advisories looks like.
var errStaleIndex = errors.New("plugin index is older than the one already verified")

type store struct {
	db *pgxpool.Pool
}

const rowColumns = `plugin_id, version, digest, source, pinned, COALESCE(previous_version, ''), COALESCE(previous_digest, ''), COALESCE(release_version, ''), COALESCE(updated_by, ''), updated_at`

func scanRow(s interface{ Scan(...any) error }) (row, error) {
	var r row
	err := s.Scan(&r.PluginID, &r.Version, &r.Digest, &r.Source, &r.Pinned, &r.PreviousVersion, &r.PreviousDigest, &r.ReleaseVersion, &r.UpdatedBy, &r.UpdatedAt)
	return r, err
}

func (s *store) list(ctx context.Context) (map[string]row, error) {
	rows, err := s.db.Query(ctx, `SELECT `+rowColumns+` FROM core_plugin_versions`)
	if err != nil {
		return nil, fmt.Errorf("listing plugin versions: %w", err)
	}
	defer rows.Close()

	out := map[string]row{}
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning plugin version: %w", err)
		}
		out[r.PluginID] = r
	}
	return out, rows.Err()
}

func (s *store) get(ctx context.Context, id string) (row, bool, error) {
	r, err := scanRow(s.db.QueryRow(ctx, `SELECT `+rowColumns+` FROM core_plugin_versions WHERE plugin_id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return row{}, false, nil
	}
	if err != nil {
		return row{}, false, fmt.Errorf("getting plugin version: %w", err)
	}
	return r, true, nil
}

// put writes a plugin's version and tells the other replicas, in one transaction so they never hear of a change they cannot yet read.
func (s *store) put(ctx context.Context, r row) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO core_plugin_versions (plugin_id, version, digest, source, pinned, previous_version, previous_digest, release_version, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NOW())
			ON CONFLICT (plugin_id) DO UPDATE SET
				version = EXCLUDED.version,
				digest = EXCLUDED.digest,
				source = EXCLUDED.source,
				pinned = EXCLUDED.pinned,
				previous_version = EXCLUDED.previous_version,
				previous_digest = EXCLUDED.previous_digest,
				release_version = EXCLUDED.release_version,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()`,
			r.PluginID, r.Version, r.Digest, r.Source, r.Pinned, r.PreviousVersion, r.PreviousDigest, r.ReleaseVersion, r.UpdatedBy)
		if err != nil {
			return fmt.Errorf("saving plugin version: %w", err)
		}
		_, err = tx.Exec(ctx, `SELECT pg_notify($1, $2)`, notifyChannel, r.PluginID)
		return err
	})
}

func (s *store) delete(ctx context.Context, id string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM core_plugin_versions WHERE plugin_id = $1`, id); err != nil {
			return fmt.Errorf("resetting plugin version: %w", err)
		}
		_, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, notifyChannel, id)
		return err
	})
}

// index reads the stored index, leaving out the body and signature unless withBody is set, since most reads only want to know whether it changed.
func (s *store) index(ctx context.Context, withBody bool) (indexRow, error) {
	var r indexRow
	var version *int64
	var lastError *string
	err := s.db.QueryRow(ctx, `SELECT version, CASE WHEN $1 THEN body END, CASE WHEN $1 THEN signature END, expires_at, fetched_at, checked_at, last_error FROM core_plugin_index`, withBody).
		Scan(&version, &r.Body, &r.Signature, &r.ExpiresAt, &r.FetchedAt, &r.CheckedAt, &lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, nil
	}
	if err != nil {
		return r, fmt.Errorf("reading plugin index: %w", err)
	}
	if version != nil {
		r.Version = *version
	}
	if lastError != nil {
		r.LastError = *lastError
	}
	return r, nil
}

func (s *store) indexVersion(ctx context.Context) (int64, error) {
	var version *int64
	err := s.db.QueryRow(ctx, `SELECT version FROM core_plugin_index`).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading plugin index version: %w", err)
	}
	if version == nil {
		return 0, nil
	}
	return *version, nil
}

// saveIndex stores a verified index unless an equal or newer one is already stored. The comparison happens in the statement, so two replicas checking at once cannot let the older one win.
func (s *store) saveIndex(ctx context.Context, version int64, body, signature []byte, expires time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO core_plugin_index (id, version, body, signature, expires_at, fetched_at, checked_at, last_error)
		VALUES (TRUE, $1, $2, $3, $4, NOW(), NOW(), NULL)
		ON CONFLICT (id) DO UPDATE SET
			version = EXCLUDED.version,
			body = EXCLUDED.body,
			signature = EXCLUDED.signature,
			expires_at = EXCLUDED.expires_at,
			fetched_at = NOW(),
			checked_at = NOW(),
			last_error = NULL
		WHERE core_plugin_index.version IS NULL OR EXCLUDED.version > core_plugin_index.version`,
		version, body, signature, expires)
	if err != nil {
		return false, fmt.Errorf("saving plugin index: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// recordCheck notes when the index was last checked and why it failed, if it did.
func (s *store) recordCheck(ctx context.Context, checkErr error) error {
	var msg *string
	if checkErr != nil {
		m := checkErr.Error()
		msg = &m
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO core_plugin_index (id, checked_at, last_error) VALUES (TRUE, NOW(), $1)
		ON CONFLICT (id) DO UPDATE SET checked_at = NOW(), last_error = EXCLUDED.last_error`, msg)
	return err
}
