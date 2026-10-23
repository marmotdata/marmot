-- Data queries: agents and users query catalogued sources through Marmot, as
-- themselves, with every query recorded. Query is a
-- capability of an ingestion source rather than a separate connection, so one
-- source both catalogs and answers queries; a query-only source is simply a
-- queryable source with no cron schedule.
ALTER TABLE ingestion_schedules ADD COLUMN queryable BOOLEAN NOT NULL DEFAULT false;
-- Refresh the source's catalog in the background after queries, rate-limited.
ALTER TABLE ingestion_schedules ADD COLUMN ingest_on_query BOOLEAN NOT NULL DEFAULT false;

-- Every query, allowed or denied. Source id and name are denormalised with no
-- FK so a deleted source keeps its history. principal_id is TEXT because
-- operator and OIDC-trust principals do not have UUIDs. context is a snapshot
-- of the catalog's view of every referenced asset when the query ran, which
-- is what an answer receipt is later built from.
CREATE TABLE data_queries (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    principal_type  VARCHAR(20) NOT NULL,
    principal_id    TEXT NOT NULL,
    audit_subject   VARCHAR(255) NOT NULL,
    client          VARCHAR(20) NOT NULL,
    source_id       UUID,
    source_name     VARCHAR(255) NOT NULL,
    statement       TEXT NOT NULL,
    referenced_mrns TEXT[] NOT NULL DEFAULT '{}',
    decision        VARCHAR(20) NOT NULL,
    decision_detail JSONB,
    status          VARCHAR(20) NOT NULL,
    rows_returned   BIGINT,
    truncated       BOOLEAN NOT NULL DEFAULT false,
    error           TEXT,
    context         JSONB,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_data_queries_started ON data_queries(started_at DESC);
CREATE INDEX idx_data_queries_principal ON data_queries(principal_type, principal_id, started_at DESC);
CREATE INDEX idx_data_queries_source ON data_queries(source_name, started_at DESC);
CREATE INDEX idx_data_queries_mrns ON data_queries USING GIN (referenced_mrns);

-- assets:query is named the way the cloud distribution expects: its IAM
-- migration renames assets to asset, making it asset:query, which is bound at
-- root, an asset or a data product there. In OSS it admits a caller to every
-- queryable source, so only admins hold it by default.
INSERT INTO permissions (name, description, resource_type, action) VALUES
    ('query_assets',  'Run read-only queries against catalogued data', 'assets', 'query'),
    ('access_view',   'View the data query log',                       'access', 'view'),
    ('access_manage', 'Choose which sources answer queries',           'access', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name IN ('query_assets', 'access_view', 'access_manage');

---- create above / drop below ----

DELETE FROM role_permissions
 WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('query_assets', 'access_view', 'access_manage')
 );
DELETE FROM permissions WHERE name IN ('query_assets', 'access_view', 'access_manage');

DROP TABLE IF EXISTS data_queries;

ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS ingest_on_query;
ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS queryable;
