---
sidebar_position: 7
---

import { CalloutCard, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

# OpenLineage

[OpenLineage](https://openlineage.io/) is an open standard for data lineage collection and analysis. It provides a unified way to track data flows across different tools and platforms by emitting standardised events during job execution.

Marmot integrates with OpenLineage to automatically discover assets and lineage relationships from your data pipelines, eliminating manual catalog maintenance.

<CalloutCard
  title="What is OpenLineage?"
  description="OpenLineage is a vendor-neutral, open standard for lineage metadata collection. It captures how data flows through your systems without locking you into a specific tool."
  href="https://openlineage.io"
  buttonText="Learn More"
  icon="mdi:source-branch"
  variant="external"
/>

:::note[Compatibility]
OpenLineage support in Marmot is still experimental and has not been tested with all sources. Please report any issues you encounter on GitHub.
:::

## What You Get

<FeatureGrid>
  <FeatureCard
    title="Automatic Asset Discovery"
    description="Jobs, tables, files and topics are added to your catalog as they run."
    icon="mdi:magnify-scan"
  />
  <FeatureCard
    title="Lineage Relationships"
    description="See how data flows between assets with upstream and downstream connections."
    icon="mdi:source-branch"
  />
  <FeatureCard
    title="Run History"
    description="Track execution status, timing and data volumes for every pipeline run."
    icon="mdi:history"
  />
  <FeatureCard
    title="Stub Assets"
    description="Lineage is captured even for undocumented datasets without polluting your catalog."
    icon="mdi:file-hidden"
  />
</FeatureGrid>

## Supported Asset Types

Marmot maps OpenLineage events to specific asset types:

| Asset Type | Description              |
| ---------- | ------------------------ |
| `DAG`      | Airflow workflows        |
| `Task`     | Individual Airflow tasks |
| `Model`    | DBT models               |
| `Project`  | DBT projects             |
| `Table`    | Database tables          |
| `File`     | Data files               |
| `Topic`    | Kafka topics             |

## Authentication

By default, the OpenLineage endpoint requires authentication via an API key. You can disable authentication for trusted environments if needed.

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

### Disable Authentication

To disable authentication for the OpenLineage endpoint, set the following configuration:

**Config file**

```yaml
openlineage:
  auth:
    enabled: false
```

**Environment variable:**

```bash
export MARMOT_OPENLINEAGE_AUTH_ENABLED=false
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

<CalloutCard
  title="OpenLineage Documentation"
  description="For detailed configuration options and supported integrations, see the official OpenLineage documentation."
  href="https://openlineage.io/docs/"
  buttonText="View Docs"
  variant="external"
  icon="mdi:book-open-variant"
/>
