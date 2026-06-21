-- Migrate data products and glossary terms from free-text tag arrays to junction tables

-- Create junction tables
CREATE TABLE IF NOT EXISTS data_product_tags (
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (data_product_id, tag_id)
);

CREATE TABLE IF NOT EXISTS glossary_term_tags (
    glossary_term_id UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (glossary_term_id, tag_id)
);

-- Extract unique tags from data_products into tags table
INSERT INTO tags (name, description)
SELECT DISTINCT tag_val, NULL
FROM data_products, LATERAL unnest(data_products.tags) AS tag_val
WHERE array_length(data_products.tags, 1) > 0
ON CONFLICT (name) DO NOTHING;

-- Backfill data_product_tags junction table
INSERT INTO data_product_tags (data_product_id, tag_id)
SELECT dp.id, t.id
FROM data_products dp
CROSS JOIN LATERAL unnest(dp.tags) AS tag_val
JOIN tags t ON t.name = tag_val
WHERE array_length(dp.tags, 1) > 0
ON CONFLICT DO NOTHING;

-- Extract unique tags from glossary_terms into tags table
INSERT INTO tags (name, description)
SELECT DISTINCT tag_val, NULL
FROM glossary_terms, LATERAL unnest(glossary_terms.tags) AS tag_val
WHERE array_length(glossary_terms.tags, 1) > 0
ON CONFLICT (name) DO NOTHING;

-- Backfill glossary_term_tags junction table
INSERT INTO glossary_term_tags (glossary_term_id, tag_id)
SELECT gt.id, t.id
FROM glossary_terms gt
CROSS JOIN LATERAL unnest(gt.tags) AS tag_val
JOIN tags t ON t.name = tag_val
WHERE array_length(gt.tags, 1) > 0
ON CONFLICT DO NOTHING;

-- Update data product trigger to read tags from junction table
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
        ARRAY(SELECT t.name FROM tags t JOIN data_product_tags dpt ON t.id = dpt.tag_id WHERE dpt.data_product_id = NEW.id),
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

-- Update glossary trigger to read tags from junction table
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
        ARRAY(SELECT t.name FROM tags t JOIN glossary_term_tags gtt ON t.id = gtt.tag_id WHERE gtt.glossary_term_id = NEW.id),
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

-- Row-level trigger function for data_product_tags to sync search_index
CREATE OR REPLACE FUNCTION sync_search_index_on_data_product_tag_change()
RETURNS TRIGGER AS $$
DECLARE
    affected_product_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_product_id := OLD.data_product_id;
    ELSE
        affected_product_id := NEW.data_product_id;
    END IF;

    UPDATE search_index
    SET tags = ARRAY(
        SELECT t.name FROM tags t
        JOIN data_product_tags dpt ON t.id = dpt.tag_id
        WHERE dpt.data_product_id = affected_product_id
    )
    WHERE type = 'data_product' AND entity_id = affected_product_id::text;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sync_search_on_data_product_tag_change
    AFTER INSERT OR DELETE ON data_product_tags
    FOR EACH ROW EXECUTE FUNCTION sync_search_index_on_data_product_tag_change();

-- Row-level trigger function for glossary_term_tags to sync search_index
CREATE OR REPLACE FUNCTION sync_search_index_on_glossary_term_tag_change()
RETURNS TRIGGER AS $$
DECLARE
    affected_term_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_term_id := OLD.glossary_term_id;
    ELSE
        affected_term_id := NEW.glossary_term_id;
    END IF;

    UPDATE search_index
    SET tags = ARRAY(
        SELECT t.name FROM tags t
        JOIN glossary_term_tags gtt ON t.id = gtt.tag_id
        WHERE gtt.glossary_term_id = affected_term_id
    )
    WHERE type = 'glossary' AND entity_id = affected_term_id::text;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sync_search_on_glossary_term_tag_change
    AFTER INSERT OR DELETE ON glossary_term_tags
    FOR EACH ROW EXECUTE FUNCTION sync_search_index_on_glossary_term_tag_change();

-- Update glossary_terms search_text to not reference tags (since tags column will be dropped)
-- First drop dependent objects
DROP INDEX IF EXISTS idx_glossary_terms_search;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;

-- Recreate search_text without tags reference
ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C')
    ) STORED;

CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

-- Backfill search_index.tags for existing data products and glossary terms
UPDATE search_index si
SET tags = ARRAY(
    SELECT t.name FROM tags t
    JOIN data_product_tags dpt ON t.id = dpt.tag_id
    WHERE dpt.data_product_id = si.entity_id::uuid
)
WHERE si.type = 'data_product';

UPDATE search_index si
SET tags = ARRAY(
    SELECT t.name FROM tags t
    JOIN glossary_term_tags gtt ON t.id = gtt.tag_id
    WHERE gtt.glossary_term_id = si.entity_id::uuid
)
WHERE si.type = 'glossary';

-- Drop old tags columns and indexes
DROP INDEX IF EXISTS idx_data_products_tags;
ALTER TABLE data_products DROP COLUMN IF EXISTS tags;

DROP INDEX IF EXISTS idx_glossary_terms_tags;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS tags;

---- create above / drop below ----

-- Restore tags columns
ALTER TABLE data_products ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE glossary_terms ADD COLUMN tags TEXT[];

-- Restore indexes
CREATE INDEX idx_data_products_tags ON data_products USING gin(tags);
CREATE INDEX idx_glossary_terms_tags ON glossary_terms USING gin(tags);

-- Drop junction table triggers
DROP TRIGGER IF EXISTS sync_search_on_data_product_tag_change ON data_product_tags;
DROP TRIGGER IF EXISTS sync_search_on_glossary_term_tag_change ON glossary_term_tags;
DROP FUNCTION IF EXISTS sync_search_index_on_data_product_tag_change();
DROP FUNCTION IF EXISTS sync_search_index_on_glossary_term_tag_change();

-- Drop junction tables
DROP TABLE IF EXISTS data_product_tags;
DROP TABLE IF EXISTS glossary_term_tags;

-- Restore triggers to read from tags columns directly
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

-- Restore glossary search_text with tags
DROP INDEX IF EXISTS idx_glossary_terms_search;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;
ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C') ||
        setweight(array_to_tsvector(tags), 'C')
    ) STORED;
CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

-- Backfill tags from search_index
UPDATE data_products dp
SET tags = COALESCE(si.tags, '{}')
FROM search_index si
WHERE si.type = 'data_product' AND si.entity_id = dp.id::text;

UPDATE glossary_terms gt
SET tags = COALESCE(si.tags, '{}')
FROM search_index si
WHERE si.type = 'glossary' AND si.entity_id = gt.id::text;
