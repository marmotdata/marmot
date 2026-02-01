---- tern: disable-tx ----

-- Asset tags junction table
CREATE TABLE IF NOT EXISTS asset_tags (
    asset_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    PRIMARY KEY (asset_id, tag)
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_tags_tag_prefix
ON asset_tags (tag text_pattern_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_tags_tag
ON asset_tags (tag);

-- Counter cache for summary queries
CREATE TABLE IF NOT EXISTS summary_counts (
    dimension TEXT NOT NULL,
    key TEXT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_summary_counts_dimension
ON summary_counts (dimension);

-- Browse query index
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_updated_at_browse
ON search_index (updated_at DESC);

-- Statement-level asset trigger for insert
CREATE OR REPLACE FUNCTION sync_asset_search_on_insert()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path, mrn,
        created_by, created_at, metadata
    )
    SELECT
        'asset', id, name, COALESCE(user_description, description),
        search_text, updated_at,
        type, providers[1], providers, tags,
        '/assets/' || id, mrn,
        created_by, created_at, metadata
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name, description = EXCLUDED.description,
        search_text = EXCLUDED.search_text, updated_at = EXCLUDED.updated_at,
        asset_type = EXCLUDED.asset_type, primary_provider = EXCLUDED.primary_provider,
        providers = EXCLUDED.providers, tags = EXCLUDED.tags, url_path = EXCLUDED.url_path,
        mrn = EXCLUDED.mrn,
        created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    INSERT INTO asset_tags (asset_id, tag)
    SELECT id, unnest(tags)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    ON CONFLICT DO NOTHING;

    -- entity_type count
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- type dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- provider dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    -- tag dimension
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Statement-level asset trigger for update
CREATE OR REPLACE FUNCTION sync_asset_search_on_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Decrement old counts
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    DELETE FROM asset_tags WHERE asset_id IN (SELECT id FROM deleted);

    -- Insert new data
    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path, mrn,
        created_by, created_at, metadata
    )
    SELECT
        'asset', id, name, COALESCE(user_description, description),
        search_text, updated_at,
        type, providers[1], providers, tags,
        '/assets/' || id, mrn,
        created_by, created_at, metadata
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (type, entity_id) DO UPDATE SET
        name = EXCLUDED.name, description = EXCLUDED.description,
        search_text = EXCLUDED.search_text, updated_at = EXCLUDED.updated_at,
        asset_type = EXCLUDED.asset_type, primary_provider = EXCLUDED.primary_provider,
        providers = EXCLUDED.providers, tags = EXCLUDED.tags, url_path = EXCLUDED.url_path,
        mrn = EXCLUDED.mrn,
        created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at,
        metadata = EXCLUDED.metadata;

    INSERT INTO asset_tags (asset_id, tag)
    SELECT id, unnest(tags)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    ON CONFLICT DO NOTHING;

    -- Increment new counts
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), COUNT(*)
    FROM inserted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    DELETE FROM search_index
    WHERE type = 'asset' AND entity_id IN (SELECT id FROM inserted WHERE is_stub = TRUE);

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Statement-level asset trigger for delete
CREATE OR REPLACE FUNCTION sync_asset_search_on_delete()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'entity_type', 'asset', -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'type', type, -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND type IS NOT NULL
    GROUP BY type
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'provider', unnest(providers), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND providers IS NOT NULL AND array_length(providers, 1) > 0
    GROUP BY unnest(providers)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    INSERT INTO summary_counts (dimension, key, count)
    SELECT 'tag', unnest(tags), -COUNT(*)
    FROM deleted
    WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
    GROUP BY unnest(tags)
    ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;

    DELETE FROM search_index WHERE type = 'asset' AND entity_id IN (SELECT id FROM deleted);
    DELETE FROM asset_tags WHERE asset_id IN (SELECT id FROM deleted);

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Glossary trigger with entity_type tracking
CREATE OR REPLACE FUNCTION search_index_glossary_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'glossary' AND entity_id = OLD.id::text;
        UPDATE summary_counts SET count = count - 1 WHERE dimension = 'entity_type' AND key = 'glossary';
        RETURN OLD;
    END IF;

    IF NEW.deleted_at IS NOT NULL THEN
        DELETE FROM search_index WHERE type = 'glossary' AND entity_id = NEW.id::text;
        IF TG_OP = 'UPDATE' AND OLD.deleted_at IS NULL THEN
            UPDATE summary_counts SET count = count - 1 WHERE dimension = 'entity_type' AND key = 'glossary';
        END IF;
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

    IF TG_OP = 'INSERT' OR (TG_OP = 'UPDATE' AND OLD.deleted_at IS NOT NULL) THEN
        INSERT INTO summary_counts (dimension, key, count)
        VALUES ('entity_type', 'glossary', 1)
        ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + 1;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Team trigger with entity_type tracking
