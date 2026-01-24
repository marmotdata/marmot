CREATE TABLE IF NOT EXISTS lineage_events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_time TIMESTAMP WITH TIME ZONE NOT NULL,
    event_type VARCHAR(20) NOT NULL,
    event_data JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lineage_events_event_time ON lineage_events (event_time);
CREATE INDEX IF NOT EXISTS idx_lineage_events_event_type ON lineage_events (event_type);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_lineage_events_event_type;
DROP INDEX IF EXISTS idx_lineage_events_event_time;
DROP TABLE IF EXISTS lineage_events;

