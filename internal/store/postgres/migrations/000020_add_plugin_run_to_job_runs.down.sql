-- Remove the plugin_run_id column
DROP INDEX IF EXISTS idx_ingestion_job_runs_plugin_run;
ALTER TABLE ingestion_job_runs DROP COLUMN IF EXISTS plugin_run_id;
