-- Agent memory attached to a catalog entity.

CREATE TABLE IF NOT EXISTS memories (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Exactly one of these is set: the entity the entry belongs to.
    data_product_id    UUID REFERENCES data_products(id) ON DELETE CASCADE,
    asset_id           VARCHAR(255) REFERENCES assets(id) ON DELETE CASCADE,
    content            TEXT NOT NULL,
    created_by_type    TEXT NOT NULL,
    created_by_id      TEXT NOT NULL,
    created_by_name    TEXT NOT NULL,
    session_id         TEXT,
    updated_by_type    TEXT NOT NULL,
    updated_by_id      TEXT NOT NULL,
    updated_by_name    TEXT NOT NULL,
    updated_session_id TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_text        tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,

    CONSTRAINT memories_one_entity CHECK (num_nonnulls(data_product_id, asset_id) = 1)
);

CREATE INDEX IF NOT EXISTS idx_memories_data_product_updated
    ON memories (data_product_id, updated_at DESC) WHERE data_product_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memories_asset_updated
    ON memories (asset_id, updated_at DESC) WHERE asset_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_memories_search
    ON memories USING gin (search_text);

-- Reading memory needs assets:view. Adding, editing and deleting it needs
-- memory:write.
INSERT INTO permissions (name, description, resource_type, action) VALUES
('write_memory', 'Add, edit and delete memory on assets and data products', 'memory', 'write');

-- Granted to admin only by default; other roles are given it explicitly.
INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name = 'write_memory';

---- create above / drop below ----

DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE name = 'write_memory');

DELETE FROM permissions WHERE name = 'write_memory';

DROP TABLE IF EXISTS memories;
