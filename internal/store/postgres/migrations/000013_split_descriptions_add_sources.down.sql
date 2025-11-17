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
