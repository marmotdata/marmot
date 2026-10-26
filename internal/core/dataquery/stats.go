package dataquery

import (
	"context"
	"fmt"
	"time"
)

// Stats intervals.
const (
	IntervalHour = "hour"
	IntervalDay  = "day"
)

// StatsFilter selects the window and, for callers who may only see their own
// activity, the principal the stats are scoped to.
type StatsFilter struct {
	From          time.Time
	To            time.Time
	Interval      string
	PrincipalType string
	PrincipalID   string
	Limit         int
}

// StatsTotals counts queries in the window. Allowed includes queries that
// passed policy but then failed at the source; Failed counts those alone.
type StatsTotals struct {
	Queries int64 `json:"queries"`
	Allowed int64 `json:"allowed"`
	Denied  int64 `json:"denied"`
	Failed  int64 `json:"failed"`
	Callers int64 `json:"callers"`
	Tables  int64 `json:"tables"`
} // @name QueryStatsTotals

// StatsPoint is one interval of the series. Succeeded, Failed and Denied add
// up to every query in the interval.
type StatsPoint struct {
	Bucket    time.Time `json:"bucket"`
	Succeeded int64     `json:"succeeded"`
	Failed    int64     `json:"failed"`
	Denied    int64     `json:"denied"`
} // @name QueryStatsPoint

// StatsGroup is one entry in a top list: a caller, a source or a table.
type StatsGroup struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	PrincipalType string   `json:"principal_type,omitempty"`
	PluginID      string   `json:"plugin_id,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Allowed       int64    `json:"allowed"`
	Denied        int64    `json:"denied"`
	Total         int64    `json:"total"`
} // @name QueryStatsGroup

// Stats summarises data query activity over a window, computed from the hourly
// rollups rather than the raw query log.
type Stats struct {
	From       time.Time    `json:"from"`
	To         time.Time    `json:"to"`
	Interval   string       `json:"interval"`
	Totals     StatsTotals  `json:"totals"`
	Series     []StatsPoint `json:"series"`
	TopCallers []StatsGroup `json:"top_callers"`
	TopSources []StatsGroup `json:"top_sources"`
	TopTables  []StatsGroup `json:"top_tables"`
} // @name QueryStats

// scope builds the shared WHERE clause for both rollup tables; alias
// qualifies the columns when the rollup is joined to another table.
func (f StatsFilter) scope(args *[]any, alias string) string {
	add := func(v any) string {
		*args = append(*args, v)
		return fmt.Sprintf("$%d", len(*args))
	}
	col := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	where := col("bucket") + " >= " + add(f.From) + " AND " + col("bucket") + " < " + add(f.To)
	if f.PrincipalID != "" {
		where += " AND " + col("principal_type") + " = " + add(f.PrincipalType) +
			" AND " + col("principal_id") + " = " + add(f.PrincipalID)
	}
	return where
}

func (r *PostgresRepository) Stats(ctx context.Context, f StatsFilter) (*Stats, error) {
	stats := &Stats{From: f.From, To: f.To, Interval: f.Interval}

	var args []any
	where := f.scope(&args, "")
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(sum(queries), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'allowed'), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'denied'), 0),
		       COALESCE(sum(queries) FILTER (WHERE status = 'failed'), 0),
		       count(DISTINCT (principal_type, principal_id))
		FROM data_query_stats_hourly WHERE `+where, args...,
	).Scan(&stats.Totals.Queries, &stats.Totals.Allowed, &stats.Totals.Denied, &stats.Totals.Failed, &stats.Totals.Callers); err != nil {
		return nil, fmt.Errorf("loading totals: %w", err)
	}

	args = nil
	where = f.scope(&args, "")
	if err := r.db.QueryRow(ctx, `SELECT count(DISTINCT mrn) FROM data_query_table_stats_hourly WHERE `+where, args...).
		Scan(&stats.Totals.Tables); err != nil {
		return nil, fmt.Errorf("counting tables: %w", err)
	}

	series, err := r.series(ctx, f)
	if err != nil {
		return nil, err
	}
	stats.Series = series

	if stats.TopCallers, err = r.topGroups(ctx, f, "", `
		SELECT principal_type || ':' || principal_id, max(audit_subject), principal_type, '', NULL::text[],
		       sum(queries) FILTER (WHERE decision = 'allowed'), sum(queries) FILTER (WHERE decision = 'denied'), sum(queries)
		FROM data_query_stats_hourly WHERE %s
		GROUP BY principal_type, principal_id`); err != nil {
		return nil, fmt.Errorf("loading top callers: %w", err)
	}
	if stats.TopSources, err = r.topGroups(ctx, f, "st", `
		SELECT st.source_name, st.source_name, '', COALESCE(max(sc.plugin_id), ''), NULL::text[],
		       sum(st.queries) FILTER (WHERE st.decision = 'allowed'), sum(st.queries) FILTER (WHERE st.decision = 'denied'), sum(st.queries)
		FROM data_query_stats_hourly st
		LEFT JOIN ingestion_schedules sc ON sc.name = st.source_name
		WHERE %s
		GROUP BY st.source_name`); err != nil {
		return nil, fmt.Errorf("loading top sources: %w", err)
	}
	if stats.TopTables, err = r.topGroups(ctx, f, "t", `
		SELECT t.mrn, COALESCE(max(a.name), ''), '', '', max(a.tags),
		       sum(t.queries) FILTER (WHERE t.decision = 'allowed'), sum(t.queries) FILTER (WHERE t.decision = 'denied'), sum(t.queries)
		FROM data_query_table_stats_hourly t
		LEFT JOIN assets a ON a.mrn = t.mrn
		WHERE %s
		GROUP BY t.mrn`); err != nil {
		return nil, fmt.Errorf("loading top tables: %w", err)
	}
	return stats, nil
}

