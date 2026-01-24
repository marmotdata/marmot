-- Configuration largely inspired from https://aws.amazon.com/blogs/database/designing-high-performance-time-series-data-tables-on-amazon-rds-for-postgresql/
-- Raw metrics table (partitioned by timestamp range)
CREATE TABLE raw_metrics (
    id BIGSERIAL,
    metric_name VARCHAR(255) NOT NULL,
    metric_type VARCHAR(50) NOT NULL CHECK (metric_type IN ('counter', 'gauge', 'histogram')),
    value REAL NOT NULL,
    labels JSONB DEFAULT '{}',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create a default partition to catch all data initially
CREATE TABLE raw_metrics_default PARTITION OF raw_metrics DEFAULT;

-- Aggregated metrics for UI queries
CREATE TABLE aggregated_metrics (
    id BIGSERIAL PRIMARY KEY,
    metric_name VARCHAR(255) NOT NULL,
    aggregation_type VARCHAR(20) NOT NULL CHECK (aggregation_type IN ('avg', 'sum', 'max', 'min', 'count')),
    value REAL NOT NULL,
    labels JSONB DEFAULT '{}',
    bucket_start TIMESTAMPTZ NOT NULL,
    bucket_end TIMESTAMPTZ NOT NULL,
    bucket_size INTERVAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (metric_name, aggregation_type, labels, bucket_start, bucket_end)
);

-- Indexes
CREATE INDEX raw_metrics_timestamp_brin_idx ON raw_metrics USING BRIN (timestamp) WITH (pages_per_range = 32);
CREATE INDEX raw_metrics_name_idx ON raw_metrics (metric_name);
CREATE INDEX raw_metrics_labels_idx ON raw_metrics USING GIN (labels);

CREATE INDEX agg_metrics_name_bucket_idx ON aggregated_metrics (metric_name, bucket_start DESC, bucket_end DESC);
CREATE INDEX agg_metrics_labels_idx ON aggregated_metrics USING GIN (labels);
CREATE INDEX agg_metrics_bucket_size_idx ON aggregated_metrics (bucket_size, bucket_start DESC);

-- Function to create daily partitions
CREATE OR REPLACE FUNCTION create_metrics_partition_for_date(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_time TIMESTAMPTZ;
    end_time TIMESTAMPTZ;
BEGIN
    partition_name := 'raw_metrics_' || TO_CHAR(partition_date, 'YYYY_MM_DD');
    start_time := partition_date::TIMESTAMPTZ;
    end_time := (partition_date + INTERVAL '1 day')::TIMESTAMPTZ;
    
    -- Check if partition already exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c 
        JOIN pg_namespace n ON n.oid = c.relnamespace 
        WHERE c.relname = partition_name AND n.nspname = 'public'
    ) THEN
        EXECUTE format('CREATE TABLE %I PARTITION OF raw_metrics FOR VALUES FROM (%L) TO (%L)',
                      partition_name, start_time, end_time);
    END IF;
END
$$ LANGUAGE plpgsql;

---- create above / drop below ----

DROP TABLE IF EXISTS aggregated_metrics;
DROP TABLE IF EXISTS raw_metrics CASCADE;
