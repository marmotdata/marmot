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

The reason for 2 methods of storage is due to the seperation between opertional metrics and analytical metrics. There's some overlap, hence the need for both a Prometheus endpoint and storing data into the database.

You should use the Prometheus endpoint for monitoring the health of your Marmot instance and alerts. The UI and API is designed for analytical monitoring of your data estate.

- **Raw Metrics Table**: `metrics_raw` with daily partitioning, 7-day retention
- **Aggregated Metrics Table**: `metrics_aggregated` with pre-computed summaries, 6-month retention

Raw metrics are inserted immediately on collection. Aggregation runs asynchronously in background goroutines.

### Aggregation Pipeline

1. **5-minute aggregation**: Runs every 5 minutes, processes data from 10-5 minutes ago
2. **Hourly aggregation**: Runs at :05 past each hour, aggregates previous full hour
3. **Daily aggregation**: Runs at 2:05 AM, aggregates previous full day

### Query Optimization

**API queries (`/api/v1/metrics`) use aggregated data:**

- Queries < 2 hours: Use 5-minute buckets
- Queries 2-48 hours: Use hourly buckets
- Queries > 48 hours: Use daily buckets

**Prometheus queries (`/metrics`) serve real-time counters and gauges directly from application memory.**
