ALTER TABLE lineage_edges
    ADD COLUMN column_lineage JSONB;

CREATE INDEX idx_lineage_edges_has_column_lineage
    ON lineage_edges (source_mrn, target_mrn)
    WHERE column_lineage IS NOT NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_lineage_edges_has_column_lineage;

ALTER TABLE lineage_edges
    DROP COLUMN IF EXISTS column_lineage;
