-- Popularity reads each asset's views over the last 30 days, both for a page of search results and for the popular sort, so the views get an index keyed by asset rather than a scan of every metric.
CREATE INDEX IF NOT EXISTS idx_metrics_ts_asset_views
    ON metrics_timeseries ((labels->>'asset_id'), day)
    WHERE metric_name = 'asset_views_total';

---- create above / drop below ----

DROP INDEX IF EXISTS idx_metrics_ts_asset_views;
