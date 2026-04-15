ALTER TABLE ingestion_job_runs
  ADD COLUMN IF NOT EXISTS pipeline_name VARCHAR(255),
  ADD COLUMN IF NOT EXISTS source_name VARCHAR(255);

---- create above / drop below ----

ALTER TABLE ingestion_job_runs
  DROP COLUMN IF EXISTS source_name,
  DROP COLUMN IF EXISTS pipeline_name;
