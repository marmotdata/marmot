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
