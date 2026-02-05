---- tern: disable-tx ----

-- Unified search index maintained by triggers
CREATE TABLE IF NOT EXISTS search_index (
    type TEXT NOT NULL CHECK (type IN ('asset', 'glossary', 'team', 'data_product')),
    entity_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    search_text tsvector NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    asset_type TEXT,
    primary_provider TEXT,
    providers TEXT[],
    tags TEXT[] DEFAULT '{}',
    url_path TEXT NOT NULL,
    mrn TEXT,
    created_by TEXT,
    created_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (type, entity_id)
);

-- Full-text search
CREATE INDEX IF NOT EXISTS idx_search_index_fts ON search_index USING GIN(search_text);

-- Trigram similarity for fuzzy matching 
CREATE INDEX IF NOT EXISTS idx_search_index_name_trgm ON search_index USING gin(name gin_trgm_ops);

-- Prefix matching with covering columns
CREATE INDEX IF NOT EXISTS idx_search_index_name_prefix ON search_index (type, lower(name) text_pattern_ops)
    INCLUDE (entity_id, description, url_path, updated_at, asset_type, primary_provider);

-- Type + recency for listing
CREATE INDEX IF NOT EXISTS idx_search_index_type_updated ON search_index (type, updated_at DESC)
    INCLUDE (entity_id, name, asset_type, providers);

-- Array containment queries
CREATE INDEX IF NOT EXISTS idx_search_index_tags ON search_index USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_search_index_providers ON search_index USING GIN(providers) WHERE type = 'asset';

-- JSONB containment queries
CREATE INDEX IF NOT EXISTS idx_search_index_metadata ON search_index USING GIN(metadata jsonb_path_ops);

-- Asset type filtering with sort
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_asset_type_updated
    ON search_index (lower(asset_type), updated_at DESC)
    WHERE type = 'asset';

-- Provider filtering with sort
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_provider_updated
    ON search_index (primary_provider, updated_at DESC)
    WHERE type = 'asset' AND primary_provider IS NOT NULL;

-- Case-insensitive MRN lookups
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_mrn_lower ON assets (LOWER(mrn))
    WHERE is_stub = FALSE;

-- Asset trigger
CREATE OR REPLACE FUNCTION search_index_asset_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'asset' AND entity_id = OLD.id;
        RETURN OLD;
    END IF;

    IF NEW.is_stub = TRUE THEN
        DELETE FROM search_index WHERE type = 'asset' AND entity_id = NEW.id;
        RETURN NEW;
    END IF;

    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path,
        mrn, created_by, created_at, metadata
    ) VALUES (
        'asset',
        NEW.id,
        NEW.name,
        NEW.description,
        COALESCE(NEW.search_text, to_tsvector('english', COALESCE(NEW.name, ''))),
        NEW.updated_at,
        NEW.type,
        CASE WHEN array_length(NEW.providers, 1) > 0 THEN NEW.providers[1] ELSE NULL END,
        NEW.providers,
        COALESCE(NEW.tags, '{}'),
        '/discover/' || NEW.type || '/' ||
            CASE WHEN array_length(NEW.providers, 1) > 0 THEN NEW.providers[1] ELSE 'unknown' END ||
            '/' || COALESCE(SUBSTRING(NEW.mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), NEW.id),
        NEW.mrn,
        NEW.created_by,
        NEW.created_at,
        COALESCE(NEW.metadata, '{}'::jsonb)
    )
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        search_text = EXCLUDED.search_text,
        updated_at = EXCLUDED.updated_at,
        asset_type = EXCLUDED.asset_type,
        primary_provider = EXCLUDED.primary_provider,
        providers = EXCLUDED.providers,
        tags = EXCLUDED.tags,
        url_path = EXCLUDED.url_path,
        mrn = EXCLUDED.mrn,
        created_by = EXCLUDED.created_by,
        created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Glossary trigger
