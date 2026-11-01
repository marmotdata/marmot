package assetdetail

import (
	"context"
	"fmt"
	"time"
)

// QueryWindow is how far back the Queries tab counts, matching the window of its activity tiles.
const QueryWindow = 14 * 24 * time.Hour

// TabCounts are the numbers shown beside the asset page's tabs. A nil count is one the caller may not see or that does not apply.
type TabCounts struct {
	// Lineage counts the distinct assets one edge away, upstream or downstream.
	Lineage int `json:"lineage"`
	// Runs counts the distinct pipeline runs recorded against the asset.
	Runs int `json:"runs"`
	// AgentRuns counts the runs of an agent asset.
	AgentRuns int `json:"agent_runs"`
	// Queries counts queries through Marmot that read the asset in the last 14 days.
	Queries *int64 `json:"queries,omitempty"`
} // @name AssetTabCounts

// QueryScope narrows the query count to one principal, for callers who may only see their own queries. A zero scope counts everyone's.
type QueryScope struct {
	PrincipalType string
	PrincipalID   string
}

// TabCounts counts an asset's lineage neighbours, runs and, when countQueries is set, recent queries in one round trip.
func (s *Store) TabCounts(ctx context.Context, assetID, mrn string, countQueries bool, scope QueryScope) (*TabCounts, error) {
	var c TabCounts
	var queries int64
	since := time.Now().Add(-QueryWindow).UTC().Truncate(time.Hour)
	if err := s.db.QueryRow(ctx, `
		SELECT
			(SELECT count(DISTINCT CASE WHEN source_mrn = $2 THEN target_mrn ELSE source_mrn END)
			   FROM lineage_edges WHERE (source_mrn = $2 OR target_mrn = $2) AND source_mrn <> target_mrn),
			(SELECT count(DISTINCT run_id) FROM run_history WHERE asset_id = $1),
			(SELECT count(*) FROM agent_runs WHERE agent_id = $1),
			CASE WHEN $3 THEN (
				SELECT COALESCE(sum(queries), 0)::bigint FROM data_query_table_stats_hourly
				WHERE bucket >= $4 AND mrn = $2
				  AND ($5 = '' OR (principal_type = $5 AND principal_id = $6))
			) ELSE 0 END`,
		assetID, mrn, countQueries, since, scope.PrincipalType, scope.PrincipalID,
	).Scan(&c.Lineage, &c.Runs, &c.AgentRuns, &queries); err != nil {
		return nil, fmt.Errorf("counting asset tabs: %w", err)
	}
	if countQueries {
		c.Queries = &queries
	}
	return &c, nil
}
