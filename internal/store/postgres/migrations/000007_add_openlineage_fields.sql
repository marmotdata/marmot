-- Add query fields to assets table
ALTER TABLE assets ADD COLUMN query TEXT NULL;
ALTER TABLE assets ADD COLUMN query_language VARCHAR(50) NULL;

-- Add stub field to assets table  
ALTER TABLE assets ADD COLUMN is_stub BOOLEAN NOT NULL DEFAULT FALSE;

-- Add run history table for job executions
CREATE TABLE IF NOT EXISTS run_history (
    id VARCHAR(255) PRIMARY KEY,
    asset_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    job_namespace VARCHAR(255) NOT NULL,
    job_name VARCHAR(255) NOT NULL,
    event_type VARCHAR(20) NOT NULL, -- START, RUNNING, COMPLETE, FAIL, ABORT, OTHER
    event_time TIMESTAMP WITH TIME ZONE NOT NULL,
    producer VARCHAR(255) NULL,
    run_facets JSONB NOT NULL DEFAULT '{}'::jsonb,
    job_facets JSONB NOT NULL DEFAULT '{}'::jsonb,
    inputs JSONB NOT NULL DEFAULT '[]'::jsonb,
    outputs JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_assets_is_stub ON assets (is_stub);
CREATE INDEX IF NOT EXISTS idx_assets_query_language ON assets (query_language);
CREATE INDEX IF NOT EXISTS idx_run_history_asset_id ON run_history (asset_id);
CREATE INDEX IF NOT EXISTS idx_run_history_run_id ON run_history (run_id);
CREATE INDEX IF NOT EXISTS idx_run_history_event_type ON run_history (event_type);
CREATE INDEX IF NOT EXISTS idx_run_history_event_time ON run_history (event_time);
CREATE INDEX IF NOT EXISTS idx_run_history_job ON run_history (job_namespace, job_name);

-- Update search index to exclude stub assets by default
DROP INDEX IF EXISTS idx_assets_search;
CREATE INDEX IF NOT EXISTS idx_assets_search ON assets USING gin(search_text) WHERE is_stub = FALSE;
-- Add new index for all assets including stubs
CREATE INDEX IF NOT EXISTS idx_assets_search_all ON assets USING gin(search_text);

---- create above / drop below ----

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
