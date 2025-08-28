package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Collector struct {
	registry prometheus.Registerer
	store    Store

	httpRequests    *prometheus.CounterVec
	httpDuration    *prometheus.HistogramVec
	activeUsers     prometheus.Gauge
	dbConnections   prometheus.Gauge
	assetOperations *prometheus.CounterVec
	userOperations  *prometheus.CounterVec
	authFailures    *prometheus.CounterVec
	searchQueries   *prometheus.CounterVec
	assetViews      *prometheus.CounterVec

	// DB metrics
	dbQueries       *prometheus.CounterVec
	dbQueryDuration *prometheus.HistogramVec
	dbErrors        *prometheus.CounterVec

	assets *prometheus.GaugeVec
}

func NewCollector(store Store) *Collector {
	c := &Collector{
		registry: prometheus.DefaultRegisterer,
		store:    store,
	}

	c.httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	c.httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "marmot_http_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	c.activeUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "marmot_active_users",
		Help: "Number of currently active users",
	})

	c.dbConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "marmot_db_connections",
		Help: "Number of active database connections",
	})

	c.assetOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_asset_operations_total",
		Help: "Total number of asset operations",
	}, []string{"operation", "status"})

	c.userOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_user_operations_total",
		Help: "Total number of user operations",
	}, []string{"operation", "status"})

	c.authFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_auth_failures_total",
		Help: "Total number of authentication failures",
	}, []string{"reason"})

	c.dbQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_db_queries_total",
		Help: "Total number of database queries",
	}, []string{"operation", "status"})

	c.dbQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "marmot_db_query_duration_seconds",
		Help:    "Database query duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
	}, []string{"operation", "status"})

	c.dbErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_db_errors_total",
		Help: "Total number of database errors",
	}, []string{"operation"})

	c.searchQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_search_queries_total",
		Help: "Total number of search queries executed",
	}, []string{"query_type"})

	c.assetViews = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_asset_views_total",
		Help: "Total number of asset views",
	}, []string{"type", "provider"})

	c.assets = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "marmot_assets",
		Help: "Number of assets by various dimensions",
	}, []string{"type", "provider", "has_schema", "owner"})

	return c
}

func (c *Collector) RecordHTTPRequest(method, path, status string) error {
	c.httpRequests.WithLabelValues(method, path, status).Inc()

	if c.shouldStoreForUI("http_requests") {
		return c.store.RecordMetric(context.Background(), Metric{
			Name:      "http_requests_total",
			Type:      Counter,
			Value:     1,
			Labels:    map[string]string{"method": method, "path": path, "status": status},
			Timestamp: time.Now(),
		})
	}
	return nil
}

func (c *Collector) RecordHTTPDuration(method, path string, duration time.Duration) {
	c.httpDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

func (c *Collector) RecordDBQuery(operation string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
		c.dbErrors.WithLabelValues(operation).Inc()
	}

	c.dbQueryDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
	c.dbQueries.WithLabelValues(operation, status).Inc()
}

func (c *Collector) SetDBConnections(count int) {
	c.dbConnections.Set(float64(count))
}

func (c *Collector) RecordSearchQuery(queryType, query string) error {
	c.searchQueries.WithLabelValues(queryType).Inc()

	return c.store.RecordMetric(context.Background(), Metric{
		Name:      "search_queries_detailed",
		Type:      Counter,
		Value:     1,
		Labels:    map[string]string{"query_type": queryType, "query": query},
		Timestamp: time.Now(),
	})
}

func (c *Collector) RecordAssetView(assetID, assetType, assetName, assetProvider string) error {
	if assetType != "" && assetProvider != "" {
		c.assetViews.WithLabelValues(assetType, assetProvider).Inc()
	}

	if c.shouldStoreForUI("asset_views") {
		return c.store.RecordMetric(context.Background(), Metric{
			Name:  "asset_views_total",
			Type:  Counter,
			Value: 1,
			Labels: map[string]string{
				"asset_id":       assetID,
				"asset_type":     assetType,
				"asset_name":     assetName,
				"asset_provider": assetProvider,
			},
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (c *Collector) UpdateAssetMetrics(breakdown []AssetBreakdown) {
	c.assets.Reset()

	for _, asset := range breakdown {
		hasSchema := "false"
		if asset.HasSchema {
			hasSchema = "true"
		}

		c.assets.WithLabelValues(asset.Type, asset.Provider, hasSchema, asset.Owner).Set(float64(asset.Count))
	}
}

func (c *Collector) shouldStoreForUI(metricName string) bool {
	uiMetrics := map[string]bool{
		"http_requests":    true,
		"active_users":     true,
		"user_operations":  true,
		"asset_operations": true,
		"search_queries":   true,
		"asset_views":      true,
	}
	return uiMetrics[metricName]
}
