CREATE TABLE IF NOT EXISTS data_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    documentation TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    search_text tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_data_products_name ON data_products(name);
CREATE INDEX IF NOT EXISTS idx_data_products_search ON data_products USING gin(search_text);
CREATE INDEX IF NOT EXISTS idx_data_products_metadata ON data_products USING gin(metadata);
CREATE INDEX IF NOT EXISTS idx_data_products_tags ON data_products USING gin(tags);
CREATE INDEX IF NOT EXISTS idx_data_products_updated_at ON data_products(updated_at);
CREATE INDEX IF NOT EXISTS idx_data_products_name_trgm ON data_products USING gin(name gin_trgm_ops);

CREATE TABLE IF NOT EXISTS data_product_assets (
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (data_product_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_data_product_assets_product ON data_product_assets(data_product_id);
CREATE INDEX IF NOT EXISTS idx_data_product_assets_asset ON data_product_assets(asset_id);

CREATE TABLE IF NOT EXISTS data_product_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    rule_type VARCHAR(50) NOT NULL CHECK (rule_type IN ('query', 'metadata_match')),
    query_expression TEXT,
    metadata_field VARCHAR(255),
    pattern_type VARCHAR(20) CHECK (pattern_type IN ('exact', 'wildcard', 'regex', 'prefix')),
    pattern_value TEXT,
    priority INT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_product_rules_product ON data_product_rules(data_product_id);
CREATE INDEX IF NOT EXISTS idx_data_product_rules_enabled ON data_product_rules(is_enabled) WHERE is_enabled = TRUE;

CREATE TABLE IF NOT EXISTS data_product_owners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK ((user_id IS NOT NULL AND team_id IS NULL) OR (user_id IS NULL AND team_id IS NOT NULL)),
    UNIQUE (data_product_id, user_id),
    UNIQUE (data_product_id, team_id)
);

CREATE INDEX IF NOT EXISTS idx_data_product_owners_product ON data_product_owners(data_product_id);
CREATE INDEX IF NOT EXISTS idx_data_product_owners_user ON data_product_owners(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_data_product_owners_team ON data_product_owners(team_id) WHERE team_id IS NOT NULL;

---- create above / drop below ----

DROP TABLE IF EXISTS data_product_owners;
DROP TABLE IF EXISTS data_product_rules;
DROP TABLE IF EXISTS data_product_assets;
DROP TABLE IF EXISTS data_products;
