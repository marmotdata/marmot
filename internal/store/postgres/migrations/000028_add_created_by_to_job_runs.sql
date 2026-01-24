ALTER TABLE ingestion_job_runs ADD COLUMN created_by VARCHAR(255);

COMMENT ON COLUMN ingestion_job_runs.created_by IS 'User or system that triggered this job run';

---- create above / drop below ----

ALTER TABLE ingestion_job_runs DROP COLUMN created_by;
