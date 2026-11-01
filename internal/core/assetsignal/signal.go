// Package assetsignal derives the at-a-glance signals shown beside an asset: how popular it is, whether its pipeline has stopped refreshing it and how big it is. Search, the asset page and other areas share one definition of each so the same asset never reads differently in two places.
package assetsignal

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PopularityWindow is how far back views count towards popularity.
const PopularityWindow = 30 * 24 * time.Hour

// MaxPopularity is the highest popularity level, matching the bars mm-popularity draws.
const MaxPopularity = 4

// popularityFloor keeps a quiet catalog from showing full bars for a handful of views, since levels are scaled against the busiest asset or this many views, whichever is higher.
const popularityFloor = 50

// maxViewsTTL is how long the busiest asset's view count is reused, as it moves slowly and costs a pass over every view in the window.
const maxViewsTTL = 5 * time.Minute

// Service reads asset signals from Postgres.
type Service struct {
	db  *pgxpool.Pool
	now func() time.Time

	mu         sync.Mutex
	maxViews   int64
	maxViewsAt time.Time
}

// New returns a Service on the given pool. It is cheap, so areas that need a signal can build their own.
func New(db *pgxpool.Pool) *Service {
	return &Service{db: db, now: time.Now}
}

// ViewsSQL is a query over metrics_timeseries giving each asset's views in the popularity window as (asset_id, views), for callers that sort or join on popularity in SQL.
const ViewsSQL = `SELECT labels->>'asset_id' AS asset_id, SUM(total_sum)::bigint AS views
	FROM metrics_timeseries
	WHERE metric_name = 'asset_views_total'
	  AND day >= (CURRENT_DATE - 30)
	  AND labels->>'asset_id' IS NOT NULL
	GROUP BY labels->>'asset_id'`

// ViewCounts returns views over the popularity window for each asset in ids; assets with no views are left out.
func (s *Service) ViewCounts(ctx context.Context, ids []string) (map[string]int64, error) {
	out := make(map[string]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT labels->>'asset_id', SUM(total_sum)::bigint
		FROM metrics_timeseries
		WHERE metric_name = 'asset_views_total'
		  AND day >= (CURRENT_DATE - 30)
		  AND labels->>'asset_id' = ANY($1)
		GROUP BY labels->>'asset_id'`, ids)
	if err != nil {
		return nil, fmt.Errorf("counting asset views: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scanning asset views: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// busiest returns the most views any single asset had in the window, cached briefly.
func (s *Service) busiest(ctx context.Context) (int64, error) {
	s.mu.Lock()
	if !s.maxViewsAt.IsZero() && s.now().Sub(s.maxViewsAt) < maxViewsTTL {
		n := s.maxViews
		s.mu.Unlock()
		return n, nil
	}
	s.mu.Unlock()

	var n int64
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(MAX(views), 0) FROM (`+ViewsSQL+`) v`).Scan(&n); err != nil {
		return 0, fmt.Errorf("finding the busiest asset: %w", err)
	}
	s.mu.Lock()
	s.maxViews, s.maxViewsAt = n, s.now()
	s.mu.Unlock()
	return n, nil
}

// Popularity returns a level from 0 to MaxPopularity for each asset in ids, from its views over the last 30 days relative to the busiest asset. Every id is present in the result.
func (s *Service) Popularity(ctx context.Context, ids []string) (map[string]int, error) {
	out := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	views, err := s.ViewCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	busiest := int64(0)
	if len(views) > 0 {
		if busiest, err = s.busiest(ctx); err != nil {
			return nil, err
		}
	}
	for _, id := range ids {
		out[id] = PopularityLevel(views[id], busiest)
	}
	return out, nil
}

// PopularityLevel maps a view count to a level from 0 to MaxPopularity on a log scale against the busiest asset's views, so a long tail of rarely viewed assets still separates from never viewed ones.
func PopularityLevel(views, busiest int64) int {
	if views <= 0 {
		return 0
	}
	ref := busiest
	if ref < popularityFloor {
		ref = popularityFloor
	}
	if views > ref {
		views = ref
	}
	level := int(math.Ceil(MaxPopularity * math.Log1p(float64(views)) / math.Log1p(float64(ref))))
	if level < 1 {
		level = 1
	}
	if level > MaxPopularity {
		level = MaxPopularity
	}
	return level
}
