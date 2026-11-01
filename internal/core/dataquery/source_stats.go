package dataquery

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SourceStats counts one source's queries over a window. Refused counts the queries policy denied; Failed counts allowed queries the source then failed.
type SourceStats struct {
	Source  string    `json:"source"`
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	Queries int64     `json:"queries"`
	Allowed int64     `json:"allowed"`
	Refused int64     `json:"refused"`
	Failed  int64     `json:"failed"`
} // @name QuerySourceStats

// sourceStatsRepository is the optional repository capability behind SourceStats, kept out of Repository so stores that do not keep rollups still satisfy it.
type sourceStatsRepository interface {
	SourceStats(ctx context.Context, source string, filter StatsFilter) (*SourceStats, error)
}

// DefaultSourceStatsWindow is the window a pipeline page summarises a source's queries over.
const DefaultSourceStatsWindow = 7 * 24 * time.Hour

// SourceStats summarises a source's queries over the filter's window, the last seven days by default, from the hourly rollups. A filter scoped to a principal counts only that caller's queries.
func (s *service) SourceStats(ctx context.Context, source string, filter StatsFilter) (*SourceStats, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("%w: source is required", ErrInvalidInput)
	}
	now := time.Now().UTC()
	if filter.To.IsZero() || filter.To.After(now.Add(time.Hour)) {
		filter.To = now.Truncate(time.Hour).Add(time.Hour)
	}
	if filter.From.IsZero() {
		filter.From = filter.To.Add(-DefaultSourceStatsWindow)
	}
	if !filter.From.Before(filter.To) {
		return nil, fmt.Errorf("%w: from must be before to", ErrInvalidInput)
	}
	if filter.To.Sub(filter.From) > maxStatsSpan {
		return nil, fmt.Errorf("%w: the window can span at most 90 days", ErrInvalidInput)
	}

	repo, ok := s.repo.(sourceStatsRepository)
	if !ok {
		return &SourceStats{Source: source, From: filter.From, To: filter.To}, nil
	}
	return repo.SourceStats(ctx, source, filter)
}

func (r *PostgresRepository) SourceStats(ctx context.Context, source string, f StatsFilter) (*SourceStats, error) {
	stats := &SourceStats{Source: source, From: f.From, To: f.To}
	args := []any{source}
	where := f.scope(&args, "")
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(sum(queries), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'allowed'), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'denied'), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'allowed' AND status = 'failed'), 0)
		FROM data_query_stats_hourly WHERE source_name = $1 AND `+where, args...,
	).Scan(&stats.Queries, &stats.Allowed, &stats.Refused, &stats.Failed); err != nil {
		return nil, fmt.Errorf("loading source stats: %w", err)
	}
	return stats, nil
}
