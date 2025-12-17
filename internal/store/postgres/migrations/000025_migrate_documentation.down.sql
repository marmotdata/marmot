-- Restore the documentation column
ALTER TABLE data_products ADD COLUMN IF NOT EXISTS documentation TEXT;

-- Migrate documentation back from doc_pages to data_products
UPDATE data_products dp
SET documentation = (
    SELECT content FROM doc_pages
    WHERE entity_type = 'data_product'
    AND entity_id = dp.id::text
    AND parent_id IS NULL
    AND position = 0
    AND title = 'Documentation'
    LIMIT 1
);

-- Remove the migrated doc_pages
DELETE FROM doc_pages
WHERE entity_type = 'data_product'
  AND title = 'Documentation'
  AND parent_id IS NULL
  AND position = 0;