CREATE OR REPLACE FUNCTION search_index_glossary_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'glossary' AND entity_id = OLD.id::text;
        RETURN OLD;
    END IF;

    IF NEW.deleted_at IS NOT NULL THEN
        DELETE FROM search_index WHERE type = 'glossary' AND entity_id = NEW.id::text;
        RETURN NEW;
    END IF;

    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path,
        created_at, metadata
    ) VALUES (
        'glossary',
        NEW.id::text,
        NEW.name,
        COALESCE(NEW.definition, NEW.description),
        COALESCE(NEW.search_text, to_tsvector('english', COALESCE(NEW.name, ''))),
        NEW.updated_at,
        NULL, NULL, NULL,
        COALESCE(NEW.tags, '{}'),
        '/glossary/' || NEW.id::text,
        NEW.created_at,
        COALESCE(NEW.metadata, '{}'::jsonb)
    )
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        search_text = EXCLUDED.search_text,
        updated_at = EXCLUDED.updated_at,
        tags = EXCLUDED.tags,
        url_path = EXCLUDED.url_path,
        created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Team trigger
CREATE OR REPLACE FUNCTION search_index_team_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'team' AND entity_id = OLD.id::text;
        RETURN OLD;
    END IF;

    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path,
        created_by, created_at, metadata
    ) VALUES (
        'team',
        NEW.id::text,
        NEW.name,
        NEW.description,
        COALESCE(NEW.search_text, to_tsvector('english', COALESCE(NEW.name, ''))),
        NEW.updated_at,
        NULL, NULL, NULL,
        COALESCE(NEW.tags, '{}'),
        '/teams/' || NEW.id::text,
        NEW.created_by::text,
        NEW.created_at,
        COALESCE(NEW.metadata, '{}'::jsonb)
    )
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        search_text = EXCLUDED.search_text,
        updated_at = EXCLUDED.updated_at,
        tags = EXCLUDED.tags,
        url_path = EXCLUDED.url_path,
        created_by = EXCLUDED.created_by,
        created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Data product trigger
CREATE OR REPLACE FUNCTION search_index_data_product_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'data_product' AND entity_id = OLD.id::text;
        RETURN OLD;
    END IF;

    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path,
        created_by, created_at, metadata
    ) VALUES (
        'data_product',
        NEW.id::text,
        NEW.name,
        NEW.description,
        COALESCE(NEW.search_text, to_tsvector('english', COALESCE(NEW.name, ''))),
        NEW.updated_at,
        NULL, NULL, NULL,
        COALESCE(NEW.tags, '{}'),
        '/products/' || NEW.id::text,
        NEW.created_by::text,
        NEW.created_at,
        COALESCE(NEW.metadata, '{}'::jsonb)
    )
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        search_text = EXCLUDED.search_text,
        updated_at = EXCLUDED.updated_at,
        tags = EXCLUDED.tags,
        url_path = EXCLUDED.url_path,
        created_by = EXCLUDED.created_by,
        created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS search_index_asset_sync ON assets;
DROP TRIGGER IF EXISTS search_index_glossary_sync ON glossary_terms;
DROP TRIGGER IF EXISTS search_index_team_sync ON teams;
DROP TRIGGER IF EXISTS search_index_data_product_sync ON data_products;

CREATE TRIGGER search_index_asset_sync
    AFTER INSERT OR UPDATE OR DELETE ON assets
    FOR EACH ROW EXECUTE FUNCTION search_index_asset_trigger();

CREATE TRIGGER search_index_glossary_sync
    AFTER INSERT OR UPDATE OR DELETE ON glossary_terms
    FOR EACH ROW EXECUTE FUNCTION search_index_glossary_trigger();

CREATE TRIGGER search_index_team_sync
    AFTER INSERT OR UPDATE OR DELETE ON teams
    FOR EACH ROW EXECUTE FUNCTION search_index_team_trigger();

