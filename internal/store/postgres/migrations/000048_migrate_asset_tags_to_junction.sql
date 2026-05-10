-- Consolidate tag system: drop legacy asset_tags table, backfill tag data,
-- replace triggers to use assets_tags junction, and drop the deprecated tags column from assets.

DROP TABLE IF EXISTS asset_tags;


-- extract unique tags from assets and insert into tags table
INSERT INTO tags (name, description)
SELECT DISTINCT tag_val, NULL
FROM assets, LATERAL unnest(assets.tags) AS tag_val
WHERE array_length(assets.tags, 1) > 0
ON CONFLICT (name) DO NOTHING;

-- map existing asset-tag relationships into the new assets_tags junction table
INSERT INTO assets_tags (asset_id, tag_id)
SELECT a.id, t.id
FROM assets a
CROSS JOIN LATERAL unnest(a.tags) AS tag_val
JOIN tags t ON t.name = tag_val
WHERE array_length(a.tags, 1) > 0
ON CONFLICT DO NOTHING;

-- Statement-level asset trigger for insert (rewritten to use assets_tags junction table)
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
        type, providers[1], providers,
        ARRAY(SELECT t.name FROM tags t JOIN assets_tags at ON t.id = at.tag_id WHERE at.asset_id = inserted.id),
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

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Statement-level asset trigger for update (rewritten to use assets_tags junction table)
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

    -- Insert new data
    INSERT INTO search_index (
        type, entity_id, name, description, search_text, updated_at,
        asset_type, primary_provider, providers, tags, url_path, mrn,
        created_by, created_at, metadata
    )
    SELECT
        'asset', id, name, COALESCE(user_description, description),
        search_text, updated_at,
        type, providers[1], providers,
        ARRAY(SELECT t.name FROM tags t JOIN assets_tags at ON t.id = at.tag_id WHERE at.asset_id = inserted.id),
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

    DELETE FROM search_index
    WHERE type = 'asset' AND entity_id IN (SELECT id FROM inserted WHERE is_stub = TRUE);

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Statement-level asset trigger for delete (rewritten to use assets_tags junction table)
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

    DELETE FROM search_index WHERE type = 'asset' AND entity_id IN (SELECT id FROM deleted);

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Row-level trigger on assets_tags to keep search_index in sync when tags change
CREATE OR REPLACE FUNCTION sync_search_index_on_asset_tag_change()
RETURNS TRIGGER AS $$
DECLARE
    affected_asset_id VARCHAR(255);
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_asset_id := OLD.asset_id;
    ELSE
        affected_asset_id := NEW.asset_id;
    END IF;

    UPDATE search_index
    SET tags = ARRAY(
        SELECT t.name FROM tags t
        JOIN assets_tags at ON t.id = at.tag_id
        WHERE at.asset_id = affected_asset_id
    )
    WHERE type = 'asset' AND entity_id = affected_asset_id;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sync_search_on_asset_tag_change
    AFTER INSERT OR DELETE ON assets_tags
    FOR EACH ROW EXECUTE FUNCTION sync_search_index_on_asset_tag_change();

-- Clear stale summary_counts tag rows before backfill from new junction table
DELETE FROM summary_counts WHERE dimension = 'tag';

-- Backfill summary_counts.tag from assets_tags junction
INSERT INTO summary_counts (dimension, key, count)
SELECT 'tag', t.name, COUNT(at.asset_id)
FROM assets_tags at
JOIN tags t ON t.id = at.tag_id
GROUP BY t.name
ON CONFLICT (dimension, key) DO UPDATE SET count = EXCLUDED.count;

-- BEFORE DELETE on tags: remove summary_counts row before cascade fires on assets_tags
CREATE OR REPLACE FUNCTION cleanup_tag_summary_counts_on_tag_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM summary_counts WHERE dimension = 'tag' AND key = OLD.name;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER cleanup_tag_summary_counts_on_tag_delete
    BEFORE DELETE ON tags
    FOR EACH ROW EXECUTE FUNCTION cleanup_tag_summary_counts_on_tag_delete();

-- Maintain summary_counts.tag when assets_tags rows are inserted or deleted
CREATE OR REPLACE FUNCTION sync_tag_summary_counts_on_asset_tag_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO summary_counts (dimension, key, count)
        SELECT 'tag', t.name, 1
        FROM tags t WHERE t.id = NEW.tag_id
        ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + 1;
    ELSIF TG_OP = 'DELETE' THEN
        -- No-op when the parent tag row is already gone (cascade from tags delete)
        INSERT INTO summary_counts (dimension, key, count)
        SELECT 'tag', t.name, -1
        FROM tags t WHERE t.id = OLD.tag_id
        ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count - 1;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sync_tag_summary_counts_on_asset_tag_change
    AFTER INSERT OR DELETE ON assets_tags
    FOR EACH ROW EXECUTE FUNCTION sync_tag_summary_counts_on_asset_tag_change();

-- Backfill search_index.tags for existing assets
UPDATE search_index si
SET tags = ARRAY(
    SELECT t.name FROM tags t
    JOIN assets_tags at ON t.id = at.tag_id
    WHERE at.asset_id = si.entity_id
)
WHERE si.type = 'asset';

-- Drop the deprecated tags column from assets
ALTER TABLE assets DROP COLUMN tags;

---- create above / drop below ----

-- Drop summary_counts tag triggers
DROP TRIGGER IF EXISTS cleanup_tag_summary_counts_on_tag_delete ON tags;
DROP FUNCTION IF EXISTS cleanup_tag_summary_counts_on_tag_delete();
DROP TRIGGER IF EXISTS sync_tag_summary_counts_on_asset_tag_change ON assets_tags;
DROP FUNCTION IF EXISTS sync_tag_summary_counts_on_asset_tag_change();

-- Restore the tags column
ALTER TABLE assets ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

-- Restore the GIN index (auto-dropped with column)
CREATE INDEX IF NOT EXISTS idx_assets_tags ON assets USING gin (tags);

-- Re-populate assets.tags from the assets_tags junction (reverse of UP-path migration)
UPDATE assets a
SET tags = ARRAY(
    SELECT t.name FROM tags t
    JOIN assets_tags at ON t.id = at.tag_id
    WHERE at.asset_id = a.id
    ORDER BY t.name
)
WHERE EXISTS (
    SELECT 1 FROM assets_tags at2 WHERE at2.asset_id = a.id
);

-- Revert triggers to read from assets.tags instead of assets_tags
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

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION sync_asset_search_on_update()
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

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS sync_search_on_asset_tag_change ON assets_tags;
DROP FUNCTION IF EXISTS sync_search_index_on_asset_tag_change();

-- Restore summary_counts.tag rows
INSERT INTO summary_counts (dimension, key, count)
SELECT 'tag', unnest(tags), COUNT(*)
FROM assets
WHERE is_stub = FALSE AND tags IS NOT NULL AND array_length(tags, 1) > 0
GROUP BY unnest(tags)
ON CONFLICT (dimension, key) DO UPDATE SET count = summary_counts.count + EXCLUDED.count;
