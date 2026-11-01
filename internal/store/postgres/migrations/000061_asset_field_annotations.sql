-- Per-field markers people set on an asset's schema: personal data, certified, an edited description and glossary terms. Plugins rewrite the schema on every run, so the markers live apart from it, keyed by the schema's name and the field's dotted path, and survive a field disappearing for a run.
CREATE TABLE IF NOT EXISTS asset_field_annotations (
    asset_id    VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    schema_name VARCHAR(255) NOT NULL DEFAULT '',
    field_path  TEXT         NOT NULL,
    pii         BOOLEAN      NOT NULL DEFAULT false,
    certified   BOOLEAN      NOT NULL DEFAULT false,
    description TEXT,
    term_ids    UUID[]       NOT NULL DEFAULT '{}',
    updated_by  VARCHAR(255),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, schema_name, field_path)
);

---- create above / drop below ----

DROP TABLE IF EXISTS asset_field_annotations;
