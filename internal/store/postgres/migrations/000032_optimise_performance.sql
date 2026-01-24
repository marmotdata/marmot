---- tern: disable-tx ----
-- Assets: composite and covering indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_type_updated
ON assets(type, updated_at DESC) WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_list_covering
ON assets(updated_at DESC)
INCLUDE (id, name, mrn, type, providers)
WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_type_name_lower
ON assets(LOWER(type), LOWER(name)) WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_mrn_lower
ON assets(LOWER(mrn)) WHERE is_stub = FALSE;

DROP INDEX CONCURRENTLY IF EXISTS idx_assets_is_stub;

ALTER TABLE assets ALTER COLUMN type SET STATISTICS 500;
ALTER TABLE assets ALTER COLUMN providers SET STATISTICS 500;
ALTER TABLE assets SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 2,
    parallel_workers = 4
);

-- Search: replace GIN trigram indexes with GiST (supports word_similarity ordering)
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_mrn_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_combined_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_name_trgm_gist
ON assets USING gist(name gist_trgm_ops(siglen=256))
WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_mrn_trgm_gist
ON assets USING gist(mrn gist_trgm_ops(siglen=256))
WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_combined_trgm_gist
ON assets USING gist((COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, '')) gist_trgm_ops(siglen=256))
WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_name_gin_trgm
ON assets USING gin(name gin_trgm_ops)
WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_glossary_name_trgm_gist
ON glossary_terms USING gist(name gist_trgm_ops(siglen=128))
WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_teams_name_trgm_gist
ON teams USING gist(name gist_trgm_ops(siglen=128));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_data_products_name_trgm_gist
ON data_products USING gist(name gist_trgm_ops(siglen=128));

-- Notifications: partial index for unread fetches, autovacuum tuning
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_user_unread_created
ON notifications(user_id, created_at DESC) WHERE read = FALSE;

DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_user_id_created_at_desc;

ALTER TABLE notifications SET (
    autovacuum_vacuum_scale_factor = 0.02,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_vacuum_cost_delay = 2,
    fillfactor = 85
);

-- Run history: composite indexes for correlated subqueries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_run_history_asset_event_time
ON run_history(asset_id, event_time DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_run_history_asset_run_event
ON run_history(asset_id, run_id, event_type, event_time DESC);

ALTER TABLE run_history SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    parallel_workers = 2
);
ALTER TABLE run_history ALTER COLUMN asset_id SET STATISTICS 500;
ALTER TABLE run_history ALTER COLUMN run_id SET STATISTICS 500;

-- Ownership: composite indexes for GetMyAssets joins
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_user_asset
ON asset_owners(user_id, asset_id) WHERE user_id IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_team_asset
ON asset_owners(team_id, asset_id) WHERE team_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_user;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_team;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_terms_asset_term
ON asset_terms(asset_id, glossary_term_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_terms_term_asset
ON asset_terms(glossary_term_id, asset_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_team_members_user_team
ON team_members(user_id, team_id);

-- Misc: runs, lineage, data products, glossary, subscriptions
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_runs_pipeline_source_started
ON runs(pipeline_name, source_name, started_at DESC);

DROP INDEX CONCURRENTLY IF EXISTS idx_runs_pipeline_source;

ALTER TABLE run_checkpoints ALTER COLUMN entity_mrn SET STATISTICS 500;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lineage_edges_job_mrn
ON lineage_edges(job_mrn) WHERE job_mrn IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_data_products_updated_at
ON data_products(updated_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_glossary_terms_name_active
ON glossary_terms(name) WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_subscriptions_asset_user
ON asset_subscriptions(asset_id, user_id);

---- create above / drop below ----

-- Revert assets indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_type_updated;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_list_covering;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_type_name_lower;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_mrn_lower;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_is_stub ON assets(is_stub);
ALTER TABLE assets ALTER COLUMN type SET STATISTICS DEFAULT;
ALTER TABLE assets ALTER COLUMN providers SET STATISTICS DEFAULT;
ALTER TABLE assets RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor,
    autovacuum_vacuum_cost_delay,
    parallel_workers
);

-- Revert search indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_mrn_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_combined_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_gin_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_glossary_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_teams_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_data_products_name_trgm_gist;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_name_trgm ON assets USING gin(name gin_trgm_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_mrn_trgm ON assets USING gin(mrn gin_trgm_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_combined_trgm
ON assets USING gin((COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, '')) gin_trgm_ops)
WHERE is_stub = FALSE;

-- Revert notifications
DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_user_unread_created;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_user_id_created_at_desc
ON notifications(user_id, created_at DESC);
ALTER TABLE notifications RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor,
    autovacuum_vacuum_cost_delay,
    fillfactor
);

-- Revert run history
DROP INDEX CONCURRENTLY IF EXISTS idx_run_history_asset_event_time;
DROP INDEX CONCURRENTLY IF EXISTS idx_run_history_asset_run_event;
ALTER TABLE run_history RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor,
    parallel_workers
);
ALTER TABLE run_history ALTER COLUMN asset_id SET STATISTICS DEFAULT;
ALTER TABLE run_history ALTER COLUMN run_id SET STATISTICS DEFAULT;

-- Revert ownership indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_user_asset;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_team_asset;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_user ON asset_owners(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_team ON asset_owners(team_id) WHERE team_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_terms_asset_term;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_terms_term_asset;
DROP INDEX CONCURRENTLY IF EXISTS idx_team_members_user_team;

-- Revert misc
DROP INDEX CONCURRENTLY IF EXISTS idx_runs_pipeline_source_started;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_runs_pipeline_source ON runs(pipeline_name, source_name);
ALTER TABLE run_checkpoints ALTER COLUMN entity_mrn SET STATISTICS DEFAULT;
DROP INDEX CONCURRENTLY IF EXISTS idx_lineage_edges_job_mrn;
DROP INDEX CONCURRENTLY IF EXISTS idx_data_products_updated_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_glossary_terms_name_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_subscriptions_asset_user;
