-- Add source tracking to asset_terms
ALTER TABLE asset_terms
ADD COLUMN source VARCHAR(100) DEFAULT 'user';

-- Add user_description field to assets
ALTER TABLE assets
ADD COLUMN user_description TEXT;

-- Rename description to technical_description for clarity
-- First, copy existing descriptions to user_description (preserving user edits)
UPDATE assets
SET user_description = description
WHERE description IS NOT NULL;

-- The description column will continue to hold technical_description
-- We'll use it as-is for now to maintain backwards compatibility
-- In code, we'll treat description as technical_description

-- Add index for source tracking
CREATE INDEX IF NOT EXISTS idx_asset_terms_source ON asset_terms(source);

-- Add comments for clarity
COMMENT ON COLUMN assets.description IS 'Technical description (from plugins/automation)';
COMMENT ON COLUMN assets.user_description IS 'User-provided notes and documentation';
COMMENT ON COLUMN asset_terms.source IS 'Source of the term association (user, plugin:name, etc)';

---- create above / drop below ----

-- Drop index
DROP INDEX IF EXISTS idx_asset_terms_source;

-- Remove source column from asset_terms
ALTER TABLE asset_terms
DROP COLUMN IF EXISTS source;

-- Remove user_description column
ALTER TABLE assets
DROP COLUMN IF EXISTS user_description;

-- Remove comments
COMMENT ON COLUMN assets.description IS NULL;
COMMENT ON COLUMN asset_terms.source IS NULL;
