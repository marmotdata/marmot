CREATE TABLE IF NOT EXISTS glossary_terms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    definition TEXT NOT NULL,
    description TEXT,
    parent_term_id UUID REFERENCES glossary_terms(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    search_text tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(definition, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'C')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_glossary_terms_parent ON glossary_terms (parent_term_id);
CREATE INDEX IF NOT EXISTS idx_glossary_terms_deleted_at ON glossary_terms (deleted_at);
CREATE INDEX IF NOT EXISTS idx_glossary_terms_search ON glossary_terms USING gin(search_text);
CREATE INDEX IF NOT EXISTS idx_glossary_terms_metadata ON glossary_terms USING gin(metadata);
CREATE INDEX IF NOT EXISTS idx_glossary_terms_updated_at ON glossary_terms (updated_at);

CREATE TABLE IF NOT EXISTS glossary_term_owners (
    glossary_term_id UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (glossary_term_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_glossary_term_owners_term ON glossary_term_owners(glossary_term_id);
CREATE INDEX IF NOT EXISTS idx_glossary_term_owners_user ON glossary_term_owners(user_id);

INSERT INTO permissions (name, description, resource_type, action) VALUES
('view_glossary', 'View glossary terms', 'glossary', 'view'),
('manage_glossary', 'Create/update/delete glossary terms', 'glossary', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name IN ('view_glossary', 'manage_glossary');

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'user'),
    id
FROM permissions
WHERE name = 'view_glossary';
