package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

type AggregatedMetric struct {
	Name            string            `json:"name"`
	AggregationType string            `json:"aggregation_type"`
	Value           float64           `json:"value"`
	Labels          map[string]string `json:"labels"`
	BucketStart     time.Time         `json:"bucket_start"`
	BucketEnd       time.Time         `json:"bucket_end"`
	BucketSize      time.Duration     `json:"bucket_size" swaggertype:"integer"`
}

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type QueryOptions struct {
	TimeRange       TimeRange         `json:"time_range"`
	MetricNames     []string          `json:"metric_names"`
	Labels          map[string]string `json:"labels"`
	AggregationType string            `json:"aggregation_type"`
	BucketSize      time.Duration     `json:"bucket_size" swaggertype:"integer"`
}

type QueryCount struct {
	Query     string `json:"query"`
	QueryType string `json:"query_type"`
	Count     int64  `json:"count"`
}

type AssetCount struct {
	AssetID       string `json:"asset_id"`
	AssetType     string `json:"asset_type"`
	AssetName     string `json:"asset_name"`
	AssetProvider string `json:"asset_provider"`
	Count         int64  `json:"count"`
}

type AssetBreakdown struct {
	Type      string `json:"type"`
	Provider  string `json:"provider"`
	HasSchema bool   `json:"has_schema"`
	Owner     string `json:"owner"`
	Count     int64  `json:"count"`
}

type Store interface {
	RecordMetric(ctx context.Context, metric Metric) error
	RecordMetrics(ctx context.Context, metrics []Metric) error

	GetMetrics(ctx context.Context, opts QueryOptions) ([]Metric, error)
	GetAggregatedMetrics(ctx context.Context, opts QueryOptions) ([]AggregatedMetric, error)

	GetTopQueries(ctx context.Context, timeRange TimeRange, limit int) ([]QueryCount, error)
	GetTopAssets(ctx context.Context, timeRange TimeRange, limit int) ([]AssetCount, error)

	// Asset statistics (from pre-computed table)
	GetTotalAssets(ctx context.Context) (int64, error)
	GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error)
	GetAssetsByType(ctx context.Context) (map[string]int64, error)
	GetAssetsByProvider(ctx context.Context) (map[string]int64, error)
	GetAssetsWithSchemas(ctx context.Context) (int64, error)
	GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error)
	GetAssetBreakdown(ctx context.Context) ([]AssetBreakdown, error)

	// Maintenance
	RefreshAssetStatistics(ctx context.Context, ownerFields []string) error
	RefreshMetadataValueCounts(ctx context.Context) error
	CreatePartition(ctx context.Context, date time.Time) error
	DeleteOldMetrics(ctx context.Context, olderThan time.Time) error
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) Store {
	return &PostgresStore{db: db}
}

// =============================================================================
// WRITE METHODS (Array-based)
// =============================================================================

func (s *PostgresStore) RecordMetric(ctx context.Context, metric Metric) error {
	return s.RecordMetrics(ctx, []Metric{metric})
}

// RecordMetrics appends metrics to the array-based timeseries table.
// Groups metrics by (name, labels, hour) and performs batch upserts.
func (s *PostgresStore) RecordMetrics(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Group metrics by (name, labels, hour) for efficient batch upsert
	type bucketKey struct {
		name   string
		labels string
		hour   time.Time
	}

	buckets := make(map[bucketKey][]Metric)
	for _, m := range metrics {
		labelsJSON, _ := json.Marshal(m.Labels)
		key := bucketKey{
			name:   m.Name,
			labels: string(labelsJSON),
			hour:   m.Timestamp.Truncate(time.Hour),
		}
		buckets[key] = append(buckets[key], m)
	}

	// Upsert each bucket
	query := `
		INSERT INTO metrics_timeseries (
			metric_name, metric_type, labels, hour, day,
			timestamps, values, point_count, total_sum, min_value, max_value,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			NOW(), NOW()
		)
		ON CONFLICT (metric_name, labels, hour, day) DO UPDATE SET
			timestamps = metrics_timeseries.timestamps || EXCLUDED.timestamps,
			values = metrics_timeseries.values || EXCLUDED.values,
			point_count = metrics_timeseries.point_count + EXCLUDED.point_count,
			total_sum = metrics_timeseries.total_sum + EXCLUDED.total_sum,
			min_value = LEAST(metrics_timeseries.min_value, EXCLUDED.min_value),
			max_value = GREATEST(metrics_timeseries.max_value, EXCLUDED.max_value),
			updated_at = NOW()`

	batch := &pgx.Batch{}
	for key, bucketMetrics := range buckets {
		timestamps := make([]time.Time, len(bucketMetrics))
		values := make([]float32, len(bucketMetrics))
		var sum float64
		var minVal, maxVal float32

		for i, m := range bucketMetrics {
			timestamps[i] = m.Timestamp
			values[i] = float32(m.Value)
			sum += m.Value
			if i == 0 || float32(m.Value) < minVal {
				minVal = float32(m.Value)
			}
			if i == 0 || float32(m.Value) > maxVal {
				maxVal = float32(m.Value)
			}
		}

		batch.Queue(query,
			key.name,
			string(bucketMetrics[0].Type),
			[]byte(key.labels),
			key.hour,
			key.hour.Truncate(24*time.Hour),
			timestamps,
			values,
			len(bucketMetrics),
			sum,
			minVal,
			maxVal,
		)
	}

	results := s.db.SendBatch(ctx, batch)
	defer results.Close()

	for range buckets {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("executing batch upsert: %w", err)
		}
	}

	return nil
}

