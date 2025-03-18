CREATE TABLE IF NOT EXISTS lineage_edges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_mrn VARCHAR(255) NOT NULL REFERENCES assets(mrn),
    target_mrn VARCHAR(255) NOT NULL REFERENCES assets(mrn),
    event_id UUID NOT NULL REFERENCES lineage_events(event_id),
    job_mrn VARCHAR(255) NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_edge UNIQUE (source_mrn, target_mrn, event_id)
);

CREATE INDEX IF NOT EXISTS idx_lineage_edges_source_target ON lineage_edges (source_mrn, target_mrn);
CREATE INDEX IF NOT EXISTS idx_lineage_edges_target_source ON lineage_edges (target_mrn, source_mrn);
CREATE INDEX IF NOT EXISTS idx_lineage_edges_job_not_null ON lineage_edges (source_mrn, target_mrn) 
WHERE job_mrn IS NOT NULL;
