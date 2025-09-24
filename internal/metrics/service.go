package metrics

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Service struct {
	collector *Collector
	store     Store

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewService(store Store) *Service {
	collector := NewCollector(store)

	return &Service{
		collector: collector,
		store:     store,
		stopCh:    make(chan struct{}),
	}
}

func (s *Service) GetRecorder() Recorder {
	return NewRecorder(s.collector)
}

func (s *Service) GetTopAssets(ctx context.Context, timeRange TimeRange, limit int) ([]AssetCount, error) {
	return s.store.GetTopAssets(ctx, timeRange, limit)
}

func (s *Service) GetTopQueries(ctx context.Context, timeRange TimeRange, limit int) ([]QueryCount, error) {
	return s.store.GetTopQueries(ctx, timeRange, limit)
}

func (s *Service) GetTotalAssets(ctx context.Context) (int64, error) {
	count, err := s.store.GetTotalAssets(ctx)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return count, err
}

func (s *Service) GetAssetsByType(ctx context.Context) (map[string]int64, error) {
	assets, err := s.store.GetAssetsByType(ctx)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return assets, err
}

func (s *Service) GetAssetsByProvider(ctx context.Context) (map[string]int64, error) {
	assets, err := s.store.GetAssetsByProvider(ctx)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return assets, err
}

func (s *Service) GetAssetsWithSchemas(ctx context.Context) (int64, error) {
	count, err := s.store.GetAssetsWithSchemas(ctx)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return count, err
}

func (s *Service) GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error) {
	count, err := s.store.GetTotalAssetsFiltered(ctx, excludedTypes, excludedProviders)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return count, err
}

func (s *Service) GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error) {
	assets, err := s.store.GetAssetsByOwner(ctx, ownerFields)
	if err == nil {
		s.updateAssetMetrics(ctx)
	}
	return assets, err
}

func (s *Service) updateAssetMetrics(ctx context.Context) {
	if s.collector == nil {
		return
	}

	breakdown, err := s.store.GetAssetBreakdown(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get asset breakdown for metrics")
		return
	}

	s.collector.UpdateAssetMetrics(breakdown)
}

func (s *Service) Start(ctx context.Context) {
	s.updateAssetMetrics(ctx)

	s.wg.Add(1)
	go s.aggregationWorker(ctx)

	s.wg.Add(1)
	go s.cleanupWorker(ctx)

	log.Info().Msg("Metrics service started")
}

func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Info().Msg("Metrics service stopped")
}

func (s *Service) Collector() *Collector {
	return s.collector
}

func (s *Service) GetMetrics(ctx context.Context, opts QueryOptions) ([]AggregatedMetric, error) {
	duration := opts.TimeRange.End.Sub(opts.TimeRange.Start)

	if duration > 24*time.Hour && opts.BucketSize == 0 {
		if duration > 30*24*time.Hour {
			opts.BucketSize = 24 * time.Hour
		} else if duration > 7*24*time.Hour {
			opts.BucketSize = time.Hour
		} else {
			opts.BucketSize = 5 * time.Minute
		}

		return s.store.GetAggregatedMetrics(ctx, opts)
	}

	rawMetrics, err := s.store.GetMetrics(ctx, opts)
	if err != nil {
		return nil, err
	}

	return s.convertToAggregated(rawMetrics, opts.BucketSize), nil
}

func (s *Service) convertToAggregated(metrics []Metric, bucketSize time.Duration) []AggregatedMetric {
	if bucketSize == 0 {
		bucketSize = time.Minute
	}

	buckets := make(map[string][]float64)
	bucketTimes := make(map[string]time.Time)
	bucketLabels := make(map[string]map[string]string)

	for _, m := range metrics {
		bucketTime := m.Timestamp.Truncate(bucketSize)
		labelsHash := hashLabels(m.Labels)
		key := m.Name + ":" + bucketTime.Format(time.RFC3339) + ":" + labelsHash

		buckets[key] = append(buckets[key], m.Value)
		bucketTimes[key] = bucketTime
		bucketLabels[key] = m.Labels
	}

	var result []AggregatedMetric
	for key, values := range buckets {
		bucketTime := bucketTimes[key]
		labels := bucketLabels[key]

		var sum float64
		for _, v := range values {
			sum += v
		}
		avg := sum / float64(len(values))

		result = append(result, AggregatedMetric{
			Name:            extractMetricName(key),
			AggregationType: "avg",
			Value:           avg,
			Labels:          labels,
			BucketStart:     bucketTime,
			BucketEnd:       bucketTime.Add(bucketSize),
			BucketSize:      bucketSize,
		})
	}

	return result
}

func (s *Service) aggregationWorker(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runAggregation(ctx)
		}
	}
}

func (s *Service) runAggregation(ctx context.Context) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Debug().Dur("duration", duration).Msg("Metrics aggregation completed")
	}()

	now := time.Now()

	startTime := now.Add(-10 * time.Minute)
	endTime := now.Add(-5 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.store.AggregateMetrics(ctx, TimeRange{Start: startTime, End: endTime}, 5*time.Minute); err != nil {
		log.Warn().Err(err).
			Time("start", startTime).
			Time("end", endTime).
			Msg("Metrics aggregation failed - will retry next cycle")
		return
	}

	if now.Minute() == 5 {
		s.runHourlyAggregation(ctx)
	}

	if now.Hour() == 2 && now.Minute() == 5 {
		s.runDailyAggregation(ctx)
	}
}

func (s *Service) runHourlyAggregation(ctx context.Context) {
	now := time.Now()
	hourStart := now.Truncate(time.Hour).Add(-time.Hour)
	hourEnd := now.Truncate(time.Hour)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := s.store.AggregateMetrics(ctx, TimeRange{Start: hourStart, End: hourEnd}, time.Hour); err != nil {
		log.Warn().Err(err).
			Time("hour_start", hourStart).
			Msg("Hourly metrics aggregation failed - will retry in 1 hour")
	}
}

func (s *Service) runDailyAggregation(ctx context.Context) {
	now := time.Now()
	dayStart := now.Truncate(24*time.Hour).AddDate(0, 0, -1)
	dayEnd := now.Truncate(24 * time.Hour)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := s.store.AggregateMetrics(ctx, TimeRange{Start: dayStart, End: dayEnd}, 24*time.Hour); err != nil {
		log.Warn().Err(err).
			Time("day_start", dayStart).
			Msg("Daily metrics aggregation failed - will retry tomorrow")
	}
}

func (s *Service) cleanupWorker(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			cutoff := time.Now().AddDate(0, 0, -7)
			if err := s.store.DeleteOldMetrics(ctx, cutoff); err != nil {
				log.Error().Err(err).Msg("Failed to cleanup old metrics")
			}
		}
	}
}

func hashLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "no_labels"
	}

	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)

	h := fnv.New64a()
	for _, pair := range pairs {
		h.Write([]byte(pair))
	}
	return fmt.Sprintf("%x", h.Sum64())
}

func extractMetricName(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return key
}