// topGroups runs a grouped query returning key, label, principal type,
// plugin, tags, allowed, denied and total, and keeps the busiest groups.
func (r *PostgresRepository) topGroups(ctx context.Context, f StatsFilter, alias, query string) ([]StatsGroup, error) {
	var args []any
	where := f.scope(&args, alias)
	args = append(args, f.Limit)
	rows, err := r.db.Query(ctx, fmt.Sprintf(query, where)+fmt.Sprintf(" ORDER BY 8 DESC, 1 LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []StatsGroup{}
	for rows.Next() {
		var g StatsGroup
		var allowed, denied *int64
		if err := rows.Scan(&g.Key, &g.Label, &g.PrincipalType, &g.PluginID, &g.Tags, &allowed, &denied, &g.Total); err != nil {
			return nil, err
		}
		if allowed != nil {
			g.Allowed = *allowed
		}
		if denied != nil {
			g.Denied = *denied
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// series returns one point per interval across the whole window, with empty
// intervals filled in so charts do not have to.
func (r *PostgresRepository) series(ctx context.Context, f StatsFilter) ([]StatsPoint, error) {
	var args []any
	where := f.scope(&args, "")
	args = append(args, f.Interval)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT date_trunc($%d, bucket, 'UTC') AS b,
		       COALESCE(sum(queries) FILTER (WHERE decision = 'allowed' AND status <> 'failed'), 0),
		       COALESCE(sum(queries) FILTER (WHERE status = 'failed'), 0),
		       COALESCE(sum(queries) FILTER (WHERE decision = 'denied'), 0)
		FROM data_query_stats_hourly WHERE %s
		GROUP BY b ORDER BY b`, len(args), where), args...)
	if err != nil {
		return nil, fmt.Errorf("loading series: %w", err)
	}
	defer rows.Close()

	found := map[int64]StatsPoint{}
	for rows.Next() {
		var p StatsPoint
		if err := rows.Scan(&p.Bucket, &p.Succeeded, &p.Failed, &p.Denied); err != nil {
			return nil, err
		}
		found[p.Bucket.Unix()] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fillSeries(found, f.From, f.To, f.Interval), nil
}

// fillSeries lays the found points onto every interval in [from, to).
func fillSeries(found map[int64]StatsPoint, from, to time.Time, interval string) []StatsPoint {
	step := time.Hour
	start := from.UTC().Truncate(time.Hour)
	if interval == IntervalDay {
		step = 24 * time.Hour
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	}
	points := []StatsPoint{}
	for t := start; t.Before(to); t = t.Add(step) {
		p, ok := found[t.Unix()]
		if !ok {
			p = StatsPoint{Bucket: t}
		}
		points = append(points, p)
	}
	return points
}
