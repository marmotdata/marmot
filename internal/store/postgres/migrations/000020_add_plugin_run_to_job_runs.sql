-- Add plugin_run_id column to link ingestion_job_runs with plugin runs
ALTER TABLE ingestion_job_runs
ADD COLUMN plugin_run_id UUID REFERENCES runs(id) ON DELETE SET NULL;

-- Add index for lookups
CREATE INDEX idx_ingestion_job_runs_plugin_run ON ingestion_job_runs(plugin_run_id);

COMMENT ON COLUMN ingestion_job_runs.plugin_run_id IS 'ID of the plugin run created when executing this job';

---- create above / drop below ----

-- Remove the plugin_run_id column
DROP INDEX IF EXISTS idx_ingestion_job_runs_plugin_run;
ALTER TABLE ingestion_job_runs DROP COLUMN IF EXISTS plugin_run_id;
