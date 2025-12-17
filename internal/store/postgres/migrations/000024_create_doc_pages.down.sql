-- Drop triggers
DROP TRIGGER IF EXISTS doc_pages_updated_at ON doc_pages;

-- Drop functions
DROP FUNCTION IF EXISTS update_doc_page_updated_at();
DROP FUNCTION IF EXISTS get_doc_storage_bytes(VARCHAR, VARCHAR);

-- Drop tables (doc_images first due to foreign key)
DROP TABLE IF EXISTS doc_images;
DROP TABLE IF EXISTS doc_pages;
