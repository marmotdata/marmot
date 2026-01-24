CREATE TABLE ingestion_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    plugin_id VARCHAR(100) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    cron_expression VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMP WITH TIME ZONE,
    next_run_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE ingestion_job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID REFERENCES ingestion_schedules(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    claimed_by VARCHAR(255),
    claimed_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    log TEXT,
    error_message TEXT,
    assets_created INT DEFAULT 0,
    assets_updated INT DEFAULT 0,
    assets_deleted INT DEFAULT 0,
    lineage_created INT DEFAULT 0,
    documentation_added INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_status CHECK (status IN ('pending', 'claimed', 'running', 'succeeded', 'failed', 'cancelled'))
);

CREATE INDEX idx_ingestion_schedules_enabled ON ingestion_schedules(enabled);
CREATE INDEX idx_ingestion_schedules_next_run ON ingestion_schedules(next_run_at) WHERE enabled = true;
CREATE INDEX idx_ingestion_job_runs_schedule ON ingestion_job_runs(schedule_id);
CREATE INDEX idx_ingestion_job_runs_status ON ingestion_job_runs(status);
CREATE INDEX idx_ingestion_job_runs_created_at ON ingestion_job_runs(created_at DESC);
CREATE INDEX idx_ingestion_job_runs_claim ON ingestion_job_runs(status, claimed_at) WHERE status IN ('pending', 'claimed');

COMMENT ON TABLE ingestion_schedules IS 'Scheduled ingestion jobs with cron expressions';
COMMENT ON TABLE ingestion_job_runs IS 'Individual executions of ingestion jobs';
COMMENT ON COLUMN ingestion_schedules.config IS 'Plugin configuration (JSONB with encrypted sensitive fields)';
COMMENT ON COLUMN ingestion_job_runs.claimed_by IS 'Worker ID that claimed this job';
COMMENT ON COLUMN ingestion_job_runs.claimed_at IS 'When the job was claimed (for lease management)';

INSERT INTO permissions (name, description, resource_type, action) VALUES
('view_ingestion', 'View ingestion schedules and job runs', 'ingestion', 'view'),
('manage_ingestion', 'Create/update/delete ingestion schedules', 'ingestion', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name IN ('view_ingestion', 'manage_ingestion');

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'user'),
    id
FROM permissions
WHERE name = 'view_ingestion';

---- create above / drop below ----

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('view_ingestion', 'manage_ingestion')
);

DELETE FROM permissions WHERE name IN ('view_ingestion', 'manage_ingestion');

DROP TABLE IF EXISTS ingestion_job_runs;
DROP TABLE IF EXISTS ingestion_schedules;