CREATE TRIGGER search_index_data_product_sync
    AFTER INSERT OR UPDATE OR DELETE ON data_products
    FOR EACH ROW EXECUTE FUNCTION search_index_data_product_trigger();

-- Backfill existing data
INSERT INTO search_index (type, entity_id, name, description, search_text, updated_at,
                          asset_type, primary_provider, providers, tags, url_path,
                          mrn, created_by, created_at, metadata)
SELECT
    'asset', id, name, description,
    COALESCE(search_text, to_tsvector('english', COALESCE(name, ''))),
    updated_at, type,
    CASE WHEN array_length(providers, 1) > 0 THEN providers[1] ELSE NULL END,
    providers, COALESCE(tags, '{}'),
    '/discover/' || type || '/' ||
        CASE WHEN array_length(providers, 1) > 0 THEN providers[1] ELSE 'unknown' END ||
        '/' || COALESCE(SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), id),
    mrn, created_by, created_at, COALESCE(metadata, '{}'::jsonb)
FROM assets WHERE is_stub = FALSE
ON CONFLICT DO NOTHING;

INSERT INTO search_index (type, entity_id, name, description, search_text, updated_at,
                          asset_type, primary_provider, providers, tags, url_path, created_at, metadata)
SELECT
    'glossary', id::text, name, COALESCE(definition, description),
    COALESCE(search_text, to_tsvector('english', COALESCE(name, ''))),
    updated_at, NULL, NULL, NULL, COALESCE(tags, '{}'), '/glossary/' || id::text,
    created_at, COALESCE(metadata, '{}'::jsonb)
FROM glossary_terms WHERE deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO search_index (type, entity_id, name, description, search_text, updated_at,
                          asset_type, primary_provider, providers, tags, url_path,
                          created_by, created_at, metadata)
SELECT
    'team', id::text, name, description,
    COALESCE(search_text, to_tsvector('english', COALESCE(name, ''))),
    updated_at, NULL, NULL, NULL, COALESCE(tags, '{}'), '/teams/' || id::text,
    created_by::text, created_at, COALESCE(metadata, '{}'::jsonb)
FROM teams
ON CONFLICT DO NOTHING;

INSERT INTO search_index (type, entity_id, name, description, search_text, updated_at,
                          asset_type, primary_provider, providers, tags, url_path,
                          created_by, created_at, metadata)
SELECT
    'data_product', id::text, name, description,
    COALESCE(search_text, to_tsvector('english', COALESCE(name, ''))),
    updated_at, NULL, NULL, NULL, COALESCE(tags, '{}'), '/products/' || id::text,
    created_by::text, created_at, COALESCE(metadata, '{}'::jsonb)
FROM data_products
ON CONFLICT DO NOTHING;

-- Fix any incorrect url_path values for assets (e.g., /assets/{id} instead of /discover/...)
UPDATE search_index
SET url_path = '/discover/' || asset_type || '/' ||
    COALESCE(primary_provider, 'unknown') || '/' ||
    COALESCE(SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), entity_id)
WHERE type = 'asset'
  AND (url_path LIKE '/assets/%' OR url_path NOT LIKE '/discover/%');

ANALYZE search_index;

-- Remove old trigram indexes (search_index handles search now)
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_mrn_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_combined_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_mrn_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_combined_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_name_gin_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_glossary_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_teams_name_trgm_gist;
DROP INDEX CONCURRENTLY IF EXISTS idx_data_products_name_trgm_gist;

-- Additional indexes for source tables
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_type_updated
ON assets(type, updated_at DESC) WHERE is_stub = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_list_covering
ON assets(updated_at DESC)
INCLUDE (id, name, mrn, type, providers)
WHERE is_stub = FALSE;

DROP INDEX CONCURRENTLY IF EXISTS idx_assets_is_stub;

-- Autovacuum tuning for high-churn tables
ALTER TABLE assets SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 2
);