// =============================================================================
// READ METHODS (From pre-computed aggregates)
// =============================================================================

// GetMetrics returns raw metric data points by unnesting arrays.
func (s *PostgresStore) GetMetrics(ctx context.Context, opts QueryOptions) ([]Metric, error) {
	query := `
		SELECT metric_name, metric_type, labels, t, v
		FROM metrics_timeseries,
			LATERAL unnest(timestamps, values) AS u(t, v)
		WHERE day >= $1::date AND day <= $2::date
		  AND hour >= date_trunc('hour', $1::timestamptz)
		  AND hour <= date_trunc('hour', $2::timestamptz)
		  AND t >= $1 AND t <= $2`

	args := []interface{}{opts.TimeRange.Start, opts.TimeRange.End}
	argNum := 3

	if len(opts.MetricNames) > 0 {
		query += fmt.Sprintf(" AND metric_name = ANY($%d)", argNum)
		args = append(args, opts.MetricNames)
		argNum++
	}

	if len(opts.Labels) > 0 {
		for key, value := range opts.Labels {
			query += fmt.Sprintf(" AND labels->>'%s' = $%d", key, argNum)
			args = append(args, value)
			argNum++
		}
	}

	query += " ORDER BY t"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying metrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var metric Metric
		var labelsJSON []byte
		var metricType string

		if err := rows.Scan(&metric.Name, &metricType, &labelsJSON, &metric.Timestamp, &metric.Value); err != nil {
			return nil, fmt.Errorf("scanning metric: %w", err)
		}

		metric.Type = MetricType(metricType)
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &metric.Labels); err != nil {
				return nil, fmt.Errorf("unmarshaling labels: %w", err)
			}
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

// GetAggregatedMetrics returns aggregated metrics using pre-computed values.
func (s *PostgresStore) GetAggregatedMetrics(ctx context.Context, opts QueryOptions) ([]AggregatedMetric, error) {
	// Determine bucket size for grouping
	bucketSize := opts.BucketSize
	if bucketSize == 0 {
		duration := opts.TimeRange.End.Sub(opts.TimeRange.Start)
		switch {
		case duration > 30*24*time.Hour:
			bucketSize = 24 * time.Hour
		case duration > 7*24*time.Hour:
			bucketSize = time.Hour
		default:
			bucketSize = 5 * time.Minute
		}
	}

	bucketExpr := getBucketExpression(bucketSize)

	query := fmt.Sprintf(`
		SELECT
			metric_name,
			'sum' as aggregation_type,
			SUM(total_sum) as value,
			labels,
			%s as bucket_start,
			%s + $3::interval as bucket_end
		FROM metrics_timeseries
		WHERE day >= $1::date AND day <= $2::date
		  AND hour >= date_trunc('hour', $1::timestamptz)
		  AND hour <= $2::timestamptz`,
		bucketExpr, bucketExpr)

	args := []interface{}{opts.TimeRange.Start, opts.TimeRange.End, bucketSize}
	argNum := 4

	if len(opts.MetricNames) > 0 {
		query += fmt.Sprintf(" AND metric_name = ANY($%d)", argNum)
		args = append(args, opts.MetricNames)
		argNum++
	}

	if len(opts.Labels) > 0 {
		for key, value := range opts.Labels {
			query += fmt.Sprintf(" AND labels->>'%s' = $%d", key, argNum)
			args = append(args, value)
			argNum++
		}
	}

	query += fmt.Sprintf(" GROUP BY metric_name, labels, %s ORDER BY bucket_start", bucketExpr)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying aggregated metrics: %w", err)
	}
	defer rows.Close()

	var metrics []AggregatedMetric
	for rows.Next() {
		var metric AggregatedMetric
		var labelsJSON []byte

		if err := rows.Scan(&metric.Name, &metric.AggregationType, &metric.Value,
			&labelsJSON, &metric.BucketStart, &metric.BucketEnd); err != nil {
			return nil, fmt.Errorf("scanning aggregated metric: %w", err)
		}

		metric.BucketSize = bucketSize
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &metric.Labels); err != nil {
				return nil, fmt.Errorf("unmarshaling labels: %w", err)
			}
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

