-- Drop indexes
DROP INDEX IF EXISTS idx_asset_terms_created_at;
DROP INDEX IF EXISTS idx_asset_terms_glossary_term_id;
DROP INDEX IF EXISTS idx_asset_terms_asset_id;

-- Drop asset_terms table
DROP TABLE IF EXISTS asset_terms;
