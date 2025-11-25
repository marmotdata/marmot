-- Add search_text to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(username, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(name, '')), 'A')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_users_search ON users USING gin(search_text);

-- Add search_text to teams table
ALTER TABLE teams ADD COLUMN IF NOT EXISTS search_text tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_teams_search ON teams USING gin(search_text);
