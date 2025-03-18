CREATE OR REPLACE FUNCTION array_to_text(arr text[]) 
RETURNS text IMMUTABLE 
LANGUAGE sql AS 
'SELECT array_to_string(arr, '' '')';

CREATE TABLE IF NOT EXISTS assets (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mrn VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(255) NOT NULL,
    providers TEXT[] NOT NULL DEFAULT '{}',
    environments JSONB NOT NULL DEFAULT '{}'::jsonb,
    description TEXT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    sources JSONB NOT NULL DEFAULT '[]'::jsonb,
    external_links JSONB NOT NULL DEFAULT '[]'::jsonb,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_sync_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    search_text tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(mrn, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(type, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_text(providers), '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_assets_mrn ON assets (mrn);
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets (type);
CREATE INDEX IF NOT EXISTS idx_assets_providers ON assets USING gin (providers);
CREATE INDEX IF NOT EXISTS idx_assets_created_by ON assets (created_by);
CREATE INDEX IF NOT EXISTS idx_assets_updated_at ON assets (updated_at);
CREATE INDEX IF NOT EXISTS idx_assets_tags ON assets USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_assets_metadata ON assets USING gin (metadata);
CREATE INDEX IF NOT EXISTS idx_assets_schema ON assets USING gin (schema);
CREATE INDEX IF NOT EXISTS idx_assets_search ON assets USING gin(search_text);
CREATE INDEX IF NOT EXISTS idx_assets_name_trgm ON assets USING gin(name gin_trgm_ops);
