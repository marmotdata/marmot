-- Drop the updated search indexes and columns
DROP INDEX IF EXISTS idx_glossary_terms_search;
DROP INDEX IF EXISTS idx_teams_search;
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;
ALTER TABLE teams DROP COLUMN IF EXISTS search_text;

-- Drop the immutable function
DROP FUNCTION IF EXISTS array_to_tsvector(TEXT[]);

-- Drop indexes for tags and metadata
DROP INDEX IF EXISTS idx_glossary_terms_tags;
DROP INDEX IF EXISTS idx_teams_metadata;
DROP INDEX IF EXISTS idx_teams_tags;

-- Remove columns
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS tags;
ALTER TABLE teams DROP COLUMN IF EXISTS metadata;
ALTER TABLE teams DROP COLUMN IF EXISTS tags;

-- Restore original search_text for glossary_terms
ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C')
    ) STORED;
CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

-- Restore original search_text for teams
ALTER TABLE teams ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B')
    ) STORED;
CREATE INDEX idx_teams_search ON teams USING gin(search_text);
