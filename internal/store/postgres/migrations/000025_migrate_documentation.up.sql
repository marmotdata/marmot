-- Migrate existing documentation from data_products to doc_pages
INSERT INTO doc_pages (entity_type, entity_id, parent_id, position, title, content, created_by, created_at, updated_at)
SELECT
    'data_product' AS entity_type,
    dp.id::text AS entity_id,
    NULL AS parent_id,
    0 AS position,
    'Documentation' AS title,
    dp.documentation AS content,
    dp.created_by,
    dp.created_at,
    dp.updated_at
FROM data_products dp
WHERE dp.documentation IS NOT NULL
  AND dp.documentation != ''
  AND NOT EXISTS (
    SELECT 1 FROM doc_pages
    WHERE entity_type = 'data_product'
    AND entity_id = dp.id::text
  );

-- Drop the old documentation column
ALTER TABLE data_products DROP COLUMN IF EXISTS documentation;
