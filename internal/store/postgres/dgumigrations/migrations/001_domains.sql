-- path is the slash-delimited chain of ancestor ids ending in the domain's own
-- id, with a trailing slash ("/a/b/"), so a subtree is a prefix match that
-- cannot bleed into a sibling whose id shares a prefix.
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES domains(id) ON DELETE RESTRICT,
    path TEXT NOT NULL,
    depth SMALLINT NOT NULL CHECK (depth BETWEEN 1 AND 8),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    tags TEXT[] NOT NULL DEFAULT '{}',
    restricted BOOLEAN NOT NULL DEFAULT false,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((parent_id IS NULL) = (depth = 1))
);

CREATE UNIQUE INDEX domains_path_key ON domains (path);
CREATE INDEX domains_path_prefix_idx ON domains (path text_pattern_ops);
CREATE UNIQUE INDEX domains_sibling_name_key ON domains (parent_id, lower(name)) WHERE parent_id IS NOT NULL;
CREATE UNIQUE INDEX domains_root_name_key ON domains (lower(name)) WHERE parent_id IS NULL;

INSERT INTO domains (id, path, depth, name, description)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    '/00000000-0000-4000-8000-000000000001/',
    1,
    'Unassigned',
    'Entities without an explicit domain. Writable only by onboarding identities.'
);

-- A missing membership row means the entity is in Unassigned, so a failed
-- assignment never leaves an entity outside domain control.
CREATE TABLE asset_domains (
    asset_id VARCHAR(255) PRIMARY KEY REFERENCES assets(id) ON DELETE CASCADE,
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX asset_domains_domain_idx ON asset_domains (domain_id);

CREATE TABLE data_product_domains (
    data_product_id UUID PRIMARY KEY REFERENCES data_products(id) ON DELETE CASCADE,
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX data_product_domains_domain_idx ON data_product_domains (domain_id);

CREATE TABLE glossary_term_domains (
    glossary_term_id UUID PRIMARY KEY REFERENCES glossary_terms(id) ON DELETE CASCADE,
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX glossary_term_domains_domain_idx ON glossary_term_domains (domain_id);

CREATE TABLE ingestion_schedule_domains (
    schedule_id UUID PRIMARY KEY REFERENCES ingestion_schedules(id) ON DELETE CASCADE,
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ingestion_schedule_domains_domain_idx ON ingestion_schedule_domains (domain_id);

---- create above / drop below ----

DROP TABLE ingestion_schedule_domains;
DROP TABLE glossary_term_domains;
DROP TABLE data_product_domains;
DROP TABLE asset_domains;
DROP TABLE domains;
