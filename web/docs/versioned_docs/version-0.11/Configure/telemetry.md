---
sidebar_position: 99
---

# Telemetry

Marmot collects **anonymous** telemetry to help us understand how the product is used and where to focus development effort. Telemetry is enabled by default and can be disabled at any time.

## What is collected

- A random install ID (UUID, not tied to any user or organization)
- Server version and runtime environment (OS, architecture, Go version)
- Deployment mode (Kubernetes, Docker, or binary)
- Uptime and the time the report was sent
- Aggregate counts: total assets, users, lineage edges, and runs per connector type

## What is never collected

- Hostnames, IP addresses, or domain names
- Asset names, descriptions, or metadata values
- User names, emails, or any PII
- Database connection strings or credentials
- Query content or API request bodies
- Any data that could identify your organization

## How to opt out

### Configuration file

```yaml
telemetry:
  enabled: false
```

### Helm chart

```yaml
config:
  telemetry:
    enabled: false
```

### Environment variable

```bash
export MARMOT_TELEMETRY_ENABLED=false
```
