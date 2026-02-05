---- tern: disable-tx ----

-- Materialised view for metadata field autocomplete (filters out unique values)
CREATE MATERIALIZED VIEW IF NOT EXISTS metadata_value_counts AS
SELECT
    field_path,
    value,
    entity_type,
    count,
    NOW() as refreshed_at
FROM (
    SELECT
        key as field_path,
        CASE
            WHEN jsonb_typeof(val) = 'string' THEN trim('"' FROM val::text)
            ELSE val::text
        END as value,
        'asset' as entity_type,
        COUNT(*) as count
    FROM assets a
    CROSS JOIN LATERAL jsonb_each(a.metadata) AS x(key, val)
    WHERE a.is_stub = FALSE
    AND a.metadata IS NOT NULL
    AND jsonb_typeof(a.metadata) = 'object'
    AND jsonb_typeof(val) IN ('string', 'number', 'boolean')
    AND length(val::text) <= 200
    GROUP BY key, val
    HAVING COUNT(*) >= 2

    UNION ALL

    SELECT
        key as field_path,
        CASE
            WHEN jsonb_typeof(val) = 'string' THEN trim('"' FROM val::text)
            ELSE val::text
        END as value,
        'glossary' as entity_type,
        COUNT(*) as count
    FROM glossary_terms g
    CROSS JOIN LATERAL jsonb_each(g.metadata) AS x(key, val)
    WHERE g.deleted_at IS NULL
    AND g.metadata IS NOT NULL
    AND jsonb_typeof(g.metadata) = 'object'
    AND jsonb_typeof(val) IN ('string', 'number', 'boolean')
    AND length(val::text) <= 200
    GROUP BY key, val
    HAVING COUNT(*) >= 2

    UNION ALL

    SELECT
        key as field_path,
        CASE
            WHEN jsonb_typeof(val) = 'string' THEN trim('"' FROM val::text)
            ELSE val::text
        END as value,
        'team' as entity_type,
        COUNT(*) as count
    FROM teams t
    CROSS JOIN LATERAL jsonb_each(t.metadata) AS x(key, val)
    WHERE t.metadata IS NOT NULL
    AND jsonb_typeof(t.metadata) = 'object'
    AND jsonb_typeof(val) IN ('string', 'number', 'boolean')
    AND length(val::text) <= 200
    GROUP BY key, val
    HAVING COUNT(*) >= 2
) combined;

CREATE UNIQUE INDEX IF NOT EXISTS idx_metadata_value_counts_pk
    ON metadata_value_counts (field_path, value, entity_type);

CREATE INDEX IF NOT EXISTS idx_metadata_value_counts_field_prefix
    ON metadata_value_counts (field_path, lower(value) text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_metadata_value_counts_field_count
    ON metadata_value_counts (field_path, count DESC);

-- Refresh function (called by background job)
CREATE OR REPLACE FUNCTION refresh_metadata_value_counts()
RETURNS VOID AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY metadata_value_counts;
END
$$ LANGUAGE plpgsql;

---- create above / drop below ----

DROP FUNCTION IF EXISTS refresh_metadata_value_counts();
DROP MATERIALIZED VIEW IF EXISTS metadata_value_counts;
