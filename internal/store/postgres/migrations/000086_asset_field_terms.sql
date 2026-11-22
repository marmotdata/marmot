-- Glossary terms on schema fields move out of the term_ids array into a join table, so a term can find the fields and assets that carry it as cheaply as an asset finds its terms, and each link keeps who made it and when.
CREATE TABLE IF NOT EXISTS asset_field_terms (
    asset_id         VARCHAR(255) NOT NULL,
    schema_name      VARCHAR(255) NOT NULL,
    field_path       TEXT         NOT NULL,
    glossary_term_id UUID         NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    source           VARCHAR(255) NOT NULL DEFAULT 'user',
    created_by       VARCHAR(255),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, schema_name, field_path, glossary_term_id),
    FOREIGN KEY (asset_id, schema_name, field_path)
        REFERENCES asset_field_annotations (asset_id, schema_name, field_path) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_asset_field_terms_term_asset
    ON asset_field_terms (glossary_term_id, asset_id);

-- Ids of terms that have since been removed are dropped rather than carried over.
INSERT INTO asset_field_terms (asset_id, schema_name, field_path, glossary_term_id, created_by, created_at)
SELECT f.asset_id, f.schema_name, f.field_path, t.id, f.updated_by, f.updated_at
FROM asset_field_annotations f
CROSS JOIN LATERAL unnest(f.term_ids) AS u(term_id)
JOIN glossary_terms t ON t.id = u.term_id
ON CONFLICT DO NOTHING;

-- A field whose only marker was a term that no longer exists has nothing left to keep.
DELETE FROM asset_field_annotations f
WHERE NOT f.pii AND NOT f.certified AND f.description IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM asset_field_terms ft
    WHERE ft.asset_id = f.asset_id AND ft.schema_name = f.schema_name AND ft.field_path = f.field_path
  );

ALTER TABLE asset_field_annotations DROP COLUMN IF EXISTS term_ids;

---- create above / drop below ----

ALTER TABLE asset_field_annotations ADD COLUMN IF NOT EXISTS term_ids UUID[] NOT NULL DEFAULT '{}';

UPDATE asset_field_annotations f
SET term_ids = sub.ids
FROM (
    SELECT asset_id, schema_name, field_path, array_agg(glossary_term_id ORDER BY created_at) AS ids
    FROM asset_field_terms
    GROUP BY asset_id, schema_name, field_path
) sub
WHERE sub.asset_id = f.asset_id AND sub.schema_name = f.schema_name AND sub.field_path = f.field_path;

DROP TABLE IF EXISTS asset_field_terms;
