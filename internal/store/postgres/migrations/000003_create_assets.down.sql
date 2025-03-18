DROP INDEX IF EXISTS idx_assets_name_trgm;
DROP INDEX IF EXISTS idx_assets_search;
DROP INDEX IF EXISTS idx_assets_schema;
DROP INDEX IF EXISTS idx_assets_metadata;
DROP INDEX IF EXISTS idx_assets_tags;
DROP INDEX IF EXISTS idx_assets_updated_at;
DROP INDEX IF EXISTS idx_assets_created_by;
DROP INDEX IF EXISTS idx_assets_providers;
DROP INDEX IF EXISTS idx_assets_type;
DROP INDEX IF EXISTS idx_assets_mrn;
DROP TABLE IF EXISTS assets;
DROP FUNCTION IF EXISTS array_to_text;

