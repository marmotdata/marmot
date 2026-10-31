-- Who declared an edge: the ingestion pipeline whose run wrote it, the OpenLineage producer or the person who added it by hand. The latest declarer wins, so an edge a second pipeline takes over reads as theirs.
ALTER TABLE lineage_edges ADD COLUMN IF NOT EXISTS declared_by VARCHAR(255);

-- Runs record each edge they write as a lineage entity named "source -> target", which is all the history there is for edges written before this column.
UPDATE lineage_edges e
SET declared_by = d.pipeline_name
FROM (
    SELECT DISTINCT ON (re.entity_name) re.entity_name, r.pipeline_name
    FROM run_entities re
    JOIN runs r ON r.id = re.run_id
    WHERE re.entity_type = 'lineage' AND re.status <> 'failed' AND re.entity_name IS NOT NULL
    ORDER BY re.entity_name, re.created_at DESC
) d
WHERE e.declared_by IS NULL
  AND e.origin = 'declared'
  AND d.entity_name = e.source_mrn || ' -> ' || e.target_mrn;

---- create above / drop below ----

ALTER TABLE lineage_edges DROP COLUMN IF EXISTS declared_by;
