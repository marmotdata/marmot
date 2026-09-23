---- tern: disable-tx ----

-- An entity's memory is part of what search matches it on.

ALTER TABLE search_index ADD COLUMN IF NOT EXISTS memory_text tsvector;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_memory
    ON search_index USING GIN (memory_text);

-- Locking the entity's row first serialises concurrent writers, so each
-- rebuild reads every memory committed before it.
CREATE OR REPLACE FUNCTION refresh_memory_search(p_type TEXT, p_id TEXT)
RETURNS void AS $$
DECLARE
    doc tsvector;
BEGIN
    PERFORM 1 FROM search_index WHERE type = p_type AND entity_id = p_id FOR UPDATE;

    IF p_type = 'asset' THEN
        SELECT to_tsvector('english', string_agg(content, ' ')) INTO doc
          FROM memories
         WHERE asset_id = p_id;
    ELSIF p_type = 'data_product' THEN
        SELECT to_tsvector('english', string_agg(content, ' ')) INTO doc
          FROM memories
         WHERE data_product_id = p_id::uuid;
    ELSE
        RETURN;
    END IF;

    UPDATE search_index SET memory_text = doc WHERE type = p_type AND entity_id = p_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION memory_search_trigger()
RETURNS TRIGGER AS $$
DECLARE
    old_type TEXT;
    old_id   TEXT;
    new_type TEXT;
    new_id   TEXT;
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        old_type := CASE WHEN OLD.asset_id IS NOT NULL THEN 'asset' ELSE 'data_product' END;
        old_id   := COALESCE(OLD.asset_id, OLD.data_product_id::text);
        PERFORM refresh_memory_search(old_type, old_id);
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
        new_type := CASE WHEN NEW.asset_id IS NOT NULL THEN 'asset' ELSE 'data_product' END;
        new_id   := COALESCE(NEW.asset_id, NEW.data_product_id::text);
        IF TG_OP = 'INSERT' OR new_type <> old_type OR new_id <> old_id THEN
            PERFORM refresh_memory_search(new_type, new_id);
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Recording a use or a find does not change what search matches.
CREATE TRIGGER memory_search_sync
    AFTER INSERT OR DELETE OR UPDATE OF content, asset_id, data_product_id ON memories
    FOR EACH ROW EXECUTE FUNCTION memory_search_trigger();

-- An entity's search row can be created after its memory, for example when
-- a stub asset becomes a real one.
CREATE OR REPLACE FUNCTION search_index_memory_trigger()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM refresh_memory_search(NEW.type, NEW.entity_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER search_index_memory_sync
    AFTER INSERT ON search_index
    FOR EACH ROW WHEN (NEW.type IN ('asset', 'data_product'))
    EXECUTE FUNCTION search_index_memory_trigger();

SELECT refresh_memory_search(entity_type, entity_id)
  FROM (
    SELECT DISTINCT
           CASE WHEN asset_id IS NOT NULL THEN 'asset' ELSE 'data_product' END AS entity_type,
           COALESCE(asset_id, data_product_id::text) AS entity_id
      FROM memories
  ) entities;

---- create above / drop below ----

DROP TRIGGER IF EXISTS search_index_memory_sync ON search_index;
DROP FUNCTION IF EXISTS search_index_memory_trigger();
DROP TRIGGER IF EXISTS memory_search_sync ON memories;
DROP FUNCTION IF EXISTS memory_search_trigger();
DROP FUNCTION IF EXISTS refresh_memory_search(TEXT, TEXT);
DROP INDEX CONCURRENTLY IF EXISTS idx_search_index_memory;
ALTER TABLE search_index DROP COLUMN IF EXISTS memory_text;
