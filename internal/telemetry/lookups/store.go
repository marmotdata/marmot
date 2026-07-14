package lookups

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists lookup counters so we don't lose them across restarts and
// so the telemetry sender knows how much has already been reported.
type Store interface {
	// AddDeltas upserts the given per-source-per-category deltas into the
	// cumulative count column. Never touches reported_count.
	AddDeltas(ctx context.Context, installID string, snap Snapshot) error

	// UnreportedDeltas returns the current (count - reported_count) per
	// source/category. Zero rows are omitted.
	UnreportedDeltas(ctx context.Context, installID string) (Snapshot, error)

	// MarkReported advances reported_count by the given deltas. Called after
	// a successful telemetry POST so subsequent sends are truly incremental.
	MarkReported(ctx context.Context, installID string, snap Snapshot) error
}

type pgStore struct {
	db *pgxpool.Pool
}

// NewPostgresStore returns a Store backed by the given pool.
func NewPostgresStore(db *pgxpool.Pool) Store {
	return &pgStore{db: db}
}

func (s *pgStore) AddDeltas(ctx context.Context, installID string, snap Snapshot) error {
	if len(snap) == 0 {
		return nil
	}
	const q = `
		INSERT INTO lookup_counters (install_id, source, category, count, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (install_id, source, category)
		DO UPDATE SET count = lookup_counters.count + EXCLUDED.count,
		              updated_at = NOW()
	`
	for source, cats := range snap {
		for cat, n := range cats {
			if n == 0 {
				continue
			}
			if _, err := s.db.Exec(ctx, q, installID, source, cat, n); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *pgStore) UnreportedDeltas(ctx context.Context, installID string) (Snapshot, error) {
	const q = `
		SELECT source, category, count - reported_count
		FROM lookup_counters
		WHERE install_id = $1 AND count > reported_count
	`
	rows, err := s.db.Query(ctx, q, installID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := Snapshot{}
	for rows.Next() {
		var source, category string
		var delta int64
		if err := rows.Scan(&source, &category, &delta); err != nil {
			return nil, err
		}
		if _, ok := out[source]; !ok {
			out[source] = map[string]int64{}
		}
		out[source][category] = delta
	}
	return out, rows.Err()
}

func (s *pgStore) MarkReported(ctx context.Context, installID string, snap Snapshot) error {
	if len(snap) == 0 {
		return nil
	}
	const q = `
		UPDATE lookup_counters
		SET reported_count = reported_count + $4,
		    updated_at = NOW()
		WHERE install_id = $1 AND source = $2 AND category = $3
	`
	for source, cats := range snap {
		for cat, n := range cats {
			if n == 0 {
				continue
			}
			if _, err := s.db.Exec(ctx, q, installID, source, cat, n); err != nil {
				return err
			}
		}
	}
	return nil
}
