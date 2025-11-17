DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('view_glossary', 'manage_glossary')
);

DELETE FROM permissions WHERE name IN ('view_glossary', 'manage_glossary');

DROP INDEX IF EXISTS idx_glossary_term_owners_user;
DROP INDEX IF EXISTS idx_glossary_term_owners_term;
DROP TABLE IF EXISTS glossary_term_owners;

DROP INDEX IF EXISTS idx_glossary_terms_updated_at;
DROP INDEX IF EXISTS idx_glossary_terms_metadata;
DROP INDEX IF EXISTS idx_glossary_terms_search;
DROP INDEX IF EXISTS idx_glossary_terms_deleted_at;
DROP INDEX IF EXISTS idx_glossary_terms_parent;

DROP TABLE IF EXISTS glossary_terms;
