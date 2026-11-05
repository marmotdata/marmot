---
sidebar_position: 7
title: OpenLineage
---

import { TipBox } from '@site/src/components/Steps';

# OpenLineage

Marmot accepts [OpenLineage](https://openlineage.io/) run events, so a tool that emits them, such as Airflow, dbt or Spark, adds its jobs, datasets and lineage to the catalog as it runs. Support is experimental and hasn't been tested with every producer; report anything that doesn't map correctly on [GitHub](https://github.com/marmotdata/marmot/issues).

For your own integrations, send lineage with the [lineage API](lineage.md#send-lineage-from-your-own-pipelines) instead: it takes edges, down to the column, without the OpenLineage event format.

## What gets cataloged

Each event adds or updates the job and the datasets it read and wrote, links them with lineage and records the run with its status and timing. A dataset Marmot hasn't seen before is added as a stub, so lineage is complete without filling the catalog with undocumented assets.

Events map to these asset types:

| Asset type | From |
| --- | --- |
| `DAG` | Airflow workflows |
| `Task` | Airflow tasks |
| `Model` | dbt models |
| `Project` | dbt projects |
| `Table` | Database tables |
| `File` | Data files |
| `Topic` | Kafka topics |

## Authentication

Producers send events to `POST /api/v1/lineage` with an API key, either as `X-API-Key: <api-key>` or as `Authorization: Bearer <api-key>`. Create a key under **Account → API keys** with **Create API key**, or use a service account's key.

To accept events without a key, turn authentication off for this endpoint:

```yaml
openlineage:
  auth:
    enabled: false
```

Or set `MARMOT_OPENLINEAGE_AUTH_ENABLED=false`.

<TipBox variant="warning" title="Anyone who can reach the endpoint can then write lineage">
Only turn authentication off on a network where every client is trusted.
</TipBox>

## Configure a producer

### Airflow

In `airflow.cfg`:

```ini
[openlineage]
transport = http
url = https://marmot.example.com/api/v1/lineage
api_key = <api-key>
```

### dbt

In `profiles.yml`:

```yaml
kelp:
  outputs:
    prod:
      # your connection details
  vars:
    openlineage:
      url: https://marmot.example.com/api/v1/lineage
      api_key: <api-key>
```

### Spark

```bash
export OPENLINEAGE_URL=https://marmot.example.com/api/v1/lineage
export OPENLINEAGE_API_KEY=<api-key>
```

The [OpenLineage documentation](https://openlineage.io/docs/) covers every producer and its options.
