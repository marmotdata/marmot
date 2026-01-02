-- Product images table for storing icons and header images for data products
CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_product_id UUID NOT NULL REFERENCES data_products(id) ON DELETE CASCADE,

    purpose VARCHAR(50) NOT NULL DEFAULT 'icon',  -- 'icon', 'header', etc.

    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    data BYTEA NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,

    CONSTRAINT valid_image_size CHECK (size_bytes <= 5242880),  -- 5MB max per image
    CONSTRAINT valid_content_type CHECK (content_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp')),
    -- Only one image per product per purpose
    CONSTRAINT unique_product_image_purpose UNIQUE (data_product_id, purpose)
);

CREATE INDEX idx_product_images_product ON product_images(data_product_id);
CREATE INDEX idx_product_images_purpose ON product_images(data_product_id, purpose);
