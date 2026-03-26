package client

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// MetricsSummary combines total assets and by-type breakdowns.
type MetricsSummary struct {
	TotalAssets  int                    `json:"total_assets"`
	AssetsByType map[string]interface{} `json:"assets_by_type"`
}

// TopQueryEntry represents a popular search query.
type TopQueryEntry struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

// TopAssetEntry represents a frequently viewed asset.
type TopAssetEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// GetTotalAssets retrieves the total asset count.
func (c *Client) GetTotalAssets(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/metrics/assets/total", nil)
}

// GetAssetsByType retrieves asset counts grouped by type.
func (c *Client) GetAssetsByType(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/metrics/assets/by-type", nil)
}

// GetAssetsByProvider retrieves asset counts grouped by provider.
func (c *Client) GetAssetsByProvider(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/metrics/assets/by-provider", nil)
}

// GetTopQueries retrieves the most popular search queries within a time range.
func (c *Client) GetTopQueries(ctx context.Context, start, end time.Time, limit int) ([]byte, error) {
	q := url.Values{}
	q.Set("start", start.Format(time.RFC3339))
	q.Set("end", end.Format(time.RFC3339))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.getRaw(ctx, "/metrics/top-queries", q)
}

// GetTopAssets retrieves the most viewed assets within a time range.
func (c *Client) GetTopAssets(ctx context.Context, start, end time.Time, limit int) ([]byte, error) {
	q := url.Values{}
	q.Set("start", start.Format(time.RFC3339))
	q.Set("end", end.Format(time.RFC3339))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.getRaw(ctx, "/metrics/top-assets", q)
}

// GetMetrics retrieves aggregated metrics within a time range.
func (c *Client) GetMetrics(ctx context.Context, start, end time.Time) ([]byte, error) {
	q := url.Values{}
	q.Set("start", start.Format(time.RFC3339))
	q.Set("end", end.Format(time.RFC3339))
	return c.getRaw(ctx, "/metrics", q)
}
