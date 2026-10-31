package dataquery

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultAssetStatsWindow is the window an asset's Queries tab summarises.
const DefaultAssetStatsWindow = 14 * 24 * time.Hour

// AssetStats summarises the queries through Marmot that read one asset. People counts distinct users and Agents every other kind of caller, such as service accounts. MedianDurationMs is over queries that ran to an end, and is nil when none did.
type AssetStats struct {
	MRN              string    `json:"mrn"`
	From             time.Time `json:"from"`
	To               time.Time `json:"to"`
	Queries          int64     `json:"queries"`
	People           int64     `json:"people"`
	Agents           int64     `json:"agents"`
	Denied           int64     `json:"denied"`
	MedianDurationMs *int64    `json:"median_duration_ms,omitempty"`
	// PerDay counts queries per UTC day across the window, oldest first, with empty days as zero.
	PerDay []int64 `json:"per_day"`
} // @name QueryAssetStats

// assetStatsRepository is the optional repository capability behind AssetStats, kept out of Repository so stores that do not keep rollups still satisfy it.
type assetStatsRepository interface {
	AssetStats(ctx context.Context, mrn string, filter StatsFilter) (*AssetStats, error)
}

// AssetStats summarises the queries that read an asset over the filter's window, the last 14 days by default. A filter scoped to a principal counts only that caller's queries.
func (s *service) AssetStats(ctx context.Context, mrn string, filter StatsFilter) (*AssetStats, error) {
	mrn = strings.TrimSpace(mrn)
	if mrn == "" {
		return nil, fmt.Errorf("%w: mrn is required", ErrInvalidInput)
	}
	now := time.Now().UTC()
	if filter.To.IsZero() || filter.To.After(now.Add(time.Hour)) {
		filter.To = now.Truncate(time.Hour).Add(time.Hour)
	}
	if filter.From.IsZero() {
		filter.From = filter.To.Add(-DefaultAssetStatsWindow)
	}
	if !filter.From.Before(filter.To) {
		return nil, fmt.Errorf("%w: from must be before to", ErrInvalidInput)
	}
	if filter.To.Sub(filter.From) > maxStatsSpan {
		return nil, fmt.Errorf("%w: the window can span at most 90 days", ErrInvalidInput)
	}

	repo, ok := s.repo.(assetStatsRepository)
	if !ok {
		return &AssetStats{MRN: mrn, From: filter.From, To: filter.To, PerDay: fillDays(nil, filter.From, filter.To)}, nil
	}
	return repo.AssetStats(ctx, mrn, filter)
}

func (r *PostgresRepository) AssetStats(ctx context.Context, mrn string, f StatsFilter) (*AssetStats, error) {
	stats := &AssetStats{MRN: mrn, From: f.From, To: f.To}

	args := []any{mrn}
	where := f.scope(&args, "")
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(sum(queries), 0),
		       count(DISTINCT principal_id) FILTER (WHERE principal_type = 'user'),
		       count(DISTINCT (principal_type, principal_id)) FILTER (WHERE principal_type <> 'user'),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'denied'), 0)
		FROM data_query_table_stats_hourly WHERE mrn = $1 AND `+where, args...,
	).Scan(&stats.Queries, &stats.People, &stats.Agents, &stats.Denied); err != nil {
		return nil, fmt.Errorf("loading asset query totals: %w", err)
	}

	args = []any{mrn}
	where = f.scope(&args, "")
	rows, err := r.db.Query(ctx, `
		SELECT date_trunc('day', bucket, 'UTC'), sum(queries)
		FROM data_query_table_stats_hourly WHERE mrn = $1 AND `+where+`
		GROUP BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("loading asset query series: %w", err)
	}
	found := map[int64]int64{}
	for rows.Next() {
		var day time.Time
		var n int64
		if err := rows.Scan(&day, &n); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning asset query series: %w", err)
		}
		found[day.Unix()] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating asset query series: %w", err)
	}
	stats.PerDay = fillDays(found, f.From, f.To)

	// The rollups carry no timings, so the median reads the log itself; the referenced_mrns GIN index keeps it to this asset's queries.
	args = []any{mrn, f.From, f.To}
	scope := ""
	if f.PrincipalID != "" {
		args = append(args, f.PrincipalType, f.PrincipalID)
		scope = " AND principal_type = $4 AND principal_id = $5"
	}
	var median *float64
	if err := r.db.QueryRow(ctx, `
		SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM completed_at - started_at) * 1000)
		FROM data_queries
		WHERE referenced_mrns @> ARRAY[$1]::text[] AND started_at >= $2 AND started_at < $3
		  AND decision = 'allowed' AND completed_at IS NOT NULL`+scope, args...,
	).Scan(&median); err != nil {
		return nil, fmt.Errorf("loading asset query median: %w", err)
	}
	if median != nil {
		ms := int64(*median + 0.5)
		stats.MedianDurationMs = &ms
	}
	return stats, nil
}

// fillDays lays per-day counts onto every UTC day the window touches, oldest first.
func fillDays(found map[int64]int64, from, to time.Time) []int64 {
	start := from.UTC()
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	days := []int64{}
	for d := start; d.Before(to); d = d.AddDate(0, 0, 1) {
		days = append(days, found[d.Unix()])
	}
	return days
}