ALTER TABLE search_index SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 2,
    fillfactor = 90
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_user_unread_created
ON notifications(user_id, created_at DESC) WHERE read = FALSE;

DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_user_id_created_at_desc;

ALTER TABLE notifications SET (
    autovacuum_vacuum_scale_factor = 0.02,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_vacuum_cost_delay = 2,
    fillfactor = 85
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_run_history_asset_event_time
ON run_history(asset_id, event_time DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_run_history_asset_run_event
ON run_history(asset_id, run_id, event_type, event_time DESC);

ALTER TABLE run_history SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);

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

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_runs_pipeline_source_started
ON runs(pipeline_name, source_name, started_at DESC);

DROP INDEX CONCURRENTLY IF EXISTS idx_runs_pipeline_source;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lineage_edges_job_mrn
ON lineage_edges(job_mrn) WHERE job_mrn IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_data_products_updated_at
ON data_products(updated_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_glossary_terms_name_active
ON glossary_terms(name) WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_subscriptions_asset_user
ON asset_subscriptions(asset_id, user_id);

---- create above / drop below ----

DROP TRIGGER IF EXISTS search_index_asset_sync ON assets;
DROP TRIGGER IF EXISTS search_index_glossary_sync ON glossary_terms;
DROP TRIGGER IF EXISTS search_index_team_sync ON teams;
DROP TRIGGER IF EXISTS search_index_data_product_sync ON data_products;

DROP FUNCTION IF EXISTS search_index_asset_trigger();
DROP FUNCTION IF EXISTS search_index_glossary_trigger();
DROP FUNCTION IF EXISTS search_index_team_trigger();
DROP FUNCTION IF EXISTS search_index_data_product_trigger();

DROP INDEX IF EXISTS idx_search_index_fts;
DROP INDEX IF EXISTS idx_search_index_name_trgm;
DROP INDEX IF EXISTS idx_search_index_name_prefix;
DROP INDEX IF EXISTS idx_search_index_type_updated;
DROP INDEX IF EXISTS idx_search_index_providers;
DROP INDEX IF EXISTS idx_search_index_tags;
DROP INDEX IF EXISTS idx_search_index_metadata;
DROP TABLE IF EXISTS search_index;

DROP INDEX CONCURRENTLY IF EXISTS idx_assets_type_updated;
DROP INDEX CONCURRENTLY IF EXISTS idx_assets_list_covering;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_is_stub ON assets(is_stub);
ALTER TABLE assets RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor,
    autovacuum_vacuum_cost_delay
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_name_trgm ON assets USING gin(name gin_trgm_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_mrn_trgm ON assets USING gin(mrn gin_trgm_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_combined_trgm
ON assets USING gin((COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, '')) gin_trgm_ops)
WHERE is_stub = FALSE;

DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_user_unread_created;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_user_id_created_at_desc
ON notifications(user_id, created_at DESC);
ALTER TABLE notifications RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor,
    autovacuum_vacuum_cost_delay,
    fillfactor
);

DROP INDEX CONCURRENTLY IF EXISTS idx_run_history_asset_event_time;
DROP INDEX CONCURRENTLY IF EXISTS idx_run_history_asset_run_event;
ALTER TABLE run_history RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_analyze_scale_factor
);

DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_user_asset;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_owners_team_asset;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_user ON asset_owners(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_owners_team ON asset_owners(team_id) WHERE team_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_terms_asset_term;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_terms_term_asset;
DROP INDEX CONCURRENTLY IF EXISTS idx_team_members_user_team;

DROP INDEX CONCURRENTLY IF EXISTS idx_runs_pipeline_source_started;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_runs_pipeline_source ON runs(pipeline_name, source_name);
DROP INDEX CONCURRENTLY IF EXISTS idx_lineage_edges_job_mrn;
DROP INDEX CONCURRENTLY IF EXISTS idx_data_products_updated_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_glossary_terms_name_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_subscriptions_asset_user;
