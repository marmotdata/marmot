-- Full-text search matches the summary people write in the UI. search_text is GENERATED ALWAYS ... STORED, so it cannot pick up user_description without being dropped and added again; the expression is otherwise unchanged from 000083 and the summary weighs the same as the description.

DROP INDEX IF EXISTS idx_assets_search;
DROP INDEX IF EXISTS idx_assets_search_all;
ALTER TABLE assets DROP COLUMN search_text;

ALTER TABLE assets ADD COLUMN search_text tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(mrn, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(type, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(array_to_text(providers), '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'C') ||
    setweight(to_tsvector('english', COALESCE(user_description, '')), 'C')
) STORED;
CREATE INDEX idx_assets_search ON assets USING gin (search_text) WHERE is_stub = FALSE;
CREATE INDEX idx_assets_search_all ON assets USING gin (search_text);

-- The search index holds a copy of search_text that the rewrite above does not reach.
UPDATE search_index si SET search_text = a.search_text
FROM assets a
WHERE si.type = 'asset' AND si.entity_id = a.id AND si.search_text IS DISTINCT FROM a.search_text;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_assets_search;
DROP INDEX IF EXISTS idx_assets_search_all;
ALTER TABLE assets DROP COLUMN search_text;

ALTER TABLE assets ADD COLUMN search_text tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(mrn, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(type, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(array_to_text(providers), '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'C')
) STORED;
CREATE INDEX idx_assets_search ON assets USING gin (search_text) WHERE is_stub = FALSE;
CREATE INDEX idx_assets_search_all ON assets USING gin (search_text);

UPDATE search_index si SET search_text = a.search_text
FROM assets a
WHERE si.type = 'asset' AND si.entity_id = a.id AND si.search_text IS DISTINCT FROM a.search_text;
