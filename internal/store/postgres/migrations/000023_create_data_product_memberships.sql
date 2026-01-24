-- Membership table: precomputed relationship between data products and assets
-- This is the source of truth for "which assets belong to which data product"
CREATE TABLE IF NOT EXISTS data_product_memberships (
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    source VARCHAR(20) NOT NULL CHECK (source IN ('manual', 'rule')),
    rule_id UUID REFERENCES data_product_rules(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (data_product_id, asset_id)
);

-- Covering index for "get assets for product" (enables index-only scan)
CREATE INDEX IF NOT EXISTS idx_dpm_by_product_covering
ON data_product_memberships(data_product_id)
INCLUDE (asset_id, source);

-- Covering index for "get products for asset" (enables index-only scan)
CREATE INDEX IF NOT EXISTS idx_dpm_by_asset_covering
ON data_product_memberships(asset_id)
INCLUDE (data_product_id, source);

-- Index for rule-based lookups (when a rule is deleted/updated)
CREATE INDEX IF NOT EXISTS idx_dpm_by_rule
ON data_product_memberships(rule_id)
WHERE rule_id IS NOT NULL;

-- Index for source-based filtering (manual vs rule assets)
CREATE INDEX IF NOT EXISTS idx_dpm_by_source
ON data_product_memberships(data_product_id, source);

-- Rule targets table: denormalized index of what each rule is looking for
-- Used for fast candidate lookup when an asset is created
CREATE TABLE IF NOT EXISTS data_product_rule_targets (
    rule_id UUID NOT NULL REFERENCES data_product_rules(id) ON DELETE CASCADE,
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    target_type VARCHAR(50) NOT NULL CHECK (target_type IN ('asset_type', 'provider', 'tag', 'metadata_key', 'query')),
    target_value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (rule_id, target_type, target_value)
);

-- Composite index for the candidate lookup query
CREATE INDEX IF NOT EXISTS idx_dprt_lookup
ON data_product_rule_targets(target_type, target_value)
INCLUDE (rule_id, data_product_id);

-- Index for cleaning up targets when a rule changes
CREATE INDEX IF NOT EXISTS idx_dprt_by_rule
ON data_product_rule_targets(rule_id);

-- Add columns to track membership freshness on data products
ALTER TABLE data_products
ADD COLUMN IF NOT EXISTS memberships_updated_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS membership_count INT NOT NULL DEFAULT 0;

-- Drop the legacy data_product_assets table (replaced by memberships)
DROP TABLE IF EXISTS data_product_assets;

---- create above / drop below ----

-- Drop rule targets table first (due to FK constraints)
DROP TABLE IF EXISTS data_product_rule_targets;

-- Drop memberships table
DROP TABLE IF EXISTS data_product_memberships;

-- Remove membership freshness columns from data products
ALTER TABLE data_products
DROP COLUMN IF EXISTS memberships_updated_at,
DROP COLUMN IF EXISTS membership_count;

-- Recreate the legacy data_product_assets table
CREATE TABLE IF NOT EXISTS data_product_assets (
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (data_product_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_data_product_assets_product ON data_product_assets(data_product_id);
CREATE INDEX IF NOT EXISTS idx_data_product_assets_asset ON data_product_assets(asset_id);
