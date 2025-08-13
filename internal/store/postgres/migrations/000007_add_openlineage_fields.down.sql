-- Drop indexes
DROP INDEX IF EXISTS idx_assets_is_stub;
DROP INDEX IF EXISTS idx_assets_query_language;
DROP INDEX IF EXISTS idx_run_history_asset_id;
DROP INDEX IF EXISTS idx_run_history_run_id;
DROP INDEX IF EXISTS idx_run_history_event_type;
DROP INDEX IF EXISTS idx_run_history_event_time;
DROP INDEX IF EXISTS idx_run_history_job;
DROP INDEX IF EXISTS idx_assets_search;
DROP INDEX IF EXISTS idx_assets_search_all;

-- Drop run_history table
DROP TABLE IF EXISTS run_history;

-- Remove columns from assets table
ALTER TABLE assets DROP COLUMN IF EXISTS query;
ALTER TABLE assets DROP COLUMN IF EXISTS query_language;
ALTER TABLE assets DROP COLUMN IF EXISTS is_stub;

-- Restore original search index
CREATE INDEX IF NOT EXISTS idx_assets_search ON assets USING gin(search_text);
