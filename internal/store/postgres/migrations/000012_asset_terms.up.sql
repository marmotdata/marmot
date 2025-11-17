-- Create asset_terms join table for associating glossary terms with assets
CREATE TABLE IF NOT EXISTS asset_terms (
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    glossary_term_id UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (asset_id, glossary_term_id)
);

-- Create index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_asset_terms_asset_id ON asset_terms(asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_terms_glossary_term_id ON asset_terms(glossary_term_id);
CREATE INDEX IF NOT EXISTS idx_asset_terms_created_at ON asset_terms(created_at);
