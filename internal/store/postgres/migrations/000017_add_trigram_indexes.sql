CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_assets_mrn_trgm ON assets USING gin(mrn gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_assets_combined_trgm ON assets
USING gin((COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, '')) gin_trgm_ops)
WHERE is_stub = FALSE;

CREATE INDEX IF NOT EXISTS idx_assets_updated_at_desc ON assets (updated_at DESC) WHERE is_stub = FALSE;

CREATE INDEX IF NOT EXISTS idx_glossary_updated_at_desc ON glossary_terms (updated_at DESC) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_teams_updated_at_desc ON teams (updated_at DESC);

SET work_mem = '16MB';
SET maintenance_work_mem = '256MB';

SET pg_trgm.similarity_threshold = 0.3;
SET pg_trgm.word_similarity_threshold = 0.5;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_teams_updated_at_desc;
DROP INDEX IF EXISTS idx_glossary_updated_at_desc;
DROP INDEX IF EXISTS idx_assets_updated_at_desc;
DROP INDEX IF EXISTS idx_assets_combined_trgm;
DROP INDEX IF EXISTS idx_assets_mrn_trgm;
