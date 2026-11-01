package assetsignal

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// staleFactor is how many schedule intervals may pass after an asset's last refresh before it counts as stale, so one late or failed run is not enough.
const staleFactor = 2

// StaleAsset is an asset whose pipeline has not refreshed it for longer than twice the pipeline's schedule interval.
type StaleAsset struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	MRN             string    `json:"mrn"`
	Type            string    `json:"type"`
	Providers       []string  `json:"providers"`
	LastRefreshedAt time.Time `json:"last_refreshed_at"`
	PipelineName    string    `json:"pipeline_name"`
} // @name StaleAsset

// StaleList is a page of stale assets.
type StaleList struct {
	Assets []StaleAsset `json:"assets"`
	Total  int          `json:"total"`
} // @name StaleAssetList

// cutoffs holds, per enabled pipeline, the time before which a refresh by it is stale.
type cutoffs struct {
	ids   []string
	times []time.Time
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduleInterval is the longest gap between the next few runs of a cron expression, so schedules that skip weekends or nights are judged by their longest pause rather than their shortest.
func ScheduleInterval(expr string, from time.Time) (time.Duration, error) {
	sched, err := cronParser.Parse(expr)
	if err != nil {
		return 0, err
	}
	var longest time.Duration
	prev := sched.Next(from)
	if prev.IsZero() {
		return 0, fmt.Errorf("cron expression %q never fires", expr)
	}
	for i := 0; i < 8; i++ {
		next := sched.Next(prev)
		if next.IsZero() {
			break
		}
		if gap := next.Sub(prev); gap > longest {
			longest = gap
		}
		prev = next
	}
	if longest <= 0 {
		return 0, fmt.Errorf("cron expression %q has no interval", expr)
	}
	return longest, nil
}

// staleCutoffs reads the enabled pipelines and works out each one's staleness cutoff. Disabled pipelines are left out, since pausing one is a deliberate choice rather than a failure to refresh.
func (s *Service) staleCutoffs(ctx context.Context) (cutoffs, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text, cron_expression FROM ingestion_schedules WHERE enabled = true`)
	if err != nil {
		return cutoffs{}, fmt.Errorf("listing pipelines: %w", err)
	}
	defer rows.Close()
	now := s.now()
	var c cutoffs
	for rows.Next() {
		var id, expr string
		if err := rows.Scan(&id, &expr); err != nil {
			return cutoffs{}, fmt.Errorf("scanning pipeline: %w", err)
		}
		interval, err := ScheduleInterval(expr, now)
		if err != nil {
			continue
		}
		c.ids = append(c.ids, id)
		c.times = append(c.times, now.Add(-staleFactor*interval))
	}
	return c, rows.Err()
}

// latestLinkSQL picks, for each asset refreshed by an enabled pipeline, the pipeline that refreshed it most recently, along with that pipeline's cutoff.
const latestLinkSQL = `
	WITH cutoffs AS (
		SELECT * FROM unnest($1::uuid[], $2::timestamptz[]) AS c(schedule_id, cutoff)
	),
	latest AS (
		SELECT DISTINCT ON (l.asset_id) l.asset_id, l.schedule_id, l.updated_at, c.cutoff
		FROM asset_schedules l
		JOIN cutoffs c ON c.schedule_id = l.schedule_id
		%s
		ORDER BY l.asset_id, l.updated_at DESC
	)`

// Stale reports, for each asset in ids, whether its pipeline has stopped refreshing it. Every id is present in the result; assets no pipeline refreshes are never stale.
func (s *Service) Stale(ctx context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	if len(ids) == 0 {
		return out, nil
	}
	c, err := s.staleCutoffs(ctx)
	if err != nil {
		return nil, err
	}
	if len(c.ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(latestLinkSQL, "WHERE l.asset_id = ANY($3)")+`
		SELECT asset_id FROM latest WHERE updated_at < cutoff`,
		c.ids, c.times, ids)
	if err != nil {
		return nil, fmt.Errorf("checking stale assets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning stale asset: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// IsStale reports whether one asset is stale.
func (s *Service) IsStale(ctx context.Context, id string) (bool, error) {
	m, err := s.Stale(ctx, []string{id})
	if err != nil {
		return false, err
	}
	return m[id], nil
}

// ListStale returns a page of stale assets, longest unrefreshed first.
func (s *Service) ListStale(ctx context.Context, limit, offset int) (*StaleList, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	list := &StaleList{Assets: []StaleAsset{}}
	c, err := s.staleCutoffs(ctx)
	if err != nil {
		return nil, err
	}
	if len(c.ids) == 0 {
		return list, nil
	}
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(latestLinkSQL, "")+`
		SELECT a.id, COALESCE(a.name, ''), COALESCE(a.mrn, ''), a.type, COALESCE(a.providers, '{}'),
		       latest.updated_at, sch.name, COUNT(*) OVER ()
		FROM latest
		JOIN assets a ON a.id = latest.asset_id
		JOIN ingestion_schedules sch ON sch.id = latest.schedule_id
		WHERE latest.updated_at < latest.cutoff AND a.is_stub = FALSE
		ORDER BY latest.updated_at ASC, a.id
		LIMIT $3 OFFSET $4`,
		c.ids, c.times, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing stale assets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a StaleAsset
		if err := rows.Scan(&a.ID, &a.Name, &a.MRN, &a.Type, &a.Providers, &a.LastRefreshedAt, &a.PipelineName, &list.Total); err != nil {
			return nil, fmt.Errorf("scanning stale asset: %w", err)
		}
		list.Assets = append(list.Assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A page past the end has no rows to carry the total, so count separately.
	if len(list.Assets) == 0 && offset > 0 {
		err := s.db.QueryRow(ctx,
			fmt.Sprintf(latestLinkSQL, "")+`
			SELECT COUNT(*) FROM latest JOIN assets a ON a.id = latest.asset_id
			WHERE latest.updated_at < latest.cutoff AND a.is_stub = FALSE`,
			c.ids, c.times).Scan(&list.Total)
		if err != nil {
			return nil, fmt.Errorf("counting stale assets: %w", err)
		}
	}
	return list, nil
}