func getBucketExpression(bucketSize time.Duration) string {
	switch bucketSize {
	case 5 * time.Minute:
		return "date_trunc('hour', hour) + INTERVAL '5 min' * (EXTRACT(minute FROM hour)::int / 5)"
	case 15 * time.Minute:
		return "date_trunc('hour', hour) + INTERVAL '15 min' * (EXTRACT(minute FROM hour)::int / 15)"
	case time.Hour:
		return "hour"
	case 6 * time.Hour:
		return "date_trunc('day', hour) + INTERVAL '6 hour' * (EXTRACT(hour FROM hour)::int / 6)"
	case 24 * time.Hour:
		return "day::timestamptz"
	default:
		return "hour"
	}
}

// GetTopQueries returns the most frequent search queries.
func (s *PostgresStore) GetTopQueries(ctx context.Context, timeRange TimeRange, limit int) ([]QueryCount, error) {
	query := `
		SELECT
			labels->>'query' as query,
			labels->>'query_type' as query_type,
			SUM(total_sum)::bigint as count
		FROM metrics_timeseries
		WHERE metric_name = 'search_queries_detailed'
		  AND day >= $1::date AND day <= $2::date
		  AND labels->>'query' IS NOT NULL
		  AND labels->>'query' != ''
		GROUP BY labels->>'query', labels->>'query_type'
		ORDER BY count DESC
		LIMIT $3`

	rows, err := s.db.Query(ctx, query, timeRange.Start, timeRange.End, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top queries: %w", err)
	}
	defer rows.Close()

	var results []QueryCount
	for rows.Next() {
		var result QueryCount
		if err := rows.Scan(&result.Query, &result.QueryType, &result.Count); err != nil {
			return nil, fmt.Errorf("scanning query count: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// GetTopAssets returns the most viewed assets.
func (s *PostgresStore) GetTopAssets(ctx context.Context, timeRange TimeRange, limit int) ([]AssetCount, error) {
	query := `
		SELECT
			COALESCE(labels->>'asset_id', '') as asset_id,
			COALESCE(labels->>'asset_type', '') as asset_type,
			COALESCE(labels->>'asset_name', '') as asset_name,
			COALESCE(labels->>'asset_provider', '') as asset_provider,
			SUM(total_sum)::bigint as count
		FROM metrics_timeseries
		WHERE metric_name = 'asset_views_total'
		  AND day >= $1::date AND day <= $2::date
		  AND labels->>'asset_id' IS NOT NULL
		GROUP BY labels->>'asset_id', labels->>'asset_type', labels->>'asset_name', labels->>'asset_provider'
		ORDER BY count DESC
		LIMIT $3`

	rows, err := s.db.Query(ctx, query, timeRange.Start, timeRange.End, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top assets: %w", err)
	}
	defer rows.Close()

	var results []AssetCount
	for rows.Next() {
		var result AssetCount
		if err := rows.Scan(&result.AssetID, &result.AssetType, &result.AssetName, &result.AssetProvider, &result.Count); err != nil {
			return nil, fmt.Errorf("scanning asset count: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// =============================================================================
// ASSET STATISTICS (From pre-computed table)
// =============================================================================

func (s *PostgresStore) GetTotalAssets(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE(total_count, 0) FROM asset_statistics WHERE id = 1`).Scan(&count)
	if err != nil {
		// Fallback to counting assets directly if asset_statistics doesn't exist
		return s.getTotalAssetsFromSource(ctx)
	}
	return count, nil
}

func (s *PostgresStore) getTotalAssetsFromSource(ctx context.Context) (int64, error) {
	var count int64
	// Try with is_stub filter first (most installations have this column)
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM assets WHERE is_stub = FALSE`).Scan(&count)
	if err != nil {
		// Fallback without is_stub filter (column might not exist)
		err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM assets`).Scan(&count)
	}
	return count, err
}

func (s *PostgresStore) GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error) {
	// For filtered counts, we need to query the breakdown and exclude
	query := `
		SELECT COALESCE(SUM((item->>'count')::bigint), 0)
		FROM asset_statistics, jsonb_array_elements(COALESCE(breakdown, '[]'::jsonb)) AS item
		WHERE id = 1
		  AND NOT (item->>'type' = ANY($1))
		  AND NOT (item->>'provider' = ANY($2))`

	var count int64
	err := s.db.QueryRow(ctx, query, excludedTypes, excludedProviders).Scan(&count)
	if err != nil {
		// Fallback to querying assets directly
		return s.getTotalAssetsFilteredFromSource(ctx, excludedTypes, excludedProviders)
	}
	return count, nil
}

func (s *PostgresStore) getTotalAssetsFilteredFromSource(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error) {
	query := `SELECT COUNT(*) FROM assets WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if len(excludedTypes) > 0 {
		query += fmt.Sprintf(" AND type != ALL($%d)", argNum)
		args = append(args, excludedTypes)
		argNum++
	}

	if len(excludedProviders) > 0 {
		query += fmt.Sprintf(" AND NOT (providers && $%d)", argNum)
		args = append(args, excludedProviders)
	}

	var count int64
	err := s.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetAssetsByType(ctx context.Context) (map[string]int64, error) {
	var byTypeJSON []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE(by_type, '{}'::jsonb) FROM asset_statistics WHERE id = 1`).Scan(&byTypeJSON)
	if err != nil {
		// Fallback to querying assets directly
		return s.getAssetsByTypeFromSource(ctx)
	}

	result := make(map[string]int64)
	if len(byTypeJSON) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(byTypeJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling by_type: %w", err)
	}
	return result, nil
}

func (s *PostgresStore) getAssetsByTypeFromSource(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `SELECT type, COUNT(*) FROM assets WHERE is_stub = FALSE GROUP BY type`)
	if err != nil {
		// Fallback without is_stub filter
		rows, err = s.db.Query(ctx, `SELECT type, COUNT(*) FROM assets GROUP BY type`)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var assetType string
		var count int64
		if err := rows.Scan(&assetType, &count); err != nil {
			return nil, err
		}
		result[assetType] = count
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetAssetsByProvider(ctx context.Context) (map[string]int64, error) {
	var byProviderJSON []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE(by_provider, '{}'::jsonb) FROM asset_statistics WHERE id = 1`).Scan(&byProviderJSON)
	if err != nil {
		// Fallback to querying assets directly
		return s.getAssetsByProviderFromSource(ctx)
	}

	result := make(map[string]int64)
	if len(byProviderJSON) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(byProviderJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling by_provider: %w", err)
	}
	return result, nil
}

func (s *PostgresStore) getAssetsByProviderFromSource(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT providers[1] as provider, COUNT(*)
		FROM assets
		WHERE is_stub = FALSE AND array_length(providers, 1) > 0
		GROUP BY providers[1]`)
	if err != nil {
		// Fallback without is_stub filter
		rows, err = s.db.Query(ctx, `
			SELECT providers[1] as provider, COUNT(*)
			FROM assets
			WHERE array_length(providers, 1) > 0
			GROUP BY providers[1]`)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var provider string
		var count int64
		if err := rows.Scan(&provider, &count); err != nil {
			return nil, err
		}
		result[provider] = count
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetAssetsWithSchemas(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE(with_schemas_count, 0) FROM asset_statistics WHERE id = 1`).Scan(&count)
	if err != nil {
		// Fallback to querying assets directly
		err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM assets WHERE schema != '{}' AND schema IS NOT NULL`).Scan(&count)
	}
	return count, err
}

func (s *PostgresStore) GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error) {
	var byOwnerJSON []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE(by_owner, '{}'::jsonb) FROM asset_statistics WHERE id = 1`).Scan(&byOwnerJSON)
	if err != nil {
		// Fallback to querying assets directly
		return s.getAssetsByOwnerFromSource(ctx, ownerFields)
	}

	result := make(map[string]int64)
	if len(byOwnerJSON) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(byOwnerJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling by_owner: %w", err)
	}
	return result, nil
}

func (s *PostgresStore) getAssetsByOwnerFromSource(ctx context.Context, ownerFields []string) (map[string]int64, error) {
	if len(ownerFields) == 0 {
		return make(map[string]int64), nil
	}

	coalesceFields := make([]string, len(ownerFields))
	for i, field := range ownerFields {
		coalesceFields[i] = fmt.Sprintf("metadata->>'%s'", field)
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(%s) as owner, COUNT(*)
		FROM assets
		WHERE COALESCE(%s) IS NOT NULL
		GROUP BY 1`,
		strings.Join(coalesceFields, ", "),
		strings.Join(coalesceFields, ", "))

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var owner string
		var count int64
		if err := rows.Scan(&owner, &count); err != nil {
			return nil, err
		}
		result[owner] = count
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetAssetBreakdown(ctx context.Context) ([]AssetBreakdown, error) {
	var breakdownJSON []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE(breakdown, '[]'::jsonb) FROM asset_statistics WHERE id = 1`).Scan(&breakdownJSON)
	if err != nil {
		// Fallback to querying assets directly
		return s.getAssetBreakdownFromSource(ctx)
	}

	var result []AssetBreakdown
	if len(breakdownJSON) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(breakdownJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling breakdown: %w", err)
	}
	return result, nil
}

func (s *PostgresStore) getAssetBreakdownFromSource(ctx context.Context) ([]AssetBreakdown, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			COALESCE(type, 'unknown') as type,
			COALESCE(providers[1], 'unknown') as provider,
			CASE WHEN schema != '{}' THEN true ELSE false END as has_schema,
			COALESCE(COALESCE(metadata->>'owner', metadata->>'ownedBy'), 'unknown') as owner,
			COUNT(*) as count
		FROM assets
		WHERE is_stub = FALSE
		GROUP BY type, providers[1], (schema != '{}'), COALESCE(metadata->>'owner', metadata->>'ownedBy')
		ORDER BY count DESC`)
	if err != nil {
		// Fallback without is_stub filter
		rows, err = s.db.Query(ctx, `
			SELECT
				COALESCE(type, 'unknown') as type,
				COALESCE(providers[1], 'unknown') as provider,
				CASE WHEN schema != '{}' THEN true ELSE false END as has_schema,
				COALESCE(COALESCE(metadata->>'owner', metadata->>'ownedBy'), 'unknown') as owner,
				COUNT(*) as count
			FROM assets
			GROUP BY type, providers[1], (schema != '{}'), COALESCE(metadata->>'owner', metadata->>'ownedBy')
			ORDER BY count DESC`)
		if err != nil {
			return nil, fmt.Errorf("querying asset breakdown: %w", err)
		}
	}
	defer rows.Close()

	var result []AssetBreakdown
	for rows.Next() {
		var breakdown AssetBreakdown
		if err := rows.Scan(&breakdown.Type, &breakdown.Provider, &breakdown.HasSchema, &breakdown.Owner, &breakdown.Count); err != nil {
			return nil, fmt.Errorf("scanning asset breakdown: %w", err)
		}
		result = append(result, breakdown)
	}
	return result, rows.Err()
}

// =============================================================================
// MAINTENANCE METHODS
// =============================================================================

// RefreshAssetStatistics updates the pre-computed asset statistics table.
func (s *PostgresStore) RefreshAssetStatistics(ctx context.Context, ownerFields []string) error {
	if len(ownerFields) == 0 {
		ownerFields = []string{"owner", "ownedBy"}
	}
	_, err := s.db.Exec(ctx, `SELECT refresh_asset_statistics($1)`, ownerFields)
	return err
}

// RefreshMetadataValueCounts refreshes the materialized view for metadata autocomplete.
func (s *PostgresStore) RefreshMetadataValueCounts(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY metadata_value_counts`)
	return err
}

// CreatePartition creates a partition for the given date.
func (s *PostgresStore) CreatePartition(ctx context.Context, date time.Time) error {
	_, err := s.db.Exec(ctx, `SELECT create_metrics_timeseries_partition($1)`, date)
	return err
}

// DeleteOldMetrics drops partitions older than the cutoff date.
func (s *PostgresStore) DeleteOldMetrics(ctx context.Context, olderThan time.Time) error {
	// Drop partitions for dates older than cutoff
	_, err := s.db.Exec(ctx, `SELECT drop_metrics_timeseries_partition($1::date)`, olderThan)
	return err
}
