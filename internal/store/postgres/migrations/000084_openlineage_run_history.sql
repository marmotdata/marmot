-- OpenLineage producers retry an event they got no answer for, so storing one event twice must be a no-op. Job names, namespaces and producer URLs routinely run past 255 characters (dbt test names, Spark plans, connection-string namespaces), which failed the insert.

ALTER TABLE run_history
    ALTER COLUMN run_id TYPE TEXT,
    ALTER COLUMN job_namespace TYPE TEXT,
    ALTER COLUMN job_name TYPE TEXT,
    ALTER COLUMN producer TYPE TEXT;

DELETE FROM run_history a
USING run_history b
WHERE a.asset_id = b.asset_id
  AND a.run_id = b.run_id
  AND a.event_type = b.event_type
  AND a.event_time = b.event_time
  AND (a.created_at, a.id) > (b.created_at, b.id);

CREATE UNIQUE INDEX idx_run_history_event_unique
    ON run_history (asset_id, run_id, event_type, event_time);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_run_history_event_unique;

ALTER TABLE run_history
    ALTER COLUMN run_id TYPE VARCHAR(255) USING left(run_id, 255),
    ALTER COLUMN job_namespace TYPE VARCHAR(255) USING left(job_namespace, 255),
    ALTER COLUMN job_name TYPE VARCHAR(255) USING left(job_name, 255),
    ALTER COLUMN producer TYPE VARCHAR(255) USING left(producer, 255);
