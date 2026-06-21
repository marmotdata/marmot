CREATE TABLE tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE assets_tags (
    asset_id    VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (asset_id, tag_id)
);

CREATE INDEX idx_assets_tags_tag_id ON assets_tags (tag_id);

CREATE TABLE column_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    VARCHAR(255) NOT NULL,
    column_path VARCHAR(512) NOT NULL,
    tag_id      UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE (asset_id, column_path, tag_id)
);

CREATE INDEX idx_column_tags_tag_id ON column_tags (tag_id);
CREATE INDEX idx_column_tags_asset_id ON column_tags (asset_id);
CREATE INDEX idx_column_tags_asset_column ON column_tags (asset_id, column_path);

---- create above / drop below ----

DROP TABLE IF EXISTS column_tags CASCADE;
DROP TABLE IF EXISTS assets_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
