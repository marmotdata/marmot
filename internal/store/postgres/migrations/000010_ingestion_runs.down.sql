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