CREATE OR REPLACE FUNCTION search_index_team_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'team' AND entity_id = OLD.id::text;
        UPDATE summary_counts SET count = count - 1 WHERE dimension = 'entity_type' AND key = 'team';
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

    IF TG_OP = 'INSERT' THEN
        INSERT INTO summary_counts (dimension, key, count)
        VALUES ('entity_type', 'team', 1)
        ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + 1;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Data product trigger with entity_type tracking
CREATE OR REPLACE FUNCTION search_index_data_product_trigger()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM search_index WHERE type = 'data_product' AND entity_id = OLD.id::text;
        UPDATE summary_counts SET count = count - 1 WHERE dimension = 'entity_type' AND key = 'data_product';
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

    IF TG_OP = 'INSERT' THEN
        INSERT INTO summary_counts (dimension, key, count)
        VALUES ('entity_type', 'data_product', 1)
        ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + 1;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Replace row-level triggers with statement-level for assets
DROP TRIGGER IF EXISTS search_index_asset_sync ON assets;
DROP TRIGGER IF EXISTS search_index_asset_sync_update ON assets;
DROP TRIGGER IF EXISTS search_index_asset_sync_delete ON assets;

CREATE TRIGGER search_index_asset_sync
AFTER INSERT ON assets
REFERENCING NEW TABLE AS inserted
FOR EACH STATEMENT EXECUTE FUNCTION sync_asset_search_on_insert();

CREATE TRIGGER search_index_asset_sync_update
AFTER UPDATE ON assets
REFERENCING OLD TABLE AS deleted NEW TABLE AS inserted
FOR EACH STATEMENT EXECUTE FUNCTION sync_asset_search_on_update();

CREATE TRIGGER search_index_asset_sync_delete
AFTER DELETE ON assets
REFERENCING OLD TABLE AS deleted
FOR EACH STATEMENT EXECUTE FUNCTION sync_asset_search_on_delete();

-- Backfill asset_tags
INSERT INTO asset_tags (asset_id, tag)
SELECT entity_id, unnest(tags)
FROM search_index
WHERE type = 'asset' AND tags IS NOT NULL AND array_length(tags, 1) > 0
ON CONFLICT DO NOTHING;

-- Backfill summary_counts
TRUNCATE summary_counts;

INSERT INTO summary_counts (dimension, key, count)
SELECT 'entity_type', type, COUNT(*)
FROM search_index
GROUP BY type;

INSERT INTO summary_counts (dimension, key, count)
SELECT 'type', asset_type, COUNT(*)
FROM search_index
WHERE type = 'asset' AND asset_type IS NOT NULL
GROUP BY asset_type;

INSERT INTO summary_counts (dimension, key, count)
SELECT 'provider', primary_provider, COUNT(*)
FROM search_index
WHERE type = 'asset' AND primary_provider IS NOT NULL
GROUP BY primary_provider;

INSERT INTO summary_counts (dimension, key, count)
SELECT 'tag', tag, COUNT(*)
FROM asset_tags
GROUP BY tag;

ANALYZE asset_tags;
ANALYZE summary_counts;
ANALYZE search_index;

---- create above / drop below ----

DROP TRIGGER IF EXISTS search_index_asset_sync ON assets;
DROP TRIGGER IF EXISTS search_index_asset_sync_update ON assets;
DROP TRIGGER IF EXISTS search_index_asset_sync_delete ON assets;

DROP FUNCTION IF EXISTS sync_asset_search_on_insert();
DROP FUNCTION IF EXISTS sync_asset_search_on_update();
DROP FUNCTION IF EXISTS sync_asset_search_on_delete();

-- Restore original row-level trigger
CREATE TRIGGER search_index_asset_sync
AFTER INSERT OR UPDATE OR DELETE ON assets
FOR EACH ROW EXECUTE FUNCTION search_index_asset_trigger();

-- Restore original glossary trigger
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

-- Restore original team trigger
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

-- Restore original data product trigger
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

DROP INDEX CONCURRENTLY IF EXISTS idx_search_index_updated_at_browse;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_tags_tag_prefix;
DROP INDEX CONCURRENTLY IF EXISTS idx_asset_tags_tag;
DROP INDEX CONCURRENTLY IF EXISTS idx_summary_counts_dimension;
DROP TABLE IF EXISTS asset_tags;
DROP TABLE IF EXISTS summary_counts;
