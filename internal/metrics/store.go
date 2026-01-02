package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
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

	GetTotalAssets(ctx context.Context) (int64, error)
	GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error)
	GetAssetsByType(ctx context.Context) (map[string]int64, error)
	GetAssetsByProvider(ctx context.Context) (map[string]int64, error)
	GetAssetsWithSchemas(ctx context.Context) (int64, error)
	GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error)
	GetAssetBreakdown(ctx context.Context) ([]AssetBreakdown, error)

	AggregateMetrics(ctx context.Context, timeRange TimeRange, bucketSize time.Duration) error
	DeleteOldMetrics(ctx context.Context, olderThan time.Time) error

	CreatePartition(ctx context.Context, date time.Time) error
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) Store {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) RecordMetric(ctx context.Context, metric Metric) error {
	return s.RecordMetrics(ctx, []Metric{metric})
}

func (s *PostgresStore) GetAssetsWithSchemas(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM assets WHERE schema != '{}' AND schema IS NOT NULL`
	var count int64
	err := s.db.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error) {
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

func (s *PostgresStore) GetTotalAssets(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM assets`
	var count int64
	err := s.db.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetAssetsByType(ctx context.Context) (map[string]int64, error) {
	query := `SELECT type, COUNT(*) FROM assets GROUP BY type ORDER BY COUNT(*) DESC`

	result := make(map[string]int64)
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var assetType string
		var count int64
		err := rows.Scan(&assetType, &count)
		if err != nil {
			return nil, err
		}
		result[assetType] = count
	}
	return result, nil
}

func (s *PostgresStore) GetAssetsByProvider(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT providers[1] as provider, COUNT(*) 
		FROM assets 
		WHERE array_length(providers, 1) > 0
		GROUP BY providers[1] 
		ORDER BY COUNT(*) DESC`

	result := make(map[string]int64)
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var count int64
		err := rows.Scan(&provider, &count)
		if err != nil {
			return nil, err
		}
		result[provider] = count
	}
	return result, nil
}

func (s *PostgresStore) GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error) {
	if len(ownerFields) == 0 {
		return make(map[string]int64), nil
	}

	coalesceFields := make([]string, len(ownerFields))
	for i, field := range ownerFields {
		coalesceFields[i] = fmt.Sprintf("metadata->>'%s'", field)
	}

	query := fmt.Sprintf(`
   	SELECT 
   		COALESCE(%s) as owner,
   		COUNT(*)
   	FROM assets
   	WHERE COALESCE(%s) IS NOT NULL
   	GROUP BY owner ORDER BY COUNT(*) DESC`,
		strings.Join(coalesceFields, ", "),
		strings.Join(coalesceFields, ", "))

	result := make(map[string]int64)
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var owner string
		var count int64
		err := rows.Scan(&owner, &count)
		if err != nil {
			return nil, err
		}
		result[owner] = count
	}
	return result, nil
}

func (s *PostgresStore) GetAssetBreakdown(ctx context.Context) ([]AssetBreakdown, error) {
	query := `
		SELECT 
			COALESCE(type, 'unknown') as type,
			COALESCE(providers[1], 'unknown') as provider,
			CASE WHEN schema != '{}' THEN true ELSE false END as has_schema,
			COALESCE(COALESCE(metadata->>'owner', metadata->>'ownedBy'), 'unknown') as owner,
			COUNT(*) as count
		FROM assets 
		GROUP BY type, providers[1], (schema != '{}'), COALESCE(metadata->>'owner', metadata->>'ownedBy')
		ORDER BY count DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying asset breakdown: %w", err)
	}
	defer rows.Close()

	var result []AssetBreakdown
	for rows.Next() {
		var breakdown AssetBreakdown
		err := rows.Scan(&breakdown.Type, &breakdown.Provider, &breakdown.HasSchema, &breakdown.Owner, &breakdown.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning asset breakdown: %w", err)
		}
		result = append(result, breakdown)
	}

	return result, rows.Err()
}

func (s *PostgresStore) RecordMetrics(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	query := `
		INSERT INTO raw_metrics (metric_name, metric_type, value, labels, timestamp)
		VALUES ($1, $2, $3, $4, $5)`

	batch := &pgx.Batch{}
	for _, metric := range metrics {
		labelsJSON, err := json.Marshal(metric.Labels)
		if err != nil {
			return fmt.Errorf("marshaling labels: %w", err)
		}

		batch.Queue(query, metric.Name, string(metric.Type), metric.Value, labelsJSON, metric.Timestamp)
	}

	results := s.db.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < len(metrics); i++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("executing batch insert: %w", err)
		}
	}

	return nil
}

