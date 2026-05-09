ALTER TABLE lineage_edges
    ADD COLUMN type              VARCHAR(40),
    ADD COLUMN origin            VARCHAR(20)  NOT NULL DEFAULT 'declared',
    ADD COLUMN observation_count INTEGER      NOT NULL DEFAULT 1,
    ADD COLUMN last_seen_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW();

UPDATE lineage_edges e
SET type = COALESCE(ev.event_data->>'type', 'DIRECT')
FROM lineage_events ev
WHERE ev.event_id = e.event_id AND e.type IS NULL;

CREATE INDEX idx_lineage_edges_type   ON lineage_edges (type)   WHERE type IS NOT NULL;
CREATE INDEX idx_lineage_edges_origin ON lineage_edges (origin);
CREATE UNIQUE INDEX idx_lineage_edges_observed_unique
    ON lineage_edges (source_mrn, target_mrn, type)
    WHERE origin = 'observed';

---- create above / drop below ----

DROP INDEX IF EXISTS idx_lineage_edges_observed_unique;
DROP INDEX IF EXISTS idx_lineage_edges_origin;
DROP INDEX IF EXISTS idx_lineage_edges_type;

ALTER TABLE lineage_edges
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS observation_count,
    DROP COLUMN IF EXISTS origin,
    DROP COLUMN IF EXISTS type;
