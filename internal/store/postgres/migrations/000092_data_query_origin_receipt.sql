-- What caused a query, such as task_run:<id>, so the queries a task or another in-process caller ran can be traced from the log. Partial, since most queries are a person's own and have none.
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS origin VARCHAR(200);
CREATE INDEX IF NOT EXISTS idx_data_queries_origin ON data_queries(origin, started_at DESC) WHERE origin IS NOT NULL;

-- A serving plugin's signed receipt for a query on a served source, with whether it verified, kept beside Marmot's own account of the query. NULL for sources Marmot runs itself and for queries recorded before receipts.
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS runner_receipt JSONB;

---- create above / drop below ----

ALTER TABLE data_queries DROP COLUMN IF EXISTS runner_receipt;
DROP INDEX IF EXISTS idx_data_queries_origin;
ALTER TABLE data_queries DROP COLUMN IF EXISTS origin;
