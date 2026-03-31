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
        '/discover/' || LOWER(type) || '/' ||
            CASE WHEN array_length(providers, 1) > 0 THEN providers[1] ELSE 'unknown' END ||
            '/' || COALESCE(SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), id),
        mrn,
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

-- Replace update trigger
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
        '/discover/' || LOWER(type) || '/' ||
            CASE WHEN array_length(providers, 1) > 0 THEN providers[1] ELSE 'unknown' END ||
            '/' || COALESCE(SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), id),
        mrn,
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

-- Backfill: fix existing rows with incorrect url_path
UPDATE search_index
SET url_path = '/discover/' || LOWER(asset_type) || '/' ||
    COALESCE(primary_provider, 'unknown') || '/' ||
    COALESCE(SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)'), entity_id)
WHERE type = 'asset'
  AND url_path NOT LIKE '/discover/%';

---- create above / drop below ----

-- Revert to the broken 000033 versions
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

    DELETE FROM asset_tags WHERE asset_id IN (SELECT id FROM deleted);

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
