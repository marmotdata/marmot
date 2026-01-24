CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_name VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    config JSONB,
    summary JSONB,
    created_by VARCHAR(255),
    UNIQUE(pipeline_name, source_name, run_id)
);

CREATE TABLE run_checkpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    entity_type VARCHAR(100) NOT NULL,
    entity_mrn VARCHAR(500) NOT NULL,
    operation VARCHAR(50) NOT NULL,
    source_fields TEXT[],
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, entity_type, entity_mrn)
);

CREATE TABLE run_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    entity_type VARCHAR(50) NOT NULL,
    entity_mrn TEXT NOT NULL,
    entity_name TEXT,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, entity_type, entity_mrn)
);

CREATE INDEX idx_runs_pipeline_source ON runs(pipeline_name, source_name);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_started_at ON runs(started_at DESC);
CREATE INDEX idx_run_checkpoints_run_entity ON run_checkpoints(run_id, entity_type, entity_mrn);
CREATE INDEX idx_run_checkpoints_run_type ON run_checkpoints(run_id, entity_type);
CREATE INDEX idx_run_checkpoints_entity_mrn ON run_checkpoints(entity_mrn);
CREATE INDEX idx_run_entities_run_id ON run_entities(run_id);
CREATE INDEX idx_run_entities_type_status ON run_entities(entity_type, status);
CREATE INDEX idx_run_entities_created_at ON run_entities(created_at DESC);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_run_entities_created_at;
DROP INDEX IF EXISTS idx_run_entities_type_status;
DROP INDEX IF EXISTS idx_run_entities_run_id;
DROP INDEX IF EXISTS idx_run_checkpoints_entity_mrn;
DROP INDEX IF EXISTS idx_run_checkpoints_run_type;
DROP INDEX IF EXISTS idx_run_checkpoints_run_entity;
DROP INDEX IF EXISTS idx_runs_started_at;
DROP INDEX IF EXISTS idx_runs_status;
DROP INDEX IF EXISTS idx_runs_pipeline_source;

DROP TABLE IF EXISTS run_entities;
DROP TABLE IF EXISTS run_checkpoints;
DROP TABLE IF EXISTS runs;
