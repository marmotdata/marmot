-- Hourly rollups of the data query log. Dashboards and insights read these
-- instead of scanning data_queries, so they stay fast with thousands of
-- callers and millions of queries. Both are maintained in the same statement
-- that records a query's outcome.
CREATE TABLE data_query_stats_hourly (
    bucket         TIMESTAMPTZ NOT NULL,
    principal_type VARCHAR(20) NOT NULL,
    principal_id   TEXT NOT NULL,
    audit_subject  VARCHAR(255) NOT NULL,
    source_name    VARCHAR(255) NOT NULL,
    decision       VARCHAR(20) NOT NULL,
    status         VARCHAR(20) NOT NULL,
    queries        BIGINT NOT NULL DEFAULT 0,
    rows_returned  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket, principal_type, principal_id, source_name, decision, status)
);

CREATE INDEX idx_data_query_stats_hourly_principal
    ON data_query_stats_hourly(principal_type, principal_id, bucket);

-- One row per caller, table and hour. Source-level fallback resources are
-- left out because they are not tables.
CREATE TABLE data_query_table_stats_hourly (
    bucket         TIMESTAMPTZ NOT NULL,
    mrn            TEXT NOT NULL,
    principal_type VARCHAR(20) NOT NULL,
    principal_id   TEXT NOT NULL,
    audit_subject  VARCHAR(255) NOT NULL,
    decision       VARCHAR(20) NOT NULL,
    queries        BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket, mrn, principal_type, principal_id, decision)
);

CREATE INDEX idx_data_query_table_stats_hourly_access
    ON data_query_table_stats_hourly(principal_type, principal_id, mrn, bucket);

INSERT INTO data_query_stats_hourly (bucket, principal_type, principal_id, audit_subject, source_name, decision, status, queries, rows_returned)
SELECT date_trunc('hour', started_at), principal_type, principal_id, max(audit_subject), source_name, decision, status,
       count(*), sum(COALESCE(rows_returned, 0))
FROM data_queries
WHERE status <> 'running'
GROUP BY 1, 2, 3, 5, 6, 7;

INSERT INTO data_query_table_stats_hourly (bucket, mrn, principal_type, principal_id, audit_subject, decision, queries)
SELECT date_trunc('hour', q.started_at), u.mrn, q.principal_type, q.principal_id, max(q.audit_subject), q.decision, count(*)
FROM data_queries q
CROSS JOIN LATERAL (SELECT DISTINCT m AS mrn FROM unnest(q.referenced_mrns) AS m) u
WHERE q.status <> 'running' AND u.mrn NOT LIKE 'mrn://source/%'
GROUP BY 1, 2, 3, 4, 6;

---- create above / drop below ----

DROP TABLE IF EXISTS data_query_table_stats_hourly;
DROP TABLE IF EXISTS data_query_stats_hourly;
