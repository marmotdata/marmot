---- tern: disable-tx ----

-- Array-based timeseries storage for metrics (hourly rows, daily partitions)
CREATE TABLE IF NOT EXISTS metrics_timeseries (
    metric_name VARCHAR(255) NOT NULL,
    metric_type VARCHAR(50) NOT NULL CHECK (metric_type IN ('counter', 'gauge', 'histogram')),
    labels JSONB NOT NULL DEFAULT '{}',
    hour TIMESTAMPTZ NOT NULL,
    day DATE NOT NULL,
    timestamps TIMESTAMPTZ[] NOT NULL DEFAULT '{}',
    values REAL[] NOT NULL DEFAULT '{}',
    point_count INTEGER NOT NULL DEFAULT 0,
    total_sum DOUBLE PRECISION NOT NULL DEFAULT 0,
    min_value REAL,
    max_value REAL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (metric_name, labels, hour, day)
) PARTITION BY RANGE (day);

-- Create partitions for past 7 days and next 7 days
DO $$
DECLARE
    partition_date DATE;
    partition_name TEXT;
BEGIN
    FOR i IN -7..7 LOOP
        partition_date := CURRENT_DATE + i;
        partition_name := 'metrics_timeseries_' || TO_CHAR(partition_date, 'YYYY_MM_DD');

        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE c.relname = partition_name AND n.nspname = 'public'
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF metrics_timeseries FOR VALUES FROM (%L) TO (%L)',
                partition_name, partition_date, partition_date + 1
            );
        END IF;
    END LOOP;
END $$;

CREATE TABLE IF NOT EXISTS metrics_timeseries_default PARTITION OF metrics_timeseries DEFAULT;

CREATE INDEX IF NOT EXISTS idx_metrics_ts_name_day
    ON metrics_timeseries (metric_name, day DESC, hour DESC);

