# Metrics

## Overview

Marmot collects various application metrics for both Prometheus monitoring and built-in dashboards in the UI.

## Prometheus/OpenMetrics Endpoints

You can enable metrics in the configuration to expose a Prometheus/OpenMetrics endpoint on `/metrics`. This endpoint does not have auth enabled, you should configure Prometheus to scrape the endpoints for each Marmot instance you have deployed.

**values.yaml:**

```yaml
metrics:
  enabled: true
  port: 9090
```

**Environment variables:**

```bash
MARMOT_METRICS_ENABLED=true
MARMOT_METRICS_PORT=9090
```

## Helm Chart

The Helm chart creates a ServiceMonitor for Prometheus Operator:

```bash
helm install marmot ./chart --set config.metrics.enabled=true --set monitoring.serviceMonitor.enabled=true
```

```yaml
config:
  metrics:
    enabled: true
    port: 9090

monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Endpoints

- `/metrics` - Prometheus endpoint (no auth)
- `/api/v1/metrics` - UI dashboard API (requires auth)

## Implementation Details

### Storage Architecture

Marmot uses two methods of metrics storage to separate operational and analytical concerns:

- **Prometheus endpoint** (`/metrics`): Real-time counters and gauges served from application memory. Use this for monitoring Marmot's health and setting up alerts.
- **PostgreSQL storage** (`/api/v1/metrics`): Historical metrics for the UI dashboards and analytics. Use this for understanding usage patterns of your data catalog.

### Array-Based Timeseries

Instead of storing one row per metric data point, Marmot uses an array-based approach that reduces storage overhead and eliminates background aggregation jobs.

The `metrics_timeseries` table stores arrays of timestamps and values grouped by hour:

```sql
CREATE TABLE metrics_timeseries (
    metric_name VARCHAR(255) NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    labels JSONB NOT NULL,
    hour TIMESTAMPTZ NOT NULL,
    day DATE NOT NULL,
    timestamps TIMESTAMPTZ[] NOT NULL,
    values REAL[] NOT NULL,
    point_count INTEGER NOT NULL,
    total_sum DOUBLE PRECISION NOT NULL,
    min_value REAL,
    max_value REAL,
    PRIMARY KEY (metric_name, labels, hour, day)
) PARTITION BY RANGE (day);
```

When a metric is recorded, it's appended to the existing arrays for that hour using an upsert:

```sql
INSERT INTO metrics_timeseries (...)
ON CONFLICT (metric_name, labels, hour, day) DO UPDATE SET
    timestamps = metrics_timeseries.timestamps || EXCLUDED.timestamps,
    values = metrics_timeseries.values || EXCLUDED.values,
    point_count = metrics_timeseries.point_count + EXCLUDED.point_count,
    total_sum = metrics_timeseries.total_sum + EXCLUDED.total_sum,
    min_value = LEAST(metrics_timeseries.min_value, EXCLUDED.min_value),
    max_value = GREATEST(metrics_timeseries.max_value, EXCLUDED.max_value)
```

This approach has several benefits:

- **No background aggregation jobs** - Aggregates (`total_sum`, `min_value`, `max_value`) are maintained inline on every upsert
- **Efficient storage** - One row per metric/hour instead of one row per data point
- **Fast time-range queries** - Daily partitioning allows the query planner to skip irrelevant partitions entirely
- **Simple retention** - Dropping old partitions is instant with no vacuum overhead

### Query Optimisation

API queries automatically select the appropriate bucket size based on the time range:

| Time Range | Bucket Size |
|------------|-------------|
| < 7 days | 5 minutes |
| 7-30 days | 1 hour |
| > 30 days | 1 day |

For raw data points, the arrays are expanded using `LATERAL unnest`:

```sql
SELECT metric_name, t, v
FROM metrics_timeseries,
     LATERAL unnest(timestamps, values) AS u(t, v)
WHERE day >= $1 AND hour >= $2
```

### Asset Statistics

Frequently-accessed aggregations like total asset counts by type/provider are stored in a pre-computed singleton table (`asset_statistics`). This table is refreshed on-demand rather than continuously, avoiding expensive `GROUP BY` queries on every dashboard load.

### Partition Management

Partitions are created automatically for the current date range. Old partitions can be dropped for retention using the built-in maintenance functions:

```sql
-- Create partition for a future date
SELECT create_metrics_timeseries_partition('2024-02-01'::date);

-- Drop partition for retention
SELECT drop_metrics_timeseries_partition('2024-01-01'::date);
```
