---
sidebar_position: 99
title: Telemetry
description: What anonymous telemetry a self-hosted Marmot sends and how to turn it off.
---

# Telemetry

A self-hosted Marmot sends anonymous telemetry once a day so we can see how it's used and where to focus. It's on by default, and you can turn it off at any time.

## What's sent

- A random install ID, not tied to any person or organisation
- The Marmot version, operating system, architecture and Go version
- How it's deployed: Kubernetes, Docker or the binary
- Uptime and when the report was sent
- Counts: assets, users, lineage edges and runs per plugin
- Counts of lookups (asset, lineage, glossary term and data product views) by where they came from: the web UI, the API, the CLI, an SDK or MCP

## What's never sent

- Hostnames, IP addresses or domain names
- Asset names, descriptions or metadata values
- Names, email addresses or anything else that identifies a person
- Connection strings or credentials
- Query text or API request bodies
- Anything that identifies your organisation

## Turn it off

In `config.yaml`:

```yaml
telemetry:
  enabled: false
```

With the Helm chart:

```yaml
config:
  telemetry:
    enabled: false
```

Or with an environment variable:

```bash
MARMOT_TELEMETRY_ENABLED=false
```

## Next steps

- [Configuration reference](index.md): the telemetry endpoint and interval settings.
