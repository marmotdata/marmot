-- Why each query ended the way it did, finer than decision and status: allowed by role, denied by permission or policy, failed at the source, timed out, could not reach the source or was cancelled. Rows recorded before this column read a reason derived from decision and status, so there is no backfill over a large log.
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS outcome_reason VARCHAR(40);

-- Who stood behind a machine caller: the service account a query ran as, the agent that identified itself (an MCP client's name) and the kind of credential the request authenticated with.
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS service_account VARCHAR(255);
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS agent VARCHAR(255);
ALTER TABLE data_queries ADD COLUMN IF NOT EXISTS credential VARCHAR(40);

---- create above / drop below ----

ALTER TABLE data_queries DROP COLUMN IF EXISTS credential;
ALTER TABLE data_queries DROP COLUMN IF EXISTS agent;
ALTER TABLE data_queries DROP COLUMN IF EXISTS service_account;
ALTER TABLE data_queries DROP COLUMN IF EXISTS outcome_reason;
