-- The phases a worker can see a run reach. Queued, picked up, started and
-- finished already have columns (created_at, claimed_at, started_at and
-- finished_at); these record when the worker began launching the plugin,
-- asked it to discover and began writing what it found, so a failed run shows
-- the phase it stopped in. Runs from before this migration leave them empty.
ALTER TABLE ingestion_job_runs
    ADD COLUMN IF NOT EXISTS connecting_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS extracting_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS writing_at    TIMESTAMPTZ;

-- The pipelines list reads each pipeline's latest run to show and filter by
-- its status, which this serves without sorting a pipeline's whole history.
CREATE INDEX IF NOT EXISTS idx_ingestion_job_runs_schedule_created
    ON ingestion_job_runs (schedule_id, created_at DESC);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_ingestion_job_runs_schedule_created;
ALTER TABLE ingestion_job_runs
    DROP COLUMN IF EXISTS writing_at,
    DROP COLUMN IF EXISTS extracting_at,
    DROP COLUMN IF EXISTS connecting_at;
