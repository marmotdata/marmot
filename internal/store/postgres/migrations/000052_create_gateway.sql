-- The query gateway: query is a capability of an ingestion source, plus the
-- grants, sessions and audit log that govern it. A source (ingestion_schedules
-- row) is one connection to a system via a plugin; cataloguing (a cron
-- schedule) and querying are two capabilities of that one connection.

-- Query capability on sources. Query-only sources have no cron and are skipped
-- by the scheduler, whose due-query already requires next_run_at IS NOT NULL.
ALTER TABLE ingestion_schedules ALTER COLUMN cron_expression SET DEFAULT '';
ALTER TABLE ingestion_schedules ADD COLUMN queryable BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ingestion_schedules ADD COLUMN query_modes TEXT[] NOT NULL DEFAULT '{direct}';

-- What a principal may query. Deny by default: a query is allowed only when
-- every resource it references matches at least one live grant.
CREATE TABLE gateway_grants (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    principal_type    VARCHAR(20) NOT NULL,
    principal_id      UUID NOT NULL,
    -- MRN glob, e.g. mrn://postgresql/ecommerce/** or mrn://target/trino.
    resource_selector TEXT NOT NULL,
    actions           TEXT[] NOT NULL DEFAULT '{query}',
    expires_at        TIMESTAMPTZ,
    created_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at        TIMESTAMPTZ,
    revoked_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    reason            TEXT
);

CREATE INDEX idx_gateway_grants_principal
    ON gateway_grants(principal_type, principal_id) WHERE revoked_at IS NULL;

-- The thing an agent holds instead of a password.
CREATE TABLE gateway_sessions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    principal_type   VARCHAR(20) NOT NULL,
    principal_id     UUID NOT NULL,
    purpose          TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ,
    revoked_at       TIMESTAMPTZ,
    revoked_by       UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_gateway_sessions_principal
    ON gateway_sessions(principal_type, principal_id);

-- Every query on every path, allowed or denied. target_id/target_name are
-- denormalised (a source id and name) and carry no FK so a deleted source
-- leaves its audit history intact.
CREATE TABLE gateway_audit_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID REFERENCES gateway_sessions(id) ON DELETE SET NULL,
    principal_type  VARCHAR(20) NOT NULL,
    principal_id    UUID NOT NULL,
    audit_subject   VARCHAR(255) NOT NULL,
    target_id       UUID,
    target_name     VARCHAR(255) NOT NULL,
    query_text      TEXT NOT NULL,
    referenced_mrns TEXT[],
    decision        VARCHAR(20) NOT NULL,
    decision_detail JSONB,
    status          VARCHAR(20) NOT NULL,
    rows_returned   BIGINT,
    error           TEXT,
    source          VARCHAR(20) NOT NULL DEFAULT 'direct',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gateway_audit_log_session ON gateway_audit_log(session_id);
CREATE INDEX idx_gateway_audit_log_principal ON gateway_audit_log(principal_type, principal_id, started_at DESC);
CREATE INDEX idx_gateway_audit_log_started ON gateway_audit_log(started_at DESC);

INSERT INTO permissions (name, description, resource_type, action) VALUES
    ('gateway_query',  'Execute queries through the query gateway',                   'gateway', 'query'),
    ('gateway_view',   'View query gateway sessions, grants and audit log',           'gateway', 'view'),
    ('gateway_manage', 'Manage query capability and grants, revoke any session',      'gateway', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name IN ('gateway_query', 'gateway_view', 'gateway_manage');

---- create above / drop below ----

DELETE FROM role_permissions
 WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE name IN ('gateway_query', 'gateway_view', 'gateway_manage')
 );
DELETE FROM permissions
 WHERE name IN ('gateway_query', 'gateway_view', 'gateway_manage');

DROP TABLE IF EXISTS gateway_audit_log;
DROP TABLE IF EXISTS gateway_sessions;
DROP TABLE IF EXISTS gateway_grants;

ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS query_modes;
ALTER TABLE ingestion_schedules DROP COLUMN IF EXISTS queryable;
ALTER TABLE ingestion_schedules ALTER COLUMN cron_expression DROP DEFAULT;
