-- Ingest-on-query: a queryable source can refresh its catalog when it is
-- queried, so a query-only source populates itself the first time an agent
-- uses it and hot sources stay fresh. Ingestion runs asynchronously and is
-- rate-limited (per source and globally) so queries are never slowed and the
-- warehouse is never re-walked on every query.
ALTER TABLE ingestion_schedules ADD COLUMN ingest_on_query BOOLEAN NOT NULL DEFAULT false;

-- Query-only sources (queryable, no catalog schedule) self-catalog by default,
-- so context fusion works out of the box without configuring a schedule.
UPDATE ingestion_schedules SET ingest_on_query = true WHERE queryable = true AND cron_expression = '';

---- create above / drop below ----

ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS ingest_on_query;
