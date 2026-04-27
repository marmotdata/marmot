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
