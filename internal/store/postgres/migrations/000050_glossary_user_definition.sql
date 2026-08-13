-- Give a glossary term a slot only people write, the way an asset already
-- has one.
--
-- assets splits its wording in two: `description` is whatever the last run
-- said, `user_description` is what a person wrote and no run ever touches
-- it. glossary_terms had no such slot, so a run had to guess who authored a
-- term from a marker it left in metadata. `user_definition` replaces that
-- guess with a column: ingestion owns `definition` and overwrites it every
-- run, `user_definition` is only ever read by ingestion, and readers prefer
-- it when it is set.

ALTER TABLE glossary_terms ADD COLUMN user_definition TEXT;

COMMENT ON COLUMN glossary_terms.definition IS 'Definition from the source system, rewritten by every run';
COMMENT ON COLUMN glossary_terms.user_definition IS 'Definition a person wrote, never written by ingestion';

-- search_text is GENERATED ALWAYS ... STORED, so it cannot pick the new
-- column up on its own. It has to: the read path shows user_definition in
-- place of definition, and a term nobody can find by the words on its own
-- page is a broken glossary. Postgres has no way to alter a generated
-- expression, so the column and the index over it are rebuilt, which is
-- also how 000018 added tags to it.
--
-- Note that this only reaches terms that carry tags. The unqualified
-- array_to_tsvector below resolves to the built-in of that name rather
-- than the one 000018 created, and the built-in returns NULL for a NULL
-- array, which makes the whole vector NULL. That predates this migration
-- and is left exactly as it was.
DROP INDEX IF EXISTS idx_glossary_terms_search;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;
ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(user_definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C') ||
        setweight(array_to_tsvector(tags), 'C')
    ) STORED;
CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_glossary_terms_search;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS user_definition;

ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C') ||
        setweight(array_to_tsvector(tags), 'C')
    ) STORED;
CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

COMMENT ON COLUMN glossary_terms.definition IS NULL;
