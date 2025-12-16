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
