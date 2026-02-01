package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/background"
	"github.com/rs/zerolog/log"
)

type Service struct {
	collector   *Collector
	store       Store
	db          *pgxpool.Pool
	ownerFields []string

	statsRefreshTask         *background.SingletonTask
	partitionTask            *background.SingletonTask
	cleanupTask              *background.SingletonTask
	metadataValueRefreshTask *background.SingletonTask
}

func NewService(store Store, db *pgxpool.Pool) *Service {
	collector := NewCollector(store)

	return &Service{
		collector:   collector,
		store:       store,
		db:          db,
		ownerFields: []string{"owner", "ownedBy"},
	}
}

// SetOwnerFields configures which metadata fields to use for owner statistics.
func (s *Service) SetOwnerFields(fields []string) {
	s.ownerFields = fields
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

func (s *Service) GetMetrics(ctx context.Context, opts QueryOptions) ([]AggregatedMetric, error) {
	// All queries now use aggregated data from the timeseries table
	return s.store.GetAggregatedMetrics(ctx, opts)
}

func (s *Service) GetTotalAssets(ctx context.Context) (int64, error) {
	return s.store.GetTotalAssets(ctx)
}

func (s *Service) GetAssetsByType(ctx context.Context) (map[string]int64, error) {
	return s.store.GetAssetsByType(ctx)
}

func (s *Service) GetAssetsByProvider(ctx context.Context) (map[string]int64, error) {
	return s.store.GetAssetsByProvider(ctx)
}

func (s *Service) GetAssetsWithSchemas(ctx context.Context) (int64, error) {
	return s.store.GetAssetsWithSchemas(ctx)
}

func (s *Service) GetTotalAssetsFiltered(ctx context.Context, excludedTypes []string, excludedProviders []string) (int64, error) {
	return s.store.GetTotalAssetsFiltered(ctx, excludedTypes, excludedProviders)
}

func (s *Service) GetAssetsByOwner(ctx context.Context, ownerFields []string) (map[string]int64, error) {
	return s.store.GetAssetsByOwner(ctx, ownerFields)
}

func (s *Service) Start(ctx context.Context) {
	// Start async metric recording worker
	s.collector.StartAsyncRecording()

	// Refresh asset statistics on startup
	s.refreshAssetStatistics(ctx)

	// Ensure tomorrow's partition exists
	s.ensurePartitions(ctx)

	// Background task: refresh asset statistics every 5 minutes
	s.statsRefreshTask = background.NewSingletonTask(background.SingletonConfig{
		Name:     "asset-statistics-refresh",
		DB:       s.db,
		Interval: 5 * time.Minute,
		TaskFn: func(ctx context.Context) error {
			s.refreshAssetStatistics(ctx)
			return nil
		},
	})
	s.statsRefreshTask.Start(ctx)

	// Background task: create partitions for upcoming days (runs daily)
	s.partitionTask = background.NewSingletonTask(background.SingletonConfig{
		Name:     "metrics-partition-management",
		DB:       s.db,
		Interval: 24 * time.Hour,
		TaskFn: func(ctx context.Context) error {
			s.ensurePartitions(ctx)
			return nil
		},
	})
	s.partitionTask.Start(ctx)

	// Background task: cleanup old partitions (runs daily)
	s.cleanupTask = background.NewSingletonTask(background.SingletonConfig{
		Name:     "metrics-cleanup",
		DB:       s.db,
		Interval: 24 * time.Hour,
		TaskFn: func(ctx context.Context) error {
			// Keep 7 days of metrics
			cutoff := time.Now().AddDate(0, 0, -7)
			return s.store.DeleteOldMetrics(ctx, cutoff)
		},
	})
	s.cleanupTask.Start(ctx)

	// Background task: refresh metadata value counts (every 5 minutes)
	s.metadataValueRefreshTask = background.NewSingletonTask(background.SingletonConfig{
		Name:     "metadata-value-counts-refresh",
		DB:       s.db,
		Interval: 5 * time.Minute,
		TaskFn: func(ctx context.Context) error {
			return s.store.RefreshMetadataValueCounts(ctx)
		},
	})
	s.metadataValueRefreshTask.Start(ctx)

	log.Info().Msg("Metrics service started (array-based storage, no aggregation jobs)")
}

func (s *Service) Stop() {
	// Stop async metric recording and flush remaining metrics
	if s.collector != nil {
		s.collector.StopAsyncRecording()
	}

	if s.statsRefreshTask != nil {
		s.statsRefreshTask.Stop()
	}
	if s.partitionTask != nil {
		s.partitionTask.Stop()
	}
	if s.cleanupTask != nil {
		s.cleanupTask.Stop()
	}
	if s.metadataValueRefreshTask != nil {
		s.metadataValueRefreshTask.Stop()
	}

	log.Info().Msg("Metrics service stopped")
}

func (s *Service) Collector() *Collector {
	return s.collector
}

// refreshAssetStatistics updates the pre-computed asset statistics table
// and updates Prometheus gauges.
func (s *Service) refreshAssetStatistics(ctx context.Context) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.store.RefreshAssetStatistics(ctx, s.ownerFields); err != nil {
		log.Warn().Err(err).Msg("Failed to refresh asset statistics")
		return
	}

	// Update Prometheus gauges with fresh breakdown data
	if s.collector != nil {
		breakdown, err := s.store.GetAssetBreakdown(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get asset breakdown for Prometheus")
			return
		}
		s.collector.UpdateAssetMetrics(breakdown)
	}

	log.Debug().
		Dur("duration", time.Since(start)).
		Msg("Asset statistics refreshed")
}

// ensurePartitions creates partitions for the next 7 days.
func (s *Service) ensurePartitions(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for i := 0; i <= 7; i++ {
		date := time.Now().AddDate(0, 0, i)
		if err := s.store.CreatePartition(ctx, date); err != nil {
			log.Warn().Err(err).Time("date", date).Msg("Failed to create metrics partition")
		}
	}

	log.Debug().Msg("Metrics partitions verified")
}