func (s *PostgresStore) GetMetrics(ctx context.Context, opts QueryOptions) ([]Metric, error) {
	query := `
		SELECT metric_name, metric_type, value, labels, timestamp
		FROM raw_metrics
		WHERE timestamp >= $1 AND timestamp <= $2`

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

	query += " ORDER BY timestamp"

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

		err := rows.Scan(&metric.Name, &metricType, &metric.Value, &labelsJSON, &metric.Timestamp)
		if err != nil {
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

func (s *PostgresStore) GetAggregatedMetrics(ctx context.Context, opts QueryOptions) ([]AggregatedMetric, error) {
	query := `
		SELECT metric_name, aggregation_type, value, labels, bucket_start, bucket_end, bucket_size
		FROM aggregated_metrics
		WHERE bucket_start >= $1 AND bucket_end <= $2`

	args := []interface{}{opts.TimeRange.Start, opts.TimeRange.End}
	argNum := 3

	if len(opts.MetricNames) > 0 {
		query += fmt.Sprintf(" AND metric_name = ANY($%d)", argNum)
		args = append(args, opts.MetricNames)
		argNum++
	}

	if opts.AggregationType != "" {
		query += fmt.Sprintf(" AND aggregation_type = $%d", argNum)
		args = append(args, opts.AggregationType)
		argNum++
	}

	if opts.BucketSize > 0 {
		query += fmt.Sprintf(" AND bucket_size = $%d", argNum)
		args = append(args, opts.BucketSize)
	}

	query += " ORDER BY bucket_start"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying aggregated metrics: %w", err)
	}
	defer rows.Close()

	var metrics []AggregatedMetric
	for rows.Next() {
		var metric AggregatedMetric
		var labelsJSON []byte
		var bucketSize string

		err := rows.Scan(&metric.Name, &metric.AggregationType, &metric.Value,
			&labelsJSON, &metric.BucketStart, &metric.BucketEnd, &bucketSize)
		if err != nil {
			return nil, fmt.Errorf("scanning aggregated metric: %w", err)
		}

		bucketDuration, err := time.ParseDuration(bucketSize)
		if err != nil {
			return nil, fmt.Errorf("parsing bucket size: %w", err)
		}
		metric.BucketSize = bucketDuration

		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &metric.Labels); err != nil {
				return nil, fmt.Errorf("unmarshaling labels: %w", err)
			}
		}

		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

func (s *PostgresStore) AggregateMetrics(ctx context.Context, timeRange TimeRange, bucketSize time.Duration) error {
	lockID := int64(12345)

	locked, err := s.tryAdvisoryLock(ctx, lockID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to acquire advisory lock for metrics aggregation")
		return err
	}
	if !locked {
		log.Debug().Msg("Metrics aggregation already running, skipping")
		return nil
	}
	defer func() {
		if err := s.releaseAdvisoryLock(ctx, lockID); err != nil {
			log.Warn().Err(err).Msg("Failed to release advisory lock")
		}
	}()

	batchDuration := time.Hour
	if bucketSize < time.Hour {
		batchDuration = bucketSize * 12
	}

	currentStart := timeRange.Start
	for currentStart.Before(timeRange.End) {
		batchEnd := currentStart.Add(batchDuration)
		if batchEnd.After(timeRange.End) {
			batchEnd = timeRange.End
		}

		batchRange := TimeRange{Start: currentStart, End: batchEnd}

		if err := s.aggregateBatch(ctx, batchRange, bucketSize); err != nil {
			log.Warn().Err(err).
				Time("batch_start", currentStart).
				Time("batch_end", batchEnd).
				Msg("Batch aggregation failed - continuing with next batch")
		}

		currentStart = batchEnd
	}

	return nil
}

func (s *PostgresStore) aggregateBatch(ctx context.Context, timeRange TimeRange, bucketSize time.Duration) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET statement_timeout = '30s'"); err != nil {
		return fmt.Errorf("setting statement timeout: %w", err)
	}

	if err := s.aggregateCountersBatch(ctx, tx, timeRange, bucketSize); err != nil {
		return fmt.Errorf("aggregating counters: %w", err)
	}

	if err := s.aggregateGaugesBatch(ctx, tx, timeRange, bucketSize); err != nil {
		return fmt.Errorf("aggregating gauges: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) aggregateCountersBatch(ctx context.Context, tx pgx.Tx, timeRange TimeRange, bucketSize time.Duration) error {
	bucketStart := getBucketStart(bucketSize)

	query := fmt.Sprintf(`
		INSERT INTO aggregated_metrics (metric_name, aggregation_type, value, labels, bucket_start, bucket_end, bucket_size)
		SELECT 
			metric_name,
			'sum' as aggregation_type,
			SUM(value) as value,
			labels,
			%s as bucket_start,
			%s + $3::INTERVAL as bucket_end,
			$3::INTERVAL as bucket_size
		FROM raw_metrics 
		WHERE metric_type = 'counter'
		  AND timestamp >= $1 AND timestamp < $2
		GROUP BY metric_name, labels, (%s)
		ON CONFLICT (metric_name, aggregation_type, labels, bucket_start, bucket_end) 
		DO UPDATE SET value = EXCLUDED.value, created_at = NOW()`,
		bucketStart, bucketStart, bucketStart)

	_, err := tx.Exec(ctx, query, timeRange.Start, timeRange.End, bucketSize)
	return err
}

func (s *PostgresStore) aggregateGaugesBatch(ctx context.Context, tx pgx.Tx, timeRange TimeRange, bucketSize time.Duration) error {
	aggregations := []string{"avg", "min", "max"}
	bucketStart := getBucketStart(bucketSize)

	for _, agg := range aggregations {
		query := fmt.Sprintf(`
			INSERT INTO aggregated_metrics (metric_name, aggregation_type, value, labels, bucket_start, bucket_end, bucket_size)
			SELECT 
				metric_name,
				$1 as aggregation_type,
				%s(value) as value,
				labels,
				%s as bucket_start,
				%s + $4::INTERVAL as bucket_end,
				$4::INTERVAL as bucket_size
			FROM raw_metrics 
			WHERE metric_type = 'gauge'
			  AND timestamp >= $2 AND timestamp < $3
			GROUP BY metric_name, labels, (%s)
			ON CONFLICT (metric_name, aggregation_type, labels, bucket_start, bucket_end) 
			DO UPDATE SET value = EXCLUDED.value, created_at = NOW()`,
			strings.ToUpper(agg), bucketStart, bucketStart, bucketStart)

		_, err := tx.Exec(ctx, query, agg, timeRange.Start, timeRange.End, bucketSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func getBucketStart(bucketSize time.Duration) string {
	switch bucketSize {
	case 5 * time.Minute:
		return "date_trunc('minute', timestamp) - INTERVAL '1 minute' * (EXTRACT(minute FROM timestamp)::int % 5)"
	case 15 * time.Minute:
		return "date_trunc('minute', timestamp) - INTERVAL '1 minute' * (EXTRACT(minute FROM timestamp)::int % 15)"
	case 6 * time.Hour:
		return "date_trunc('hour', timestamp) - INTERVAL '1 hour' * (EXTRACT(hour FROM timestamp)::int % 6)"
	default:
		return "date_trunc('" + getBucketString(bucketSize) + "', timestamp)"
	}
}

func (s *PostgresStore) CreatePartition(ctx context.Context, date time.Time) error {
	_, err := s.db.Exec(ctx, "SELECT create_metrics_partition_for_date($1)", date)
	if err != nil {
		return fmt.Errorf("creating partition for date %v: %w", date, err)
	}
	return nil
}

func (s *PostgresStore) GetTopQueries(ctx context.Context, timeRange TimeRange, limit int) ([]QueryCount, error) {
	query := `
		SELECT 
			labels->>'query' as query,
			labels->>'query_type' as query_type,
			SUM(value)::bigint as count
		FROM raw_metrics
		WHERE metric_name = 'search_queries_detailed'
		AND timestamp >= $1 AND timestamp <= $2
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
		err := rows.Scan(&result.Query, &result.QueryType, &result.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning query count: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

func (s *PostgresStore) GetTopAssets(ctx context.Context, timeRange TimeRange, limit int) ([]AssetCount, error) {
	query := `
		SELECT 
			COALESCE(labels->>'asset_id', '') as asset_id,
			COALESCE(labels->>'asset_type', '') as asset_type,
			COALESCE(labels->>'asset_name', '') as asset_name,
			COALESCE(labels->>'asset_provider', '') as asset_provider,
			SUM(value)::bigint as count
		FROM raw_metrics
		WHERE metric_name = 'asset_views_total'
		AND timestamp >= $1 AND timestamp <= $2
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
		err := rows.Scan(&result.AssetID, &result.AssetType, &result.AssetName, &result.AssetProvider, &result.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning asset count: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

func (s *PostgresStore) DeleteOldMetrics(ctx context.Context, olderThan time.Time) error {
	_, err := s.db.Exec(ctx, "DELETE FROM raw_metrics WHERE timestamp < $1", olderThan)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete old raw metrics")
		return err
	}

	aggregatedCutoff := olderThan.AddDate(0, -12, 0)
	_, err = s.db.Exec(ctx, "DELETE FROM aggregated_metrics WHERE bucket_end < $1", aggregatedCutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete old aggregated metrics")
		return err
	}

	return nil
}

func (s *PostgresStore) tryAdvisoryLock(ctx context.Context, lockID int64) (bool, error) {
	var acquired bool
	err := s.db.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	return acquired, err
}

func (s *PostgresStore) releaseAdvisoryLock(ctx context.Context, lockID int64) error {
	_, err := s.db.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	return err
}

func getBucketString(duration time.Duration) string {
	switch duration {
	case time.Minute:
		return "minute"
	case 5 * time.Minute:
		return "minute"
	case 15 * time.Minute:
		return "minute"
	case time.Hour:
		return "hour"
	case 6 * time.Hour:
		return "hour"
	case 24 * time.Hour:
		return "day"
	default:
		return "hour"
	}
}
