-- Asset rules: governance rules that associate external links AND/OR glossary terms with assets via Marmot queries
CREATE TABLE IF NOT EXISTS asset_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    links JSONB NOT NULL DEFAULT '[]'::jsonb,
    rule_type VARCHAR(50) NOT NULL CHECK (rule_type IN ('query', 'metadata_match')),
    query_expression TEXT,
    metadata_field VARCHAR(255),
    pattern_type VARCHAR(20) CHECK (pattern_type IN ('exact', 'wildcard', 'regex', 'prefix')),
    pattern_value TEXT,
    priority INT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    memberships_updated_at TIMESTAMP WITH TIME ZONE,
    membership_count INT NOT NULL DEFAULT 0,
    last_reconciled_at TIMESTAMP WITH TIME ZONE,
    reconciliation_hash TEXT,
    search_text tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_asset_rules_name ON asset_rules(name);
CREATE INDEX IF NOT EXISTS idx_asset_rules_search ON asset_rules USING gin(search_text);
CREATE INDEX IF NOT EXISTS idx_asset_rules_enabled ON asset_rules(is_enabled) WHERE is_enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_asset_rules_updated_at ON asset_rules(updated_at);
CREATE INDEX IF NOT EXISTS idx_asset_rules_name_trgm ON asset_rules USING gin(name gin_trgm_ops);

-- Asset rule terms: which glossary terms a rule assigns to matching assets
CREATE TABLE IF NOT EXISTS asset_rule_terms (
    asset_rule_id UUID NOT NULL REFERENCES asset_rules(id) ON DELETE CASCADE,
    glossary_term_id UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_rule_id, glossary_term_id)
);

CREATE INDEX IF NOT EXISTS idx_art_by_rule ON asset_rule_terms(asset_rule_id);
CREATE INDEX IF NOT EXISTS idx_art_by_term ON asset_rule_terms(glossary_term_id);

-- Asset rule memberships: precomputed relationship between asset rules and assets
CREATE TABLE IF NOT EXISTS asset_rule_memberships (
    asset_rule_id UUID NOT NULL REFERENCES asset_rules(id) ON DELETE CASCADE,
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_rule_id, asset_id)
);

-- Covering index for "get assets for rule" (enables index-only scan)
CREATE INDEX IF NOT EXISTS idx_arm_by_rule_covering
ON asset_rule_memberships(asset_rule_id)
INCLUDE (asset_id);

-- Covering index for "get rules for asset" (enables index-only scan)
CREATE INDEX IF NOT EXISTS idx_arm_by_asset_covering
ON asset_rule_memberships(asset_id)
INCLUDE (asset_rule_id);

-- Aggressive autovacuum for high-churn membership table
ALTER TABLE asset_rule_memberships SET (
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_vacuum_threshold = 50000,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_analyze_threshold = 50000
);

-- Asset rule targets: denormalized index for fast candidate lookup on asset creation
CREATE TABLE IF NOT EXISTS asset_rule_targets (
    rule_id UUID NOT NULL REFERENCES asset_rules(id) ON DELETE CASCADE,
    target_type VARCHAR(50) NOT NULL CHECK (target_type IN ('asset_type', 'provider', 'tag', 'metadata_key', 'query')),
    target_value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (rule_id, target_type, target_value)
);

-- Composite index for the candidate lookup query
CREATE INDEX IF NOT EXISTS idx_argt_lookup
ON asset_rule_targets(target_type, target_value)
INCLUDE (rule_id);

-- Index for cleaning up targets when a rule changes
CREATE INDEX IF NOT EXISTS idx_argt_by_rule
ON asset_rule_targets(rule_id);

-- Partial index on asset_terms for efficient rule cleanup
-- Asset rules write to asset_terms with source = 'rule:<rule-id>'
CREATE INDEX IF NOT EXISTS idx_asset_terms_rule_source
ON asset_terms(source) WHERE source LIKE 'rule:%';

-- Aggressive autovacuum on asset_terms for high-churn rule-managed rows
ALTER TABLE asset_terms SET (
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_vacuum_threshold = 50000,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_analyze_threshold = 50000
);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_asset_terms_rule_source;
DROP TABLE IF EXISTS asset_rule_targets;
DROP TABLE IF EXISTS asset_rule_memberships;
DROP TABLE IF EXISTS asset_rule_terms;
DROP TABLE IF EXISTS asset_rules;

-- Reset autovacuum settings on asset_terms
ALTER TABLE asset_terms RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_vacuum_threshold,
    autovacuum_analyze_scale_factor,
    autovacuum_analyze_threshold
);
