-- Staleness looks up the pipeline that last refreshed each asset, which is the asset's newest schedule link.
CREATE INDEX IF NOT EXISTS idx_asset_schedules_asset_updated
    ON asset_schedules (asset_id, updated_at DESC);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_asset_schedules_asset_updated;