CREATE INDEX IF NOT EXISTS idx_metrics_ts_labels
    ON metrics_timeseries USING GIN (labels jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_metrics_ts_type_day
    ON metrics_timeseries (metric_type, day DESC);

-- Create future partition
CREATE OR REPLACE FUNCTION create_metrics_timeseries_partition(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
BEGIN
    partition_name := 'metrics_timeseries_' || TO_CHAR(partition_date, 'YYYY_MM_DD');

    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = partition_name AND n.nspname = 'public'
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF metrics_timeseries FOR VALUES FROM (%L) TO (%L)',
            partition_name, partition_date, partition_date + 1
        );
    END IF;
END
$$ LANGUAGE plpgsql;

-- Drop old partition for retention
CREATE OR REPLACE FUNCTION drop_metrics_timeseries_partition(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
BEGIN
    partition_name := 'metrics_timeseries_' || TO_CHAR(partition_date, 'YYYY_MM_DD');

    IF EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = partition_name AND n.nspname = 'public'
    ) THEN
        EXECUTE format('DROP TABLE %I', partition_name);
    END IF;
END
$$ LANGUAGE plpgsql;

-- Singleton table for asset statistics
CREATE TABLE IF NOT EXISTS asset_statistics (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    total_count BIGINT NOT NULL DEFAULT 0,
    with_schemas_count BIGINT NOT NULL DEFAULT 0,
    by_type JSONB NOT NULL DEFAULT '{}',
    by_provider JSONB NOT NULL DEFAULT '{}',
    by_owner JSONB NOT NULL DEFAULT '{}',
    breakdown JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO asset_statistics (id) VALUES (1) ON CONFLICT DO NOTHING;

-- Refresh asset statistics (called by background job)
CREATE OR REPLACE FUNCTION refresh_asset_statistics(owner_fields TEXT[] DEFAULT ARRAY['owner', 'ownedBy'])
RETURNS VOID AS $$
DECLARE
    coalesce_expr TEXT;
    has_is_stub BOOLEAN;
    stub_filter TEXT;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'assets' AND column_name = 'is_stub'
    ) INTO has_is_stub;

    stub_filter := CASE WHEN has_is_stub THEN 'is_stub = FALSE' ELSE 'TRUE' END;

    SELECT COALESCE(string_agg('metadata->>''' || f || '''', ', '), '''unknown''')
    INTO coalesce_expr
    FROM unnest(owner_fields) AS f;

    coalesce_expr := 'COALESCE(' || coalesce_expr || ', ''unknown'')';

    EXECUTE format($SQL$
        WITH
        totals AS (
            SELECT
                COUNT(*) AS total,
                COUNT(*) FILTER (WHERE schema != '{}' AND schema IS NOT NULL) AS with_schemas
            FROM assets WHERE %s
        ),
        type_counts AS (
            SELECT COALESCE(jsonb_object_agg(type, cnt), '{}'::jsonb) AS data
            FROM (SELECT type, COUNT(*) AS cnt FROM assets WHERE %s GROUP BY type) t
        ),
        provider_counts AS (
            SELECT COALESCE(jsonb_object_agg(provider, cnt), '{}'::jsonb) AS data
            FROM (
                SELECT providers[1] AS provider, COUNT(*) AS cnt
                FROM assets WHERE %s AND array_length(providers, 1) > 0
                GROUP BY providers[1]
            ) p
        ),
        owner_counts AS (
            SELECT COALESCE(jsonb_object_agg(owner, cnt), '{}'::jsonb) AS data
            FROM (
                SELECT %s AS owner, COUNT(*) AS cnt
                FROM assets WHERE %s GROUP BY 1 HAVING %s IS NOT NULL
            ) o
        ),
        full_breakdown AS (
            SELECT COALESCE(jsonb_agg(row_to_json(b)), '[]'::jsonb) AS data
            FROM (
                SELECT
                    COALESCE(type, 'unknown') AS type,
                    COALESCE(providers[1], 'unknown') AS provider,
                    (schema != '{}' AND schema IS NOT NULL) AS has_schema,
                    %s AS owner,
                    COUNT(*) AS count
                FROM assets WHERE %s
                GROUP BY type, providers[1], (schema != '{}' AND schema IS NOT NULL), %s
            ) b
        )
        UPDATE asset_statistics SET
            total_count = (SELECT total FROM totals),
            with_schemas_count = (SELECT with_schemas FROM totals),
            by_type = (SELECT data FROM type_counts),
            by_provider = (SELECT data FROM provider_counts),
            by_owner = (SELECT data FROM owner_counts),
            breakdown = (SELECT data FROM full_breakdown),
            updated_at = NOW()
        WHERE id = 1
    $SQL$,
    stub_filter, stub_filter, stub_filter,
    coalesce_expr, stub_filter, coalesce_expr,
    coalesce_expr, stub_filter, coalesce_expr);
END
$$ LANGUAGE plpgsql;

SELECT refresh_asset_statistics();

-- Migrate existing data from raw_metrics
INSERT INTO metrics_timeseries (
    metric_name, metric_type, labels, hour, day,
    timestamps, values, point_count, total_sum, min_value, max_value,
    created_at, updated_at
)
SELECT
    metric_name,
    metric_type,
    labels,
    date_trunc('hour', timestamp) AS hour,
    timestamp::date AS day,
    array_agg(timestamp ORDER BY timestamp) AS timestamps,
    array_agg(value ORDER BY timestamp) AS values,
    COUNT(*) AS point_count,
    SUM(value) AS total_sum,
    MIN(value) AS min_value,
    MAX(value) AS max_value,
    MIN(timestamp) AS created_at,
    MAX(timestamp) AS updated_at
FROM raw_metrics
GROUP BY metric_name, metric_type, labels, date_trunc('hour', timestamp), timestamp::date
ON CONFLICT (metric_name, labels, hour, day) DO UPDATE SET
    timestamps = metrics_timeseries.timestamps || EXCLUDED.timestamps,
    values = metrics_timeseries.values || EXCLUDED.values,
    point_count = metrics_timeseries.point_count + EXCLUDED.point_count,
    total_sum = metrics_timeseries.total_sum + EXCLUDED.total_sum,
    min_value = LEAST(metrics_timeseries.min_value, EXCLUDED.min_value),
    max_value = GREATEST(metrics_timeseries.max_value, EXCLUDED.max_value),
    updated_at = NOW();

COMMENT ON TABLE raw_metrics IS 'DEPRECATED: Replaced by metrics_timeseries';
COMMENT ON TABLE aggregated_metrics IS 'DEPRECATED: Replaced by metrics_timeseries';

ANALYZE metrics_timeseries;
ANALYZE asset_statistics;

---- create above / drop below ----

DROP FUNCTION IF EXISTS refresh_asset_statistics(TEXT[]);
DROP TABLE IF EXISTS asset_statistics;
DROP FUNCTION IF EXISTS drop_metrics_timeseries_partition(DATE);
DROP FUNCTION IF EXISTS create_metrics_timeseries_partition(DATE);
DROP TABLE IF EXISTS metrics_timeseries CASCADE;

COMMENT ON TABLE raw_metrics IS NULL;
COMMENT ON TABLE aggregated_metrics IS NULL;
