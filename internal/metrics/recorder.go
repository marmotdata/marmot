package metrics

import (
	"context"
	"time"
)

type Recorder interface {
	RecordSearchQuery(ctx context.Context, queryType, query string)
	RecordAssetView(ctx context.Context, assetID, assetType, assetName, assetProvider string)
	RecordDBQuery(ctx context.Context, operation string, duration time.Duration, success bool)
	WrapDBQuery(ctx context.Context, operation string, fn func() error) error
	RecordCustomMetrics(ctx context.Context, metrics []Metric) error
}

type recorder struct {
	collector *Collector
}

func NewRecorder(collector *Collector) Recorder {
	return &recorder{collector: collector}
}

func (r *recorder) RecordSearchQuery(ctx context.Context, queryType, query string) {
	r.collector.RecordSearchQuery(queryType, query)
}

func (r *recorder) RecordAssetView(ctx context.Context, assetID, assetType, assetName, assetProvider string) {
	r.collector.RecordAssetView(assetID, assetType, assetName, assetProvider)
}

func (r *recorder) RecordDBQuery(ctx context.Context, operation string, duration time.Duration, success bool) {
	r.collector.RecordDBQuery(operation, duration, success)
}

func (r *recorder) WrapDBQuery(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	r.collector.RecordDBQuery(operation, duration, err == nil)
	return err
}

func (r *recorder) RecordCustomMetrics(ctx context.Context, metrics []Metric) error {
	return r.collector.store.RecordMetrics(ctx, metrics)
}
