-- Requests for access made from a refused query. Each one is kept so the people who can grant access see who asked, for what and why, and so a caller asking again for the same thing within a day does not notify everyone twice. The query and source are denormalised with no FK so a request outlives both.
CREATE TABLE IF NOT EXISTS data_query_access_requests (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    principal_type VARCHAR(20)  NOT NULL,
    principal_id   TEXT         NOT NULL,
    audit_subject  VARCHAR(255) NOT NULL,
    query_id       UUID,
    source_name    VARCHAR(255) NOT NULL,
    permission     VARCHAR(100),
    needed         TEXT,
    message        TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_query_access_requests_principal
    ON data_query_access_requests (principal_type, principal_id, source_name, created_at DESC);

---- create above / drop below ----

DROP TABLE IF EXISTS data_query_access_requests;
