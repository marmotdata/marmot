---
sidebar_position: 3
---

# Open Lineage

## Overview

[OpenLineage](https://openlineage.io/) is an open standard for data lineage collection and analysis. It provides a unified way to track data flows across different tools and platforms by emitting standardized events during job execution.

Marmot integrates with OpenLineage to automatically discover assets and lineage relationships from your data pipelines, eliminating manual catalog maintenance.

## How Marmot Uses OpenLineage

### Asset Discovery

OpenLineage events automatically create assets in Marmot's catalog:

- **Jobs/Tasks**: Airflow DAGs, DBT models, Spark jobs
- **Datasets**: Tables, files, topics from various data sources
- **Lineage**: Relationships between jobs and datasets

### Asset Types

Marmot maps OpenLineage events to specific asset types:

- `DAG` - Airflow workflows
- `Task` - Individual Airflow tasks
- `Model` - DBT models
- `Project` - DBT projects
- `Table` - Database tables
- `File` - Data files
- `Topic` - Kafka topics

### Stub Assets

Assets discovered for the first time via OpenLineage are marked as "stub assets" until enhanced by other integrations. This allows lineage tracking even for undocumented datasets without poluting the catalog with potential bad data.

### Run History

All OpenLineage events are stored as run history, providing:

- Execution timeline and status
- Input/output data volumes
- Error messages and debugging info
- Performance metrics

## Authentication

By default, the OpenLineage endpoint requires authentication via API key. However, you can disable authentication for this endpoint if needed.

The OpenLineage endpoint requires authentication via API key.

### Generate API Key

1. Navigate to **Profile** → **API Keys**
2. Click **New Key**
3. Copy the generated key
4. Configure your OpenLineage producer

### Endpoint URL

```
POST /api/v1/lineage
Authorization: X-API-Key <your-api-key>
```

## Disable Authentication

To disable authentication for the OpenLineage endpoint, set the following configuration:

**Config file**

```yaml
auth:
  openlineage:
    enabled: false
```

**Environment variable:**

```bash
export MARMOT_AUTH_OPENLINEAGE_ENABLED=false
```

:::warning
Disabling authentication allows anyone to send lineage events to your Marmot instance. Only use this in trusted environments.
:::

## Configuration Examples

### Airflow

Configure the OpenLineage provider in `airflow.cfg`:

```ini
[openlineage]
transport = http
url = https://your-marmot-instance.com/api/v1/lineage
api_key = your-api-key
```

### DBT

Add to your `profiles.yml`:

```yaml
your_profile:
  outputs:
    prod:
      # ... your connection details
  vars:
    openlineage:
      url: https://your-marmot-instance.com/api/v1/lineage
      api_key: your-api-key
```

### Spark

Set environment variables:

```bash
export OPENLINEAGE_URL=https://your-marmot-instance.com/api/v1/lineage
export OPENLINEAGE_API_KEY=your-api-key
```

For detailed OpenLineage configuration, see the [official documentation](https://openlineage.io/docs/).
