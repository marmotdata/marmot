-- The search index takes types beyond the catalogue's four, so an edition can index records of its own (the cloud's tasks) and have them found by the same search. Each search applies access rules per type, and a type an edition does not know is simply never returned to a principal it has no rule for.
ALTER TABLE search_index DROP CONSTRAINT IF EXISTS search_index_type_check;

---- create above / drop below ----

DELETE FROM search_index WHERE type NOT IN ('asset', 'glossary', 'team', 'data_product');
ALTER TABLE search_index ADD CONSTRAINT search_index_type_check
    CHECK (type IN ('asset', 'glossary', 'team', 'data_product'));
