package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/metrics"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// TotalAssets is the catalog-wide asset count.
type TotalAssets = models.V1MetricsTotalAssetsResponse

// AssetsByType is the by-type asset breakdown.
type AssetsByType = models.V1MetricsAssetsByTypeResponse

// AssetsByProvider is the by-provider asset breakdown.
type AssetsByProvider = models.V1MetricsAssetsByProviderResponse

// AssetCount is one (asset, count) entry.
type AssetCount = models.MetricsAssetCount

// QueryCount is one (query, count) entry.
type QueryCount = models.MetricsQueryCount

// TopOptions sets the time window and limit for top-N queries.
// Start and End must be RFC3339 timestamps.
type TopOptions struct {
	Start string
	End   string
	Limit int64
}

// MetricsService returns catalog usage and asset breakdown metrics.
type MetricsService struct {
	gen *apiclient.Marmot
}

// TotalAssets returns the total number of assets in the catalog.
func (s *MetricsService) TotalAssets(ctx context.Context) (*TotalAssets, error) {
	p := metrics.NewGetMetricsAssetsTotalParams().WithContext(ctx)
	resp, err := s.gen.Metrics.GetMetricsAssetsTotal(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AssetsByType returns asset counts grouped by type.
func (s *MetricsService) AssetsByType(ctx context.Context) (*AssetsByType, error) {
	p := metrics.NewGetMetricsAssetsByTypeParams().WithContext(ctx)
	resp, err := s.gen.Metrics.GetMetricsAssetsByType(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AssetsByProvider returns asset counts grouped by provider.
func (s *MetricsService) AssetsByProvider(ctx context.Context) (*AssetsByProvider, error) {
	p := metrics.NewGetMetricsAssetsByProviderParams().WithContext(ctx)
	resp, err := s.gen.Metrics.GetMetricsAssetsByProvider(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// TopAssets returns the most-viewed assets in the given time range.
func (s *MetricsService) TopAssets(ctx context.Context, opts TopOptions) ([]*AssetCount, error) {
	p := metrics.NewGetMetricsTopAssetsParams().WithContext(ctx).WithStart(opts.Start).WithEnd(opts.End)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	resp, err := s.gen.Metrics.GetMetricsTopAssets(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// TopQueries returns the most-run queries in the given time range.
func (s *MetricsService) TopQueries(ctx context.Context, opts TopOptions) ([]*QueryCount, error) {
	p := metrics.NewGetMetricsTopQueriesParams().WithContext(ctx).WithStart(opts.Start).WithEnd(opts.End)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	resp, err := s.gen.Metrics.GetMetricsTopQueries(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
