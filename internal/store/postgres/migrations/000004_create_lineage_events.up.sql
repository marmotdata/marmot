CREATE TABLE IF NOT EXISTS lineage_events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_time TIMESTAMP WITH TIME ZONE NOT NULL,
    event_type VARCHAR(20) NOT NULL,
    event_data JSONB NOT NULL,
    producer VARCHAR(255) NOT NULL,
    schema_url VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lineage_events_event_time ON lineage_events (event_time);
CREATE INDEX IF NOT EXISTS idx_lineage_events_event_type ON lineage_events (event_type);
CREATE INDEX IF NOT EXISTS idx_lineage_events_producer ON lineage_events (producer);

