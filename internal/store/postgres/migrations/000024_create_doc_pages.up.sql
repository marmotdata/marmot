-- Documentation pages (multi-page support)
CREATE TABLE doc_pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Polymorphic association: can belong to asset OR data_product
    entity_type VARCHAR(50) NOT NULL,  -- 'asset' or 'data_product'
    entity_id VARCHAR(255) NOT NULL,   -- asset MRN or data_product UUID

    -- Page hierarchy
    parent_id UUID REFERENCES doc_pages(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,

    -- Content
    title VARCHAR(255) NOT NULL DEFAULT 'Untitled',
    emoji VARCHAR(32),  -- Optional emoji icon for the page
    content TEXT,  -- Markdown content with image references

    -- Metadata
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Full-text search
    search_text tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(content, '')), 'B')
    ) STORED
);

-- Indexes for doc_pages
CREATE INDEX idx_doc_pages_entity ON doc_pages(entity_type, entity_id);
CREATE INDEX idx_doc_pages_parent ON doc_pages(parent_id);
CREATE INDEX idx_doc_pages_position ON doc_pages(entity_type, entity_id, parent_id, position);
CREATE INDEX idx_doc_pages_search ON doc_pages USING gin(search_text);

-- Documentation images
CREATE TABLE doc_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES doc_pages(id) ON DELETE CASCADE,

    -- Image data
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    data BYTEA NOT NULL,

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_image_size CHECK (size_bytes <= 5242880),  -- 5MB max per image
    CONSTRAINT valid_content_type CHECK (content_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp'))
);

-- Indexes for doc_images
CREATE INDEX idx_doc_images_page ON doc_images(page_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_doc_page_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update updated_at
CREATE TRIGGER doc_pages_updated_at
    BEFORE UPDATE ON doc_pages
    FOR EACH ROW
    EXECUTE FUNCTION update_doc_page_updated_at();

-- Function to get total storage for an entity
CREATE OR REPLACE FUNCTION get_doc_storage_bytes(p_entity_type VARCHAR, p_entity_id VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    total_bytes BIGINT;
BEGIN
    SELECT COALESCE(SUM(di.size_bytes), 0)
    INTO total_bytes
    FROM doc_images di
    INNER JOIN doc_pages dp ON di.page_id = dp.id
    WHERE dp.entity_type = p_entity_type AND dp.entity_id = p_entity_id;

    RETURN total_bytes;
END;
$$ LANGUAGE plpgsql;
