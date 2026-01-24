-- Add tags to glossary_terms
ALTER TABLE glossary_terms ADD COLUMN tags TEXT[];

-- Add metadata and tags to teams
ALTER TABLE teams ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE teams ADD COLUMN tags TEXT[];

-- Create GIN indexes for efficient filtering
CREATE INDEX idx_glossary_terms_tags ON glossary_terms USING GIN(tags);
CREATE INDEX idx_teams_metadata ON teams USING GIN(metadata);
CREATE INDEX idx_teams_tags ON teams USING GIN(tags);

-- Create immutable function to convert array to tsvector
CREATE OR REPLACE FUNCTION array_to_tsvector(TEXT[])
RETURNS tsvector AS $$
    SELECT to_tsvector('english', COALESCE(array_to_string($1, ' '), ''))
$$ LANGUAGE SQL IMMUTABLE;

-- Update search_text for glossary_terms to include tags
-- First drop the old generated column
ALTER TABLE glossary_terms DROP COLUMN IF EXISTS search_text;
-- Recreate with tags included
ALTER TABLE glossary_terms ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C') ||
        setweight(array_to_tsvector(tags), 'C')
    ) STORED;
-- Recreate the index
CREATE INDEX idx_glossary_terms_search ON glossary_terms USING gin(search_text);

-- Update search_text for teams to include tags
-- First drop the old generated column
ALTER TABLE teams DROP COLUMN IF EXISTS search_text;
-- Recreate with tags included
ALTER TABLE teams ADD COLUMN search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
        setweight(array_to_tsvector(tags), 'C')
    ) STORED;
-- Recreate the index
CREATE INDEX idx_teams_search ON teams USING gin(search_text);

---- create above / drop below ----

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
