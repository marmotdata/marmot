---- tern: disable-tx ----

-- Discover can sort by name, and browsing the whole catalog that way should read names in order rather than sort every row.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_lower_name
    ON search_index (lower(name), entity_id);

---- create above / drop below ----

DROP INDEX CONCURRENTLY IF EXISTS idx_search_index_lower_name;
