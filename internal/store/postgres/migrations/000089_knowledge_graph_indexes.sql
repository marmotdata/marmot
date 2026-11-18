---- tern: disable-tx ----

-- Walking up containment (database, schema, table) reads only CONTAINS edges, so a table with many upstream data-flow edges does not slow the walk that finds inherited knowledge.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lineage_edges_contains ON lineage_edges (target_mrn) WHERE type = 'CONTAINS';

-- Knowledge lists match asset types case-insensitively. Measured at 100,000 entries: counting fell from 101 ms to 10 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_lower_type ON assets (lower(type));

---- create above / drop below ----

DROP INDEX CONCURRENTLY IF EXISTS idx_assets_lower_type;
DROP INDEX CONCURRENTLY IF EXISTS idx_lineage_edges_contains;
